package bencode

import (
	"bytes"
	"errors"
	"io"
	"math/big"
	"reflect"
	"runtime"
	"strconv"
)

// Decoder fasdfasdf
type Decoder struct {
	reader interface {
		io.ByteScanner
		io.Reader
	}
	offset int64
	buf    bytes.Buffer // with Read and Write methods
}

// Decode : entry function of decoding
func (decoder *Decoder) Decode(val interface{}) (err error) {
	// Still no idea what this is
	defer func() {
		if err != nil {
			return
		}
		r := recover()
		_, ok := r.(runtime.Error)
		if ok {
			panic(r)
		}
		err, ok = r.(error)
		if !ok && r != nil {
			panic(r)
		}
	}()

	pv := reflect.ValueOf(val)
	if pv.Kind() != reflect.Ptr || pv.IsNil() {
		return &UnmarshalInvalidArgError{reflect.TypeOf(val)}
	}

	ok, err := decoder.parseValue(pv.Elem())
	if err != nil {
		return
	}
	if !ok {
		decoder.throwSyntaxError(decoder.offset-1, errors.New("unexpected 'e'"))
	}
	return
}

// parseValue : returns true if thers is a value and it is now stored in 'val',
//				otherwise, there was an end symbol ("e") and no value is stored
func (decoder *Decoder) parseValue(val reflect.Value) (bool, error) {
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			// Elem returns a type's element type.
			// It panics if the type's Kind is not Array, Chan, Map, Ptr, or Slice.
			// New returns a Value representing a pointer to a new zero value for the specified type.
			// That is, the returned Value's Type is PtrTo(typ).
			val.Set(reflect.New(val.Type().Elem()))
		}
		// Elem returns the value that the interface val contains or that the pointer val points
		// to. It panics if val's Kind is not Interface or Ptr. It returns the zero Value if v is
		// nil.
		val = val.Elem()
	}

	// No idea what this is
	if decoder.parseUnmarshaler(val) {
		return true, nil
	}

	// Common case: val is an interface
	if val.Kind() == reflect.Interface && val.NumMethod() == 0 {
		iface, _ := decoder.parseValueInterface()
		val.Set(reflect.ValueOf(iface))
		return true, nil
	}

	character, err := decoder.reader.ReadByte()
	if err != nil {
		panic(err)
	}
	decoder.offset++

	switch character {
	case 'e': // End of torrent file
		return false, nil
	case 'd':
		return true, decoder.parseDict(val)
	case 'l':
		return true, decoder.parseList(val)
	case 'i':
		decoder.parseInt(val)
		return true, nil
	default:
		if character >= '0' && character <= '9' {
			decoder.buf.Reset()
			decoder.buf.WriteByte(character)
			return true, decoder.parseString(val)
		}
	}

	panic("Unreachable")
}

// parseUnmarshaler :
func (decoder *Decoder) parseUnmarshaler(val reflect.Value) bool {
	// Interface returns val's current value as an interface{}
	m, ok := val.Interface().(Unmarshaler) // I don't understand the rest
	if !ok {
		// A value is addressable if it is an element of a slice, an element of an addressable
		// array, a field of an addressable struct, or the result of dereferencing a pointer.
		if val.Kind() != reflect.Ptr && val.CanAddr() {
			m, ok = val.Addr().Interface().(Unmarshaler)
			if ok {
				val = val.Addr()
			}
		}
	}

	if ok && (val.Kind() != reflect.Ptr || !val.IsNil()) {
		if decoder.readOneValue() {
			// Bytes returns a slice of length buf.Len() holding the unread portion of the buffer.
			err := m.UnmarshalBencode(decoder.buf.Bytes())
			decoder.buf.Reset() // Reset resets the buffer to be empty
			if err != nil {
				panic(&UnmarshalerError{val.Type(), err})
			}
			return true
		}
		decoder.buf.Reset()
	}

	return false
}

// parseValueInterface() :
func (decoder *Decoder) parseValueInterface() (interface{}, bool) {
	b, err := decoder.reader.ReadByte()
	if err != nil {
		panic(err)
	}
	decoder.offset++

	switch b {
	case 'e':
		return nil, false
	case 'd':
		return decoder.parseDictInterface(), true
	case 'l':
		return decoder.parseListInterface(), true
	case 'i':
		return decoder.parseIntInterface(), true
	default:
		if b >= '0' && b <= '9' {
			// String, append first digit of the length to the buffer
			decoder.buf.WriteByte(b)
			return decoder.parseStringInterface(), true
		}

		decoder.raiseUnknownValueType(b, decoder.offset-1)
		panic("Unreachable")
	}
}

// parseIntInterface :
func (decoder *Decoder) parseIntInterface() (ret interface{}) {
	start := decoder.offset - 1
	decoder.readUntil('e')
	if decoder.buf.Len() == 0 {
		panic(&SyntaxError{
			Offset: start,
			What:   errors.New("Empty integer value"),
		})
	}

	n, err := strconv.ParseInt(decoder.buf.String(), 10, 64)
	// If err is an out of range error
	if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
		// The new built-in function allocates memory. The first argument is a type, not a value,
		// and the value returned is a pointer to a newly allocated zero value of that type.
		i := new(big.Int)
		_, ok := i.SetString(decoder.buf.String(), 10)
		if !ok {
			panic(&SyntaxError{
				Offset: start,
				What:   errors.New("Failed to parse integer"),
			})
		}
		ret = i
	} else {
		checkForIntParseError(err, start)
		ret = n
	}

	decoder.buf.Reset()
	return
}

// parseStringInterface :
func (decoder *Decoder) parseStringInterface() interface{} {
	start := decoder.offset - 1

	// read the string length first
	decoder.readUntil(':')
	length, err := strconv.ParseInt(decoder.buf.String(), 10, 64)
	checkForIntParseError(err, start)

	decoder.buf.Reset()
	n, err := io.CopyN(&decoder.buf, decoder.reader, length)
	decoder.offset += n
	if err != nil {
		checkForUnexpectedEOF(err, decoder.offset)
		panic(&SyntaxError{
			Offset: decoder.offset,
			What:   errors.New("unexpected I/O error: " + err.Error()),
		})
	}

	s := decoder.buf.String()
	decoder.buf.Reset()
	return s
}

// parseDictInterface :
func (decoder *Decoder) parseDictInterface() interface{} {
	dict := make(map[string]interface{})
	for {
		keyi, ok := decoder.parseValueInterface()
		if !ok {
			break
		}

		key, ok := keyi.(string)
		if !ok {
			panic(&SyntaxError{
				Offset: decoder.offset,
				What:   errors.New("non-string key in a dict"),
			})
		}

		valuei, ok := decoder.parseValueInterface()
		if !ok {
			break
		}

		dict[key] = valuei
	}
	return dict
}

// parseListInterface :
func (decoder *Decoder) parseListInterface() interface{} {
	var list []interface{}
	for {
		valuei, ok := decoder.parseValueInterface()
		if !ok {
			break
		}

		list = append(list, valuei)
	}
	if list == nil {
		list = make([]interface{}, 0, 0)
	}
	return list
}

// parseInt :
func (decoder *Decoder) parseInt(val reflect.Value) {
	start := decoder.offset - 1
	decoder.readUntil('e')
	if decoder.buf.Len() == 0 {
		panic(&SyntaxError{
			Offset: start,
			What:   errors.New("empty integer value"),
		})
	}

	s := decoder.buf.String()

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		checkForIntParseError(err, start)

		if v.OverflowInt(n) {
			panic(&UnmarshalTypeError{
				Value: "integer " + s,
				Type:  v.Type(),
			})
		}
		v.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, 64)
		checkForIntParseError(err, start)

		if v.OverflowUint(n) {
			panic(&UnmarshalTypeError{
				Value: "integer " + s,
				Type:  v.Type(),
			})
		}
		v.SetUint(n)
	case reflect.Bool:
		v.SetBool(s != "0")
	default:
		panic(&UnmarshalTypeError{
			Value: "integer " + s,
			Type:  v.Type(),
		})
	}
	decoder.buf.Reset()
}

// parseString :
func (decoder *Decoder) parseString(val reflect.Value) error {
	start := decoder.offset - 1

	// read the string length first
	decoder.readUntil(':')
	length, err := strconv.ParseInt(decoder.buf.String(), 10, 64)
	checkForIntParseError(err, start)

	decoder.buf.Reset()
	n, err := io.CopyN(&decoder.buf, decoder.reader, length)
	decoder.offset += n
	if err != nil {
		checkForUnexpectedEOF(err, decoder.offset)
		panic(&SyntaxError{
			Offset: decoder.offset,
			What:   errors.New("unexpected I/O error: " + err.Error()),
		})
	}

	defer decoder.buf.Reset()
	switch v.Kind() {
	case reflect.String:
		v.SetString(decoder.buf.String())
		return nil
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			break
		}
		v.SetBytes(append([]byte(nil), decoder.buf.Bytes()...))
		return nil
	case reflect.Array:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			break
		}
		reflect.Copy(v, reflect.ValueOf(decoder.buf.Bytes()))
		return nil
	}
	// I believe we return here to support "ignore_unmarshal_type_error".
	return &UnmarshalTypeError{
		Value: "string",
		Type:  v.Type(),
	}
}
