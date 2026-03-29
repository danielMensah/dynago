package dynago

import (
	"errors"
	"testing"
	"time"
)

// customMarshaler implements Marshaler for testing.
type customMarshaler struct {
	Value string
}

func (c customMarshaler) MarshalDynamo() (AttributeValue, error) {
	return AttributeValue{Type: TypeS, S: "custom:" + c.Value}, nil
}

type customMarshalerErr struct{}

func (c customMarshalerErr) MarshalDynamo() (AttributeValue, error) {
	return AttributeValue{}, errors.New("custom error")
}

func TestMarshal_String(t *testing.T) {
	type S struct {
		Name string
	}
	got, err := Marshal(S{Name: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if got["Name"].Type != TypeS || got["Name"].S != "alice" {
		t.Fatalf("expected S=alice, got %+v", got["Name"])
	}
}

func TestMarshal_Numbers(t *testing.T) {
	type S struct {
		I   int
		I8  int8
		I16 int16
		I32 int32
		I64 int64
		U   uint
		U8  uint8
		U16 uint16
		U32 uint32
		U64 uint64
		F32 float32
		F64 float64
	}
	got, err := Marshal(S{
		I: 1, I8: 2, I16: 3, I32: 4, I64: 5,
		U: 6, U8: 7, U16: 8, U32: 9, U64: 10,
		F32: 1.5, F64: 2.5,
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		field string
		want  string
	}{
		{"I", "1"}, {"I8", "2"}, {"I16", "3"}, {"I32", "4"}, {"I64", "5"},
		{"U", "6"}, {"U8", "7"}, {"U16", "8"}, {"U32", "9"}, {"U64", "10"},
		{"F32", "1.5"}, {"F64", "2.5"},
	}
	for _, tt := range tests {
		av := got[tt.field]
		if av.Type != TypeN {
			t.Errorf("%s: expected TypeN, got %v", tt.field, av.Type)
		}
		if av.N != tt.want {
			t.Errorf("%s: expected N=%q, got %q", tt.field, tt.want, av.N)
		}
	}
}

func TestMarshal_Bool(t *testing.T) {
	type S struct {
		Active bool
	}
	got, err := Marshal(S{Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if got["Active"].Type != TypeBOOL || !got["Active"].BOOL {
		t.Fatalf("expected BOOL=true, got %+v", got["Active"])
	}
}

func TestMarshal_Bytes(t *testing.T) {
	type S struct {
		Data []byte
	}
	got, err := Marshal(S{Data: []byte{1, 2, 3}})
	if err != nil {
		t.Fatal(err)
	}
	av := got["Data"]
	if av.Type != TypeB || len(av.B) != 3 {
		t.Fatalf("expected B with 3 bytes, got %+v", av)
	}
}

func TestMarshal_TimeISO8601(t *testing.T) {
	type S struct {
		Created time.Time
	}
	ts := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	got, err := Marshal(S{Created: ts})
	if err != nil {
		t.Fatal(err)
	}
	av := got["Created"]
	if av.Type != TypeS || av.S != "2024-06-15T12:00:00Z" {
		t.Fatalf("expected ISO 8601 string, got %+v", av)
	}
}

func TestMarshal_TimeUnix(t *testing.T) {
	type S struct {
		Created time.Time `dynamo:",unixtime"`
	}
	ts := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	got, err := Marshal(S{Created: ts})
	if err != nil {
		t.Fatal(err)
	}
	av := got["Created"]
	if av.Type != TypeN {
		t.Fatalf("expected TypeN for unixtime, got %v", av.Type)
	}
	want := "1718452800"
	if av.N != want {
		t.Fatalf("expected N=%q, got %q", want, av.N)
	}
}

func TestMarshal_Slice_AsList(t *testing.T) {
	type S struct {
		Tags []string
	}
	got, err := Marshal(S{Tags: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	av := got["Tags"]
	if av.Type != TypeL || len(av.L) != 2 {
		t.Fatalf("expected L with 2 elements, got %+v", av)
	}
	if av.L[0].S != "a" || av.L[1].S != "b" {
		t.Fatalf("unexpected list values: %+v", av.L)
	}
}

func TestMarshal_StringSet(t *testing.T) {
	type S struct {
		Tags []string `dynamo:",set"`
	}
	got, err := Marshal(S{Tags: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	av := got["Tags"]
	if av.Type != TypeSS || len(av.SS) != 2 {
		t.Fatalf("expected SS with 2 elements, got %+v", av)
	}
}

func TestMarshal_NumberSet(t *testing.T) {
	type S struct {
		Scores []int `dynamo:",set"`
	}
	got, err := Marshal(S{Scores: []int{1, 2, 3}})
	if err != nil {
		t.Fatal(err)
	}
	av := got["Scores"]
	if av.Type != TypeNS || len(av.NS) != 3 {
		t.Fatalf("expected NS with 3 elements, got %+v", av)
	}
	if av.NS[0] != "1" || av.NS[1] != "2" || av.NS[2] != "3" {
		t.Fatalf("unexpected NS values: %v", av.NS)
	}
}

func TestMarshal_FloatSet(t *testing.T) {
	type S struct {
		Vals []float64 `dynamo:",set"`
	}
	got, err := Marshal(S{Vals: []float64{1.1, 2.2}})
	if err != nil {
		t.Fatal(err)
	}
	av := got["Vals"]
	if av.Type != TypeNS || len(av.NS) != 2 {
		t.Fatalf("expected NS with 2 elements, got %+v", av)
	}
}

func TestMarshal_BinarySet(t *testing.T) {
	type S struct {
		Keys [][]byte `dynamo:",set"`
	}
	got, err := Marshal(S{Keys: [][]byte{{1}, {2}}})
	if err != nil {
		t.Fatal(err)
	}
	av := got["Keys"]
	if av.Type != TypeBS || len(av.BS) != 2 {
		t.Fatalf("expected BS with 2 elements, got %+v", av)
	}
}

func TestMarshal_Map(t *testing.T) {
	type S struct {
		Meta map[string]string
	}
	got, err := Marshal(S{Meta: map[string]string{"k": "v"}})
	if err != nil {
		t.Fatal(err)
	}
	av := got["Meta"]
	if av.Type != TypeM || len(av.M) != 1 {
		t.Fatalf("expected M with 1 entry, got %+v", av)
	}
	if av.M["k"].S != "v" {
		t.Fatalf("expected M[k]=v, got %+v", av.M["k"])
	}
}

func TestMarshal_NestedStruct(t *testing.T) {
	type Address struct {
		City string
	}
	type S struct {
		Addr Address
	}
	got, err := Marshal(S{Addr: Address{City: "NYC"}})
	if err != nil {
		t.Fatal(err)
	}
	av := got["Addr"]
	if av.Type != TypeM {
		t.Fatalf("expected M for nested struct, got %v", av.Type)
	}
	if av.M["City"].S != "NYC" {
		t.Fatalf("expected City=NYC, got %+v", av.M["City"])
	}
}

func TestMarshal_PointerField_Nil(t *testing.T) {
	type S struct {
		Name *string
	}
	got, err := Marshal(S{Name: nil})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["Name"]; ok {
		t.Fatal("nil pointer field should be omitted")
	}
}

func TestMarshal_PointerField_NonNil(t *testing.T) {
	type S struct {
		Name *string
	}
	s := "alice"
	got, err := Marshal(S{Name: &s})
	if err != nil {
		t.Fatal(err)
	}
	if got["Name"].Type != TypeS || got["Name"].S != "alice" {
		t.Fatalf("expected S=alice, got %+v", got["Name"])
	}
}

func TestMarshal_OmitEmpty(t *testing.T) {
	type S struct {
		A string  `dynamo:",omitempty"`
		B int     `dynamo:",omitempty"`
		C bool    `dynamo:",omitempty"`
		D []byte  `dynamo:",omitempty"`
		E float64 `dynamo:",omitempty"`
	}
	got, err := Marshal(S{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map with omitempty zero values, got %+v", got)
	}
}

func TestMarshal_OmitEmpty_NonZero(t *testing.T) {
	type S struct {
		A string `dynamo:",omitempty"`
	}
	got, err := Marshal(S{A: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["A"]; !ok {
		t.Fatal("non-zero omitempty field should be present")
	}
}

func TestMarshal_OmitEmpty_EmptySliceAndMap(t *testing.T) {
	type S struct {
		Tags []string          `dynamo:",omitempty"`
		Meta map[string]string `dynamo:",omitempty"`
	}
	got, err := Marshal(S{Tags: []string{}, Meta: map[string]string{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map for empty slice/map with omitempty, got %+v", got)
	}
}

func TestMarshal_CustomName(t *testing.T) {
	type S struct {
		Name string `dynamo:"user_name"`
	}
	got, err := Marshal(S{Name: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["user_name"]; !ok {
		t.Fatal("expected custom attribute name 'user_name'")
	}
}

func TestMarshal_Skip(t *testing.T) {
	type S struct {
		Public  string
		Private string `dynamo:"-"`
	}
	got, err := Marshal(S{Public: "yes", Private: "no"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["Private"]; ok {
		t.Fatal("field tagged with - should be skipped")
	}
}

func TestMarshal_CustomMarshaler(t *testing.T) {
	type S struct {
		C customMarshaler
	}
	got, err := Marshal(S{C: customMarshaler{Value: "test"}})
	if err != nil {
		t.Fatal(err)
	}
	if got["C"].S != "custom:test" {
		t.Fatalf("expected custom:test, got %+v", got["C"])
	}
}

func TestMarshal_CustomMarshaler_Error(t *testing.T) {
	type S struct {
		C customMarshalerErr
	}
	_, err := Marshal(S{})
	if err == nil {
		t.Fatal("expected error from custom marshaler")
	}
}

func TestMarshal_NilValue(t *testing.T) {
	_, err := Marshal(nil)
	if err == nil {
		t.Fatal("expected error for nil")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestMarshal_NonStruct(t *testing.T) {
	_, err := Marshal("not a struct")
	if err == nil {
		t.Fatal("expected error for non-struct")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestMarshal_Pointer(t *testing.T) {
	type S struct {
		Name string
	}
	s := &S{Name: "alice"}
	got, err := Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if got["Name"].S != "alice" {
		t.Fatalf("expected alice, got %+v", got["Name"])
	}
}

func TestMarshal_NestedSlice(t *testing.T) {
	type Inner struct {
		V int
	}
	type S struct {
		Items []Inner
	}
	got, err := Marshal(S{Items: []Inner{{V: 1}, {V: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	av := got["Items"]
	if av.Type != TypeL || len(av.L) != 2 {
		t.Fatalf("expected L with 2 elements, got %+v", av)
	}
	if av.L[0].Type != TypeM || av.L[0].M["V"].N != "1" {
		t.Fatalf("expected nested struct marshal, got %+v", av.L[0])
	}
}

func TestMarshal_MapWithIntValues(t *testing.T) {
	type S struct {
		Counts map[string]int
	}
	got, err := Marshal(S{Counts: map[string]int{"a": 1}})
	if err != nil {
		t.Fatal(err)
	}
	av := got["Counts"]
	if av.Type != TypeM || av.M["a"].N != "1" {
		t.Fatalf("expected M with int values, got %+v", av)
	}
}

func TestMarshal_CachedCodec(t *testing.T) {
	type S struct {
		Name string
	}
	// Call twice to exercise the cache path.
	_, err := Marshal(S{Name: "a"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Marshal(S{Name: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if got["Name"].S != "b" {
		t.Fatalf("expected b on second call, got %+v", got["Name"])
	}
}

func TestMarshal_ComplexStruct(t *testing.T) {
	type Address struct {
		City    string
		ZipCode string `dynamo:"zip"`
	}
	type User struct {
		PK       string            `dynamo:"PK,hash"`
		SK       string            `dynamo:"SK,range"`
		Name     string            `dynamo:"name"`
		Age      int               `dynamo:"age"`
		Active   bool              `dynamo:"active"`
		Tags     []string          `dynamo:"tags,set"`
		Address  Address           `dynamo:"address"`
		Metadata map[string]string `dynamo:"metadata"`
		Score    *float64          `dynamo:"score,omitempty"`
		Internal string            `dynamo:"-"`
	}

	score := 9.5
	u := User{
		PK:       "USER#123",
		SK:       "PROFILE",
		Name:     "Alice",
		Age:      30,
		Active:   true,
		Tags:     []string{"admin", "user"},
		Address:  Address{City: "NYC", ZipCode: "10001"},
		Metadata: map[string]string{"role": "admin"},
		Score:    &score,
		Internal: "should be skipped",
	}

	got, err := Marshal(u)
	if err != nil {
		t.Fatal(err)
	}

	if got["PK"].S != "USER#123" {
		t.Errorf("PK: %+v", got["PK"])
	}
	if got["SK"].S != "PROFILE" {
		t.Errorf("SK: %+v", got["SK"])
	}
	if got["name"].S != "Alice" {
		t.Errorf("name: %+v", got["name"])
	}
	if got["age"].N != "30" {
		t.Errorf("age: %+v", got["age"])
	}
	if got["active"].BOOL != true {
		t.Errorf("active: %+v", got["active"])
	}
	if got["tags"].Type != TypeSS || len(got["tags"].SS) != 2 {
		t.Errorf("tags: %+v", got["tags"])
	}
	if got["address"].Type != TypeM || got["address"].M["City"].S != "NYC" {
		t.Errorf("address: %+v", got["address"])
	}
	if got["address"].M["zip"].S != "10001" {
		t.Errorf("address.zip: %+v", got["address"].M["zip"])
	}
	if got["metadata"].M["role"].S != "admin" {
		t.Errorf("metadata: %+v", got["metadata"])
	}
	if got["score"].N != "9.5" {
		t.Errorf("score: %+v", got["score"])
	}
	if _, ok := got["Internal"]; ok {
		t.Error("Internal should be skipped")
	}
}

func TestMarshal_OmitEmpty_NilPointer(t *testing.T) {
	type S struct {
		Name *string `dynamo:",omitempty"`
	}
	got, err := Marshal(S{Name: nil})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map for nil pointer with omitempty, got %+v", got)
	}
}

func TestMarshal_NilSlice_NotSet(t *testing.T) {
	type S struct {
		Tags []string
	}
	got, err := Marshal(S{Tags: nil})
	if err != nil {
		t.Fatal(err)
	}
	// nil non-byte slice → NULL
	if got["Tags"].Type != TypeNULL {
		t.Fatalf("expected NULL for nil slice, got %+v", got["Tags"])
	}
}

func TestMarshal_NilMap(t *testing.T) {
	type S struct {
		Meta map[string]string
	}
	got, err := Marshal(S{Meta: nil})
	if err != nil {
		t.Fatal(err)
	}
	if got["Meta"].Type != TypeNULL {
		t.Fatalf("expected NULL for nil map, got %+v", got["Meta"])
	}
}

func TestMarshal_UintSet(t *testing.T) {
	type S struct {
		IDs []uint `dynamo:",set"`
	}
	got, err := Marshal(S{IDs: []uint{10, 20}})
	if err != nil {
		t.Fatal(err)
	}
	av := got["IDs"]
	if av.Type != TypeNS || len(av.NS) != 2 {
		t.Fatalf("expected NS with 2 elements, got %+v", av)
	}
	if av.NS[0] != "10" || av.NS[1] != "20" {
		t.Fatalf("unexpected NS values: %v", av.NS)
	}
}
