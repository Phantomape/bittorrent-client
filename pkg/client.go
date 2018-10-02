package bittorrentclient

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"./network"
	"github.com/anacrolix/dht"
	"github.com/anacrolix/missinggo"
	"github.com/anacrolix/missinggo/bitmap"
	"github.com/anacrolix/missinggo/perf"
	"github.com/anacrolix/missinggo/pproffd"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/peer_protocol"
	"github.com/anacrolix/torrent/storage"
)

var allNetworkProtocols = []string{"tcp4", "tcp6", "udp4", "udp6"}

// defaultPeerExtensionBytes : default reserved bytes
func defaultPeerExtensionBytes() peer_protocol.PeerExtensionBits {
	return peer_protocol.NewPeerExtensionBytes(peer_protocol.ExtensionBitFast)
}

// ClientConfig : config for client
type ClientConfig struct {
	dataDir     string
	listenHost  func(network string) string
	listenPort  int
	peerID      string
	disableUTP  bool
	disableIPv6 bool
	BEP20       string
	proxyURL    string
	Debug       bool
}

// Torrent : parsed information about the torrent
type Torrent struct {
	c               *Client
	info            *metainfo.Info
	infoHash        metainfo.Hash
	metaInfo        *metainfo.MetaInfo
	storageClient   *storage.Client
	closed          missinggo.Event
	pendingRequests map[request]int
	conns           map[*Connection]struct{} // Active peer connections, running message stream loops.

	// A cache of completed piece indices.
	completedPieces bitmap.Bitmap
	chunkPool       *sync.Pool
}

func (t *Torrent) addConnection(conn *Connection) (err error) {
	if t.closed.IsSet() {
		return errors.New("torrent closed")
	}

	for c0 := range t.conns {
		if conn.PeerID != c0.PeerID {
			continue // what ?
		}
		if left, ok := conn.hasPreferredNetworkOver(c0); ok && left {
			c0.Close()
			t.deleteConnection(c0)
		} else {
			return errors.New("existing connection preferred")
		}
	}
	t.conns[conn] = struct{}{}
	return nil
}

type pieceIndex = int

func (t *Torrent) numPieces() pieceIndex {
	return pieceIndex(t.info.NumPieces())
}

func (t *Torrent) haveInfo() bool {
	return t.info != nil
}

func (t *Torrent) haveAllPieces() bool {
	if !t.haveInfo() {
		return false
	}
	return t.completedPieces.Len() == bitmap.BitIndex(t.numPieces())
}

func (t *Torrent) haveAnyPieces() bool {
	return t.completedPieces.Len() != 0
}

func (t *Torrent) dropConnection(conn *Connection) {
	t.c.event.Broadcast()
	conn.Close()
	if t.deleteConnection(conn) {
		t.openNewConnections()
	}
}

// bitfield : generate
func (t *Torrent) bitfield() (bf []bool) {
	bf = make([]bool, t.numPieces())
	t.completedPieces.IterTyped(func(piece int) (again bool) {
		bf[piece] = true
		return true
	})
	return
}

func (l *Connection) hasPreferredNetworkOver(r *Connection) (left, ok bool) {
	// Have no idea of what the multiLess type means, use a better approach
	// to handle this
	return
}

func (c *Client) sendInitialMessages(conn *Connection, t *Torrent) {
	if conn.PeerExtensionBytes.SupportsExtended() && c.extensionBytes.SupportsExtended() {
		conn.Post(peer_protocol.Message{})
	}

	func() {
		if conn.fastEnabled() {
			if t.haveAllPieces() {
				conn.Post(peer_protocol.Message{Type: peer_protocol.HaveAll})
				//conn.sentHaves.AddRange(0, bitmap.BitIndex(conn.t.NumPieces()))
				return
			} else if !t.haveAnyPieces() {
				conn.Post(peer_protocol.Message{Type: peer_protocol.HaveNone})
				//conn.sentHaves.Clear()
				return
			}
		}
		conn.PostBitfield()
	}()

	// DHT support
}

// openNewConnections : open connection with peers based on their priorities
func (t *Torrent) openNewConnections() {

}

func (t *Torrent) deleteConnection(conn *Connection) (res bool) {
	// Must close connection before delete
	if !conn.closed.IsSet() {
		panic("connection is not closed")
	}

	_, res = t.conns[conn]
	delete(t.conns, conn)
	conn.deleteAllRequests()
	if len(t.conns) == 0 {
		t.assertNoPendingRequests()
	}
	return
}

func (t *Torrent) assertNoPendingRequests() {
	if len(t.pendingRequests) != 0 {
		panic(t.pendingRequests)
	}
}

// Client : mainly declarations here
type Client struct {
	closed         missinggo.Event
	config         *ClientConfig
	conns          []network.Socket
	mu             sync.RWMutex
	torrents       map[metainfo.Hash]*Torrent // Where is the InfoHash type ?
	defaultStorage *storage.Client
	dhtServers     []*dht.Server // why is dht a server T.T
	extensionBytes peer_protocol.PeerExtensionBits
	peerID         [20]byte
	event          sync.Cond
}

func (c *Client) lock() {
	c.mu.Lock()
}

func (c *Client) unlock() {
	c.mu.Unlock()
}

func (c *Client) rLock() {
	c.mu.RLock()
}

func (c *Client) rUnlock() {
	c.mu.RUnlock()
}

func (c *Client) eachDhtServer(f func(*dht.Server)) {
	for _, ds := range c.dhtServers {
		f(ds)
	}
}

func (c *Client) firewallCallback(net.Addr) bool {
	return false
}

// AddTorrentInfoHash : add torrent
func (c *Client) AddTorrentInfoHash(infoHash metainfo.Hash) (t *Torrent, new bool) {
	return c.AddTorrentInfoHashWithStorage(infoHash, nil)
}

// AddTorrentInfoHashWithStorage : add torrent with custom Storage implementation
func (c *Client) AddTorrentInfoHashWithStorage(infoHash metainfo.Hash, storageSpec storage.ClientImpl) (t *Torrent, new bool) {
	c.lock()
	defer c.unlock()
	t, ok := c.torrents[infoHash]
	if ok {
		return
	}
	new = true

	t = c.newTorrent(infoHash, storageSpec)
	return
}

// newTorrent : return a Torrent ready for insertion into a Client
func (c *Client) newTorrent(infoHash metainfo.Hash, storageSpec storage.ClientImpl) (t *Torrent) {
	storageClient := c.defaultStorage
	if storageSpec != nil {
		storageClient = storage.NewClient(storageSpec) // Change name to NewStorageClient
	}

	t = &Torrent{
		c:             c,
		infoHash:      infoHash,
		storageClient: storageClient,
	}
	return
}

func (c *Client) checkEnabledNetworkProtocols() (ns []string) {
	for _, n := range allNetworkProtocols {
		if enabledNetworkProtocol(n, c.config) {
			ns = append(ns, n)
		}
	}
	return
}

func enabledNetworkProtocol(network string, cfg *ClientConfig) bool {
	c := func(s string) bool {
		return strings.Contains(network, s)
	}
	if cfg.disableUTP {
		if c("udp") || c("utp") {
			return false
		}
	}
	if cfg.disableUTP && c("tcp") {
		return false
	}
	if cfg.disableIPv6 && c("6") {
		return false
	}
	return true
}

// Close : stops the client and sever all connections
func (c *Client) Close() {
	c.lock()
	defer c.unlock()
	// TODO: close socket
}

// NewClient : client constructor
func NewClient(cfg *ClientConfig) (c *Client, err error) {
	c = &Client{
		config:   cfg,
		torrents: make(map[metainfo.Hash]*Torrent),
	}

	defer func() {
		if err == nil {
			return
		}
		c.Close()
	}()

	// Set extension bytes
	c.extensionBytes = defaultPeerExtensionBytes()
	if cfg.peerID != "" {
		missinggo.CopyExact(&c.peerID, cfg.peerID)
	} else {
		o := copy(c.peerID[:], cfg.BEP20) // related to BEP20
		_, err = rand.Read(c.peerID[o:])
		if err != nil {
			panic("error generating peer id")
		}
	}

	c.conns, err = network.ListenAll(c.checkEnabledNetworkProtocols(), c.config.listenHost, c.config.listenPort, c.config.proxyURL, c.firewallCallback)
	if err != nil {
		return
	}

	for _, s := range c.conns {
		if enabledNetworkProtocol(s.Addr().Network(), c.config) {
			go c.acceptConnection(s)
		}
	}
	return
}

// LocalPort : no idea
func (c *Client) LocalPort() (port int) {
	return
}

func (c *Client) acceptConnection(l net.Listener) {
	for {
		conn, err := l.Accept()
		conn = pproffd.WrapNetConn(conn) // used for detecting resource leaks
		c.rLock()
		closed := c.closed.IsSet() // no idea
		reject := false
		if conn != nil {
			reject = c.rejectAccepted(conn)
		}
		c.rUnlock()
		if closed {
			if conn != nil {
				conn.Close()
			}
			return
		}
		if err != nil {
			log.Printf("error accepting connecting: %s", err)
			continue
		}

		go func() {
			if reject {
				conn.Close()
			} else {
				go c.incomingConnection(conn)
			}
			// do some logging here and some mysteious func calls
		}()
	}
}

func (c *Client) incomingConnection(nc net.Conn) {
	defer nc.Close()
	if tc, ok := nc.(*net.TCPConn); ok {
		tc.SetLinger(0) // why discard everything
	}
	conn := c.newConnection(nc, false)
	conn.Discovery = peerSourceIncoming
	c.runReceivedConnection(conn)
}

// runReceivedConnection : entry for handshake and stream messages
func (c *Client) runReceivedConnection(conn *Connection) {
	t, err := c.receiveHandshakes(conn)
	if err != nil {
		c.lock()
		c.onBadAccept(conn.getRemoteAddr())
		c.unlock()
		return
	}

	if t == nil {
		c.lock()
		c.onBadAccept(conn.getRemoteAddr())
		c.unlock()
		return
	}

	c.lock()
	defer c.unlock()
	c.runHandshookConnection(conn, t)
}

type deadlineReader struct {
	nc net.Conn
	r  io.Reader
}

func (dr deadlineReader) Read(b []byte) (int, error) {
	err := dr.nc.SetReadDeadline(time.Now().Add(150 * time.Second))
	if err != nil {
		return 0, fmt.Errorf("error setting read deadline: %s", err)
	}
	return dr.r.Read(b)
}

func (c *Client) runHandshookConnection(conn *Connection, t *Torrent) {
	conn.setTorrent(t)
	// When will they not equal
	if conn.PeerID == c.peerID {

	}

	// Where is this function ?
	// c.conns.SetWriteDeadline(time.Time{})
	conn.r = deadlineReader{conn.conn, conn.r}
	if err := t.addConnection(conn); err != nil {
		return
	}
	defer t.dropConnection(conn)
	go conn.writer(time.Minute)
	c.sendInitialMessages(conn, t)
	err := conn.mainReadLoop()
	if err != nil && c.config.Debug {
		log.Printf("error during connection main read loop: %s", err)
	}
}

func (c *Client) onBadAccept(addr net.Addr) {

}

func (c *Client) receiveHandshakes(conn *Connection) (t *Torrent, err error) {
	defer perf.ScopeTimerErr(&err)()
	var rw io.ReadWriter
	err = nil // Skip encryption
	conn.setRW(rw)
	ih, ok, err := c.connBTHandshake(conn, nil)
	if err != nil {
		err = fmt.Errorf("error during handshake: %s", err)
		return
	}

	// What is this
	if !ok {
		return
	}
	c.lock()
	t = c.torrents[ih]
	c.unlock()
	return
}

// connBTHandshake : the name and api sucks
func (c *Client) connBTHandshake(conn *Connection, ih *metainfo.Hash) (ret metainfo.Hash, ok bool, err error) {
	res, ok, err := peer_protocol.Handshake(conn.getRW(), ih, c.peerID, c.extensionBytes)
	if err != nil || !ok {
		return
	}
	ret = res.Hash
	conn.PeerExtensionBytes = res.PeerExtensionBits
	conn.PeerID = res.PeerID
	conn.completedHandshake = time.Now()
	return
}

func (c *Client) rejectAccepted(conn net.Conn) bool {
	ra := conn.RemoteAddr()
	rip := missinggo.AddrIP(ra)
	if c.config.disableIPv6 && len(rip) == net.IPv6len && rip.To4() == nil {
		return true
	}
	return c.isBadPeerIPPort(rip, missinggo.AddrPort(ra))
}

func (c *Client) isBadPeerIPPort(ip net.IP, port int) bool {
	if port == 0 {
		return true
	}
	return false
}

type clientLocker struct {
	*Client
}

func (cl clientLocker) Lock() {
	cl.lock()
}

func (cl clientLocker) Unlock() {
	cl.unlock()
}

func (c *Client) getLocker() sync.Locker {
	return clientLocker{c}
}

func (c *Client) newConnection(nc net.Conn, outgoing bool) (conn *Connection) {
	conn = &Connection{
		conn:            nc,
		outgoing:        outgoing,
		Choked:          true,
		PeerMaxRequests: 250,
		writeBuffer:     new(bytes.Buffer),
	}

	return
}
