package protocol

import (
	"fmt"
	"io"

	"github.com/anacrolix/missinggo"

	"github.com/anacrolix/torrent/metainfo"
)

// Header : fixed header for the beginning of handshake message
const Header = "\x13BitTorrent protocol"

// ExtensionBitFast : enabled by setting the third least significant bit of the last reserved byte
const ExtensionBitFast = 2 // http://www.bittorrent.org/beps/bep_0006.html

// ExtensionBit : 8 bit integer
type ExtensionBit uint

// PeerExtensionBytes : reserved bits after the fixed header
type PeerExtensionBytes [8]byte

// SetBit : set corresponding bit in the reserved bytes
func (peb *PeerExtensionBytes) SetBit(bit ExtensionBit) {
	peb[7-bit/8] |= 1 << (bit % 8)
}

// CheckBit : check whether corresponding has been set
func (peb PeerExtensionBytes) CheckBit(bit ExtensionBit) bool {
	return peb[7-bit/8]&(1<<(bit%8)) != 0
}

// SupportsFast : check whether fast extension is supported
func (peb PeerExtensionBytes) SupportsFast() bool {
	return peb.CheckBit(ExtensionBitFast)
}

// NewPeerExtensionBytes : generate extension bytes
func NewPeerExtensionBytes(bits ...ExtensionBit) (res PeerExtensionBytes) {
	for _, b := range bits {
		res.SetBit(b)
	}
	return
}

// HandshakeResult : handshake result
type HandshakeResult struct {
	PeerExtensionBytes
	PeerID [20]byte
	metainfo.Hash
}

// handshakeWriter : read from channel and write to socket
func handshakeWriter(w io.Writer, bb <-chan []byte, done chan<- error) {
	var err error
	for b := range bb {
		_, err = w.Write(b)
		if err != nil {
			break
		}
	}
	done <- err
}

// Handshake : transfer handshake message
func Handshake(sock io.ReadWriter, ih *metainfo.Hash, peerID [20]byte, extensions PeerExtensionBytes) (res HandshakeResult, ok bool, err error) {
	writeDone := make(chan error, 1) // error value sent when the writer completes
	postCh := make(chan []byte, 4)   // 4 means buffer capacity ???
	go handshakeWriter(sock, postCh, writeDone)

	defer func() {
		// Under what occasion will it return not ok without error ?
		close(postCh)
		if !ok {
			return
		}
		if err != nil {
			panic(err)
		}

		err = <-writeDone
		if err != nil {
			err = fmt.Errorf("error writing: %s", err)
		}
	}()

	post := func(bb []byte) {
		select {
		case postCh <- bb:
		default:
			panic("mustn't block while posting")
		}
	}

	// Start handshake
	post([]byte(Header))
	post(extensions[:])
	if ih != nil {
		post(ih[:])
		post(peerID[:])
	}

	// Receive acknowledge signal
	var b [68]byte
	_, err = io.ReadFull(sock, b[:68])
	if err != nil {
		err = nil // What ?
		return
	}
	if string(b[:20]) != Header {
		return
	}
	missinggo.CopyExact(&res.PeerExtensionBytes, b[20:28])
	missinggo.CopyExact(&res.Hash, b[28:48])
	missinggo.CopyExact(&res.PeerID, b[48:68])

	// Don't know what it is, BEP003 didn't specify this part
	if ih == nil {
		post(res.Hash[:])
		post(peerID[:])
	}

	ok = true
	return
}
