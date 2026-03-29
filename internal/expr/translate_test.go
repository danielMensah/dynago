package expr

import (
	"testing"

	"github.com/danielmensah/dynago"
)

func TestToDynamo_CompareNode(t *testing.T) {
	node := CompareNode{
		Left:  PathNode{Parts: []string{"Age"}},
		Op:    GT,
		Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeN, N: "21"}},
	}
	expr, names, values := ToDynamo(node)

	if expr != "#Age > :v0" {
		t.Errorf("expr = %q, want %q", expr, "#Age > :v0")
	}
	if names["#Age"] != "Age" {
		t.Errorf("names = %v, want #Age -> Age", names)
	}
	if values[":v0"].N != "21" {
		t.Errorf("values[:v0] = %v, want N=21", values[":v0"])
	}
}

func TestToDynamo_CompareReservedWord(t *testing.T) {
	// "Status" is a DynamoDB reserved word
	node := CompareNode{
		Left:  PathNode{Parts: []string{"Status"}},
		Op:    EQ,
		Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: "active"}},
	}
	expr, names, values := ToDynamo(node)

	if expr != "#Status = :v0" {
		t.Errorf("expr = %q, want %q", expr, "#Status = :v0")
	}
	if names["#Status"] != "Status" {
		t.Errorf("names = %v, want #Status -> Status", names)
	}
	if values[":v0"].S != "active" {
		t.Errorf("values[:v0] = %v, want S=active", values[":v0"])
	}
}

func TestToDynamo_NestedPath(t *testing.T) {
	node := CompareNode{
		Left:  PathNode{Parts: []string{"Address", "City"}},
		Op:    EQ,
		Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: "London"}},
	}
	expr, names, _ := ToDynamo(node)

	if expr != "#Address.#City = :v0" {
		t.Errorf("expr = %q, want %q", expr, "#Address.#City = :v0")
	}
	if names["#Address"] != "Address" || names["#City"] != "City" {
		t.Errorf("names = %v, want #Address -> Address, #City -> City", names)
	}
}

func TestToDynamo_LogicalAND(t *testing.T) {
	node := LogicalNode{
		Op: AND,
		Left: CompareNode{
			Left:  PathNode{Parts: []string{"Status"}},
			Op:    EQ,
			Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: "active"}},
		},
		Right: CompareNode{
			Left:  PathNode{Parts: []string{"Age"}},
			Op:    GT,
			Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeN, N: "21"}},
		},
	}
	expr, names, values := ToDynamo(node)

	want := "(#Status = :v0) AND (#Age > :v1)"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
	if len(names) != 2 {
		t.Errorf("names count = %d, want 2", len(names))
	}
	if len(values) != 2 {
		t.Errorf("values count = %d, want 2", len(values))
	}
}

func TestToDynamo_LogicalOR(t *testing.T) {
	node := LogicalNode{
		Op: OR,
		Left: CompareNode{
			Left:  PathNode{Parts: []string{"Type"}},
			Op:    EQ,
			Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: "A"}},
		},
		Right: CompareNode{
			Left:  PathNode{Parts: []string{"Type"}},
			Op:    EQ,
			Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: "B"}},
		},
	}
	expr, _, _ := ToDynamo(node)

	want := "(#Type = :v0) OR (#Type = :v1)"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
}

func TestToDynamo_LogicalNOT(t *testing.T) {
	node := LogicalNode{
		Op: NOT,
		Left: CompareNode{
			Left:  PathNode{Parts: []string{"Active"}},
			Op:    EQ,
			Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeBOOL, BOOL: false}},
		},
	}
	expr, _, _ := ToDynamo(node)

	want := "NOT (#Active = :v0)"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
}

func TestToDynamo_CompoundNested(t *testing.T) {
	// (A = 1 AND B = 2) OR C = 3
	node := LogicalNode{
		Op: OR,
		Left: LogicalNode{
			Op: AND,
			Left: CompareNode{
				Left:  PathNode{Parts: []string{"A"}},
				Op:    EQ,
				Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeN, N: "1"}},
			},
			Right: CompareNode{
				Left:  PathNode{Parts: []string{"B"}},
				Op:    EQ,
				Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeN, N: "2"}},
			},
		},
		Right: CompareNode{
			Left:  PathNode{Parts: []string{"C"}},
			Op:    EQ,
			Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeN, N: "3"}},
		},
	}
	expr, _, values := ToDynamo(node)

	want := "((#A = :v0) AND (#B = :v1)) OR (#C = :v2)"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
	if len(values) != 3 {
		t.Errorf("values count = %d, want 3", len(values))
	}
}

func TestToDynamo_FuncBeginsWith(t *testing.T) {
	node := FuncNode{
		Name: "begins_with",
		Args: []Node{
			PathNode{Parts: []string{"SK"}},
			ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: "ORDER#"}},
		},
	}
	expr, names, values := ToDynamo(node)

	want := "begins_with(#SK, :v0)"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
	if names["#SK"] != "SK" {
		t.Errorf("names = %v, want #SK -> SK", names)
	}
	if values[":v0"].S != "ORDER#" {
		t.Errorf("values[:v0] = %v, want S=ORDER#", values[":v0"])
	}
}

func TestToDynamo_FuncContains(t *testing.T) {
	node := FuncNode{
		Name: "contains",
		Args: []Node{
			PathNode{Parts: []string{"Tags"}},
			ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: "go"}},
		},
	}
	expr, _, _ := ToDynamo(node)

	want := "contains(#Tags, :v0)"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
}

func TestToDynamo_FuncAttributeExists(t *testing.T) {
	node := FuncNode{
		Name: "attribute_exists",
		Args: []Node{
			PathNode{Parts: []string{"Email"}},
		},
	}
	expr, names, values := ToDynamo(node)

	want := "attribute_exists(#Email)"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
	if names["#Email"] != "Email" {
		t.Errorf("names = %v, want #Email -> Email", names)
	}
	if len(values) != 0 {
		t.Errorf("values count = %d, want 0", len(values))
	}
}

func TestToDynamo_FuncAttributeNotExists(t *testing.T) {
	node := FuncNode{
		Name: "attribute_not_exists",
		Args: []Node{
			PathNode{Parts: []string{"DeletedAt"}},
		},
	}
	expr, _, _ := ToDynamo(node)

	want := "attribute_not_exists(#DeletedAt)"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
}

func TestToDynamo_FuncSize(t *testing.T) {
	// size(#Tags) > :v0
	node := CompareNode{
		Left: FuncNode{
			Name: "size",
			Args: []Node{
				PathNode{Parts: []string{"Tags"}},
			},
		},
		Op:    GT,
		Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeN, N: "5"}},
	}
	expr, _, _ := ToDynamo(node)

	want := "size(#Tags) > :v0"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
}

func TestToDynamo_UpdateSET(t *testing.T) {
	nodes := []UpdateNode{
		{
			Action: SET,
			Path:   PathNode{Parts: []string{"Name"}},
			Value:  ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: "Alice"}},
		},
	}
	expr, names, values := ToDynamoUpdates(nodes)

	want := "SET #Name = :v0"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
	if names["#Name"] != "Name" {
		t.Errorf("names = %v, want #Name -> Name", names)
	}
	if values[":v0"].S != "Alice" {
		t.Errorf("values[:v0] = %v, want S=Alice", values[":v0"])
	}
}

func TestToDynamo_UpdateMultipleSET(t *testing.T) {
	nodes := []UpdateNode{
		{
			Action: SET,
			Path:   PathNode{Parts: []string{"Name"}},
			Value:  ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: "Alice"}},
		},
		{
			Action: SET,
			Path:   PathNode{Parts: []string{"Age"}},
			Value:  ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeN, N: "30"}},
		},
	}
	expr, _, values := ToDynamoUpdates(nodes)

	want := "SET #Name = :v0, #Age = :v1"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
	if len(values) != 2 {
		t.Errorf("values count = %d, want 2", len(values))
	}
}

func TestToDynamo_UpdateADD(t *testing.T) {
	nodes := []UpdateNode{
		{
			Action: ADD,
			Path:   PathNode{Parts: []string{"Count"}},
			Value:  ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeN, N: "1"}},
		},
	}
	expr, _, _ := ToDynamoUpdates(nodes)

	want := "ADD #Count :v0"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
}

func TestToDynamo_UpdateREMOVE(t *testing.T) {
	nodes := []UpdateNode{
		{
			Action: REMOVE,
			Path:   PathNode{Parts: []string{"TempField"}},
		},
	}
	expr, names, values := ToDynamoUpdates(nodes)

	want := "REMOVE #TempField"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
	if names["#TempField"] != "TempField" {
		t.Errorf("names = %v, want #TempField -> TempField", names)
	}
	if len(values) != 0 {
		t.Errorf("values count = %d, want 0", len(values))
	}
}

func TestToDynamo_UpdateDELETE(t *testing.T) {
	nodes := []UpdateNode{
		{
			Action: DELETE,
			Path:   PathNode{Parts: []string{"Tags"}},
			Value:  ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeSS, SS: []string{"old"}}},
		},
	}
	expr, _, _ := ToDynamoUpdates(nodes)

	want := "DELETE #Tags :v0"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
}

func TestToDynamo_UpdateGroupedActions(t *testing.T) {
	nodes := []UpdateNode{
		{
			Action: SET,
			Path:   PathNode{Parts: []string{"Name"}},
			Value:  ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: "Alice"}},
		},
		{
			Action: SET,
			Path:   PathNode{Parts: []string{"Email"}},
			Value:  ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: "alice@example.com"}},
		},
		{
			Action: ADD,
			Path:   PathNode{Parts: []string{"LoginCount"}},
			Value:  ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeN, N: "1"}},
		},
		{
			Action: REMOVE,
			Path:   PathNode{Parts: []string{"TempToken"}},
		},
	}
	expr, names, values := ToDynamoUpdates(nodes)

	want := "SET #Name = :v0, #Email = :v1 ADD #LoginCount :v2 REMOVE #TempToken"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
	if len(names) != 4 {
		t.Errorf("names count = %d, want 4", len(names))
	}
	if len(values) != 3 {
		t.Errorf("values count = %d, want 3", len(values))
	}
}

func TestToDynamo_Projection(t *testing.T) {
	node := ProjectionNode{
		Paths: []PathNode{
			{Parts: []string{"Name"}},
			{Parts: []string{"Age"}},
			{Parts: []string{"Address", "City"}},
		},
	}
	expr, names := ToDynamoProjection(node)

	want := "#Name, #Age, #Address.#City"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
	if len(names) != 4 {
		t.Errorf("names count = %d, want 4", len(names))
	}
}

func TestToDynamo_NameReuse(t *testing.T) {
	// Same attribute name used twice should reuse the same alias
	node := LogicalNode{
		Op: AND,
		Left: CompareNode{
			Left:  PathNode{Parts: []string{"Status"}},
			Op:    EQ,
			Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: "active"}},
		},
		Right: CompareNode{
			Left:  PathNode{Parts: []string{"Status"}},
			Op:    NE,
			Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: "deleted"}},
		},
	}
	expr, names, _ := ToDynamo(node)

	want := "(#Status = :v0) AND (#Status <> :v1)"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
	// Should only have one name entry since "Status" is reused
	if len(names) != 1 {
		t.Errorf("names count = %d, want 1; names = %v", len(names), names)
	}
}

func TestToDynamo_RoundTrip(t *testing.T) {
	// Build from placeholder string, then translate to DynamoDB format
	ast, err := ParseCondition("Status = ? AND Age > ?", "active", 21)
	if err != nil {
		t.Fatalf("ParseCondition: %v", err)
	}

	expr, names, values := ToDynamo(ast)

	wantExpr := "(#Status = :v0) AND (#Age > :v1)"
	if expr != wantExpr {
		t.Errorf("expr = %q, want %q", expr, wantExpr)
	}
	if names["#Status"] != "Status" || names["#Age"] != "Age" {
		t.Errorf("names = %v", names)
	}
	if values[":v0"].S != "active" {
		t.Errorf("values[:v0] = %v, want S=active", values[":v0"])
	}
	if values[":v1"].N != "21" {
		t.Errorf("values[:v1] = %v, want N=21", values[":v1"])
	}
}

func TestToDynamo_RoundTripFunction(t *testing.T) {
	ast, err := ParseCondition("begins_with(SK, ?)", "ORDER#")
	if err != nil {
		t.Fatalf("ParseCondition: %v", err)
	}

	expr, _, values := ToDynamo(ast)

	want := "begins_with(#SK, :v0)"
	if expr != want {
		t.Errorf("expr = %q, want %q", expr, want)
	}
	if values[":v0"].S != "ORDER#" {
		t.Errorf("values[:v0] = %v, want S=ORDER#", values[":v0"])
	}
}

func TestToDynamo_AllCompareOps(t *testing.T) {
	ops := []struct {
		op   CompareOp
		want string
	}{
		{EQ, "="},
		{NE, "<>"},
		{LT, "<"},
		{LE, "<="},
		{GT, ">"},
		{GE, ">="},
	}

	for _, tc := range ops {
		node := CompareNode{
			Left:  PathNode{Parts: []string{"X"}},
			Op:    tc.op,
			Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeN, N: "1"}},
		}
		expr, _, _ := ToDynamo(node)
		want := "#X " + tc.want + " :v0"
		if expr != want {
			t.Errorf("op %v: expr = %q, want %q", tc.op, expr, want)
		}
	}
}
