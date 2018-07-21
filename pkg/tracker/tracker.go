package tracker

// AnnounceEvent : enum for "event" in request parameters
type AnnounceEvent int32

// Enum
const (
	None AnnounceEvent = iota
	Completed
	Started
	Stopped
)

// AnnounceRequest : a request
type AnnounceRequest struct {
	InfoHash   [20]byte
	PeerID     [20]byte
	Downloaded int64
	Left       uint64 // Maybe use int64 for consistency
	Uploaded   int64
	Event      AnnounceEvent
	IP         uint32
	Port       uint16
	Key        int32
	NumWant    int32
}

// AnnounceResponse : a response
type AnnounceResponse struct {
	Interval int32
	Peers    []Peer
}
