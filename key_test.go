package dynago

import (
	"reflect"
	"testing"
)

func TestKey_HashOnly_String(t *testing.T) {
	kv := Key("PK", "user#123")
	m := kv.Map()

	if len(m) != 1 {
		t.Fatalf("expected 1 key, got %d", len(m))
	}
	av, ok := m["PK"]
	if !ok {
		t.Fatal("expected key 'PK' to exist")
	}
	if av.Type != TypeS {
		t.Fatalf("expected TypeS, got %d", av.Type)
	}
	if av.S != "user#123" {
		t.Fatalf("expected 'user#123', got %q", av.S)
	}
}

func TestKey_HashOnly_Int(t *testing.T) {
	kv := Key("ID", 42)
	m := kv.Map()

	av := m["ID"]
	if av.Type != TypeN {
		t.Fatalf("expected TypeN, got %d", av.Type)
	}
	if av.N != "42" {
		t.Fatalf("expected '42', got %q", av.N)
	}
}

func TestKey_HashOnly_Int64(t *testing.T) {
	kv := Key("ID", int64(9999999999))
	m := kv.Map()

	av := m["ID"]
	if av.Type != TypeN {
		t.Fatalf("expected TypeN, got %d", av.Type)
	}
	if av.N != "9999999999" {
		t.Fatalf("expected '9999999999', got %q", av.N)
	}
}

func TestKey_HashOnly_Float64(t *testing.T) {
	kv := Key("Score", float64(3.14))
	m := kv.Map()

	av := m["Score"]
	if av.Type != TypeN {
		t.Fatalf("expected TypeN, got %d", av.Type)
	}
	if av.N != "3.14" {
		t.Fatalf("expected '3.14', got %q", av.N)
	}
}

func TestKey_HashOnly_Uint(t *testing.T) {
	kv := Key("ID", uint(7))
	m := kv.Map()

	av := m["ID"]
	if av.Type != TypeN {
		t.Fatalf("expected TypeN, got %d", av.Type)
	}
	if av.N != "7" {
		t.Fatalf("expected '7', got %q", av.N)
	}
}

func TestKey_HashOnly_Uint64(t *testing.T) {
	kv := Key("ID", uint64(18446744073709551615))
	m := kv.Map()

	av := m["ID"]
	if av.Type != TypeN {
		t.Fatalf("expected TypeN, got %d", av.Type)
	}
	if av.N != "18446744073709551615" {
		t.Fatalf("expected '18446744073709551615', got %q", av.N)
	}
}

func TestKey_HashOnly_Binary(t *testing.T) {
	data := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	kv := Key("BinaryKey", data)
	m := kv.Map()

	av := m["BinaryKey"]
	if av.Type != TypeB {
		t.Fatalf("expected TypeB, got %d", av.Type)
	}
	if len(av.B) != 4 || av.B[0] != 0xDE || av.B[3] != 0xEF {
		t.Fatalf("unexpected binary value: %v", av.B)
	}
}

func TestKey_HashAndRange(t *testing.T) {
	kv := Key("PK", "user#123", "SK", "order#2024-01-15")
	m := kv.Map()

	if len(m) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(m))
	}

	pk := m["PK"]
	if pk.Type != TypeS || pk.S != "user#123" {
		t.Fatalf("unexpected PK: %+v", pk)
	}

	sk := m["SK"]
	if sk.Type != TypeS || sk.S != "order#2024-01-15" {
		t.Fatalf("unexpected SK: %+v", sk)
	}
}

func TestKey_HashAndRange_MixedTypes(t *testing.T) {
	kv := Key("PK", "tenant#abc", "SK", 100)
	m := kv.Map()

	pk := m["PK"]
	if pk.Type != TypeS || pk.S != "tenant#abc" {
		t.Fatalf("unexpected PK: %+v", pk)
	}

	sk := m["SK"]
	if sk.Type != TypeN || sk.N != "100" {
		t.Fatalf("unexpected SK: %+v", sk)
	}
}

func TestKey_PanicOnWrongArgCount(t *testing.T) {
	tests := []struct {
		name string
		args []any
	}{
		{"zero args", nil},
		{"one arg", []any{"PK"}},
		{"three args", []any{"PK", "val", "SK"}},
		{"five args", []any{"PK", "a", "SK", "b", "extra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatal("expected panic, got none")
				}
			}()
			Key(tt.args...)
		})
	}
}

func TestKey_PanicOnNonStringName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic, got none")
		}
	}()
	Key(123, "value")
}

func TestKey_PanicOnUnsupportedValueType(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic, got none")
		}
	}()
	Key("PK", struct{}{})
}

func TestKey_PanicOnBoolValue(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic, got none")
		}
	}()
	Key("Active", true)
}

func TestStringKey(t *testing.T) {
	kv := StringKey("PK", "user#123")
	m := kv.Map()

	if len(m) != 1 {
		t.Fatalf("expected 1 key, got %d", len(m))
	}
	av := m["PK"]
	if av.Type != TypeS || av.S != "user#123" {
		t.Fatalf("unexpected PK: %+v", av)
	}
}

func TestStringPairKey(t *testing.T) {
	kv := StringPairKey("PK", "user#123", "SK", "order#2024-01-15")
	m := kv.Map()

	if len(m) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(m))
	}
	pk := m["PK"]
	if pk.Type != TypeS || pk.S != "user#123" {
		t.Fatalf("unexpected PK: %+v", pk)
	}
	sk := m["SK"]
	if sk.Type != TypeS || sk.S != "order#2024-01-15" {
		t.Fatalf("unexpected SK: %+v", sk)
	}
}

func TestKeyEquivalence(t *testing.T) {
	// Hash-only
	generic := Key("PK", "val")
	typed := StringKey("PK", "val")
	if !reflect.DeepEqual(generic.Map(), typed.Map()) {
		t.Fatal("StringKey and Key produce different maps")
	}

	// Hash+range
	generic2 := Key("PK", "a", "SK", "b")
	typed2 := StringPairKey("PK", "a", "SK", "b")
	if !reflect.DeepEqual(generic2.Map(), typed2.Map()) {
		t.Fatal("StringPairKey and Key produce different maps")
	}
}
