package client

import (
	"sync"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

// Torrent : parsed information about the torrent
type Torrent struct {
	c             *Client
	infoHash      metainfo.Hash
	metaInfo      *metainfo.MetaInfo
	storageClient *storage.Client
}

// Client : mainly declarations here
type Client struct {
	mu             sync.Mutex
	torrents       map[metainfo.Hash]*Torrent // Where is the InfoHash type ?
	defaultStorage *storage.Client
}

func (c *Client) lock() {
	c.mu.Lock()
}

func (c *Client) unlock() {
	c.mu.Unlock()
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

// Close : stops the client and sever all connections
func (c *Client) Close() {
	c.lock()
	defer c.unlock()
	// TODO: close socket
}

// NewClient : client constructor
func NewClient() (c *Client, err error) {
	c = &Client{
		torrents: make(map[metainfo.Hash]*Torrent),
	}

	defer func() {
		if err == nil {
			return
		}
		c.Close()
	}()

	// Set extension bytes
	return
}
