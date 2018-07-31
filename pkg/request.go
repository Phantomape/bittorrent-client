package bittorrentclient

import "github.com/anacrolix/torrent/peer_protocol"

type chunkSpec struct {
	Begin, Length peer_protocol.Integer
}

type request struct {
	Index peer_protocol.Integer
	chunkSpec
}
