package dynago

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test struct types for encoding/decoding benchmarks
// ---------------------------------------------------------------------------

// flatStruct has 5 common field types.
type flatStruct struct {
	Name      string    `dynamo:"name"`
	Age       int       `dynamo:"age"`
	Active    bool      `dynamo:"active"`
	CreatedAt time.Time `dynamo:"created_at"`
	Avatar    []byte    `dynamo:"avatar"`
}

// address is a nested struct used by nestedStruct.
type address struct {
	Street string `dynamo:"street"`
	City   string `dynamo:"city"`
	Zip    string `dynamo:"zip"`
}

// nestedStruct has nested structs, slices, and maps.
type nestedStruct struct {
	ID       string            `dynamo:"id"`
	Name     string            `dynamo:"name"`
	Address  address           `dynamo:"address"`
	Tags     []string          `dynamo:"tags"`
	Metadata map[string]string `dynamo:"metadata"`
	Scores   []int             `dynamo:"scores"`
}

// largeStruct has 30+ fields of various types.
type largeStruct struct {
	F01 string  `dynamo:"f01"`
	F02 string  `dynamo:"f02"`
	F03 string  `dynamo:"f03"`
	F04 string  `dynamo:"f04"`
	F05 string  `dynamo:"f05"`
	F06 int     `dynamo:"f06"`
	F07 int     `dynamo:"f07"`
	F08 int     `dynamo:"f08"`
	F09 int     `dynamo:"f09"`
	F10 int     `dynamo:"f10"`
	F11 bool    `dynamo:"f11"`
	F12 bool    `dynamo:"f12"`
	F13 bool    `dynamo:"f13"`
	F14 float64 `dynamo:"f14"`
	F15 float64 `dynamo:"f15"`
	F16 string  `dynamo:"f16"`
	F17 string  `dynamo:"f17"`
	F18 string  `dynamo:"f18"`
	F19 string  `dynamo:"f19"`
	F20 string  `dynamo:"f20"`
	F21 int     `dynamo:"f21"`
	F22 int     `dynamo:"f22"`
	F23 int     `dynamo:"f23"`
	F24 int     `dynamo:"f24"`
	F25 int     `dynamo:"f25"`
	F26 bool    `dynamo:"f26"`
	F27 bool    `dynamo:"f27"`
	F28 float64 `dynamo:"f28"`
	F29 float64 `dynamo:"f29"`
	F30 string  `dynamo:"f30"`
	F31 string  `dynamo:"f31"`
	F32 int64   `dynamo:"f32"`
}

// setStruct has string sets and number sets.
type setStruct struct {
	ID      string   `dynamo:"id"`
	Tags    []string `dynamo:"tags,set"`
	Scores  []int    `dynamo:"scores,set"`
	Ratings []int    `dynamo:"ratings,set"`
}

// coldCache types: multiple distinct struct types for cache-miss benchmarking.
type cold1 struct {
	A string `dynamo:"a"`
	B int    `dynamo:"b"`
}
type cold2 struct {
	X string `dynamo:"x"`
	Y bool   `dynamo:"y"`
}
type cold3 struct {
	P float64 `dynamo:"p"`
	Q string  `dynamo:"q"`
}
type cold4 struct {
	M string `dynamo:"m"`
	N int    `dynamo:"n"`
}
type cold5 struct {
	R bool   `dynamo:"r"`
	S string `dynamo:"s"`
}
type cold6 struct {
	U int    `dynamo:"u"`
	V string `dynamo:"v"`
}
type cold7 struct {
	W string  `dynamo:"w"`
	Z float64 `dynamo:"z"`
}
type cold8 struct {
	AA string `dynamo:"aa"`
	BB int    `dynamo:"bb"`
}

// ---------------------------------------------------------------------------
// Sample data constructors
// ---------------------------------------------------------------------------

func newFlatStruct() flatStruct {
	return flatStruct{
		Name:      "Alice",
		Age:       30,
		Active:    true,
		CreatedAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Avatar:    []byte("avatar-bytes-here"),
	}
}

func newNestedStruct() nestedStruct {
	return nestedStruct{
		ID:   "user#123",
		Name: "Bob",
		Address: address{
			Street: "123 Main St",
			City:   "Springfield",
			Zip:    "62704",
		},
		Tags:     []string{"admin", "premium", "verified"},
		Metadata: map[string]string{"role": "admin", "dept": "eng", "team": "backend"},
		Scores:   []int{100, 95, 88, 72, 99},
	}
}

func newLargeStruct() largeStruct {
	return largeStruct{
		F01: "alpha", F02: "bravo", F03: "charlie", F04: "delta", F05: "echo",
		F06: 1, F07: 2, F08: 3, F09: 4, F10: 5,
		F11: true, F12: false, F13: true,
		F14: 3.14, F15: 2.71,
		F16: "foxtrot", F17: "golf", F18: "hotel", F19: "india", F20: "juliet",
		F21: 6, F22: 7, F23: 8, F24: 9, F25: 10,
		F26: false, F27: true,
		F28: 1.618, F29: 0.577,
		F30: "kilo", F31: "lima", F32: 42,
	}
}

func newSetStruct() setStruct {
	return setStruct{
		ID:      "item#456",
		Tags:    []string{"go", "dynamodb", "serverless", "cloud"},
		Scores:  []int{100, 200, 300, 400, 500},
		Ratings: []int{1, 2, 3, 4, 5},
	}
}

// ---------------------------------------------------------------------------
// US-020: Encoding Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkEncode_Flat(b *testing.B) {
	b.ReportAllocs()
	v := newFlatStruct()
	for b.Loop() {
		_, err := Marshal(v)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncode_Nested(b *testing.B) {
	b.ReportAllocs()
	v := newNestedStruct()
	for b.Loop() {
		_, err := Marshal(v)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncode_Large(b *testing.B) {
	b.ReportAllocs()
	v := newLargeStruct()
	for b.Loop() {
		_, err := Marshal(v)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncode_WithSets(b *testing.B) {
	b.ReportAllocs()
	v := newSetStruct()
	for b.Loop() {
		_, err := Marshal(v)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncode_ColdCache(b *testing.B) {
	b.ReportAllocs()
	// Use multiple pre-defined types and cycle through them.
	// Clear the codec cache before each iteration to simulate cold cache.
	items := []any{
		cold1{A: "a", B: 1},
		cold2{X: "x", Y: true},
		cold3{P: 3.14, Q: "q"},
		cold4{M: "m", N: 42},
		cold5{R: true, S: "s"},
		cold6{U: 7, V: "v"},
		cold7{W: "w", Z: 2.71},
		cold8{AA: "aa", BB: 99},
	}
	n := len(items)
	for i := 0; b.Loop(); i++ {
		// Clear codec cache to force re-building.
		codecCache.Range(func(key, _ any) bool {
			codecCache.Delete(key)
			return true
		})
		_, err := Marshal(items[i%n])
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncode_WarmCache(b *testing.B) {
	b.ReportAllocs()
	v := newFlatStruct()
	// Warm up the cache first.
	_, _ = Marshal(v)
	for b.Loop() {
		_, err := Marshal(v)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// US-020: Decoding Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkDecode_Flat(b *testing.B) {
	b.ReportAllocs()
	item, _ := Marshal(newFlatStruct())
	for b.Loop() {
		var out flatStruct
		if err := Unmarshal(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecode_Nested(b *testing.B) {
	b.ReportAllocs()
	item, _ := Marshal(newNestedStruct())
	for b.Loop() {
		var out nestedStruct
		if err := Unmarshal(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecode_Large(b *testing.B) {
	b.ReportAllocs()
	item, _ := Marshal(newLargeStruct())
	for b.Loop() {
		var out largeStruct
		if err := Unmarshal(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Noop backend for round-trip benchmarks
// ---------------------------------------------------------------------------

type noopBackend struct {
	getResp   *GetItemResponse
	putResp   *PutItemResponse
	queryResp *QueryResponse
}

func (n *noopBackend) GetItem(_ context.Context, _ *GetItemRequest) (*GetItemResponse, error) {
	return n.getResp, nil
}
func (n *noopBackend) PutItem(_ context.Context, _ *PutItemRequest) (*PutItemResponse, error) {
	return n.putResp, nil
}
func (n *noopBackend) DeleteItem(_ context.Context, _ *DeleteItemRequest) (*DeleteItemResponse, error) {
	return &DeleteItemResponse{}, nil
}
func (n *noopBackend) UpdateItem(_ context.Context, _ *UpdateItemRequest) (*UpdateItemResponse, error) {
	return &UpdateItemResponse{}, nil
}
func (n *noopBackend) Query(_ context.Context, _ *QueryRequest) (*QueryResponse, error) {
	return n.queryResp, nil
}
func (n *noopBackend) Scan(_ context.Context, _ *ScanRequest) (*ScanResponse, error) {
	return &ScanResponse{}, nil
}
func (n *noopBackend) BatchGetItem(_ context.Context, _ *BatchGetItemRequest) (*BatchGetItemResponse, error) {
	return &BatchGetItemResponse{}, nil
}
func (n *noopBackend) BatchWriteItem(_ context.Context, _ *BatchWriteItemRequest) (*BatchWriteItemResponse, error) {
	return &BatchWriteItemResponse{}, nil
}
func (n *noopBackend) TransactGetItems(_ context.Context, _ *TransactGetItemsRequest) (*TransactGetItemsResponse, error) {
	return &TransactGetItemsResponse{}, nil
}
func (n *noopBackend) TransactWriteItems(_ context.Context, _ *TransactWriteItemsRequest) (*TransactWriteItemsResponse, error) {
	return &TransactWriteItemsResponse{}, nil
}

// ---------------------------------------------------------------------------
// US-021: Expression Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkExprBuild_SimpleFilter(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, err := buildCondition("Active = ? AND Age > ?", true, 21)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExprBuild_ComplexCondition(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, err := buildCondition(
			"Status = ? AND Age > ? AND NOT Active = ? AND contains(Name, ?) AND Score >= ? AND attribute_exists(Email)",
			"active", 21, false, "test", 100,
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkExprTranslate_ToDynamo lives in internal/expr/bench_test.go
// to avoid an import cycle (internal/expr imports dynago).

// ---------------------------------------------------------------------------
// US-021: Round-trip Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkGetRoundTrip(b *testing.B) {
	b.ReportAllocs()
	// Pre-build response item.
	item, _ := Marshal(newFlatStruct())
	backend := &noopBackend{
		getResp: &GetItemResponse{Item: item},
	}
	db := New(backend)
	table := db.Table("TestTable")
	key := Key("PK", "user#123")
	ctx := context.Background()

	for b.Loop() {
		_, err := Get[flatStruct](ctx, table, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPutRoundTrip(b *testing.B) {
	b.ReportAllocs()
	backend := &noopBackend{
		putResp: &PutItemResponse{},
	}
	db := New(backend)
	table := db.Table("TestTable")
	item := newFlatStruct()
	ctx := context.Background()

	for b.Loop() {
		if err := table.Put(ctx, item); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkQueryRoundTrip(b *testing.B) {
	b.ReportAllocs()
	// Pre-build 100 response items.
	items := make([]map[string]AttributeValue, 100)
	for i := 0; i < 100; i++ {
		f := flatStruct{
			Name:      "User" + strconv.Itoa(i),
			Age:       20 + i,
			Active:    i%2 == 0,
			CreatedAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			Avatar:    []byte(fmt.Sprintf("avatar-%d", i)),
		}
		items[i], _ = Marshal(f)
	}
	backend := &noopBackend{
		queryResp: &QueryResponse{
			Items: items,
			Count: 100,
		},
	}
	db := New(backend)
	table := db.Table("TestTable")
	kc := Partition("PK", "user#123")
	ctx := context.Background()

	for b.Loop() {
		_, err := Query[flatStruct](ctx, table, kc)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// US-021: Key Construction Benchmark
// ---------------------------------------------------------------------------

func BenchmarkKeyConstruction(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		Key("PK", "user#123", "SK", "order#2024-01-15")
	}
}

func BenchmarkKey_HashOnly_String(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		Key("PK", "user#123")
	}
}

func BenchmarkKey_HashOnly_Int(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		Key("ID", 42)
	}
}

func BenchmarkKey_Mixed_StringInt(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		Key("PK", "user#123", "Version", 7)
	}
}

func BenchmarkStringKey(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		StringKey("PK", "user#123")
	}
}

func BenchmarkStringPairKey(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		StringPairKey("PK", "user#123", "SK", "order#2024-01-15")
	}
}

func BenchmarkKey_WithMap(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		kv := Key("PK", "user#123", "SK", "order#2024-01-15")
		_ = kv.Map()
	}
}

func BenchmarkStringPairKey_WithMap(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		kv := StringPairKey("PK", "user#123", "SK", "order#2024-01-15")
		_ = kv.Map()
	}
}
