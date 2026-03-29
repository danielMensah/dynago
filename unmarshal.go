package dynago

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"time"
)

// Unmarshaler is implemented by types that can unmarshal an AttributeValue
// into themselves.
type Unmarshaler interface {
	UnmarshalDynamo(AttributeValue) error
}

var (
	unmarshalerType = reflect.TypeFor[Unmarshaler]()
)

// Unmarshal decodes a DynamoDB item (map[string]AttributeValue) into the
// struct pointed to by out. out must be a non-nil pointer to a struct.
func Unmarshal(item map[string]AttributeValue, out any) error {
	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return newError(ErrValidation, "dynago.Unmarshal: out must be a non-nil pointer to a struct")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return newError(ErrValidation, "dynago.Unmarshal: out must point to a struct")
	}

	codec := getCodec(rv.Type())

	for _, fi := range codec.Fields {
		av, ok := item[fi.Options.Name]
		if !ok {
			// Missing attribute: for pointer fields, leave as nil (zero value).
			continue
		}

		fv := rv.FieldByIndex(fi.Field.Index)
		if err := unmarshalValue(av, fv, fi.Options); err != nil {
			return &Error{
				Sentinel: ErrValidation,
				Message:  fmt.Sprintf("dynago.Unmarshal: field %s: %v", fi.Field.Name, err),
			}
		}
	}
	return nil
}

// unmarshalValue decodes a single AttributeValue into the reflect.Value dst.
func unmarshalValue(av AttributeValue, dst reflect.Value, opts fieldOptions) error {
	// Handle pointer indirection: allocate if needed.
	if dst.Kind() == reflect.Pointer {
		if av.Type == TypeNULL && av.NULL {
			dst.Set(reflect.Zero(dst.Type()))
			return nil
		}
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		dst = dst.Elem()
	}

	// Check for Unmarshaler interface (pointer receiver).
	if dst.CanAddr() && dst.Addr().Type().Implements(unmarshalerType) {
		return dst.Addr().Interface().(Unmarshaler).UnmarshalDynamo(av)
	}
	// Check for Unmarshaler interface (value receiver).
	if dst.Type().Implements(unmarshalerType) {
		return dst.Interface().(Unmarshaler).UnmarshalDynamo(av)
	}

	// Handle time.Time specially.
	if dst.Type() == reflect.TypeFor[time.Time]() {
		return unmarshalTime(av, dst, opts)
	}

	switch dst.Kind() {
	case reflect.String:
		if av.Type != TypeS {
			return fmt.Errorf("expected S, got type %d", av.Type)
		}
		dst.SetString(av.S)

	case reflect.Bool:
		if av.Type != TypeBOOL {
			return fmt.Errorf("expected BOOL, got type %d", av.Type)
		}
		dst.SetBool(av.BOOL)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if av.Type != TypeN {
			return fmt.Errorf("expected N, got type %d", av.Type)
		}
		n, err := strconv.ParseInt(av.N, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing int: %w", err)
		}
		if dst.OverflowInt(n) {
			return fmt.Errorf("value %d overflows %s", n, dst.Type())
		}
		dst.SetInt(n)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if av.Type != TypeN {
			return fmt.Errorf("expected N, got type %d", av.Type)
		}
		n, err := strconv.ParseUint(av.N, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing uint: %w", err)
		}
		if dst.OverflowUint(n) {
			return fmt.Errorf("value %d overflows %s", n, dst.Type())
		}
		dst.SetUint(n)

	case reflect.Float32, reflect.Float64:
		if av.Type != TypeN {
			return fmt.Errorf("expected N, got type %d", av.Type)
		}
		f, err := strconv.ParseFloat(av.N, 64)
		if err != nil {
			return fmt.Errorf("parsing float: %w", err)
		}
		if dst.Kind() == reflect.Float32 && (f > math.MaxFloat32 || f < -math.MaxFloat32) && !math.IsInf(f, 0) {
			return fmt.Errorf("value %g overflows float32", f)
		}
		dst.SetFloat(f)

	case reflect.Slice:
		return unmarshalSlice(av, dst, opts)

	case reflect.Map:
		return unmarshalMap(av, dst)

	case reflect.Struct:
		if av.Type != TypeM {
			return fmt.Errorf("expected M for struct, got type %d", av.Type)
		}
		return Unmarshal(av.M, dst.Addr().Interface())

	default:
		return fmt.Errorf("unsupported kind %s", dst.Kind())
	}
	return nil
}

// unmarshalTime decodes a time.Time from either N (unix timestamp) or S (ISO 8601).
func unmarshalTime(av AttributeValue, dst reflect.Value, opts fieldOptions) error {
	if opts.UnixTime {
		if av.Type != TypeN {
			return fmt.Errorf("expected N for unixtime, got type %d", av.Type)
		}
		sec, err := strconv.ParseInt(av.N, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing unix time: %w", err)
		}
		dst.Set(reflect.ValueOf(time.Unix(sec, 0).UTC()))
		return nil
	}

	if av.Type != TypeS {
		return fmt.Errorf("expected S for time, got type %d", av.Type)
	}
	t, err := time.Parse(time.RFC3339Nano, av.S)
	if err != nil {
		// Try RFC3339 without nanos.
		t, err = time.Parse(time.RFC3339, av.S)
		if err != nil {
			return fmt.Errorf("parsing time: %w", err)
		}
	}
	dst.Set(reflect.ValueOf(t.UTC()))
	return nil
}

// unmarshalSlice handles list (L) and set (SS/NS/BS) decoding.
func unmarshalSlice(av AttributeValue, dst reflect.Value, opts fieldOptions) error {
	elemType := dst.Type().Elem()

	// []byte is handled as binary (B).
	if elemType.Kind() == reflect.Uint8 {
		if av.Type != TypeB {
			return fmt.Errorf("expected B for []byte, got type %d", av.Type)
		}
		dst.SetBytes(av.B)
		return nil
	}

	// Set-tagged fields.
	if opts.Set {
		return unmarshalSet(av, dst)
	}

	// Regular list (L).
	if av.Type != TypeL {
		return fmt.Errorf("expected L for slice, got type %d", av.Type)
	}

	slice := reflect.MakeSlice(dst.Type(), len(av.L), len(av.L))
	for i, elem := range av.L {
		if err := unmarshalValue(elem, slice.Index(i), fieldOptions{}); err != nil {
			return fmt.Errorf("index %d: %w", i, err)
		}
	}
	dst.Set(slice)
	return nil
}

// unmarshalSet decodes SS, NS, or BS into a typed slice.
func unmarshalSet(av AttributeValue, dst reflect.Value) error {
	elemType := dst.Type().Elem()

	switch av.Type {
	case TypeSS:
		if elemType.Kind() != reflect.String {
			return fmt.Errorf("SS requires []string target, got %s", dst.Type())
		}
		slice := reflect.MakeSlice(dst.Type(), len(av.SS), len(av.SS))
		for i, s := range av.SS {
			slice.Index(i).SetString(s)
		}
		dst.Set(slice)

	case TypeNS:
		slice := reflect.MakeSlice(dst.Type(), len(av.NS), len(av.NS))
		for i, n := range av.NS {
			switch elemType.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				v, err := strconv.ParseInt(n, 10, 64)
				if err != nil {
					return fmt.Errorf("NS index %d: %w", i, err)
				}
				slice.Index(i).SetInt(v)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				v, err := strconv.ParseUint(n, 10, 64)
				if err != nil {
					return fmt.Errorf("NS index %d: %w", i, err)
				}
				slice.Index(i).SetUint(v)
			case reflect.Float32, reflect.Float64:
				v, err := strconv.ParseFloat(n, 64)
				if err != nil {
					return fmt.Errorf("NS index %d: %w", i, err)
				}
				slice.Index(i).SetFloat(v)
			default:
				return fmt.Errorf("NS requires numeric slice target, got %s", dst.Type())
			}
		}
		dst.Set(slice)

	case TypeBS:
		if elemType.Kind() != reflect.Slice || elemType.Elem().Kind() != reflect.Uint8 {
			return fmt.Errorf("BS requires [][]byte target, got %s", dst.Type())
		}
		slice := reflect.MakeSlice(dst.Type(), len(av.BS), len(av.BS))
		for i, b := range av.BS {
			slice.Index(i).SetBytes(b)
		}
		dst.Set(slice)

	default:
		return fmt.Errorf("expected SS/NS/BS for set field, got type %d", av.Type)
	}
	return nil
}

// unmarshalMap decodes an M AttributeValue into a map.
func unmarshalMap(av AttributeValue, dst reflect.Value) error {
	if av.Type != TypeM {
		return fmt.Errorf("expected M for map, got type %d", av.Type)
	}

	mapType := dst.Type()
	if mapType.Key().Kind() != reflect.String {
		return fmt.Errorf("map key must be string, got %s", mapType.Key())
	}

	m := reflect.MakeMapWithSize(mapType, len(av.M))
	elemType := mapType.Elem()

	for k, v := range av.M {
		elem := reflect.New(elemType).Elem()
		if err := unmarshalValue(v, elem, fieldOptions{}); err != nil {
			return fmt.Errorf("map key %q: %w", k, err)
		}
		m.SetMapIndex(reflect.ValueOf(k), elem)
	}
	dst.Set(m)
	return nil
}
