package bencode

import (
	"reflect"
)

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
	return d.savedError
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
	Value string       // description of value
	Type  reflect.Type // type of Go value it could not be assigned to
}

func (e *UnmarshalTypeError) Error() string {
	return "bencode: cannot unmarshal " + e.Value + " into Go value of type " + e.Type.String()
}
