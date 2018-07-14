package bencode

import (
	"fmt"
	"io"
	"bufio"
	"sync"
	"strconv"
	"errors"
)

// Implements parsing but not the actions.  Those are
// carried out by the implementation of the builder interface.
// A builder represents the object being created.
// Calling a method like Int64(i) sets that object to i.
// Calling a method like Elem(i) or Key(s) creates a
// new builder for a subpiece of the object (logically,
// a slice element or a map key).

// There are two Builders, in other files.
// The decoder builds a generic bencode structures
// in which maps are maps.
// The structBuilder copies data into a possibly
// nested data structure, using the "map keys"
// as struct field names.

// A builder is an interface implemented by clients and passed
// to the bencode parser.  It gives clients full control over the
// eventual representation returned by the parser.
type builder interface {
	// Set value
	Int64(i int64)
	Uint64(i uint64)
	Float64(f float64)
	String(s string)
	Array()
	Map()

	// Create sub-Builders
	Elem(i int) builder
	Key(s string) builder

	// Flush changes to parent builder if necessary.
	Flush()
}

/* Decoder implements the builder interface */
type Decoder struct {
	// A value being constructed.
	value interface{}
	// Container entity to flush into.  Can be either []interface{} or
	// map[string]interface{}.
	container interface{}
	// The index into the container interface.  Either int or string.
	index interface{}
}

func NewDecoder(container interface{}, key interface{}) *Decoder {
	return &Decoder{container: container, index: key}
}

/* The entry function of the decode action */
func Decode(reader io.Reader) () {
	decoder := Decoder{*bufio.NewReader(reader)}
}

func (j *Decoder) Int64(i int64) { j.value = int64(i) }

func (j *Decoder) Uint64(i uint64) { j.value = uint64(i) }

func (j *Decoder) Float64(f float64) { j.value = float64(f) }

func (j *Decoder) String(s string) { j.value = s }

func (j *Decoder) Bool(b bool) { j.value = b }

func (j *Decoder) Null() { j.value = nil }

func (j *Decoder) Array() { j.value = make([]interface{}, 0, 8) }

func (j *Decoder) Map() { j.value = make(map[string]interface{}) }

func (j *Decoder) Elem(i int) builder {
	v, ok := j.value.([]interface{})
	if !ok {
		v = make([]interface{}, 0, 8)
		j.value = v
	}
	/* XXX There is a bug in here somewhere, but append() works fine.
	lens := len(v)
	if cap(v) <= lens {
		news := make([]interface{}, 0, lens*2)
		copy(news, j.value.([]interface{}))
		v = news
	}
	v = v[0 : lens+1]
	*/
	v = append(v, nil)
	j.value = v
	return NewDecoder(v, i)
}

func (j *Decoder) Key(s string) builder {
	m, ok := j.value.(map[string]interface{})
	if !ok {
		m = make(map[string]interface{})
		j.value = m
	}
	return NewDecoder(m, s)
}

func (j *Decoder) Flush() {
	switch c := j.container.(type) {
	case []interface{}:
		index := j.index.(int)
		c[index] = j.Copy()
	case map[string]interface{}:
		index := j.index.(string)
		c[index] = j.Copy()
	}
}

// Get the value built by this builder.
func (j *Decoder) Copy() interface{} {
	return j.value
}

func CollectInt(reader *bufio.Reader, delimeter byte) (buf []byte, err error) {
	for {
		var c byte
		c, err = reader.ReadByte()
		if (err != nil || c == delimeter) {
			return 
		}
		if !(c == '-' || c == '.' || c == '+' || c == 'E' || (c >= '0' && c <= '9')) {
			err = error.New("Unexpected character")
			return
		}
		buf = append(buf, c)
	}
}

func DecodeInt64(reader *bufio.Reader, delim byte) (data int64, err error) {
	buf, err := CollectInt(reader, delim)
	if err != nil {
		return
	}
	data, err = strconv.ParseInt(string(buf), 10, 64)
	return
}

func DecodeString(reader *bufio.Reader) (data string, err error) {
	length, err := DecodeInt64(r, ':')
	if err != nil {
		return
	}
	if length < 0 {
		err = errors.New("Bad string length")
		return
	}
	var buf = make([]byte, length)
	_, err = io.ReadFull(reader, buf)
	if err != nil {
		return
	}
	data = string(buf)
	return
}

// Parse parses the bencode stream and makes calls to
// the builder to construct a parsed representation.
func parse(reader io.Reader, build builder) (err error) {
	buf := NewBufioReader(reader)
	defer BufioReaderPool.Put(buf)
	return ParseFromReader(buf, build)
}

var BufioReaderPool sync.Pool

func NewBufioReader(reader io.Reader) *bufio.Reader {
	if v := BufioReaderPool.Get(); v != nil {
		br := v.(*bufio.Reader)
		br.Reset(r)
		return br
	}
	return bufio.NewReader(reader)
}

func ParseFromReader(reader *bufio.Reader, build builder) (err error) {
	// Read a byte from reader
	c, err := reader.ReadByte()
	if err != nil {
		goto exit
	}

	// Honestly, I don't like the way it decodes the bytes
	switch {
 	// Strings, cause in bencode, string starts with length
	case c >= '0' && c <= '9':
		err = reader.UnreadByte()
		if err != nil {
			goto exit
		}
		var str string
		str, err = DecodeString(reader)
		if err != nil {
			goto exit
		}
		build.String(str)

	// Dictionaries
	case c == 'd':
		build.Map()
		for {
			c, err = reader.ReadByte()
			if err != nil {
				goto exit
			}
			if c == 'e' {
				break
			}
			err = reader.UnreadByte()
			if err != nil {
				goto exit
			}
			var key string
			key, err = decodeString(r)
			if err != nil {
				goto exit
			}
			// TODO: in pendantic mode, check for keys in ascending order.
			err = ParseFromReader(reader, build.Key(key))
			if err != nil {
				goto exit
			}
		}

	// Number
	case c == 'i':
		var buf []byte
		buf, err = CollectInt(reader, 'e')
		if err != nil {
			goto exit
		}
		var str string
		var i int64
		var i2 uint64
		var f float64
		str = string(buf)
		// If the number is exactly an integer, use that.
		if i, err = strconv.ParseInt(str, 10, 64); err == nil {
			build.Int64(i)
		} else if i2, err = strconv.ParseUint(str, 10, 64); err == nil {
			build.Uint64(i2)
		} else if f, err = strconv.ParseFloat(str, 64); err == nil {
			build.Float64(f)
		} else {
			err = errors.New("Bad integer")
		}

	// Lists
	case c == 'l':
		// array
		build.Array()
		n := 0
		for {
			c, err = reader.ReadByte()
			if err != nil {
				goto exit
			}
			if c == 'e' {
				break
			}
			err = reader.UnreadByte()
			if err != nil {
				goto exit
			}
			err = ParseFromReader(reader, build.Elem(n))
			if err != nil {
				goto exit
			}
			n++
		}
	default:
		err = fmt.Errorf("Unexpected character: '%v'", c)
	}
exit:
	build.Flush()
	return
}