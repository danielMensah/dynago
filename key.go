package dynago

import (
	"fmt"
	"strconv"
)

// KeyValue is an opaque type that resolves to map[string]AttributeValue
// internally. Use the Key(), StringKey(), or StringPairKey() helpers to
// construct values.
type KeyValue struct {
	hashName  string
	hashVal   AttributeValue
	rangeName string
	rangeVal  AttributeValue
	hasRange  bool
}

// Map returns the underlying attribute map. This is used internally by
// operations that need to pass the key to a Backend.
func (kv KeyValue) Map() map[string]AttributeValue {
	if kv.hasRange {
		return map[string]AttributeValue{
			kv.hashName:  kv.hashVal,
			kv.rangeName: kv.rangeVal,
		}
	}
	return map[string]AttributeValue{
		kv.hashName: kv.hashVal,
	}
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
//
// For the highest performance in hot paths, prefer [StringKey] or
// [StringPairKey] when all key values are strings.
func Key(pairs ...any) KeyValue {
	if len(pairs) != 2 && len(pairs) != 4 {
		panic(fmt.Sprintf("dynago.Key: expected 2 or 4 arguments, got %d", len(pairs)))
	}

	name0, ok := pairs[0].(string)
	if !ok {
		panic(fmt.Sprintf("dynago.Key: argument 0 must be a string attribute name, got %T", pairs[0]))
	}

	kv := KeyValue{
		hashName: name0,
		hashVal:  toAttributeValue(pairs[1]),
	}

	if len(pairs) == 4 {
		name2, ok := pairs[2].(string)
		if !ok {
			panic(fmt.Sprintf("dynago.Key: argument 2 must be a string attribute name, got %T", pairs[2]))
		}
		kv.rangeName = name2
		kv.rangeVal = toAttributeValue(pairs[3])
		kv.hasRange = true
	}

	return kv
}

// StringKey builds a KeyValue with a single string hash key.
// This is the zero-allocation fast path for the most common case.
func StringKey(name, val string) KeyValue {
	return KeyValue{
		hashName: name,
		hashVal:  AttributeValue{Type: TypeS, S: val},
	}
}

// StringPairKey builds a KeyValue with a string hash key and a string range key.
// This is the zero-allocation fast path for the most common composite key case.
func StringPairKey(hashName, hashVal, rangeName, rangeVal string) KeyValue {
	return KeyValue{
		hashName:  hashName,
		hashVal:   AttributeValue{Type: TypeS, S: hashVal},
		rangeName: rangeName,
		rangeVal:  AttributeValue{Type: TypeS, S: rangeVal},
		hasRange:  true,
	}
}

// toAttributeValue converts a Go value to an AttributeValue for use in keys.
func toAttributeValue(v any) AttributeValue {
	switch val := v.(type) {
	case string:
		return AttributeValue{Type: TypeS, S: val}
	case int:
		return AttributeValue{Type: TypeN, N: strconv.Itoa(val)}
	case int64:
		return AttributeValue{Type: TypeN, N: strconv.FormatInt(val, 10)}
	case float64:
		return AttributeValue{Type: TypeN, N: strconv.FormatFloat(val, 'g', -1, 64)}
	case uint:
		return AttributeValue{Type: TypeN, N: strconv.FormatUint(uint64(val), 10)}
	case uint64:
		return AttributeValue{Type: TypeN, N: strconv.FormatUint(val, 10)}
	case []byte:
		return AttributeValue{Type: TypeB, B: val}
	default:
		panic(fmt.Sprintf("dynago.Key: unsupported value type %T", v))
	}
}
