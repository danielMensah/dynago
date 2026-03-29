package awsbackend

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/danielmensah/dynago"
)

func TestToAWSAttributeValue_AllTypes(t *testing.T) {
	tests := []struct {
		name string
		av   dynago.AttributeValue
		want types.AttributeValue
	}{
		{
			name: "string",
			av:   dynago.AttributeValue{Type: dynago.TypeS, S: "hello"},
			want: &types.AttributeValueMemberS{Value: "hello"},
		},
		{
			name: "number",
			av:   dynago.AttributeValue{Type: dynago.TypeN, N: "42"},
			want: &types.AttributeValueMemberN{Value: "42"},
		},
		{
			name: "binary",
			av:   dynago.AttributeValue{Type: dynago.TypeB, B: []byte{0x01, 0x02}},
			want: &types.AttributeValueMemberB{Value: []byte{0x01, 0x02}},
		},
		{
			name: "bool true",
			av:   dynago.AttributeValue{Type: dynago.TypeBOOL, BOOL: true},
			want: &types.AttributeValueMemberBOOL{Value: true},
		},
		{
			name: "bool false",
			av:   dynago.AttributeValue{Type: dynago.TypeBOOL, BOOL: false},
			want: &types.AttributeValueMemberBOOL{Value: false},
		},
		{
			name: "null",
			av:   dynago.AttributeValue{Type: dynago.TypeNULL, NULL: true},
			want: &types.AttributeValueMemberNULL{Value: true},
		},
		{
			name: "string set",
			av:   dynago.AttributeValue{Type: dynago.TypeSS, SS: []string{"a", "b"}},
			want: &types.AttributeValueMemberSS{Value: []string{"a", "b"}},
		},
		{
			name: "number set",
			av:   dynago.AttributeValue{Type: dynago.TypeNS, NS: []string{"1", "2"}},
			want: &types.AttributeValueMemberNS{Value: []string{"1", "2"}},
		},
		{
			name: "binary set",
			av:   dynago.AttributeValue{Type: dynago.TypeBS, BS: [][]byte{{0x01}, {0x02}}},
			want: &types.AttributeValueMemberBS{Value: [][]byte{{0x01}, {0x02}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toAWSAttributeValue(tt.av)
			assertAWSAVEqual(t, tt.want, got)
		})
	}
}

func TestToAWSAttributeValue_List(t *testing.T) {
	av := dynago.AttributeValue{
		Type: dynago.TypeL,
		L: []dynago.AttributeValue{
			{Type: dynago.TypeS, S: "hello"},
			{Type: dynago.TypeN, N: "42"},
		},
	}
	got := toAWSAttributeValue(av)
	l, ok := got.(*types.AttributeValueMemberL)
	if !ok {
		t.Fatalf("expected *types.AttributeValueMemberL, got %T", got)
	}
	if len(l.Value) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(l.Value))
	}
	if s, ok := l.Value[0].(*types.AttributeValueMemberS); !ok || s.Value != "hello" {
		t.Errorf("expected S=hello, got %v", l.Value[0])
	}
	if n, ok := l.Value[1].(*types.AttributeValueMemberN); !ok || n.Value != "42" {
		t.Errorf("expected N=42, got %v", l.Value[1])
	}
}

func TestToAWSAttributeValue_Map(t *testing.T) {
	av := dynago.AttributeValue{
		Type: dynago.TypeM,
		M: map[string]dynago.AttributeValue{
			"name": {Type: dynago.TypeS, S: "Alice"},
			"age":  {Type: dynago.TypeN, N: "30"},
		},
	}
	got := toAWSAttributeValue(av)
	m, ok := got.(*types.AttributeValueMemberM)
	if !ok {
		t.Fatalf("expected *types.AttributeValueMemberM, got %T", got)
	}
	if len(m.Value) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m.Value))
	}
	if s, ok := m.Value["name"].(*types.AttributeValueMemberS); !ok || s.Value != "Alice" {
		t.Errorf("expected name=Alice, got %v", m.Value["name"])
	}
}

func TestFromAWSAttributeValue_AllTypes(t *testing.T) {
	tests := []struct {
		name     string
		aws      types.AttributeValue
		wantType dynago.AttributeType
	}{
		{"string", &types.AttributeValueMemberS{Value: "x"}, dynago.TypeS},
		{"number", &types.AttributeValueMemberN{Value: "1"}, dynago.TypeN},
		{"binary", &types.AttributeValueMemberB{Value: []byte{1}}, dynago.TypeB},
		{"bool", &types.AttributeValueMemberBOOL{Value: true}, dynago.TypeBOOL},
		{"null", &types.AttributeValueMemberNULL{Value: true}, dynago.TypeNULL},
		{"string set", &types.AttributeValueMemberSS{Value: []string{"a"}}, dynago.TypeSS},
		{"number set", &types.AttributeValueMemberNS{Value: []string{"1"}}, dynago.TypeNS},
		{"binary set", &types.AttributeValueMemberBS{Value: [][]byte{{1}}}, dynago.TypeBS},
		{"list", &types.AttributeValueMemberL{Value: []types.AttributeValue{&types.AttributeValueMemberS{Value: "a"}}}, dynago.TypeL},
		{"map", &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"k": &types.AttributeValueMemberS{Value: "v"}}}, dynago.TypeM},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fromAWSAttributeValue(tt.aws)
			if got.Type != tt.wantType {
				t.Errorf("expected type %d, got %d", tt.wantType, got.Type)
			}
		})
	}
}

func TestRoundTrip_Item(t *testing.T) {
	original := map[string]dynago.AttributeValue{
		"pk":     {Type: dynago.TypeS, S: "user#123"},
		"sk":     {Type: dynago.TypeN, N: "1"},
		"data":   {Type: dynago.TypeB, B: []byte{0xDE, 0xAD}},
		"flag":   {Type: dynago.TypeBOOL, BOOL: true},
		"empty":  {Type: dynago.TypeNULL, NULL: true},
		"tags":   {Type: dynago.TypeSS, SS: []string{"go", "aws"}},
		"scores": {Type: dynago.TypeNS, NS: []string{"100", "200"}},
		"nested": {Type: dynago.TypeM, M: map[string]dynago.AttributeValue{
			"inner": {Type: dynago.TypeS, S: "value"},
		}},
		"list": {Type: dynago.TypeL, L: []dynago.AttributeValue{
			{Type: dynago.TypeS, S: "a"},
			{Type: dynago.TypeN, N: "1"},
		}},
	}

	awsItem := toAWSItem(original)
	roundTripped := fromAWSItem(awsItem)

	if len(roundTripped) != len(original) {
		t.Fatalf("expected %d keys, got %d", len(original), len(roundTripped))
	}

	// Check a few fields
	if roundTripped["pk"].Type != dynago.TypeS || roundTripped["pk"].S != "user#123" {
		t.Errorf("pk mismatch: %+v", roundTripped["pk"])
	}
	if roundTripped["sk"].Type != dynago.TypeN || roundTripped["sk"].N != "1" {
		t.Errorf("sk mismatch: %+v", roundTripped["sk"])
	}
	if roundTripped["flag"].Type != dynago.TypeBOOL || !roundTripped["flag"].BOOL {
		t.Errorf("flag mismatch: %+v", roundTripped["flag"])
	}
	if roundTripped["nested"].Type != dynago.TypeM {
		t.Errorf("nested mismatch: %+v", roundTripped["nested"])
	}
	if roundTripped["nested"].M["inner"].S != "value" {
		t.Errorf("nested.inner mismatch: %+v", roundTripped["nested"].M["inner"])
	}
	if roundTripped["list"].Type != dynago.TypeL || len(roundTripped["list"].L) != 2 {
		t.Errorf("list mismatch: %+v", roundTripped["list"])
	}
}

func TestToAWSItem_Nil(t *testing.T) {
	if got := toAWSItem(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestFromAWSItem_Nil(t *testing.T) {
	if got := fromAWSItem(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// assertAWSAVEqual compares two AWS AttributeValues by type.
func assertAWSAVEqual(t *testing.T, want, got types.AttributeValue) {
	t.Helper()
	switch w := want.(type) {
	case *types.AttributeValueMemberS:
		g, ok := got.(*types.AttributeValueMemberS)
		if !ok || g.Value != w.Value {
			t.Errorf("S: want %q, got %v", w.Value, got)
		}
	case *types.AttributeValueMemberN:
		g, ok := got.(*types.AttributeValueMemberN)
		if !ok || g.Value != w.Value {
			t.Errorf("N: want %q, got %v", w.Value, got)
		}
	case *types.AttributeValueMemberB:
		g, ok := got.(*types.AttributeValueMemberB)
		if !ok || string(g.Value) != string(w.Value) {
			t.Errorf("B: want %v, got %v", w.Value, got)
		}
	case *types.AttributeValueMemberBOOL:
		g, ok := got.(*types.AttributeValueMemberBOOL)
		if !ok || g.Value != w.Value {
			t.Errorf("BOOL: want %v, got %v", w.Value, got)
		}
	case *types.AttributeValueMemberNULL:
		g, ok := got.(*types.AttributeValueMemberNULL)
		if !ok || g.Value != w.Value {
			t.Errorf("NULL: want %v, got %v", w.Value, got)
		}
	case *types.AttributeValueMemberSS:
		g, ok := got.(*types.AttributeValueMemberSS)
		if !ok || len(g.Value) != len(w.Value) {
			t.Errorf("SS: want %v, got %v", w.Value, got)
		}
	case *types.AttributeValueMemberNS:
		g, ok := got.(*types.AttributeValueMemberNS)
		if !ok || len(g.Value) != len(w.Value) {
			t.Errorf("NS: want %v, got %v", w.Value, got)
		}
	case *types.AttributeValueMemberBS:
		g, ok := got.(*types.AttributeValueMemberBS)
		if !ok || len(g.Value) != len(w.Value) {
			t.Errorf("BS: want %v, got %v", w.Value, got)
		}
	default:
		t.Errorf("unexpected type %T", want)
	}
}
