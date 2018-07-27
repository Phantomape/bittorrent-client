package protocol

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	"github.com/pkg/errors"
)

// Decoder : responsible for converting bytes to Message type
type Decoder struct {
	R         *bufio.Reader
	Pool      *sync.Pool // What does it do?
	MaxLength uint32
}

func readByte(r io.Reader) (b byte, err error) {
	var arr [1]byte
	n, err := r.Read(arr[:])
	b = arr[0]
	if n == 1 {
		err = nil
		return
	}
	if err != nil {
		panic(err)
	}
	return
}

func unmarshalBitfield(b []byte) (bf []bool) {
	for _, c := range b {
		for i := 7; i >= 0; i-- {
			bf = append(bf, (c>>uint(i))&1 == 1)
		}
	}
	return
}

// Decode : convert bytes into Message type
func (d *Decoder) Decode(msg *Message) (err error) {
	// Read message length first
	var length uint32
	err = binary.Read(d.R, binary.BigEndian, &length)
	if err != nil {
		if err != io.EOF {
			err = fmt.Errorf("error reading message: %s", err)
		}
		return
	}
	// Under what scenario?
	if length > d.MaxLength {
		return errors.New("message too long")
	}

	// Handle keepalive messages
	if length == 0 {
		msg.Keepalive = true
		return
	}

	msg.Keepalive = false
	r := &io.LimitedReader{R: d.R, N: int64(length)}
	defer func() {
		if err != nil {
			return
		}
		if r.N != 0 {
			err = fmt.Errorf("%d bytes unused in message type %d", r.N, msg.Type)
		}
	}()

	c, err := readByte(r)
	if err != nil {
		return
	}
	msg.Type = MessageType(c)
	switch msg.Type {
	case Choke, Unchoke, Interested, NotInterested, HaveAll, HaveNone:
		return
	case Have, AllowedFast, SuggestPiece:
		err = msg.Index.Read(r)
	case Request, Cancel, RejectRequest:
		for _, data := range []*Integer{&msg.Index, &msg.Begin, &msg.Length} {
			err = data.Read(r) // use binary.Read instead
			if err != nil {
				break
			}
		}
	case Bitfield:
		b := make([]byte, length-1)
		_, err = io.ReadFull(r, b)
		msg.Bitfield = unmarshalBitfield(b)
	case Piece:
		for _, pi := range []*Integer{&msg.Index, &msg.Begin} {
			err := pi.Read(r)
			if err != nil {
				return err
			}
		}
		dataLen := r.N
		// Get sth from the pool and convert it to pointer to bytes
		msg.Piece = (*d.Pool.Get().(*[]byte))
		if int64(cap(msg.Piece)) < dataLen {
			return errors.New("piece data longer than expected")
		}
		msg.Piece = msg.Piece[:dataLen] // is it necessary
		_, err := io.ReadFull(r, msg.Piece)
		if err != nil {
			return errors.Wrap(err, "reading piece data")
		}
	default:
		err = fmt.Errorf("unknown message type %#v", c)
	}
	return
}
