package dynago

import (
	"reflect"
	"testing"
)

func TestParseTag_DefaultFieldName(t *testing.T) {
	type Item struct {
		Name string
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	if len(codec.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(codec.Fields))
	}
	if codec.Fields[0].Options.Name != "Name" {
		t.Errorf("expected Name, got %q", codec.Fields[0].Options.Name)
	}
}

func TestParseTag_CustomName(t *testing.T) {
	type Item struct {
		UserID string `dynamo:"PK"`
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	if codec.Fields[0].Options.Name != "PK" {
		t.Errorf("expected PK, got %q", codec.Fields[0].Options.Name)
	}
}

func TestParseTag_Skip(t *testing.T) {
	type Item struct {
		Secret string `dynamo:"-"`
		Name   string
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	if len(codec.Fields) != 1 {
		t.Fatalf("expected 1 field (Secret skipped), got %d", len(codec.Fields))
	}
	if codec.Fields[0].Options.Name != "Name" {
		t.Errorf("expected Name, got %q", codec.Fields[0].Options.Name)
	}
}

func TestParseTag_UnexportedSkipped(t *testing.T) {
	type Item struct {
		Name   string
		secret string //nolint:unused
	}
	_ = Item{secret: ""}
	codec := getCodec(reflect.TypeOf(Item{}))
	if len(codec.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(codec.Fields))
	}
}

func TestParseTag_OmitEmpty(t *testing.T) {
	type Item struct {
		Name string `dynamo:"name,omitempty"`
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	f := codec.Fields[0]
	if f.Options.Name != "name" {
		t.Errorf("expected name, got %q", f.Options.Name)
	}
	if !f.Options.OmitEmpty {
		t.Error("expected OmitEmpty to be true")
	}
}

func TestParseTag_Hash(t *testing.T) {
	type Item struct {
		PK string `dynamo:"PK,hash"`
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	f := codec.Fields[0]
	if !f.Options.Hash {
		t.Error("expected Hash to be true")
	}
	if f.Options.Name != "PK" {
		t.Errorf("expected PK, got %q", f.Options.Name)
	}
}

func TestParseTag_Range(t *testing.T) {
	type Item struct {
		SK string `dynamo:"SK,range"`
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	if !codec.Fields[0].Options.Range {
		t.Error("expected Range to be true")
	}
}

func TestParseTag_GSI(t *testing.T) {
	type Item struct {
		GSI1PK string `dynamo:"GSI1PK,gsi:GSI1,hash"`
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	f := codec.Fields[0]
	if f.Options.GSI != "GSI1" {
		t.Errorf("expected GSI1, got %q", f.Options.GSI)
	}
	if !f.Options.Hash {
		t.Error("expected Hash to be true")
	}
	if f.Options.Name != "GSI1PK" {
		t.Errorf("expected GSI1PK, got %q", f.Options.Name)
	}
}

func TestParseTag_Set(t *testing.T) {
	type Item struct {
		Tags []string `dynamo:"Tags,set"`
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	if !codec.Fields[0].Options.Set {
		t.Error("expected Set to be true")
	}
}

func TestParseTag_UnixTime(t *testing.T) {
	type Item struct {
		CreatedAt int64 `dynamo:"CreatedAt,unixtime"`
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	if !codec.Fields[0].Options.UnixTime {
		t.Error("expected UnixTime to be true")
	}
}

func TestParseTag_AllOptions(t *testing.T) {
	type Item struct {
		PK string `dynamo:"GSI1PK,gsi:GSI1,hash,omitempty"`
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	f := codec.Fields[0]
	if f.Options.Name != "GSI1PK" {
		t.Errorf("name: got %q, want GSI1PK", f.Options.Name)
	}
	if f.Options.GSI != "GSI1" {
		t.Errorf("gsi: got %q, want GSI1", f.Options.GSI)
	}
	if !f.Options.Hash {
		t.Error("expected Hash")
	}
	if !f.Options.OmitEmpty {
		t.Error("expected OmitEmpty")
	}
}

func TestParseTag_OptionsOnlyNoName(t *testing.T) {
	// When all parts are known options and no explicit name, default to field name.
	type Item struct {
		Status string `dynamo:",omitempty"`
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	f := codec.Fields[0]
	if f.Options.Name != "Status" {
		t.Errorf("expected Status, got %q", f.Options.Name)
	}
	if !f.Options.OmitEmpty {
		t.Error("expected OmitEmpty")
	}
}

func TestParseTag_HashAsFirstOption(t *testing.T) {
	// "hash" as first token is a known option, so name should default.
	type Item struct {
		PK string `dynamo:"hash"`
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	f := codec.Fields[0]
	if f.Options.Name != "PK" {
		t.Errorf("expected PK, got %q", f.Options.Name)
	}
	if !f.Options.Hash {
		t.Error("expected Hash")
	}
}

func TestParseTag_CombinedHashRange(t *testing.T) {
	// A field can be tagged with both hash and range (unusual but parsed).
	type Item struct {
		ID string `dynamo:"id,hash,range"`
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	f := codec.Fields[0]
	if !f.Options.Hash || !f.Options.Range {
		t.Error("expected both Hash and Range")
	}
}

func TestParseTag_MultipleFields(t *testing.T) {
	type Item struct {
		PK   string `dynamo:"PK,hash"`
		SK   string `dynamo:"SK,range"`
		Data string `dynamo:"data,omitempty"`
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	if len(codec.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(codec.Fields))
	}
	if !codec.Fields[0].Options.Hash {
		t.Error("PK should be hash")
	}
	if !codec.Fields[1].Options.Range {
		t.Error("SK should be range")
	}
	if !codec.Fields[2].Options.OmitEmpty {
		t.Error("Data should be omitempty")
	}
}

func TestGetCodec_Cached(t *testing.T) {
	type CacheTest struct {
		A string
	}
	typ := reflect.TypeOf(CacheTest{})
	c1 := getCodec(typ)
	c2 := getCodec(typ)
	if c1 != c2 {
		t.Error("expected same pointer from cache")
	}
}

func TestParseTag_GSIWithRange(t *testing.T) {
	type Item struct {
		GSI1SK string `dynamo:"GSI1SK,gsi:GSI1,range"`
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	f := codec.Fields[0]
	if f.Options.GSI != "GSI1" {
		t.Errorf("expected GSI1, got %q", f.Options.GSI)
	}
	if !f.Options.Range {
		t.Error("expected Range to be true")
	}
}

func TestParseTag_SetAndOmitEmpty(t *testing.T) {
	type Item struct {
		Tags []string `dynamo:"tags,set,omitempty"`
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	f := codec.Fields[0]
	if !f.Options.Set {
		t.Error("expected Set")
	}
	if !f.Options.OmitEmpty {
		t.Error("expected OmitEmpty")
	}
	if f.Options.Name != "tags" {
		t.Errorf("expected tags, got %q", f.Options.Name)
	}
}

func TestParseTag_EmptyTag(t *testing.T) {
	type Item struct {
		Name string `dynamo:""`
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	f := codec.Fields[0]
	if f.Options.Name != "Name" {
		t.Errorf("expected Name, got %q", f.Options.Name)
	}
}

func TestParseTag_NoTag(t *testing.T) {
	type Item struct {
		Age int
	}
	codec := getCodec(reflect.TypeOf(Item{}))
	f := codec.Fields[0]
	if f.Options.Name != "Age" {
		t.Errorf("expected Age, got %q", f.Options.Name)
	}
}
