package tracker

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/anacrolix/dht/krpc"
	"github.com/anacrolix/missinggo/httptoo"
	"github.com/anacrolix/torrent/bencode"
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

// String : convert type AnnounceEvent to string
func (e AnnounceEvent) String() string {
	return []string{"empty", "completed", "started", "stopped"}[e]
}

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
	UserAgent  string
	HostHeader string
	HTTPClient *http.Client
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
		return announceHTTP(anc, trackerURL)
	default:
		err = errors.New("unknown scheme")
		return
	}
}

type httpResponse struct {
	FailureReason string `bencode:"failure reason"`
	Interval      int32  `bencode:"interval"`
	TrackerID     string `bencode:"tracker id"`
	Complete      int32  `bencode:"complete"`
	Incomplete    int32  `bencode:"incomplete"`
	Peers         []Peer `bencode:"peers"`
}

func setAnnounceParams(trackerURL *url.URL, ar *AnnounceRequest, anc Announce) {
	q := trackerURL.Query()

	q.Set("info_hash", string(ar.InfoHash[:]))
	q.Set("peer_id", string(ar.PeerID[:]))
	q.Set("port", fmt.Sprintf("%d", ar.Port))
	q.Set("uploaded", strconv.FormatInt(ar.Uploaded, 10))
	q.Set("downloaded", strconv.FormatInt(ar.Downloaded, 10))
	q.Set("left", strconv.FormatUint(ar.Left, 10))

	if ar.Event != None {
		q.Set("event", ar.Event.String())
	}

	// https://stackoverflow.com/questions/17418004/why-does-tracker-server-not-understand-my-request-bittorrent-protocol
	q.Set("compact", "1")

	if anc.ClientIPv4.IP != nil {
		q.Set("ipv4", anc.ClientIPv4.String())
	}
	trackerURL.RawQuery = q.Encode()
}

func announceHTTP(anc Announce, trackerURL *url.URL) (res AnnounceResponse, err error) {
	trackerURL = httptoo.CopyURL(trackerURL) // Deep copy the url
	setAnnounceParams(trackerURL, &anc.Request, anc)
	req, err := http.NewRequest("GET", trackerURL.String(), nil)
	req.Header.Set("User-Agent", anc.UserAgent)
	req.Host = anc.HostHeader
	resp, err := anc.HTTPClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	io.Copy(&buf, resp.Body)
	if resp.StatusCode != 200 {
		err = fmt.Errorf("response from tracker: %s: %s", resp.Status, buf.String())
		return
	}

	var trackerResponse httpResponse
	err = bencode.Unmarshal(buf.Bytes(), &trackerResponse)
	if err != nil {
		err = fmt.Errorf("error decoding %q: %s", buf.Bytes(), err)
		return
	}

	// If the query failed, return immediately
	if trackerResponse.FailureReason != "" {
		err = errors.New(trackerResponse.FailureReason)
		return
	}

	res.Interval = trackerResponse.Interval
	res.Peers = trackerResponse.Peers
	return
}
