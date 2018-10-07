package bencode

import (
	"bytes"
	"encoding"
	"reflect"
	"sort"
	"strconv"
	"sync"
)

// UnsupportedTypeError unsupported type
type UnsupportedTypeError struct {
	Type reflect.Type
}

func (e *UnsupportedTypeError) Error() string {
	return "bencode: unsupported type: " + e.Type.String()
}

// Marshaler marshaler interface
type Marshaler interface {
	MarshalBENCODE() ([]byte, error)
}

// MarshalerError marshaler error
type MarshalerError struct {
	Type reflect.Type
	Err  error
}

func (e *MarshalerError) Error() string {
	return "bencode: error calling MarshalBENCODE for type " + e.Type.String() + ": " + e.Err.Error()
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
	marshalerType     = reflect.TypeOf((*Marshaler)(nil)).Elem()
	textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
)

// newTypeEncoder constructs an encoderFunc for a type.
// The returned encoder only checks CanAddr when allowAddr is true.
func newTypeEncoder(t reflect.Type, allowAddr bool) encoderFunc {
	// Not sure if the following code is necessary
	/*
		if t.Implements(marshalerType) {
			return marshalerEncoder
		}
		if t.Kind() != reflect.Ptr && allowAddr {
			if reflect.PtrTo(t).Implements(marshalerType) {
				return newCondAddrEncoder(addrMarshalerEncoder, newTypeEncoder(t, false))
			}
		}

		if t.Implements(textMarshalerType) {
			return textMarshalerEncoder
		}
		if t.Kind() != reflect.Ptr && allowAddr {
			if reflect.PtrTo(t).Implements(textMarshalerType) {
				return newCondAddrEncoder(addrTextMarshalerEncoder, newTypeEncoder(t, false))
			}
		}
	*/

	switch t.Kind() {
	case reflect.Bool:
		return boolEncoder
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intEncoder
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return uintEncoder
	case reflect.String:
		return stringEncoder
	case reflect.Interface:
		// return interfaceEncoder
		return interfaceEncoder
	case reflect.Struct:
		// return newStructEncoder(t)
		return structEncoder
	case reflect.Map:
		// return newMapEncoder(t)
		return mapEncoder
	case reflect.Slice:
		// return newSliceEncoder(t)
		return sliceEncoder
	case reflect.Array:
		// return newArrayEncoder(t)
		return arrayEncoder
	case reflect.Ptr:
		// return newPtrEncoder(t)
		return ptrEncoder
	default:
		return unsupportedTypeEncoder
	}
}

func marshalerEncoder(e *encodeState, v reflect.Value) {
	if v.Kind() == reflect.Ptr && v.IsNil() {
		e.WriteString("null")
		return
	}
	m, ok := v.Interface().(Marshaler)
	if !ok {
		e.WriteString("null")
		return
	}
	_, err := m.MarshalBENCODE()
	//if err == nil {
	// copy BENCODE into buffer, checking validity.
	// err = compact(&e.Buffer, b)
	//}
	if err != nil {
		e.error(&MarshalerError{v.Type(), err})
	}
}

func boolEncoder(e *encodeState, v reflect.Value) {
	if v.Bool() {
		e.WriteString("i1e")
	} else {
		e.WriteString("i0e")
	}
}

func intEncoder(e *encodeState, v reflect.Value) {
	e.WriteString("i")
	b := strconv.AppendInt(e.scratch[:0], v.Int(), 10)
	e.Write(b)
	e.WriteString("e")
}

func uintEncoder(e *encodeState, v reflect.Value) {
	e.WriteString("i")
	b := strconv.AppendUint(e.scratch[:0], v.Uint(), 10)
	e.Write(b)
	e.WriteString("e")
}

func stringEncoder(e *encodeState, v reflect.Value) {
	s := v.String()
	b := strconv.AppendInt(e.scratch[:0], int64(len(s)), 10)
	e.Write(b)
	e.WriteString(":")
	e.WriteString(s)
}

// It appears that Ans's package doesn't support nested bencode
func structEncoder(e *encodeState, v reflect.Value) {
	e.WriteString("d")
	for _, ef := range encodeFields(v.Type()) {
		fieldValue := v.Field(ef.i)
		if ef.omitEmpty && isEmptyValue(fieldValue) {
			continue
		}
		b := strconv.AppendInt(e.scratch[:0], int64(len(ef.tag)), 10)
		e.Write(b)
		e.WriteString(":")
		e.WriteString(ef.tag)
		e.reflectValue(fieldValue)
	}
	e.WriteString("e")
}

func interfaceEncoder(e *encodeState, v reflect.Value) {
	e.reflectValue(v.Elem())
}

type stringValues []reflect.Value

func (sv stringValues) Len() int           { return len(sv) }
func (sv stringValues) Swap(i, j int)      { sv[i], sv[j] = sv[j], sv[i] }
func (sv stringValues) Less(i, j int) bool { return sv.get(i) < sv.get(j) }
func (sv stringValues) get(i int) string   { return sv[i].String() }

func mapEncoder(e *encodeState, v reflect.Value) {
	if v.Type().Key().Kind() != reflect.String {
		// panic(&MarshalerError{v.Type()})
		panic(&UnsupportedTypeError{v.Type()})
	}
	if v.IsNil() {
		e.WriteString("de")
		return
	}
	e.WriteString("d")
	sv := stringValues(v.MapKeys())
	sort.Sort(sv)
	for _, key := range sv {
		s := key.String()
		b := strconv.AppendInt(e.scratch[:0], int64(len(s)), 10)
		e.Write(b)
		e.WriteString(":")
		e.WriteString(s)
		e.reflectValue(v.MapIndex(key))
	}
	e.WriteString("e")
}

func arrayEncoder(e *encodeState, v reflect.Value) {
	e.WriteString("l")
	for i, n := 0, v.Len(); i < n; i++ {
		e.reflectValue(v.Index(i))
	}
	e.WriteString("e")
}

// the sliceEncoder in Ana's impl looks odd, maybe incomplete
func sliceEncoder(e *encodeState, v reflect.Value) {
	if v.IsNil() {
		e.WriteString("le")
		return
	}
	if v.Type().Elem().Kind() == reflect.Uint8 {
		s := v.Bytes()
		b := strconv.AppendInt(e.scratch[:0], int64(len(s)), 10)
		e.Write(b)
		e.WriteString(":")
		e.Write(s)
	}
}

func ptrEncoder(e *encodeState, v reflect.Value) {
	if v.IsNil() {
		v = reflect.Zero(v.Type().Elem())
	} else {
		v = v.Elem()
	}
	e.reflectValue(v)
}

func unsupportedTypeEncoder(e *encodeState, v reflect.Value) {
	e.error(&UnsupportedTypeError{v.Type()})
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

type encodeField struct {
	i         int
	tag       string
	omitEmpty bool
}

var (
	typeCacheLock     sync.RWMutex
	encodeFieldsCache = make(map[reflect.Type][]encodeField)
)

type encodeFieldsSortType []encodeField

func (ef encodeFieldsSortType) Len() int           { return len(ef) }
func (ef encodeFieldsSortType) Swap(i, j int)      { ef[i], ef[j] = ef[j], ef[i] }
func (ef encodeFieldsSortType) Less(i, j int) bool { return ef[i].tag < ef[j].tag }

// Why use lock?
func encodeFields(t reflect.Type) []encodeField {
	typeCacheLock.RLock()
	fs, ok := encodeFieldsCache[t]
	typeCacheLock.RUnlock()
	if ok {
		return fs
	}

	// Double locking ?
	typeCacheLock.Lock()
	defer typeCacheLock.Unlock()
	fs, ok = encodeFieldsCache[t]
	if ok {
		return fs
	}

	for i, n := 0, t.NumField(); i < n; i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		if f.Anonymous {
			continue
		}
		var ef encodeField
		ef.i = i
		ef.tag = f.Name

		tv := getTag(f.Tag)
		if tv.Ignore() {
			continue
		}
		if tv.Key() != "" {
			ef.tag = tv.Key()
		}
		ef.omitEmpty = tv.OmitEmpty()
		fs = append(fs, ef)
	}
	fss := encodeFieldsSortType(fs)
	sort.Sort(fss)
	encodeFieldsCache[t] = fs
	return fs
}
