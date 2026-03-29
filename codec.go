package dynago

import (
	"reflect"
	"strings"
	"sync"
)

// fieldOptions holds the parsed metadata from a dynamo struct tag.
type fieldOptions struct {
	// Name is the DynamoDB attribute name. Defaults to the Go field name.
	Name string

	// Skip is true when the tag is "-".
	Skip bool

	// OmitEmpty causes zero-value fields to be omitted during marshalling.
	OmitEmpty bool

	// Hash marks this field as a hash key.
	Hash bool

	// Range marks this field as a range (sort) key.
	Range bool

	// GSI holds the GSI name when the field is tagged with gsi:<name>.
	GSI string

	// Set marks the field for SS/NS/BS encoding instead of L.
	Set bool

	// UnixTime marks a time.Time field for unix-timestamp encoding (N) instead
	// of ISO 8601 string (S).
	UnixTime bool
}

// fieldInfo pairs a reflected struct field with its parsed tag options.
type fieldInfo struct {
	Field   reflect.StructField
	Options fieldOptions
}

// typeCodec caches the parsed field metadata for a struct type.
type typeCodec struct {
	Fields []fieldInfo
}

// codecCache stores *typeCodec keyed by reflect.Type.
var codecCache sync.Map

// getCodec returns the cached typeCodec for t, building and caching it on first
// access. t must be a struct type.
func getCodec(t reflect.Type) *typeCodec {
	if v, ok := codecCache.Load(t); ok {
		return v.(*typeCodec)
	}

	codec := buildCodec(t)
	actual, _ := codecCache.LoadOrStore(t, codec)
	return actual.(*typeCodec)
}

// buildCodec parses all exported fields of struct type t.
func buildCodec(t reflect.Type) *typeCodec {
	var fields []fieldInfo
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)

		// Skip unexported fields.
		if !sf.IsExported() {
			continue
		}

		opts := parseTag(sf)
		if opts.Skip {
			continue
		}

		fields = append(fields, fieldInfo{
			Field:   sf,
			Options: opts,
		})
	}
	return &typeCodec{Fields: fields}
}

// parseTag extracts fieldOptions from the dynamo struct tag on sf.
func parseTag(sf reflect.StructField) fieldOptions {
	tag := sf.Tag.Get("dynamo")

	// No tag — use field name.
	if tag == "" {
		return fieldOptions{Name: sf.Name}
	}

	// "-" means skip entirely.
	if tag == "-" {
		return fieldOptions{Skip: true}
	}

	opts := fieldOptions{}
	parts := strings.Split(tag, ",")

	for i, part := range parts {
		part = strings.TrimSpace(part)
		switch {
		case i == 0 && part != "" && !isKnownOption(part):
			// First element is the attribute name if it's not a known option.
			opts.Name = part
		case part == "hash":
			opts.Hash = true
		case part == "range":
			opts.Range = true
		case strings.HasPrefix(part, "gsi:"):
			opts.GSI = part[4:]
		case part == "omitempty":
			opts.OmitEmpty = true
		case part == "set":
			opts.Set = true
		case part == "unixtime":
			opts.UnixTime = true
		}
	}

	// Default name to the Go field name if none was specified in the tag.
	if opts.Name == "" {
		opts.Name = sf.Name
	}

	return opts
}

// isKnownOption reports whether s is one of the recognised non-name tag tokens.
func isKnownOption(s string) bool {
	switch {
	case s == "hash", s == "range", s == "omitempty", s == "set", s == "unixtime":
		return true
	case strings.HasPrefix(s, "gsi:"):
		return true
	}
	return false
}
