package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// MessageType : indicate different peer message types
type MessageType byte

// Different message type enum
const (
	Choke         MessageType = 0
	Unchoke       MessageType = 1
	Interested    MessageType = 2
	NotInterested MessageType = 3
	Have          MessageType = 4
	Bitfield      MessageType = 5
	Request       MessageType = 6
	Piece         MessageType = 7
	Cancel        MessageType = 8
)

// Message : represent the stream messages
type Message struct {
	Keepalive            bool
	Type                 MessageType
	Index, Begin, Length uint32
	Piece                []byte
	Bitfield             []bool
}

// MarshalBinary : marshal the message into bytes
func (msg Message) MarshalBinary() (data []byte, err error) {
	buf := &bytes.Buffer{}
	if !msg.Keepalive {
		// Append the message type to the buffer
		err = buf.WriteByte(byte(msg.Type))
		if err != nil {
			return
		}

		switch msg.Type {
		case Choke, Unchoke, Interested, NotInterested:
		case Have:
			err = binary.Write(buf, binary.BigEndian, msg.Index)
		case Request, Cancel:
			for _, i := range []uint32{msg.Index, msg.Begin, msg.Length} {
				err = binary.Write(buf, binary.BigEndian, i)
				if err != nil {
					break
				}
			}
		case Bitfield:
			// Convert []bool into []byte
			b := make([]byte, (len(msg.Bitfield)+7)/8)
			for i, have := range msg.Bitfield {
				if !have {
					continue
				}
				c := b[i/8]
				c |= 1 << uint(7-i%8)
				b[i/8] = c
			}
			_, err = buf.Write(b)
		case Piece:
			for _, i := range []uint32{msg.Index, msg.Begin} {
				err = binary.Write(buf, binary.BigEndian, i)
				if err != nil {
					return
				}
			}

			// Since msg.Piece is already []byte, we can just write it into buffer
			n, err := buf.Write(msg.Piece)
			if err != nil {
				break
			}
			if n != len(msg.Piece) {
				panic(n)
			}
		default:
			err = fmt.Errorf("unknown message type: %v", msg.Type)
		}
	}

	// Each message starts with its length
	data = make([]byte, 4+buf.Len())
	binary.BigEndian.PutUint32(data, uint32(buf.Len()))
	if buf.Len() != copy(data[4:], buf.Bytes()) {
		panic("bad copy")
	}
	return
}
