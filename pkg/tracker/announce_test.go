package tracker

import (
	"crypto/tls"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/anacrolix/torrent/bencode"
)

func TestUnmarshalHTTPResponsePeerDicts(t *testing.T) {
	var hr httpResponse
	require.NoError(t, bencode.Unmarshal(
		[]byte("d5:peersl"+
			"d2:ip7:1.2.3.47:peer id20:thisisthe20bytepeeri4:porti9999ee"+
			"d7:peer id20:thisisthe20bytepeeri2:ip39:2001:0db8:85a3:0000:0000:8a2e:0370:73344:porti9998ee"+
			"e"+
			"6:peers618:123412341234123456"+
			"e"),
		&hr))

	require.Len(t, hr.Peers, 2)
	assert.Equal(t, []byte("thisisthe20bytepeeri"), hr.Peers[0].ID)
	assert.EqualValues(t, 9999, hr.Peers[0].Port)
	assert.EqualValues(t, 9998, hr.Peers[1].Port)
	assert.NotNil(t, hr.Peers[0].IP)
	assert.NotNil(t, hr.Peers[1].IP)
}

func TestUnmarshalHttpResponseNoPeers(t *testing.T) {
	var hr httpResponse
	require.NoError(t, bencode.Unmarshal(
		[]byte("d6:peers618:123412341234123456e"),
		&hr,
	))
	require.Len(t, hr.Peers, 0)
}

var defaultClient = &http.Client{
	Timeout: time.Second * 15,
	Transport: &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 15 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 15 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	},
}

func TestUnsupportedTrackerScheme(t *testing.T) {
	t.Parallel()
	_, err := Announce{TrackerUrl: "lol://tracker.openbittorrent.com:80/announce"}.Do()
	require.Equal(t, ErrBadScheme, err)
}
