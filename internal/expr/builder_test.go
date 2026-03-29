package expr

import (
	"testing"

	"github.com/danielmensah/dynago"
)

func TestParseCondition_SimpleEquality(t *testing.T) {
	node, err := ParseCondition("Active = ?", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cmp, ok := node.(CompareNode)
	if !ok {
		t.Fatalf("expected CompareNode, got %T", node)
	}
	if cmp.Op != EQ {
		t.Errorf("expected EQ, got %v", cmp.Op)
	}
	path, ok := cmp.Left.(PathNode)
	if !ok {
		t.Fatalf("expected PathNode on left, got %T", cmp.Left)
	}
	if len(path.Parts) != 1 || path.Parts[0] != "Active" {
		t.Errorf("expected path [Active], got %v", path.Parts)
	}
	val, ok := cmp.Right.(ValueNode)
	if !ok {
		t.Fatalf("expected ValueNode on right, got %T", cmp.Right)
	}
	if val.Value.Type != dynago.TypeBOOL || val.Value.BOOL != true {
		t.Errorf("expected BOOL true, got %+v", val.Value)
	}
}

func TestParseCondition_ANDExpression(t *testing.T) {
	node, err := ParseCondition("Active = ? AND Age > ?", true, 21)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logical, ok := node.(LogicalNode)
	if !ok {
		t.Fatalf("expected LogicalNode, got %T", node)
	}
	if logical.Op != AND {
		t.Errorf("expected AND, got %v", logical.Op)
	}

	// Left: Active = ?
	leftCmp, ok := logical.Left.(CompareNode)
	if !ok {
		t.Fatalf("expected CompareNode on left, got %T", logical.Left)
	}
	if leftCmp.Op != EQ {
		t.Errorf("left: expected EQ, got %v", leftCmp.Op)
	}

	// Right: Age > ?
	rightCmp, ok := logical.Right.(CompareNode)
	if !ok {
		t.Fatalf("expected CompareNode on right, got %T", logical.Right)
	}
	if rightCmp.Op != GT {
		t.Errorf("right: expected GT, got %v", rightCmp.Op)
	}
	rightVal, ok := rightCmp.Right.(ValueNode)
	if !ok {
		t.Fatalf("expected ValueNode, got %T", rightCmp.Right)
	}
	if rightVal.Value.Type != dynago.TypeN || rightVal.Value.N != "21" {
		t.Errorf("expected N 21, got %+v", rightVal.Value)
	}
}

func TestParseCondition_ORExpression(t *testing.T) {
	node, err := ParseCondition("A = ? OR B = ?", "x", "y")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logical, ok := node.(LogicalNode)
	if !ok {
		t.Fatalf("expected LogicalNode, got %T", node)
	}
	if logical.Op != OR {
		t.Errorf("expected OR, got %v", logical.Op)
	}
}

func TestParseCondition_NOTExpression(t *testing.T) {
	node, err := ParseCondition("NOT Active = ?", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logical, ok := node.(LogicalNode)
	if !ok {
		t.Fatalf("expected LogicalNode, got %T", node)
	}
	if logical.Op != NOT {
		t.Errorf("expected NOT, got %v", logical.Op)
	}
	if logical.Right != nil {
		t.Errorf("NOT should have nil Right, got %T", logical.Right)
	}
}

func TestParseCondition_Precedence_NOTHigherThanAND(t *testing.T) {
	// "NOT A = ? AND B = ?" should parse as "(NOT (A = ?)) AND (B = ?)"
	node, err := ParseCondition("NOT A = ? AND B = ?", "x", "y")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logical, ok := node.(LogicalNode)
	if !ok {
		t.Fatalf("expected LogicalNode, got %T", node)
	}
	if logical.Op != AND {
		t.Errorf("expected AND at top, got %v", logical.Op)
	}
	// Left should be NOT
	notNode, ok := logical.Left.(LogicalNode)
	if !ok {
		t.Fatalf("expected LogicalNode on left, got %T", logical.Left)
	}
	if notNode.Op != NOT {
		t.Errorf("expected NOT on left, got %v", notNode.Op)
	}
}

func TestParseCondition_Precedence_ANDHigherThanOR(t *testing.T) {
	// "A = ? OR B = ? AND C = ?" should parse as "A = ? OR (B = ? AND C = ?)"
	node, err := ParseCondition("A = ? OR B = ? AND C = ?", "a", "b", "c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logical, ok := node.(LogicalNode)
	if !ok {
		t.Fatalf("expected LogicalNode, got %T", node)
	}
	if logical.Op != OR {
		t.Errorf("expected OR at top, got %v", logical.Op)
	}
	// Right should be AND
	andNode, ok := logical.Right.(LogicalNode)
	if !ok {
		t.Fatalf("expected LogicalNode on right, got %T", logical.Right)
	}
	if andNode.Op != AND {
		t.Errorf("expected AND on right, got %v", andNode.Op)
	}
}

func TestParseCondition_NestedPath(t *testing.T) {
	node, err := ParseCondition("Address.City = ?", "NYC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cmp, ok := node.(CompareNode)
	if !ok {
		t.Fatalf("expected CompareNode, got %T", node)
	}
	path, ok := cmp.Left.(PathNode)
	if !ok {
		t.Fatalf("expected PathNode, got %T", cmp.Left)
	}
	if len(path.Parts) != 2 || path.Parts[0] != "Address" || path.Parts[1] != "City" {
		t.Errorf("expected [Address City], got %v", path.Parts)
	}
}

func TestParseCondition_ReservedWordPlaceholder(t *testing.T) {
	node, err := ParseCondition("#Status = ?", "active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cmp, ok := node.(CompareNode)
	if !ok {
		t.Fatalf("expected CompareNode, got %T", node)
	}
	path, ok := cmp.Left.(PathNode)
	if !ok {
		t.Fatalf("expected PathNode, got %T", cmp.Left)
	}
	if len(path.Parts) != 1 || path.Parts[0] != "Status" {
		t.Errorf("expected [Status], got %v", path.Parts)
	}
}

func TestParseCondition_FunctionCall_BeginsWith(t *testing.T) {
	node, err := ParseCondition("begins_with(SK, ?)", "ORDER#")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn, ok := node.(FuncNode)
	if !ok {
		t.Fatalf("expected FuncNode, got %T", node)
	}
	if fn.Name != "begins_with" {
		t.Errorf("expected begins_with, got %s", fn.Name)
	}
	if len(fn.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(fn.Args))
	}
	path, ok := fn.Args[0].(PathNode)
	if !ok {
		t.Fatalf("expected PathNode as first arg, got %T", fn.Args[0])
	}
	if path.Parts[0] != "SK" {
		t.Errorf("expected SK, got %s", path.Parts[0])
	}
	val, ok := fn.Args[1].(ValueNode)
	if !ok {
		t.Fatalf("expected ValueNode as second arg, got %T", fn.Args[1])
	}
	if val.Value.S != "ORDER#" {
		t.Errorf("expected ORDER#, got %s", val.Value.S)
	}
}

func TestParseCondition_FunctionCall_AttributeExists(t *testing.T) {
	node, err := ParseCondition("attribute_exists(Email)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn, ok := node.(FuncNode)
	if !ok {
		t.Fatalf("expected FuncNode, got %T", node)
	}
	if fn.Name != "attribute_exists" {
		t.Errorf("expected attribute_exists, got %s", fn.Name)
	}
	if len(fn.Args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(fn.Args))
	}
}

func TestParseCondition_FunctionCall_Contains(t *testing.T) {
	node, err := ParseCondition("contains(Tags, ?)", "urgent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn, ok := node.(FuncNode)
	if !ok {
		t.Fatalf("expected FuncNode, got %T", node)
	}
	if fn.Name != "contains" {
		t.Errorf("expected contains, got %s", fn.Name)
	}
	if len(fn.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(fn.Args))
	}
}

func TestParseCondition_FunctionCallANDComparison(t *testing.T) {
	node, err := ParseCondition("begins_with(SK, ?) AND Active = ?", "ORDER#", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logical, ok := node.(LogicalNode)
	if !ok {
		t.Fatalf("expected LogicalNode, got %T", node)
	}
	if logical.Op != AND {
		t.Errorf("expected AND, got %v", logical.Op)
	}
	_, ok = logical.Left.(FuncNode)
	if !ok {
		t.Fatalf("expected FuncNode on left, got %T", logical.Left)
	}
	_, ok = logical.Right.(CompareNode)
	if !ok {
		t.Fatalf("expected CompareNode on right, got %T", logical.Right)
	}
}

func TestParseCondition_AllComparisonOps(t *testing.T) {
	tests := []struct {
		expr string
		op   CompareOp
	}{
		{"X = ?", EQ},
		{"X <> ?", NE},
		{"X < ?", LT},
		{"X <= ?", LE},
		{"X > ?", GT},
		{"X >= ?", GE},
	}
	for _, tt := range tests {
		t.Run(tt.op.String(), func(t *testing.T) {
			node, err := ParseCondition(tt.expr, 1)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			cmp, ok := node.(CompareNode)
			if !ok {
				t.Fatalf("expected CompareNode, got %T", node)
			}
			if cmp.Op != tt.op {
				t.Errorf("expected %v, got %v", tt.op, cmp.Op)
			}
		})
	}
}

func TestParseCondition_MultipleConditions(t *testing.T) {
	node, err := ParseCondition("A = ? AND B > ? AND C < ?", "a", 10, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should be left-associative: ((A = ? AND B > ?) AND C < ?)
	top, ok := node.(LogicalNode)
	if !ok {
		t.Fatalf("expected LogicalNode, got %T", node)
	}
	if top.Op != AND {
		t.Errorf("expected AND at top, got %v", top.Op)
	}
	inner, ok := top.Left.(LogicalNode)
	if !ok {
		t.Fatalf("expected LogicalNode on left, got %T", top.Left)
	}
	if inner.Op != AND {
		t.Errorf("expected AND in inner, got %v", inner.Op)
	}
}

func TestParseCondition_ParenthesizedExpression(t *testing.T) {
	// "(A = ? OR B = ?) AND C = ?"
	node, err := ParseCondition("(A = ? OR B = ?) AND C = ?", "a", "b", "c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	top, ok := node.(LogicalNode)
	if !ok {
		t.Fatalf("expected LogicalNode, got %T", node)
	}
	if top.Op != AND {
		t.Errorf("expected AND at top, got %v", top.Op)
	}
	// Left should be OR (parenthesized)
	orNode, ok := top.Left.(LogicalNode)
	if !ok {
		t.Fatalf("expected LogicalNode on left, got %T", top.Left)
	}
	if orNode.Op != OR {
		t.Errorf("expected OR on left, got %v", orNode.Op)
	}
}

func TestParseCondition_ErrorTooFewArgs(t *testing.T) {
	_, err := ParseCondition("A = ? AND B = ?", "only_one")
	if err == nil {
		t.Fatal("expected error for too few args")
	}
}

func TestParseCondition_ErrorTooManyArgs(t *testing.T) {
	_, err := ParseCondition("A = ?", "one", "two")
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestBuildSet(t *testing.T) {
	node := BuildSet("Name", "Alice")
	if node.Action != SET {
		t.Errorf("expected SET, got %v", node.Action)
	}
	if node.Path.Parts[0] != "Name" {
		t.Errorf("expected Name, got %v", node.Path.Parts)
	}
	val, ok := node.Value.(ValueNode)
	if !ok {
		t.Fatalf("expected ValueNode, got %T", node.Value)
	}
	if val.Value.Type != dynago.TypeS || val.Value.S != "Alice" {
		t.Errorf("expected S Alice, got %+v", val.Value)
	}
}

func TestBuildAdd(t *testing.T) {
	node := BuildAdd("Count", 5)
	if node.Action != ADD {
		t.Errorf("expected ADD, got %v", node.Action)
	}
	val, ok := node.Value.(ValueNode)
	if !ok {
		t.Fatalf("expected ValueNode, got %T", node.Value)
	}
	if val.Value.Type != dynago.TypeN || val.Value.N != "5" {
		t.Errorf("expected N 5, got %+v", val.Value)
	}
}

func TestBuildRemove(t *testing.T) {
	node := BuildRemove("TempAttr")
	if node.Action != REMOVE {
		t.Errorf("expected REMOVE, got %v", node.Action)
	}
	if node.Value != nil {
		t.Errorf("expected nil Value for REMOVE, got %v", node.Value)
	}
}

func TestBuildDelete(t *testing.T) {
	node := BuildDelete("Tags", []string{"old"})
	if node.Action != DELETE {
		t.Errorf("expected DELETE, got %v", node.Action)
	}
	val, ok := node.Value.(ValueNode)
	if !ok {
		t.Fatalf("expected ValueNode, got %T", node.Value)
	}
	if val.Value.Type != dynago.TypeSS {
		t.Errorf("expected SS type, got %v", val.Value.Type)
	}
}

func TestBuildSet_NestedPath(t *testing.T) {
	node := BuildSet("Address.City", "NYC")
	if len(node.Path.Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(node.Path.Parts))
	}
	if node.Path.Parts[0] != "Address" || node.Path.Parts[1] != "City" {
		t.Errorf("expected [Address City], got %v", node.Path.Parts)
	}
}

func TestGoToAttributeValue_Types(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		wantType dynago.AttributeType
	}{
		{"string", "hello", dynago.TypeS},
		{"bool", true, dynago.TypeBOOL},
		{"int", 42, dynago.TypeN},
		{"int64", int64(42), dynago.TypeN},
		{"float64", 3.14, dynago.TypeN},
		{"[]byte", []byte("data"), dynago.TypeB},
		{"[]string", []string{"a", "b"}, dynago.TypeSS},
		{"nil", nil, dynago.TypeNULL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			av := goToAttributeValue(tt.input)
			if av.Type != tt.wantType {
				t.Errorf("expected type %v, got %v", tt.wantType, av.Type)
			}
		})
	}
}

func TestGoToAttributeValue_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for unsupported type")
		}
	}()
	type custom struct{}
	goToAttributeValue(custom{})
}

func TestCompareOpString(t *testing.T) {
	tests := []struct {
		op   CompareOp
		want string
	}{
		{EQ, "="}, {NE, "<>"}, {LT, "<"}, {LE, "<="}, {GT, ">"}, {GE, ">="},
	}
	for _, tt := range tests {
		if got := tt.op.String(); got != tt.want {
			t.Errorf("CompareOp(%d).String() = %q, want %q", tt.op, got, tt.want)
		}
	}
}

func TestLogicalOpString(t *testing.T) {
	tests := []struct {
		op   LogicalOp
		want string
	}{
		{AND, "AND"}, {OR, "OR"}, {NOT, "NOT"},
	}
	for _, tt := range tests {
		if got := tt.op.String(); got != tt.want {
			t.Errorf("LogicalOp(%d).String() = %q, want %q", tt.op, got, tt.want)
		}
	}
}

func TestUpdateActionString(t *testing.T) {
	tests := []struct {
		a    UpdateAction
		want string
	}{
		{SET, "SET"}, {ADD, "ADD"}, {REMOVE, "REMOVE"}, {DELETE, "DELETE"},
	}
	for _, tt := range tests {
		if got := tt.a.String(); got != tt.want {
			t.Errorf("UpdateAction(%d).String() = %q, want %q", tt.a, got, tt.want)
		}
	}
}
