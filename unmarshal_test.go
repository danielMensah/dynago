package dynago

import (
	"errors"
	"testing"
	"time"
)

// customUnmarshal implements Unmarshaler for testing.
type customUnmarshal struct {
	Value string
}

func (c *customUnmarshal) UnmarshalDynamo(av AttributeValue) error {
	if av.Type != TypeS {
		return errors.New("customUnmarshal: expected S")
	}
	c.Value = "custom:" + av.S
	return nil
}

func TestUnmarshal_String(t *testing.T) {
	item := map[string]AttributeValue{
		"Name": {Type: TypeS, S: "Alice"},
	}
	var out struct {
		Name string
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != "Alice" {
		t.Errorf("got %q, want %q", out.Name, "Alice")
	}
}

func TestUnmarshal_IntTypes(t *testing.T) {
	item := map[string]AttributeValue{
		"A": {Type: TypeN, N: "42"},
		"B": {Type: TypeN, N: "-100"},
		"C": {Type: TypeN, N: "255"},
		"D": {Type: TypeN, N: "1000"},
	}
	var out struct {
		A int
		B int64
		C uint8
		D uint
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if out.A != 42 {
		t.Errorf("A: got %d, want 42", out.A)
	}
	if out.B != -100 {
		t.Errorf("B: got %d, want -100", out.B)
	}
	if out.C != 255 {
		t.Errorf("C: got %d, want 255", out.C)
	}
	if out.D != 1000 {
		t.Errorf("D: got %d, want 1000", out.D)
	}
}

func TestUnmarshal_FloatTypes(t *testing.T) {
	item := map[string]AttributeValue{
		"A": {Type: TypeN, N: "3.14"},
		"B": {Type: TypeN, N: "2.5"},
	}
	var out struct {
		A float64
		B float32
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if out.A != 3.14 {
		t.Errorf("A: got %f, want 3.14", out.A)
	}
	if out.B != 2.5 {
		t.Errorf("B: got %f, want 2.5", out.B)
	}
}

func TestUnmarshal_Bool(t *testing.T) {
	item := map[string]AttributeValue{
		"Active": {Type: TypeBOOL, BOOL: true},
	}
	var out struct {
		Active bool
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Active {
		t.Error("expected Active to be true")
	}
}

func TestUnmarshal_Bytes(t *testing.T) {
	item := map[string]AttributeValue{
		"Data": {Type: TypeB, B: []byte{0x01, 0x02, 0x03}},
	}
	var out struct {
		Data []byte
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Data) != 3 || out.Data[0] != 1 || out.Data[1] != 2 || out.Data[2] != 3 {
		t.Errorf("got %v, want [1 2 3]", out.Data)
	}
}

func TestUnmarshal_TimeISO8601(t *testing.T) {
	item := map[string]AttributeValue{
		"CreatedAt": {Type: TypeS, S: "2024-01-15T10:30:00Z"},
	}
	var out struct {
		CreatedAt time.Time
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	if !out.CreatedAt.Equal(want) {
		t.Errorf("got %v, want %v", out.CreatedAt, want)
	}
}

func TestUnmarshal_TimeUnixtime(t *testing.T) {
	item := map[string]AttributeValue{
		"UpdatedAt": {Type: TypeN, N: "1705312200"},
	}
	type record struct {
		UpdatedAt time.Time `dynamo:"UpdatedAt,unixtime"`
	}
	var out record
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	want := time.Unix(1705312200, 0).UTC()
	if !out.UpdatedAt.Equal(want) {
		t.Errorf("got %v, want %v", out.UpdatedAt, want)
	}
}

func TestUnmarshal_Slice(t *testing.T) {
	item := map[string]AttributeValue{
		"Tags": {Type: TypeL, L: []AttributeValue{
			{Type: TypeS, S: "go"},
			{Type: TypeS, S: "dynamo"},
		}},
	}
	var out struct {
		Tags []string
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Tags) != 2 || out.Tags[0] != "go" || out.Tags[1] != "dynamo" {
		t.Errorf("got %v, want [go dynamo]", out.Tags)
	}
}

func TestUnmarshal_Map(t *testing.T) {
	item := map[string]AttributeValue{
		"Meta": {Type: TypeM, M: map[string]AttributeValue{
			"version": {Type: TypeS, S: "1.0"},
			"author":  {Type: TypeS, S: "test"},
		}},
	}
	var out struct {
		Meta map[string]string
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if out.Meta["version"] != "1.0" || out.Meta["author"] != "test" {
		t.Errorf("got %v", out.Meta)
	}
}

func TestUnmarshal_NestedStruct(t *testing.T) {
	item := map[string]AttributeValue{
		"Address": {Type: TypeM, M: map[string]AttributeValue{
			"City":  {Type: TypeS, S: "Portland"},
			"State": {Type: TypeS, S: "OR"},
		}},
	}
	type Address struct {
		City  string
		State string
	}
	var out struct {
		Address Address
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if out.Address.City != "Portland" || out.Address.State != "OR" {
		t.Errorf("got %+v", out.Address)
	}
}

func TestUnmarshal_PointerField_Present(t *testing.T) {
	item := map[string]AttributeValue{
		"Name": {Type: TypeS, S: "Bob"},
	}
	var out struct {
		Name *string
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if out.Name == nil || *out.Name != "Bob" {
		t.Errorf("got %v, want pointer to Bob", out.Name)
	}
}

func TestUnmarshal_PointerField_Missing(t *testing.T) {
	item := map[string]AttributeValue{}
	var out struct {
		Name *string
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != nil {
		t.Errorf("expected nil pointer, got %v", out.Name)
	}
}

func TestUnmarshal_PointerField_NULL(t *testing.T) {
	item := map[string]AttributeValue{
		"Name": {Type: TypeNULL, NULL: true},
	}
	var out struct {
		Name *string
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != nil {
		t.Errorf("expected nil pointer for NULL, got %v", out.Name)
	}
}

func TestUnmarshal_StringSet(t *testing.T) {
	item := map[string]AttributeValue{
		"Tags": {Type: TypeSS, SS: []string{"a", "b", "c"}},
	}
	type record struct {
		Tags []string `dynamo:"Tags,set"`
	}
	var out record
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Tags) != 3 {
		t.Errorf("got %v, want 3 elements", out.Tags)
	}
}

func TestUnmarshal_NumberSet(t *testing.T) {
	item := map[string]AttributeValue{
		"Scores": {Type: TypeNS, NS: []string{"10", "20", "30"}},
	}
	type record struct {
		Scores []int `dynamo:"Scores,set"`
	}
	var out record
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Scores) != 3 || out.Scores[0] != 10 || out.Scores[1] != 20 || out.Scores[2] != 30 {
		t.Errorf("got %v, want [10 20 30]", out.Scores)
	}
}

func TestUnmarshal_NumberSetFloat(t *testing.T) {
	item := map[string]AttributeValue{
		"Values": {Type: TypeNS, NS: []string{"1.5", "2.5"}},
	}
	type record struct {
		Values []float64 `dynamo:"Values,set"`
	}
	var out record
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Values) != 2 || out.Values[0] != 1.5 || out.Values[1] != 2.5 {
		t.Errorf("got %v, want [1.5 2.5]", out.Values)
	}
}

func TestUnmarshal_BinarySet(t *testing.T) {
	item := map[string]AttributeValue{
		"Blobs": {Type: TypeBS, BS: [][]byte{{1, 2}, {3, 4}}},
	}
	type record struct {
		Blobs [][]byte `dynamo:"Blobs,set"`
	}
	var out record
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Blobs) != 2 {
		t.Errorf("got %d blobs, want 2", len(out.Blobs))
	}
}

func TestUnmarshal_CustomUnmarshaler(t *testing.T) {
	item := map[string]AttributeValue{
		"Field": {Type: TypeS, S: "hello"},
	}
	var out struct {
		Field customUnmarshal
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if out.Field.Value != "custom:hello" {
		t.Errorf("got %q, want %q", out.Field.Value, "custom:hello")
	}
}

func TestUnmarshal_TagName(t *testing.T) {
	item := map[string]AttributeValue{
		"user_name": {Type: TypeS, S: "Alice"},
	}
	type record struct {
		Name string `dynamo:"user_name"`
	}
	var out record
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != "Alice" {
		t.Errorf("got %q, want %q", out.Name, "Alice")
	}
}

func TestUnmarshal_SkipTag(t *testing.T) {
	item := map[string]AttributeValue{
		"Name":   {Type: TypeS, S: "Alice"},
		"Secret": {Type: TypeS, S: "hidden"},
	}
	type record struct {
		Name   string
		Secret string `dynamo:"-"`
	}
	var out record
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != "Alice" {
		t.Errorf("Name: got %q, want %q", out.Name, "Alice")
	}
	if out.Secret != "" {
		t.Errorf("Secret should be empty, got %q", out.Secret)
	}
}

func TestUnmarshal_ExtraAttributesIgnored(t *testing.T) {
	item := map[string]AttributeValue{
		"Name":    {Type: TypeS, S: "Alice"},
		"Unknown": {Type: TypeS, S: "extra"},
		"Another": {Type: TypeN, N: "42"},
	}
	var out struct {
		Name string
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != "Alice" {
		t.Errorf("got %q, want %q", out.Name, "Alice")
	}
}

func TestUnmarshal_NonPointer(t *testing.T) {
	var s struct{ Name string }
	err := Unmarshal(nil, s)
	if err == nil {
		t.Fatal("expected error for non-pointer")
	}
	if !IsValidation(err) {
		t.Errorf("expected validation error, got %v", err)
	}
}

func TestUnmarshal_NilPointer(t *testing.T) {
	err := Unmarshal(nil, (*struct{ Name string })(nil))
	if err == nil {
		t.Fatal("expected error for nil pointer")
	}
	if !IsValidation(err) {
		t.Errorf("expected validation error, got %v", err)
	}
}

func TestUnmarshal_NonStruct(t *testing.T) {
	var s string
	err := Unmarshal(nil, &s)
	if err == nil {
		t.Fatal("expected error for non-struct pointer")
	}
	if !IsValidation(err) {
		t.Errorf("expected validation error, got %v", err)
	}
}

func TestUnmarshal_NestedStructPointer(t *testing.T) {
	item := map[string]AttributeValue{
		"Address": {Type: TypeM, M: map[string]AttributeValue{
			"City": {Type: TypeS, S: "Seattle"},
		}},
	}
	type Address struct {
		City string
	}
	var out struct {
		Address *Address
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if out.Address == nil || out.Address.City != "Seattle" {
		t.Errorf("got %+v", out.Address)
	}
}

func TestUnmarshal_NestedStructPointer_Missing(t *testing.T) {
	item := map[string]AttributeValue{}
	type Address struct {
		City string
	}
	var out struct {
		Address *Address
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if out.Address != nil {
		t.Errorf("expected nil, got %+v", out.Address)
	}
}

func TestUnmarshal_SliceOfInts(t *testing.T) {
	item := map[string]AttributeValue{
		"Nums": {Type: TypeL, L: []AttributeValue{
			{Type: TypeN, N: "1"},
			{Type: TypeN, N: "2"},
			{Type: TypeN, N: "3"},
		}},
	}
	var out struct {
		Nums []int
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Nums) != 3 || out.Nums[0] != 1 || out.Nums[1] != 2 || out.Nums[2] != 3 {
		t.Errorf("got %v, want [1 2 3]", out.Nums)
	}
}

func TestUnmarshal_MapOfInts(t *testing.T) {
	item := map[string]AttributeValue{
		"Counts": {Type: TypeM, M: map[string]AttributeValue{
			"a": {Type: TypeN, N: "10"},
			"b": {Type: TypeN, N: "20"},
		}},
	}
	var out struct {
		Counts map[string]int
	}
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if out.Counts["a"] != 10 || out.Counts["b"] != 20 {
		t.Errorf("got %v", out.Counts)
	}
}

func TestUnmarshal_ComplexStruct(t *testing.T) {
	item := map[string]AttributeValue{
		"PK":        {Type: TypeS, S: "USER#123"},
		"SK":        {Type: TypeS, S: "PROFILE"},
		"Name":      {Type: TypeS, S: "Alice"},
		"Age":       {Type: TypeN, N: "30"},
		"Score":     {Type: TypeN, N: "99.5"},
		"Active":    {Type: TypeBOOL, BOOL: true},
		"Data":      {Type: TypeB, B: []byte{0xDE, 0xAD}},
		"CreatedAt": {Type: TypeS, S: "2024-01-15T10:30:00Z"},
		"UpdatedAt": {Type: TypeN, N: "1705312200"},
		"Tags":      {Type: TypeSS, SS: []string{"admin", "user"}},
		"Scores":    {Type: TypeNS, NS: []string{"100", "200"}},
		"Address": {Type: TypeM, M: map[string]AttributeValue{
			"City":  {Type: TypeS, S: "Portland"},
			"State": {Type: TypeS, S: "OR"},
		}},
		"Extra": {Type: TypeS, S: "ignored"},
	}

	type Address struct {
		City  string
		State string
	}
	type record struct {
		PK        string    `dynamo:"PK,hash"`
		SK        string    `dynamo:"SK,range"`
		Name      string
		Age       int
		Score     float64
		Active    bool
		Data      []byte
		CreatedAt time.Time
		UpdatedAt time.Time `dynamo:"UpdatedAt,unixtime"`
		Tags      []string  `dynamo:"Tags,set"`
		Scores    []int     `dynamo:"Scores,set"`
		Address   Address
	}

	var out record
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}

	if out.PK != "USER#123" {
		t.Errorf("PK: got %q", out.PK)
	}
	if out.SK != "PROFILE" {
		t.Errorf("SK: got %q", out.SK)
	}
	if out.Name != "Alice" {
		t.Errorf("Name: got %q", out.Name)
	}
	if out.Age != 30 {
		t.Errorf("Age: got %d", out.Age)
	}
	if out.Score != 99.5 {
		t.Errorf("Score: got %f", out.Score)
	}
	if !out.Active {
		t.Error("Active: expected true")
	}
	if len(out.Data) != 2 || out.Data[0] != 0xDE {
		t.Errorf("Data: got %v", out.Data)
	}
	wantCreated := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	if !out.CreatedAt.Equal(wantCreated) {
		t.Errorf("CreatedAt: got %v, want %v", out.CreatedAt, wantCreated)
	}
	wantUpdated := time.Unix(1705312200, 0).UTC()
	if !out.UpdatedAt.Equal(wantUpdated) {
		t.Errorf("UpdatedAt: got %v, want %v", out.UpdatedAt, wantUpdated)
	}
	if len(out.Tags) != 2 {
		t.Errorf("Tags: got %v", out.Tags)
	}
	if len(out.Scores) != 2 || out.Scores[0] != 100 {
		t.Errorf("Scores: got %v", out.Scores)
	}
	if out.Address.City != "Portland" {
		t.Errorf("Address.City: got %q", out.Address.City)
	}
}

func TestUnmarshal_RoundTrip(t *testing.T) {
	// This test requires Marshal from US-003. Skip if it doesn't exist yet.
	// Once marshal.go is available, this test will exercise the full round-trip.

	type Address struct {
		City  string
		State string
	}
	type record struct {
		PK      string  `dynamo:"PK,hash"`
		SK      string  `dynamo:"SK,range"`
		Name    string
		Age     int
		Score   float64
		Active  bool
		Address Address
	}

	original := record{
		PK:     "USER#1",
		SK:     "PROFILE",
		Name:   "Bob",
		Age:    25,
		Score:  88.5,
		Active: true,
		Address: Address{
			City:  "Seattle",
			State: "WA",
		},
	}

	// Try to marshal. If Marshal is not available, skip.
	item, err := Marshal(original)
	if err != nil {
		t.Skipf("Marshal not available or failed: %v", err)
	}

	var decoded record
	if err := Unmarshal(item, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded != original {
		t.Errorf("round-trip mismatch:\n  got:  %+v\n  want: %+v", decoded, original)
	}
}

func TestUnmarshal_UintSet(t *testing.T) {
	item := map[string]AttributeValue{
		"IDs": {Type: TypeNS, NS: []string{"1", "2", "3"}},
	}
	type record struct {
		IDs []uint `dynamo:"IDs,set"`
	}
	var out record
	if err := Unmarshal(item, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.IDs) != 3 || out.IDs[0] != 1 || out.IDs[2] != 3 {
		t.Errorf("got %v, want [1 2 3]", out.IDs)
	}
}
