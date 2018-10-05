package bencode

import (
	"bytes"
	"reflect"
	"sync"
)

// Marshaler marshaler interface
type Marshaler interface {
	MarshalBENCODE() ([]byte, error)
}

type bencodeError struct{ error }

var encodeStatePool sync.Pool

type encodeState struct {
	bytes.Buffer
	scratch [64]byte
}

func (e *encodeState) marshal(v interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if be, ok := r.(bencodeError); ok {
				err = be.error
			} else {
				panic(r)
			}
		}
	}()
	e.reflectValue(reflect.ValueOf(v))
	return nil
}

func (e *encodeState) reflectValue(v reflect.Value) {
	valueEncoder(v)(e, v)
}

func (e *encodeState) error(err error) {
	panic(bencodeError{err})
}

func newEncodeState() *encodeState {
	if v := encodeStatePool.Get(); v != nil {
		e := v.(*encodeState)
		e.Reset()
		return e
	}
	return new(encodeState)
}

// Marshal marshal
func Marshal(v interface{}) ([]byte, error) {
	e := newEncodeState()

	err := e.marshal(v)
	if err != nil {
		return nil, err
	}
	buf := append([]byte(nil), e.Bytes()...)

	e.Reset()
	encodeStatePool.Put(e) // this thing cause memory leak
	return buf, nil
}

func invalidValueEncoder(e *encodeState, v reflect.Value) {
	e.WriteString("null")
}

type encoderFunc func(e *encodeState, v reflect.Value)

func valueEncoder(v reflect.Value) encoderFunc {
	if !v.IsValid() {
		return invalidValueEncoder
	}
	return typeEncoder(v.Type())
}

var encoderCache sync.Map // map[reflect.Type]encoderFunc

func typeEncoder(t reflect.Type) encoderFunc {
	if fi, ok := encoderCache.Load(t); ok {
		return fi.(encoderFunc)
	}

	// func is only used for recursive types.
	var (
		wg sync.WaitGroup
		f  encoderFunc
	)
	wg.Add(1)
	fi, loaded := encoderCache.LoadOrStore(t, encoderFunc(func(e *encodeState, v reflect.Value) {
		wg.Wait()
		f(e, v)
	}))
	if loaded {
		return fi.(encoderFunc)
	}

	// Compute the real encoder and replace the indirect func with it.
	f = newTypeEncoder(t, true)
	wg.Done()
	encoderCache.Store(t, f)
	return f
}

var (
	marshalerType = reflect.TypeOf((*Marshaler)(nil)).Elem()
)

// newTypeEncoder constructs an encoderFunc for a type.
// The returned encoder only checks CanAddr when allowAddr is true.
func newTypeEncoder(t reflect.Type, allowAddr bool) encoderFunc {
	return nil
	// To be continued ...
}
