package tracker

// Peer : a peer, the implementation data type for ip and port is different
// across files in anacrolix's implementation, which should be a mistake
type Peer struct {
	ID   []byte
	IP   uint32
	Port uint16
}
