package expr

import (
	"testing"

	dynago "github.com/danielmensah/dynago"
)

// helper to build a string AttributeValue
func strVal(s string) dynago.AttributeValue {
	return dynago.AttributeValue{Type: dynago.TypeS, S: s}
}

// helper to build a number AttributeValue
func numVal(n string) dynago.AttributeValue {
	return dynago.AttributeValue{Type: dynago.TypeN, N: n}
}

// helper to build a bool AttributeValue
func boolVal(b bool) dynago.AttributeValue {
	return dynago.AttributeValue{Type: dynago.TypeBOOL, BOOL: b}
}

func TestResolvePathValue(t *testing.T) {
	item := map[string]dynago.AttributeValue{
		"Name": strVal("Alice"),
		"Address": {
			Type: dynago.TypeM,
			M: map[string]dynago.AttributeValue{
				"City":  strVal("Seattle"),
				"State": strVal("WA"),
			},
		},
	}

	tests := []struct {
		name   string
		parts  []string
		wantOK bool
		wantS  string
	}{
		{"top-level", []string{"Name"}, true, "Alice"},
		{"nested", []string{"Address", "City"}, true, "Seattle"},
		{"missing top", []string{"Age"}, false, ""},
		{"missing nested", []string{"Address", "Zip"}, false, ""},
		{"non-map nested", []string{"Name", "Foo"}, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := resolvePathValue(tt.parts, item)
			if ok != tt.wantOK {
				t.Fatalf("got ok=%v, want %v", ok, tt.wantOK)
			}
			if ok && val.S != tt.wantS {
				t.Fatalf("got S=%q, want %q", val.S, tt.wantS)
			}
		})
	}
}

func TestEvalCompare(t *testing.T) {
	item := map[string]dynago.AttributeValue{
		"Name":  strVal("Alice"),
		"Age":   numVal("30"),
		"Score": numVal("85.5"),
	}

	tests := []struct {
		name string
		node Node
		want bool
	}{
		{
			"string EQ true",
			CompareNode{Left: PathNode{Parts: []string{"Name"}}, Op: EQ, Right: ValueNode{Value: strVal("Alice")}},
			true,
		},
		{
			"string EQ false",
			CompareNode{Left: PathNode{Parts: []string{"Name"}}, Op: EQ, Right: ValueNode{Value: strVal("Bob")}},
			false,
		},
		{
			"string NE",
			CompareNode{Left: PathNode{Parts: []string{"Name"}}, Op: NE, Right: ValueNode{Value: strVal("Bob")}},
			true,
		},
		{
			"string LT",
			CompareNode{Left: PathNode{Parts: []string{"Name"}}, Op: LT, Right: ValueNode{Value: strVal("Bob")}},
			true, // "Alice" < "Bob" lexicographically
		},
		{
			"number EQ",
			CompareNode{Left: PathNode{Parts: []string{"Age"}}, Op: EQ, Right: ValueNode{Value: numVal("30")}},
			true,
		},
		{
			"number GT",
			CompareNode{Left: PathNode{Parts: []string{"Age"}}, Op: GT, Right: ValueNode{Value: numVal("25")}},
			true,
		},
		{
			"number LE",
			CompareNode{Left: PathNode{Parts: []string{"Score"}}, Op: LE, Right: ValueNode{Value: numVal("85.5")}},
			true,
		},
		{
			"number LT false",
			CompareNode{Left: PathNode{Parts: []string{"Age"}}, Op: LT, Right: ValueNode{Value: numVal("20")}},
			false,
		},
		{
			"missing path returns false",
			CompareNode{Left: PathNode{Parts: []string{"Missing"}}, Op: EQ, Right: ValueNode{Value: strVal("x")}},
			false,
		},
		{
			"type mismatch returns false",
			CompareNode{Left: PathNode{Parts: []string{"Name"}}, Op: EQ, Right: ValueNode{Value: numVal("30")}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.node, item)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvalLogical(t *testing.T) {
	item := map[string]dynago.AttributeValue{
		"Age":    numVal("30"),
		"Active": boolVal(true),
	}

	trueNode := CompareNode{Left: PathNode{Parts: []string{"Age"}}, Op: EQ, Right: ValueNode{Value: numVal("30")}}
	falseNode := CompareNode{Left: PathNode{Parts: []string{"Age"}}, Op: EQ, Right: ValueNode{Value: numVal("99")}}

	tests := []struct {
		name string
		node Node
		want bool
	}{
		{"AND true true", LogicalNode{Op: AND, Left: trueNode, Right: trueNode}, true},
		{"AND true false", LogicalNode{Op: AND, Left: trueNode, Right: falseNode}, false},
		{"AND false (short-circuit)", LogicalNode{Op: AND, Left: falseNode, Right: trueNode}, false},
		{"OR true (short-circuit)", LogicalNode{Op: OR, Left: trueNode, Right: falseNode}, true},
		{"OR false true", LogicalNode{Op: OR, Left: falseNode, Right: trueNode}, true},
		{"OR false false", LogicalNode{Op: OR, Left: falseNode, Right: falseNode}, false},
		{"NOT true", LogicalNode{Op: NOT, Left: trueNode}, false},
		{"NOT false", LogicalNode{Op: NOT, Left: falseNode}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.node, item)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvalFuncAttributeExists(t *testing.T) {
	item := map[string]dynago.AttributeValue{
		"Name": strVal("Alice"),
		"Address": {
			Type: dynago.TypeM,
			M: map[string]dynago.AttributeValue{
				"City": strVal("Seattle"),
			},
		},
	}

	tests := []struct {
		name string
		node Node
		want bool
	}{
		{
			"exists top-level",
			FuncNode{Name: "attribute_exists", Args: []Node{PathNode{Parts: []string{"Name"}}}},
			true,
		},
		{
			"exists nested",
			FuncNode{Name: "attribute_exists", Args: []Node{PathNode{Parts: []string{"Address", "City"}}}},
			true,
		},
		{
			"not exists",
			FuncNode{Name: "attribute_exists", Args: []Node{PathNode{Parts: []string{"Missing"}}}},
			false,
		},
		{
			"attribute_not_exists present",
			FuncNode{Name: "attribute_not_exists", Args: []Node{PathNode{Parts: []string{"Name"}}}},
			false,
		},
		{
			"attribute_not_exists absent",
			FuncNode{Name: "attribute_not_exists", Args: []Node{PathNode{Parts: []string{"Missing"}}}},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.node, item)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvalFuncBeginsWith(t *testing.T) {
	item := map[string]dynago.AttributeValue{
		"Name": strVal("Alice"),
	}

	tests := []struct {
		name   string
		prefix string
		want   bool
	}{
		{"match", "Ali", true},
		{"full match", "Alice", true},
		{"no match", "Bob", false},
		{"empty prefix", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := FuncNode{
				Name: "begins_with",
				Args: []Node{
					PathNode{Parts: []string{"Name"}},
					ValueNode{Value: strVal(tt.prefix)},
				},
			}
			got, err := Eval(node, item)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvalFuncContains(t *testing.T) {
	item := map[string]dynago.AttributeValue{
		"Name": strVal("Alice"),
		"Tags": {
			Type: dynago.TypeL,
			L: []dynago.AttributeValue{
				strVal("admin"),
				strVal("user"),
			},
		},
		"Roles": {
			Type: dynago.TypeSS,
			SS:   []string{"editor", "viewer"},
		},
	}

	tests := []struct {
		name string
		node Node
		want bool
	}{
		{
			"string substring",
			FuncNode{Name: "contains", Args: []Node{
				PathNode{Parts: []string{"Name"}},
				ValueNode{Value: strVal("lic")},
			}},
			true,
		},
		{
			"string no match",
			FuncNode{Name: "contains", Args: []Node{
				PathNode{Parts: []string{"Name"}},
				ValueNode{Value: strVal("bob")},
			}},
			false,
		},
		{
			"list contains element",
			FuncNode{Name: "contains", Args: []Node{
				PathNode{Parts: []string{"Tags"}},
				ValueNode{Value: strVal("admin")},
			}},
			true,
		},
		{
			"list missing element",
			FuncNode{Name: "contains", Args: []Node{
				PathNode{Parts: []string{"Tags"}},
				ValueNode{Value: strVal("superadmin")},
			}},
			false,
		},
		{
			"string set contains",
			FuncNode{Name: "contains", Args: []Node{
				PathNode{Parts: []string{"Roles"}},
				ValueNode{Value: strVal("editor")},
			}},
			true,
		},
		{
			"string set not contains",
			FuncNode{Name: "contains", Args: []Node{
				PathNode{Parts: []string{"Roles"}},
				ValueNode{Value: strVal("owner")},
			}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.node, item)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvalFuncSize(t *testing.T) {
	item := map[string]dynago.AttributeValue{
		"Name": strVal("Alice"),
		"Tags": {Type: dynago.TypeL, L: []dynago.AttributeValue{strVal("a"), strVal("b")}},
		"Data": {Type: dynago.TypeB, B: []byte{1, 2, 3}},
		"Meta": {Type: dynago.TypeM, M: map[string]dynago.AttributeValue{"k": strVal("v")}},
	}

	tests := []struct {
		name    string
		path    string
		wantCmp CompareOp
		wantVal string
		want    bool
	}{
		{"string size", "Name", EQ, "5", true},
		{"list size", "Tags", EQ, "2", true},
		{"binary size", "Data", EQ, "3", true},
		{"map size", "Meta", EQ, "1", true},
		{"size GT", "Name", GT, "3", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := CompareNode{
				Left:  FuncNode{Name: "size", Args: []Node{PathNode{Parts: []string{tt.path}}}},
				Op:    tt.wantCmp,
				Right: ValueNode{Value: numVal(tt.wantVal)},
			}
			got, err := Eval(node, item)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvalBinaryCompare(t *testing.T) {
	item := map[string]dynago.AttributeValue{
		"Data": {Type: dynago.TypeB, B: []byte{1, 2, 3}},
	}

	tests := []struct {
		name string
		op   CompareOp
		val  []byte
		want bool
	}{
		{"EQ", EQ, []byte{1, 2, 3}, true},
		{"NE", NE, []byte{1, 2, 4}, true},
		{"LT", LT, []byte{1, 2, 4}, true},
		{"GT", GT, []byte{1, 2, 2}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := CompareNode{
				Left:  PathNode{Parts: []string{"Data"}},
				Op:    tt.op,
				Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeB, B: tt.val}},
			}
			got, err := Eval(node, item)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// ==================== Update Evaluator Tests ====================

func TestEvalUpdateSet(t *testing.T) {
	item := map[string]dynago.AttributeValue{
		"Name": strVal("Alice"),
		"Age":  numVal("30"),
	}

	nodes := []Node{
		UpdateNode{Action: SET, Path: PathNode{Parts: []string{"Name"}}, Value: ValueNode{Value: strVal("Bob")}},
		UpdateNode{Action: SET, Path: PathNode{Parts: []string{"Email"}}, Value: ValueNode{Value: strVal("bob@test.com")}},
	}

	result, err := EvalUpdate(nodes, item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original not mutated.
	if item["Name"].S != "Alice" {
		t.Fatal("original item was mutated")
	}

	if result["Name"].S != "Bob" {
		t.Fatalf("Name: got %q, want %q", result["Name"].S, "Bob")
	}
	if result["Email"].S != "bob@test.com" {
		t.Fatalf("Email: got %q, want %q", result["Email"].S, "bob@test.com")
	}
	if result["Age"].N != "30" {
		t.Fatal("Age should be unchanged")
	}
}

func TestEvalUpdateSetNested(t *testing.T) {
	item := map[string]dynago.AttributeValue{
		"Address": {
			Type: dynago.TypeM,
			M: map[string]dynago.AttributeValue{
				"City":  strVal("Seattle"),
				"State": strVal("WA"),
			},
		},
	}

	nodes := []Node{
		UpdateNode{Action: SET, Path: PathNode{Parts: []string{"Address", "City"}}, Value: ValueNode{Value: strVal("Portland")}},
	}

	result, err := EvalUpdate(nodes, item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original not mutated.
	if item["Address"].M["City"].S != "Seattle" {
		t.Fatal("original item was mutated")
	}

	if result["Address"].M["City"].S != "Portland" {
		t.Fatalf("got %q, want %q", result["Address"].M["City"].S, "Portland")
	}
	// Other nested fields preserved.
	if result["Address"].M["State"].S != "WA" {
		t.Fatal("State should be unchanged")
	}
}

func TestEvalUpdateAddNumber(t *testing.T) {
	item := map[string]dynago.AttributeValue{
		"Count": numVal("10"),
	}

	nodes := []Node{
		UpdateNode{Action: ADD, Path: PathNode{Parts: []string{"Count"}}, Value: ValueNode{Value: numVal("5")}},
	}

	result, err := EvalUpdate(nodes, item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["Count"].N != "15" {
		t.Fatalf("got %q, want %q", result["Count"].N, "15")
	}
}

func TestEvalUpdateAddNewNumber(t *testing.T) {
	item := map[string]dynago.AttributeValue{}

	nodes := []Node{
		UpdateNode{Action: ADD, Path: PathNode{Parts: []string{"Count"}}, Value: ValueNode{Value: numVal("5")}},
	}

	result, err := EvalUpdate(nodes, item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["Count"].N != "5" {
		t.Fatalf("got %q, want %q", result["Count"].N, "5")
	}
}

func TestEvalUpdateAddStringSet(t *testing.T) {
	item := map[string]dynago.AttributeValue{
		"Tags": {Type: dynago.TypeSS, SS: []string{"a", "b"}},
	}

	nodes := []Node{
		UpdateNode{Action: ADD, Path: PathNode{Parts: []string{"Tags"}}, Value: ValueNode{
			Value: dynago.AttributeValue{Type: dynago.TypeSS, SS: []string{"b", "c"}},
		}},
	}

	result, err := EvalUpdate(nodes, item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result["Tags"].SS) != 3 {
		t.Fatalf("got %d elements, want 3: %v", len(result["Tags"].SS), result["Tags"].SS)
	}
}

func TestEvalUpdateAddTypeMismatch(t *testing.T) {
	item := map[string]dynago.AttributeValue{
		"Name": strVal("Alice"),
	}

	nodes := []Node{
		UpdateNode{Action: ADD, Path: PathNode{Parts: []string{"Name"}}, Value: ValueNode{Value: numVal("5")}},
	}

	_, err := EvalUpdate(nodes, item)
	if err == nil {
		t.Fatal("expected error for ADD on string")
	}
}

func TestEvalUpdateRemove(t *testing.T) {
	item := map[string]dynago.AttributeValue{
		"Name": strVal("Alice"),
		"Age":  numVal("30"),
	}

	nodes := []Node{
		UpdateNode{Action: REMOVE, Path: PathNode{Parts: []string{"Age"}}},
	}

	result, err := EvalUpdate(nodes, item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := result["Age"]; ok {
		t.Fatal("Age should be removed")
	}
	if result["Name"].S != "Alice" {
		t.Fatal("Name should be unchanged")
	}
}

func TestEvalUpdateRemoveNested(t *testing.T) {
	item := map[string]dynago.AttributeValue{
		"Address": {
			Type: dynago.TypeM,
			M: map[string]dynago.AttributeValue{
				"City":  strVal("Seattle"),
				"State": strVal("WA"),
			},
		},
	}

	nodes := []Node{
		UpdateNode{Action: REMOVE, Path: PathNode{Parts: []string{"Address", "City"}}},
	}

	result, err := EvalUpdate(nodes, item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := result["Address"].M["City"]; ok {
		t.Fatal("City should be removed")
	}
	if result["Address"].M["State"].S != "WA" {
		t.Fatal("State should be unchanged")
	}
}

func TestEvalUpdateDelete(t *testing.T) {
	item := map[string]dynago.AttributeValue{
		"Tags": {Type: dynago.TypeSS, SS: []string{"a", "b", "c"}},
	}

	nodes := []Node{
		UpdateNode{Action: DELETE, Path: PathNode{Parts: []string{"Tags"}}, Value: ValueNode{
			Value: dynago.AttributeValue{Type: dynago.TypeSS, SS: []string{"b"}},
		}},
	}

	result, err := EvalUpdate(nodes, item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ss := result["Tags"].SS
	if len(ss) != 2 {
		t.Fatalf("got %d elements, want 2: %v", len(ss), ss)
	}
	for _, s := range ss {
		if s == "b" {
			t.Fatal("b should have been deleted")
		}
	}
}

func TestEvalUpdateDeleteFromMissing(t *testing.T) {
	item := map[string]dynago.AttributeValue{}

	nodes := []Node{
		UpdateNode{Action: DELETE, Path: PathNode{Parts: []string{"Tags"}}, Value: ValueNode{
			Value: dynago.AttributeValue{Type: dynago.TypeSS, SS: []string{"a"}},
		}},
	}

	// Should not error when deleting from non-existent attribute.
	_, err := EvalUpdate(nodes, item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvalUpdateMultipleActions(t *testing.T) {
	item := map[string]dynago.AttributeValue{
		"Name":  strVal("Alice"),
		"Age":   numVal("30"),
		"Score": numVal("100"),
	}

	nodes := []Node{
		UpdateNode{Action: SET, Path: PathNode{Parts: []string{"Name"}}, Value: ValueNode{Value: strVal("Bob")}},
		UpdateNode{Action: ADD, Path: PathNode{Parts: []string{"Age"}}, Value: ValueNode{Value: numVal("1")}},
		UpdateNode{Action: REMOVE, Path: PathNode{Parts: []string{"Score"}}},
	}

	result, err := EvalUpdate(nodes, item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["Name"].S != "Bob" {
		t.Fatalf("Name: got %q, want Bob", result["Name"].S)
	}
	if result["Age"].N != "31" {
		t.Fatalf("Age: got %q, want 31", result["Age"].N)
	}
	if _, ok := result["Score"]; ok {
		t.Fatal("Score should be removed")
	}
}

func TestEvalUpdateDoesNotMutateInput(t *testing.T) {
	original := map[string]dynago.AttributeValue{
		"Count": numVal("10"),
	}

	nodes := []Node{
		UpdateNode{Action: SET, Path: PathNode{Parts: []string{"Count"}}, Value: ValueNode{Value: numVal("99")}},
	}

	_, err := EvalUpdate(nodes, original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if original["Count"].N != "10" {
		t.Fatal("original item was mutated")
	}
}
