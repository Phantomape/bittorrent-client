package bittorrentclient

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/anacrolix/missinggo"
	"github.com/anacrolix/missinggo/bitmap"

	"github.com/anacrolix/torrent/peer_protocol"
)

type peerSource string

const (
	peerSourceTracker         = "Tr"
	peerSourceIncoming        = "I"
	peerSourceDHTGetPeers     = "Hg" // Peers we found by searching a DHT.
	peerSourceDHTAnnouncePeer = "Ha" // Peers that were announced to us by a DHT.
	peerSourcePEX             = "X"
)

// Connection : maintains the state of a connection with a peer
type Connection struct {
	w                       io.Writer
	r                       io.Reader
	Discovery               peerSource
	conn                    net.Conn
	t                       *Torrent
	Choked                  bool
	PeerMaxRequests         int
	writeBuffer             *bytes.Buffer
	outgoing                bool
	lastMessageReceived     time.Time
	lastUsefulChunkReceived time.Time
	lastChunkSent           time.Time
	completedHandshake      time.Time
	closed                  missinggo.Event
	writerCond              sync.Cond
	requests                map[request]struct{}
	sentHaves               bitmap.Bitmap

	// Controlled by the local, and maybe change the names for they're confusing
	Interested                          bool
	lastTimeBecameInterested            time.Time
	lastStartedExpectingToReceiveChunks time.Time
	cumulativeExpectedToReceiveChunks   time.Duration

	// Controlled by the remote peer
	PeerID             [20]byte
	PeerInterested     bool
	PeerChoked         bool
	PeerRequests       map[request]struct{}
	PeerExtensionBytes peer_protocol.PeerExtensionBits
}

func (conn *Connection) setTorrent(t *Torrent) {
	if conn.t != nil {
		panic("connection already associated with a torrent")
	}
	conn.t = t
	//t.reconcileHandshakeStats(conn)
}

func (conn *Connection) setRW(rw io.ReadWriter) {
	conn.r = rw
	conn.w = rw
}

func (conn *Connection) getRW() io.ReadWriter {
	return struct {
		io.Reader
		io.Writer
	}{conn.r, conn.w}
}

func (conn *Connection) getRemoteAddr() net.Addr {
	return conn.conn.RemoteAddr()
}

// Close : close connection
func (conn *Connection) Close() {
	if conn.conn != nil {
		go conn.conn.Close()
	}
}

func (conn *Connection) deleteAllRequests() {
	for r := range conn.requests {
		conn.deleteRequest(r)
	}
	if len(conn.requests) != 0 {
		panic(len(conn.requests))
	}
}

func (conn *Connection) deleteRequest(r request) bool {
	if _, ok := conn.requests[r]; !ok {
		return false
	}
	delete(conn.requests, r)
	conn.updateExpectingChunks()

	// Update requests

	return true
}

func (conn *Connection) updateExpectingChunks() {
	if conn.expectingChunks() {
		if conn.lastStartedExpectingToReceiveChunks.IsZero() {
			conn.lastStartedExpectingToReceiveChunks = time.Now()
		}
	} else {
		if !conn.lastStartedExpectingToReceiveChunks.IsZero() {
			conn.cumulativeExpectedToReceiveChunks += time.Since(conn.lastStartedExpectingToReceiveChunks)
			conn.lastStartedExpectingToReceiveChunks = time.Time{}
		}
	}
}

func (conn *Connection) expectingChunks() bool {
	return conn.Interested && !conn.PeerChoked
}

// Post : writes a message into buffer
func (conn *Connection) Post(msg peer_protocol.Message) {
	conn.writeBuffer.Write(msg.MustMarshalBinary())
	conn.wroteMsg(&msg)
	conn.tickleWriter()
}

func (conn *Connection) wroteMsg(msg *peer_protocol.Message) {
	// no idea
}

// writer : the go routine that writes to the peer
func (conn *Connection) writer(keepAliveTimeout time.Duration) {
	lastWrite := time.Now()

	var keepAliveTimer *time.Timer
	keepAliveTimer = time.AfterFunc(keepAliveTimeout, func() {

	})

}

func (conn *Connection) mainReadLoop() (err error) {
	t := conn.t
	c := t.c

	decoder := peer_protocol.Decoder{
		R:         bufio.NewReaderSize(conn.r, 1<<17),
		MaxLength: 256 * 1024,
		Pool:      t.chunkPool,
	}

	for {
		var msg peer_protocol.Message
		func() {
			c.unlock()
			defer c.lock()
			err = decoder.Decode(&msg)
		}()
		if t.closed.IsSet() || conn.closed.IsSet() || err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		conn.readMsg(&msg)
		conn.lastMessageReceived = time.Now()
		if msg.Keepalive {
			//receivedKeepalives.Add(1)
			continue
		}

		if msg.Type.FastExtension() && !conn.fastEnabled() {
			return fmt.Errorf("received fast extension message (type=%v) but extension is disabled", msg.Type)
		}
		switch msg.Type {
		case peer_protocol.Choke:
			//conn.PeerChoked = true
			conn.deleteAllRequests()
			conn.updateRequests()
			conn.updateExpectingChunks()
		default:
			err = fmt.Errorf("received unknown message type: %#v", msg.Type)
		}
		if err != nil {
			return err
		}

	}
}

func (conn *Connection) readMsg(msg *peer_protocol.Message) {
	// cn.allStats(func(cs *ConnStats) { cs.readMsg(msg) })
}

func (conn *Connection) updateRequests() {
	conn.tickleWriter()
}

// tickleWriter : wakes all goroutines waiting on writerCond
func (conn *Connection) tickleWriter() {
	conn.writerCond.Broadcast()
}

func (conn *Connection) fastEnabled() bool {
	return conn.PeerExtensionBytes.SupportsFast() && conn.t.c.extensionBytes.SupportsFast()
}

// PostBitfield : post
func (conn *Connection) PostBitfield() {
	// Have no idea what this is
	if conn.sentHaves.Len() != 0 {
		panic("bitfield must be first have-related message sent")
	}
	if !conn.t.haveAnyPieces() {
		return
	}
	conn.Post(peer_protocol.Message{
		Type:     peer_protocol.Bitfield,
		Bitfield: conn.t.bitfield(),
	})
	conn.sentHaves = conn.t.completedPieces.Copy()
}
