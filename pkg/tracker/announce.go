package tracker

import (
	"errors"
	"net/url"

	"github.com/anacrolix/dht/krpc"
)

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

// Announce : the abstraction for sending requests and receiving response
type Announce struct {
	TrackerURL string
	Request    AnnounceRequest
	ClientIPv4 krpc.NodeAddr // the struct combining ip and port
	ClientIpv6 krpc.NodeAddr
}

// Do : sends an HTTP request and returns an HTTP response
func (anc Announce) Do() (res AnnounceResponse, err error) {
	trackerURL, err := url.Parse(anc.TrackerURL)
	if err != nil {
		return
	}

	// We support http and udp
	switch trackerURL.Scheme {
	case "http", "https":
		return
	case "udp", "udp4", "udp6":
		return
	default:
		err = errors.New("unknown scheme")
		return
	}
}
