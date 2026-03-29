package expr

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	dynago "github.com/danielmensah/dynago"
)

// Eval evaluates a condition or filter expression AST against an item.
// It returns true if the condition is satisfied.
func Eval(node Node, item map[string]dynago.AttributeValue) (bool, error) {
	switch n := node.(type) {
	case CompareNode:
		return evalCompare(n, item)
	case LogicalNode:
		return evalLogical(n, item)
	case FuncNode:
		return evalFunc(n, item)
	default:
		return false, fmt.Errorf("eval: unsupported node type %T", node)
	}
}

// resolvePathValue traverses into nested M attributes to resolve a dotted path.
func resolvePathValue(parts []string, item map[string]dynago.AttributeValue) (dynago.AttributeValue, bool) {
	if len(parts) == 0 {
		return dynago.AttributeValue{}, false
	}
	val, ok := item[parts[0]]
	if !ok {
		return dynago.AttributeValue{}, false
	}
	for _, part := range parts[1:] {
		if val.Type != dynago.TypeM || val.M == nil {
			return dynago.AttributeValue{}, false
		}
		val, ok = val.M[part]
		if !ok {
			return dynago.AttributeValue{}, false
		}
	}
	return val, true
}

// resolveNode resolves a Node to an AttributeValue. PathNodes are looked up in
// the item; ValueNodes return their value directly. FuncNode for size() is
// resolved to a numeric value.
func resolveNode(node Node, item map[string]dynago.AttributeValue) (dynago.AttributeValue, bool, error) {
	switch n := node.(type) {
	case PathNode:
		v, ok := resolvePathValue(n.Parts, item)
		return v, ok, nil
	case ValueNode:
		return n.Value, true, nil
	case FuncNode:
		if n.Name == "size" {
			sz, err := evalSize(n, item)
			if err != nil {
				return dynago.AttributeValue{}, false, err
			}
			return dynago.AttributeValue{Type: dynago.TypeN, N: strconv.FormatInt(sz, 10)}, true, nil
		}
		return dynago.AttributeValue{}, false, fmt.Errorf("eval: cannot resolve function %s as value", n.Name)
	default:
		return dynago.AttributeValue{}, false, fmt.Errorf("eval: cannot resolve %T as value", node)
	}
}

func evalCompare(n CompareNode, item map[string]dynago.AttributeValue) (bool, error) {
	left, leftOK, err := resolveNode(n.Left, item)
	if err != nil {
		return false, err
	}
	right, rightOK, err := resolveNode(n.Right, item)
	if err != nil {
		return false, err
	}

	// If either side is missing, comparison is false (DynamoDB behavior).
	if !leftOK || !rightOK {
		return false, nil
	}

	// Types must match for comparison.
	if left.Type != right.Type {
		return false, nil
	}

	cmp, err := compareValues(left, right)
	if err != nil {
		return false, err
	}

	switch n.Op {
	case EQ:
		return cmp == 0, nil
	case NE:
		return cmp != 0, nil
	case LT:
		return cmp < 0, nil
	case LE:
		return cmp <= 0, nil
	case GT:
		return cmp > 0, nil
	case GE:
		return cmp >= 0, nil
	default:
		return false, fmt.Errorf("eval: unknown compare op %d", n.Op)
	}
}

// compareValues returns -1, 0, or 1 for ordering of two AttributeValues of the
// same type.
func compareValues(a, b dynago.AttributeValue) (int, error) {
	switch a.Type {
	case dynago.TypeS:
		return strings.Compare(a.S, b.S), nil
	case dynago.TypeN:
		af, err := strconv.ParseFloat(a.N, 64)
		if err != nil {
			return 0, fmt.Errorf("eval: invalid number %q: %w", a.N, err)
		}
		bf, err := strconv.ParseFloat(b.N, 64)
		if err != nil {
			return 0, fmt.Errorf("eval: invalid number %q: %w", b.N, err)
		}
		switch {
		case af < bf:
			return -1, nil
		case af > bf:
			return 1, nil
		default:
			return 0, nil
		}
	case dynago.TypeB:
		return bytes.Compare(a.B, b.B), nil
	case dynago.TypeBOOL:
		switch {
		case a.BOOL == b.BOOL:
			return 0, nil
		case !a.BOOL && b.BOOL:
			return -1, nil
		default:
			return 1, nil
		}
	default:
		return 0, fmt.Errorf("eval: comparison not supported for type %d", a.Type)
	}
}

func evalLogical(n LogicalNode, item map[string]dynago.AttributeValue) (bool, error) {
	switch n.Op {
	case AND:
		left, err := Eval(n.Left, item)
		if err != nil {
			return false, err
		}
		if !left {
			return false, nil // short-circuit
		}
		return Eval(n.Right, item)
	case OR:
		left, err := Eval(n.Left, item)
		if err != nil {
			return false, err
		}
		if left {
			return true, nil // short-circuit
		}
		return Eval(n.Right, item)
	case NOT:
		left, err := Eval(n.Left, item)
		if err != nil {
			return false, err
		}
		return !left, nil
	default:
		return false, fmt.Errorf("eval: unknown logical op %d", n.Op)
	}
}

func evalFunc(n FuncNode, item map[string]dynago.AttributeValue) (bool, error) {
	switch n.Name {
	case "attribute_exists":
		if len(n.Args) != 1 {
			return false, fmt.Errorf("eval: attribute_exists requires 1 argument")
		}
		path, ok := n.Args[0].(PathNode)
		if !ok {
			return false, fmt.Errorf("eval: attribute_exists argument must be a path")
		}
		_, exists := resolvePathValue(path.Parts, item)
		return exists, nil

	case "attribute_not_exists":
		if len(n.Args) != 1 {
			return false, fmt.Errorf("eval: attribute_not_exists requires 1 argument")
		}
		path, ok := n.Args[0].(PathNode)
		if !ok {
			return false, fmt.Errorf("eval: attribute_not_exists argument must be a path")
		}
		_, exists := resolvePathValue(path.Parts, item)
		return !exists, nil

	case "begins_with":
		if len(n.Args) != 2 {
			return false, fmt.Errorf("eval: begins_with requires 2 arguments")
		}
		val, valOK, err := resolveNode(n.Args[0], item)
		if err != nil {
			return false, err
		}
		prefix, prefixOK, err := resolveNode(n.Args[1], item)
		if err != nil {
			return false, err
		}
		if !valOK || !prefixOK {
			return false, nil
		}
		if val.Type != dynago.TypeS || prefix.Type != dynago.TypeS {
			return false, nil
		}
		return strings.HasPrefix(val.S, prefix.S), nil

	case "contains":
		if len(n.Args) != 2 {
			return false, fmt.Errorf("eval: contains requires 2 arguments")
		}
		val, valOK, err := resolveNode(n.Args[0], item)
		if err != nil {
			return false, err
		}
		operand, operandOK, err := resolveNode(n.Args[1], item)
		if err != nil {
			return false, err
		}
		if !valOK || !operandOK {
			return false, nil
		}
		// String substring check.
		if val.Type == dynago.TypeS && operand.Type == dynago.TypeS {
			return strings.Contains(val.S, operand.S), nil
		}
		// List element membership.
		if val.Type == dynago.TypeL {
			for _, elem := range val.L {
				if elem.Type == operand.Type {
					cmp, err := compareValues(elem, operand)
					if err == nil && cmp == 0 {
						return true, nil
					}
				}
			}
			return false, nil
		}
		// String set membership.
		if val.Type == dynago.TypeSS && operand.Type == dynago.TypeS {
			for _, s := range val.SS {
				if s == operand.S {
					return true, nil
				}
			}
			return false, nil
		}
		// Number set membership.
		if val.Type == dynago.TypeNS && operand.Type == dynago.TypeN {
			for _, ns := range val.NS {
				if ns == operand.N {
					return true, nil
				}
			}
			return false, nil
		}
		return false, nil

	case "size":
		// size() used as a boolean context doesn't make sense by itself,
		// but it can appear in comparisons. When evaluated standalone,
		// treat it as size > 0.
		sz, err := evalSize(n, item)
		if err != nil {
			return false, err
		}
		return sz > 0, nil

	default:
		return false, fmt.Errorf("eval: unknown function %s", n.Name)
	}
}

func evalSize(n FuncNode, item map[string]dynago.AttributeValue) (int64, error) {
	if len(n.Args) != 1 {
		return 0, fmt.Errorf("eval: size requires 1 argument")
	}
	val, ok, err := resolveNode(n.Args[0], item)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	switch val.Type {
	case dynago.TypeS:
		return int64(len(val.S)), nil
	case dynago.TypeB:
		return int64(len(val.B)), nil
	case dynago.TypeL:
		return int64(len(val.L)), nil
	case dynago.TypeM:
		return int64(len(val.M)), nil
	case dynago.TypeSS:
		return int64(len(val.SS)), nil
	case dynago.TypeNS:
		return int64(len(val.NS)), nil
	case dynago.TypeBS:
		return int64(len(val.BS)), nil
	default:
		return 0, fmt.Errorf("eval: size not supported for type %d", val.Type)
	}
}
