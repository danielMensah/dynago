package benchmarks

import (
	"testing"
	"time"

	"github.com/danielmensah/dynago"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/guregu/dynamo/v2"
)

// ---------------------------------------------------------------------------
// Test structs
// ---------------------------------------------------------------------------

// Small has 5 fields.
type Small struct {
	ID        string `dynamo:"PK"`
	SK        string `dynamo:"SK"`
	Name      string `dynamo:"Name"`
	Age       int    `dynamo:"Age"`
	Active    bool   `dynamo:"Active"`
}

// Medium has 15 fields.
type Medium struct {
	ID        string    `dynamo:"PK"`
	SK        string    `dynamo:"SK"`
	Name      string    `dynamo:"Name"`
	Email     string    `dynamo:"Email"`
	Age       int       `dynamo:"Age"`
	Active    bool      `dynamo:"Active"`
	Score     float64   `dynamo:"Score"`
	CreatedAt time.Time `dynamo:"CreatedAt,unixtime"`
	UpdatedAt time.Time `dynamo:"UpdatedAt,unixtime"`
	City      string    `dynamo:"City"`
	Country   string    `dynamo:"Country"`
	Phone     string    `dynamo:"Phone"`
	Status    string    `dynamo:"Status"`
	Version   int       `dynamo:"Version"`
	Rank      int       `dynamo:"Rank"`
}

// Large has 30 fields.
type Large struct {
	ID         string    `dynamo:"PK"`
	SK         string    `dynamo:"SK"`
	Name       string    `dynamo:"Name"`
	Email      string    `dynamo:"Email"`
	Age        int       `dynamo:"Age"`
	Active     bool      `dynamo:"Active"`
	Score      float64   `dynamo:"Score"`
	CreatedAt  time.Time `dynamo:"CreatedAt,unixtime"`
	UpdatedAt  time.Time `dynamo:"UpdatedAt,unixtime"`
	City       string    `dynamo:"City"`
	Country    string    `dynamo:"Country"`
	Phone      string    `dynamo:"Phone"`
	Status     string    `dynamo:"Status"`
	Version    int       `dynamo:"Version"`
	Rank       int       `dynamo:"Rank"`
	Bio        string    `dynamo:"Bio"`
	Website    string    `dynamo:"Website"`
	Company    string    `dynamo:"Company"`
	Title      string    `dynamo:"Title"`
	Salary     float64   `dynamo:"Salary"`
	Verified   bool      `dynamo:"Verified"`
	LoginCount int       `dynamo:"LoginCount"`
	LastLogin  time.Time `dynamo:"LastLogin,unixtime"`
	IPAddress  string    `dynamo:"IPAddress"`
	UserAgent  string    `dynamo:"UserAgent"`
	Referrer   string    `dynamo:"Referrer"`
	Plan       string    `dynamo:"Plan"`
	Credits    int       `dynamo:"Credits"`
	Quota      int       `dynamo:"Quota"`
	Region     string    `dynamo:"Region"`
}

// Nested has nested structs.
type Address struct {
	Street  string `dynamo:"Street"`
	City    string `dynamo:"City"`
	Country string `dynamo:"Country"`
	ZIP     string `dynamo:"ZIP"`
}

type Nested struct {
	ID      string  `dynamo:"PK"`
	Name    string  `dynamo:"Name"`
	Home    Address `dynamo:"Home"`
	Work    Address `dynamo:"Work"`
	Active  bool    `dynamo:"Active"`
}

// WithSets has set-type fields.
type WithSets struct {
	ID     string   `dynamo:"PK"`
	Tags   []string `dynamo:"Tags,set"`
	Scores []int    `dynamo:"Scores,set"`
	Name   string   `dynamo:"Name"`
}

// ---------------------------------------------------------------------------
// Sample data
// ---------------------------------------------------------------------------

var (
	smallVal = Small{ID: "user#1", SK: "profile", Name: "Alice", Age: 30, Active: true}
	medVal   = Medium{
		ID: "user#1", SK: "profile", Name: "Alice", Email: "alice@example.com",
		Age: 30, Active: true, Score: 99.5,
		CreatedAt: time.Unix(1700000000, 0), UpdatedAt: time.Unix(1700001000, 0),
		City: "London", Country: "UK", Phone: "+44123456789",
		Status: "active", Version: 3, Rank: 42,
	}
	largeVal = Large{
		ID: "user#1", SK: "profile", Name: "Alice", Email: "alice@example.com",
		Age: 30, Active: true, Score: 99.5,
		CreatedAt: time.Unix(1700000000, 0), UpdatedAt: time.Unix(1700001000, 0),
		City: "London", Country: "UK", Phone: "+44123456789",
		Status: "active", Version: 3, Rank: 42,
		Bio: "Software engineer", Website: "https://alice.dev", Company: "ACME",
		Title: "Senior Engineer", Salary: 120000.0, Verified: true, LoginCount: 500,
		LastLogin: time.Unix(1700002000, 0), IPAddress: "192.168.1.1",
		UserAgent: "Mozilla/5.0", Referrer: "google.com", Plan: "enterprise",
		Credits: 1000, Quota: 5000, Region: "eu-west-1",
	}
	nestedVal = Nested{
		ID: "user#1", Name: "Alice",
		Home: Address{Street: "123 Main St", City: "London", Country: "UK", ZIP: "SW1A 1AA"},
		Work: Address{Street: "456 Corp Ave", City: "Manchester", Country: "UK", ZIP: "M1 1AA"},
		Active: true,
	}
	setsVal = WithSets{
		ID: "user#1", Tags: []string{"admin", "beta", "vip"}, Scores: []int{100, 200, 300}, Name: "Alice",
	}
)

// ---------------------------------------------------------------------------
// DynaGo Encode Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkDynaGo_Encode_Small(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := dynago.Marshal(smallVal); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDynaGo_Encode_Medium(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := dynago.Marshal(medVal); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDynaGo_Encode_Large(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := dynago.Marshal(largeVal); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDynaGo_Encode_Nested(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := dynago.Marshal(nestedVal); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDynaGo_Encode_WithSets(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := dynago.Marshal(setsVal); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// DynaGo Decode Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkDynaGo_Decode_Small(b *testing.B) {
	item, _ := dynago.Marshal(smallVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out Small
		if err := dynago.Unmarshal(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDynaGo_Decode_Medium(b *testing.B) {
	item, _ := dynago.Marshal(medVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out Medium
		if err := dynago.Unmarshal(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDynaGo_Decode_Large(b *testing.B) {
	item, _ := dynago.Marshal(largeVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out Large
		if err := dynago.Unmarshal(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDynaGo_Decode_Nested(b *testing.B) {
	item, _ := dynago.Marshal(nestedVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out Nested
		if err := dynago.Unmarshal(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDynaGo_Decode_WithSets(b *testing.B) {
	item, _ := dynago.Marshal(setsVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out WithSets
		if err := dynago.Unmarshal(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// guregu/dynamo Encode Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkGuregu_Encode_Small(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := dynamo.MarshalItem(smallVal); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGuregu_Encode_Medium(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := dynamo.MarshalItem(medVal); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGuregu_Encode_Large(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := dynamo.MarshalItem(largeVal); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGuregu_Encode_Nested(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := dynamo.MarshalItem(nestedVal); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGuregu_Encode_WithSets(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := dynamo.MarshalItem(setsVal); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// guregu/dynamo Decode Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkGuregu_Decode_Small(b *testing.B) {
	item, _ := dynamo.MarshalItem(smallVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out Small
		if err := dynamo.UnmarshalItem(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGuregu_Decode_Medium(b *testing.B) {
	item, _ := dynamo.MarshalItem(medVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out Medium
		if err := dynamo.UnmarshalItem(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGuregu_Decode_Large(b *testing.B) {
	item, _ := dynamo.MarshalItem(largeVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out Large
		if err := dynamo.UnmarshalItem(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGuregu_Decode_Nested(b *testing.B) {
	item, _ := dynamo.MarshalItem(nestedVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out Nested
		if err := dynamo.UnmarshalItem(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGuregu_Decode_WithSets(b *testing.B) {
	item, _ := dynamo.MarshalItem(setsVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out WithSets
		if err := dynamo.UnmarshalItem(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// AWS SDK attributevalue Encode Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkAWSSDK_Encode_Small(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := attributevalue.MarshalMap(smallVal); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAWSSDK_Encode_Medium(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := attributevalue.MarshalMap(medVal); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAWSSDK_Encode_Large(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := attributevalue.MarshalMap(largeVal); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAWSSDK_Encode_Nested(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := attributevalue.MarshalMap(nestedVal); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAWSSDK_Encode_WithSets(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := attributevalue.MarshalMap(setsVal); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// AWS SDK attributevalue Decode Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkAWSSDK_Decode_Small(b *testing.B) {
	item, _ := attributevalue.MarshalMap(smallVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out Small
		if err := attributevalue.UnmarshalMap(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAWSSDK_Decode_Medium(b *testing.B) {
	item, _ := attributevalue.MarshalMap(medVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out Medium
		if err := attributevalue.UnmarshalMap(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAWSSDK_Decode_Large(b *testing.B) {
	item, _ := attributevalue.MarshalMap(largeVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out Large
		if err := attributevalue.UnmarshalMap(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAWSSDK_Decode_Nested(b *testing.B) {
	item, _ := attributevalue.MarshalMap(nestedVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out Nested
		if err := attributevalue.UnmarshalMap(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAWSSDK_Decode_WithSets(b *testing.B) {
	// AWS SDK encodes []string as L by default, not SS; use TagKey "dynamodbav" for sets.
	// For a fair comparison we use the same struct and let each library use its own encoding.
	item, _ := attributevalue.MarshalMap(setsVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out WithSets
		if err := attributevalue.UnmarshalMap(item, &out); err != nil {
			b.Fatal(err)
		}
	}
}

// Sink prevents compiler optimizations.
var sink any

func init() {
	_ = types.AttributeValueMemberS{}
}
