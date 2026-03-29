package dynago

import (
	"fmt"
	"reflect"
	"strconv"
	"time"
)

// Marshaler is implemented by types that can marshal themselves into a
// DynamoDB AttributeValue.
type Marshaler interface {
	MarshalDynamo() (AttributeValue, error)
}

var (
	timeType      = reflect.TypeOf(time.Time{})
	byteSliceType = reflect.TypeOf([]byte(nil))
)

// Marshal converts a Go struct into a DynamoDB item representation.
// The struct's exported fields are encoded using the cached typeCodec from the
// struct tag parser (US-002). Struct tags use the "dynamo" key.
func Marshal(v any) (map[string]AttributeValue, error) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return nil, &Error{Sentinel: ErrValidation, Message: "dynago.Marshal: nil value"}
	}

	// Dereference pointer.
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, &Error{Sentinel: ErrValidation, Message: "dynago.Marshal: nil pointer"}
		}
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return nil, &Error{Sentinel: ErrValidation, Message: fmt.Sprintf("dynago.Marshal: expected struct, got %s", rv.Kind())}
	}

	codec := getCodec(rv.Type())
	out := make(map[string]AttributeValue, len(codec.Fields))

	for _, fi := range codec.Fields {
		fv := rv.FieldByIndex(fi.Field.Index)

		// Nil pointer → omit.
		if fv.Kind() == reflect.Pointer && fv.IsNil() {
			continue
		}

		// Dereference pointer for value inspection.
		elem := fv
		if elem.Kind() == reflect.Pointer {
			elem = elem.Elem()
		}

		// omitempty: skip zero-value fields.
		if fi.Options.OmitEmpty && isZero(elem) {
			continue
		}

		av, err := marshalValue(elem, fi.Options)
		if err != nil {
			return nil, fmt.Errorf("dynago.Marshal: field %s: %w", fi.Options.Name, err)
		}
		out[fi.Options.Name] = av
	}

	return out, nil
}

// marshalValue converts a reflect.Value into an AttributeValue. It handles the
// Marshaler interface, time.Time, and all primitive/composite types recursively.
func marshalValue(v reflect.Value, opts fieldOptions) (AttributeValue, error) {
	// Check Marshaler interface (value or pointer receiver).
	if v.IsValid() && v.CanInterface() {
		if m, ok := v.Interface().(Marshaler); ok {
			return m.MarshalDynamo()
		}
	}
	if v.IsValid() && v.CanAddr() && v.Addr().CanInterface() {
		if m, ok := v.Addr().Interface().(Marshaler); ok {
			return m.MarshalDynamo()
		}
	}

	// Handle time.Time specially.
	if v.Type() == timeType {
		return marshalTime(v, opts)
	}

	switch v.Kind() {
	case reflect.String:
		return AttributeValue{Type: TypeS, S: v.String()}, nil

	case reflect.Bool:
		return AttributeValue{Type: TypeBOOL, BOOL: v.Bool()}, nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return AttributeValue{Type: TypeN, N: strconv.FormatInt(v.Int(), 10)}, nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return AttributeValue{Type: TypeN, N: strconv.FormatUint(v.Uint(), 10)}, nil

	case reflect.Float32:
		return AttributeValue{Type: TypeN, N: strconv.FormatFloat(v.Float(), 'f', -1, 32)}, nil

	case reflect.Float64:
		return AttributeValue{Type: TypeN, N: strconv.FormatFloat(v.Float(), 'f', -1, 64)}, nil

	case reflect.Slice:
		return marshalSlice(v, opts)

	case reflect.Map:
		return marshalMap(v)

	case reflect.Struct:
		return marshalStruct(v)

	case reflect.Pointer:
		if v.IsNil() {
			return AttributeValue{Type: TypeNULL, NULL: true}, nil
		}
		return marshalValue(v.Elem(), opts)

	default:
		return AttributeValue{}, &Error{
			Sentinel: ErrValidation,
			Message:  fmt.Sprintf("dynago.Marshal: unsupported type %s", v.Type()),
		}
	}
}

// marshalTime encodes a time.Time as either a unix timestamp (N) or ISO 8601
// string (S), depending on the unixtime tag option.
func marshalTime(v reflect.Value, opts fieldOptions) (AttributeValue, error) {
	t := v.Interface().(time.Time)
	if opts.UnixTime {
		return AttributeValue{Type: TypeN, N: strconv.FormatInt(t.Unix(), 10)}, nil
	}
	return AttributeValue{Type: TypeS, S: t.UTC().Format(time.RFC3339)}, nil
}

// marshalSlice encodes a slice. []byte becomes B; tagged-set slices become
// SS/NS/BS; everything else becomes L.
func marshalSlice(v reflect.Value, opts fieldOptions) (AttributeValue, error) {
	// []byte → Binary.
	if v.Type() == byteSliceType {
		return AttributeValue{Type: TypeB, B: v.Bytes()}, nil
	}

	if v.IsNil() {
		return AttributeValue{Type: TypeNULL, NULL: true}, nil
	}

	// Set encoding.
	if opts.Set {
		return marshalSet(v)
	}

	// List encoding.
	list := make([]AttributeValue, v.Len())
	for i := 0; i < v.Len(); i++ {
		av, err := marshalValue(v.Index(i), fieldOptions{})
		if err != nil {
			return AttributeValue{}, err
		}
		list[i] = av
	}
	return AttributeValue{Type: TypeL, L: list}, nil
}

// marshalSet encodes a slice as a DynamoDB set (SS, NS, or BS).
func marshalSet(v reflect.Value) (AttributeValue, error) {
	elemType := v.Type().Elem()

	switch {
	case elemType.Kind() == reflect.String:
		ss := make([]string, v.Len())
		for i := 0; i < v.Len(); i++ {
			ss[i] = v.Index(i).String()
		}
		return AttributeValue{Type: TypeSS, SS: ss}, nil

	case elemType == byteSliceType:
		bs := make([][]byte, v.Len())
		for i := 0; i < v.Len(); i++ {
			bs[i] = v.Index(i).Bytes()
		}
		return AttributeValue{Type: TypeBS, BS: bs}, nil

	case isIntKind(elemType.Kind()):
		ns := make([]string, v.Len())
		for i := 0; i < v.Len(); i++ {
			ns[i] = strconv.FormatInt(v.Index(i).Int(), 10)
		}
		return AttributeValue{Type: TypeNS, NS: ns}, nil

	case isUintKind(elemType.Kind()):
		ns := make([]string, v.Len())
		for i := 0; i < v.Len(); i++ {
			ns[i] = strconv.FormatUint(v.Index(i).Uint(), 10)
		}
		return AttributeValue{Type: TypeNS, NS: ns}, nil

	case isFloatKind(elemType.Kind()):
		ns := make([]string, v.Len())
		for i := 0; i < v.Len(); i++ {
			ns[i] = strconv.FormatFloat(v.Index(i).Float(), 'f', -1, 64)
		}
		return AttributeValue{Type: TypeNS, NS: ns}, nil

	default:
		return AttributeValue{}, &Error{
			Sentinel: ErrValidation,
			Message:  fmt.Sprintf("dynago.Marshal: unsupported set element type %s", elemType),
		}
	}
}

// marshalMap encodes a map[string]T as a DynamoDB M attribute.
func marshalMap(v reflect.Value) (AttributeValue, error) {
	if v.IsNil() {
		return AttributeValue{Type: TypeNULL, NULL: true}, nil
	}
	if v.Type().Key().Kind() != reflect.String {
		return AttributeValue{}, &Error{
			Sentinel: ErrValidation,
			Message:  fmt.Sprintf("dynago.Marshal: map key must be string, got %s", v.Type().Key()),
		}
	}

	m := make(map[string]AttributeValue, v.Len())
	iter := v.MapRange()
	for iter.Next() {
		av, err := marshalValue(iter.Value(), fieldOptions{})
		if err != nil {
			return AttributeValue{}, err
		}
		m[iter.Key().String()] = av
	}
	return AttributeValue{Type: TypeM, M: m}, nil
}

// marshalStruct encodes a nested struct as a DynamoDB M attribute using its
// cached typeCodec.
func marshalStruct(v reflect.Value) (AttributeValue, error) {
	codec := getCodec(v.Type())
	m := make(map[string]AttributeValue, len(codec.Fields))

	for _, fi := range codec.Fields {
		fv := v.FieldByIndex(fi.Field.Index)

		if fv.Kind() == reflect.Pointer && fv.IsNil() {
			continue
		}

		elem := fv
		if elem.Kind() == reflect.Pointer {
			elem = elem.Elem()
		}

		if fi.Options.OmitEmpty && isZero(elem) {
			continue
		}

		av, err := marshalValue(elem, fi.Options)
		if err != nil {
			return AttributeValue{}, fmt.Errorf("field %s: %w", fi.Options.Name, err)
		}
		m[fi.Options.Name] = av
	}
	return AttributeValue{Type: TypeM, M: m}, nil
}

// isZero reports whether v is the zero value for its type, used for omitempty.
func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Slice, reflect.Map:
		return v.IsNil() || v.Len() == 0
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	case reflect.Struct:
		if v.Type() == timeType {
			return v.Interface().(time.Time).IsZero()
		}
		return false
	default:
		return false
	}
}

func isIntKind(k reflect.Kind) bool {
	return k == reflect.Int || k == reflect.Int8 || k == reflect.Int16 || k == reflect.Int32 || k == reflect.Int64
}

func isUintKind(k reflect.Kind) bool {
	return k == reflect.Uint || k == reflect.Uint8 || k == reflect.Uint16 || k == reflect.Uint32 || k == reflect.Uint64
}

func isFloatKind(k reflect.Kind) bool {
	return k == reflect.Float32 || k == reflect.Float64
}
