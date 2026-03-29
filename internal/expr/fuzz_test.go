package expr

import (
	"testing"

	"github.com/danielmensah/dynago"
)

// FuzzExpressionEval fuzzes the expression evaluator with random attribute
// values and comparison operators. It verifies the evaluator does not panic
// on arbitrary inputs.
func FuzzExpressionEval(f *testing.F) {
	// Seed corpus.
	f.Add("hello", "world", "42", "100")
	f.Add("", "", "0", "0")
	f.Add("abc", "abc", "-1", "999")
	f.Add("prefix_test", "prefix", "3.14", "2.71")

	f.Fuzz(func(t *testing.T, strVal string, strCmp string, numVal string, numCmp string) {
		item := map[string]dynago.AttributeValue{
			"Name":  {Type: dynago.TypeS, S: strVal},
			"Count": {Type: dynago.TypeN, N: numVal},
		}

		// Test string equality.
		node := CompareNode{
			Left:  PathNode{Parts: []string{"Name"}},
			Op:    EQ,
			Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: strCmp}},
		}
		// Should not panic.
		Eval(node, item)

		// Test string inequality.
		node.Op = NE
		Eval(node, item)

		// Test begins_with.
		funcNode := FuncNode{
			Name: "begins_with",
			Args: []Node{
				PathNode{Parts: []string{"Name"}},
				ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: strCmp}},
			},
		}
		Eval(funcNode, item)

		// Test contains.
		containsNode := FuncNode{
			Name: "contains",
			Args: []Node{
				PathNode{Parts: []string{"Name"}},
				ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: strCmp}},
			},
		}
		Eval(containsNode, item)

		// Test number comparison (may fail on invalid numbers, that's OK).
		numNode := CompareNode{
			Left:  PathNode{Parts: []string{"Count"}},
			Op:    LT,
			Right: ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeN, N: numCmp}},
		}
		Eval(numNode, item)

		// Test attribute_exists.
		existsNode := FuncNode{
			Name: "attribute_exists",
			Args: []Node{PathNode{Parts: []string{"Name"}}},
		}
		Eval(existsNode, item)

		// Test attribute_not_exists on missing attribute.
		notExistsNode := FuncNode{
			Name: "attribute_not_exists",
			Args: []Node{PathNode{Parts: []string{"Missing"}}},
		}
		Eval(notExistsNode, item)

		// Test logical AND.
		andNode := LogicalNode{
			Op:    AND,
			Left:  node,
			Right: funcNode,
		}
		Eval(andNode, item)

		// Test logical OR.
		orNode := LogicalNode{
			Op:    OR,
			Left:  node,
			Right: funcNode,
		}
		Eval(orNode, item)

		// Test logical NOT.
		notNode := LogicalNode{
			Op:   NOT,
			Left: node,
		}
		Eval(notNode, item)
	})
}
