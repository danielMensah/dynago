package dynago

import "fmt"

// KeyValue is an opaque type that resolves to map[string]AttributeValue
// internally. Use the Key() helper to construct values.
type KeyValue struct {
	m map[string]AttributeValue
}

// Map returns the underlying attribute map. This is used internally by
// operations that need to pass the key to a Backend.
func (kv KeyValue) Map() map[string]AttributeValue {
	return kv.m
}

// Key builds a KeyValue from pairs of (attributeName, value).
// It accepts exactly 2 arguments (hash key only) or 4 arguments (hash + range).
//
// Supported value types:
//   - string        → AttributeValue{Type: TypeS}
//   - int, int64, float64, uint, uint64 → AttributeValue{Type: TypeN}
//   - []byte        → AttributeValue{Type: TypeB}
//
// Key panics on invalid input (wrong argument count, non-string attribute
// name, or unsupported value type). This follows the same convention as
// regexp.MustCompile — key construction is a programmer error.
func Key(pairs ...any) KeyValue {
	if len(pairs) != 2 && len(pairs) != 4 {
		panic(fmt.Sprintf("dynago.Key: expected 2 or 4 arguments, got %d", len(pairs)))
	}

	m := make(map[string]AttributeValue, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		name, ok := pairs[i].(string)
		if !ok {
			panic(fmt.Sprintf("dynago.Key: argument %d must be a string attribute name, got %T", i, pairs[i]))
		}
		m[name] = toAttributeValue(pairs[i+1])
	}
	return KeyValue{m: m}
}

// toAttributeValue converts a Go value to an AttributeValue for use in keys.
func toAttributeValue(v any) AttributeValue {
	switch val := v.(type) {
	case string:
		return AttributeValue{Type: TypeS, S: val}
	case int:
		return AttributeValue{Type: TypeN, N: fmt.Sprintf("%d", val)}
	case int64:
		return AttributeValue{Type: TypeN, N: fmt.Sprintf("%d", val)}
	case float64:
		return AttributeValue{Type: TypeN, N: fmt.Sprintf("%g", val)}
	case uint:
		return AttributeValue{Type: TypeN, N: fmt.Sprintf("%d", val)}
	case uint64:
		return AttributeValue{Type: TypeN, N: fmt.Sprintf("%d", val)}
	case []byte:
		return AttributeValue{Type: TypeB, B: val}
	default:
		panic(fmt.Sprintf("dynago.Key: unsupported value type %T", v))
	}
}
