package bencode

import (
	"encoding"
	"reflect"
)

const phasePanicMsg = "BENCODE decoder out of sync - data changing underfoot?"

// indirect walks down v allocating pointers as needed, until it gets to a non-pointer.
func indirect(v reflect.Value, decodingNull bool) (Unmarshaler, encoding.TextUnmarshaler, reflect.Value) {
	v0 := v
	haveAddr := false
	if v.Kind() != reflect.Ptr && v.Type().Name() != "" && v.CanAddr() {
		haveAddr = true
		v = v.Addr()
	}
	// I have no idea what the following code is doing
	for {
		// Load value from interface, but only if the result is addressable
		if v.Kind() == reflect.Interface && !v.IsNil() {
			e := v.Elem()
			if e.Kind() == reflect.Ptr && !e.IsNil() && (!decodingNull || e.Elem().Kind() == reflect.Ptr) {
				haveAddr = false
				v = e
				continue
			}
		}

		if v.Kind() != reflect.Ptr {
			break
		}

		if v.Elem().Kind() != reflect.Ptr && decodingNull && v.CanSet() {
			break
		}
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		if v.Type().NumMethod() > 0 {
			if u, ok := v.Interface().(Unmarshaler); ok {
				return u, nil, reflect.Value{}
			}
			if !decodingNull {
				if u, ok := v.Interface().(encoding.TextUnmarshaler); ok {
					return nil, u, reflect.Value{}
				}
			}
		}

		if haveAddr {
			v = v0 // restore original value after round-trip Value.Addr().Elem()
			haveAddr = false
		} else {
			v = v.Elem()
		}
	}

	return nil, nil, v
}

// Unmarshaler implement UnmarshalBENCODE([]byte)
type Unmarshaler interface {
	UnmarshalBENCODE([]byte) error
}

// InvalidUnmarshalError describes an invalid argument passed to Unmarshal
type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e *InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "bencode: Unmarshal(nil)"
	}

	if e.Type.Kind() != reflect.Ptr {
		return "bencode: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "bencode: Unmarshal(nil " + e.Type.String() + ")"
}

// decodeState represents the state while decoding a BENCODE value.
type decodeState struct {
	data         []byte
	off          int
	opcode       int
	scan         scanner
	savedError   error
	errorContext struct {
		Struct reflect.Type
		Field  string
	}
}

func (d *decodeState) init(data []byte) *decodeState {
	d.data = data
	d.off = 0
	d.savedError = nil
	d.errorContext.Struct = nil
	d.errorContext.Field = ""
	return d
}

func (d *decodeState) unmarshal(v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}

	d.scan.reset()
	d.scanWhile(scanSkipSpace)
	err := d.value(rv)
	if err != nil {
		return d.addErrorContext(err)
	}
	return d.savedError
}

func (d *decodeState) value(v reflect.Value) error {
	switch d.opcode {
	default:
		panic(phasePanicMsg)
	// From here, it kinda diverges from the json decoder cause
	// bencode has 4 primitives: list, string, number, dict
	case scanBeginList:
		if v.IsValid() {
			if err := d.list(v); err != nil {
				return err
			}
		} else {
			d.skip()
		}
		d.scanNext()
	}
	return nil
}

// readIndex return the position of the last byte read
func (d *decodeState) readIndex() int {
	return d.off - 1
}

func (d *decodeState) saveError(err error) {
	if d.savedError == nil {
		d.savedError = d.addErrorContext(err)
	}
}

// list this part is kinda similar to parseValue function in Ana's package
func (d *decodeState) list(v reflect.Value) error {
	u, tu, rv := indirect(v, false)
	if u != nil {
		start := d.readIndex()
		d.skip()
		return u.UnmarshalBENCODE(d.data[start:d.off])
	}
	if tu != nil {
		d.saveError(&UnmarshalTypeError{Value: "list", Type: v.Type(), Offset: int64(d.off)})
		d.skip()
		return nil
	}
	v = rv

	switch v.Kind() {
	case reflect.Interface:
		if v.NumMethod() == 0 {
			// TODO: wtf is here
			return nil
		}
		fallthrough
	default:
		d.saveError(&UnmarshalTypeError{Value: "list", Type: v.Type(), Offset: int64(d.off)})
		d.skip()
		return nil
	// I don't know what these two lines do
	case reflect.Array, reflect.Slice:
		break
	}

	// To be continued ...
	return nil
}

// scanNext process the byte at d.data[d.off]
func (d *decodeState) scanNext() {
	if d.off < len(d.data) {
		d.opcode = d.scan.step(&d.scan, d.data[d.off])
		d.off++
	} else {
		d.opcode = d.scan.eof()
		d.off = len(d.data) + 1
	}
}

// skip scans to the end of what was started
func (d *decodeState) skip() {
	s, data, i := &d.scan, d.data, d.off
	depth := len(s.parseState)
	for {
		op := s.step(s, data[i])
		i++
		if len(s.parseState) < depth {
			d.off = i
			d.opcode = op
			return
		}
	}
}

func (d *decodeState) addErrorContext(err error) error {
	if d.errorContext.Struct != nil || d.errorContext.Field != "" {
		switch err := err.(type) {
		case *UnmarshalTypeError:
			err.Struct = d.errorContext.Struct.Name()
			err.Field = d.errorContext.Field
			return err
		}
	}
	return err
}

func (d *decodeState) scanWhile(op int) {
	s, data, i := &d.scan, d.data, d.off
	for i < len(data) {
		newOp := s.step(s, data[i])
		i++
		if newOp != op {
			d.opcode = newOp
			d.off = i
			return
		}
	}

	d.off = len(data) + 1
	d.opcode = d.scan.eof()
}

// Unmarshal unmarshal
func Unmarshal(data []byte, v interface{}) error {
	// Check for well-formedness, how?
	var d decodeState
	d.init(data)
	return d.unmarshal(v)
}

// UnmarshalTypeError describes a value that does not fit in Go type
type UnmarshalTypeError struct {
	Value  string       // description of value
	Type   reflect.Type // type of Go value it could not be assigned to
	Offset int64
	Struct string // name of the struct type containing the field
	Field  string // name of the field holding the Go value
}

func (e *UnmarshalTypeError) Error() string {
	return "bencode: cannot unmarshal " + e.Value + " into Go value of type " + e.Type.String()
}
