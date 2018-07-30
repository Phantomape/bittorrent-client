package storage

// ClientImpl : represents data storage for an unspecified torrent
type ClientImpl interface {
	OpenTorrent()
	Close()
}
