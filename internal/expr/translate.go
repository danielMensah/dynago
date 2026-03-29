package expr

import (
	"fmt"
	"strings"

	"github.com/danielmensah/dynago"
	"github.com/danielmensah/dynago/internal"
)

// translator holds the state for a single ToDynamo call, tracking
// counters for name aliases and value placeholders.
type translator struct {
	names    map[string]string              // e.g. "#Status" -> "Status"
	values   map[string]dynago.AttributeValue // e.g. ":v0" -> AttributeValue
	nameIdx  int                            // counter for name aliases when not reserved
	valueIdx int                            // counter for value placeholders
	// nameCache maps attribute name -> alias to reuse aliases for the same name
	nameCache map[string]string
}

func newTranslator() *translator {
	return &translator{
		names:     make(map[string]string),
		values:    make(map[string]dynago.AttributeValue),
		nameCache: make(map[string]string),
	}
}

// aliasName returns the #alias for an attribute name, reusing an existing
// alias if the same name was seen before. All attribute names get aliases
// for consistency.
func (t *translator) aliasName(name string) string {
	if alias, ok := t.nameCache[name]; ok {
		return alias
	}
	alias := "#" + name
	// If the simple alias collides with an existing entry pointing to a
	// different real name, or if the name is reserved, use a numbered alias.
	if existing, ok := t.names[alias]; ok && existing != name {
		alias = fmt.Sprintf("#n%d", t.nameIdx)
		t.nameIdx++
	} else if internal.IsReserved(name) {
		// Reserved words always get aliased (the simple form is fine).
		// Just make sure we register it.
	}
	t.names[alias] = name
	t.nameCache[name] = alias
	return alias
}

// aliasValue returns the :vN placeholder for a value, incrementing the counter.
func (t *translator) aliasValue(val dynago.AttributeValue) string {
	placeholder := fmt.Sprintf(":v%d", t.valueIdx)
	t.valueIdx++
	t.values[placeholder] = val
	return placeholder
}

// translateNode recursively translates an AST node to a DynamoDB expression string.
func (t *translator) translateNode(n Node) string {
	switch node := n.(type) {
	case CompareNode:
		left := t.translateNode(node.Left)
		right := t.translateNode(node.Right)
		return fmt.Sprintf("%s %s %s", left, node.Op, right)

	case LogicalNode:
		if node.Op == NOT {
			operand := t.translateNode(node.Left)
			return fmt.Sprintf("NOT (%s)", operand)
		}
		left := t.translateNode(node.Left)
		right := t.translateNode(node.Right)
		return fmt.Sprintf("(%s) %s (%s)", left, node.Op, right)

	case FuncNode:
		args := make([]string, len(node.Args))
		for i, arg := range node.Args {
			args[i] = t.translateNode(arg)
		}
		return fmt.Sprintf("%s(%s)", node.Name, strings.Join(args, ", "))

	case PathNode:
		return t.translatePath(node)

	case ValueNode:
		return t.aliasValue(node.Value)

	case UpdateNode:
		return t.translateUpdate([]UpdateNode{node})

	case ProjectionNode:
		return t.translateProjection(node)

	default:
		return ""
	}
}

// translatePath translates a PathNode to aliased form (e.g. #Address.#City).
func (t *translator) translatePath(p PathNode) string {
	parts := make([]string, len(p.Parts))
	for i, part := range p.Parts {
		parts[i] = t.aliasName(part)
	}
	return strings.Join(parts, ".")
}

// translateUpdate translates multiple UpdateNodes grouped by action.
func (t *translator) translateUpdate(nodes []UpdateNode) string {
	// Group by action.
	groups := map[UpdateAction][]string{}
	order := []UpdateAction{}

	for _, node := range nodes {
		path := t.translatePath(node.Path)
		var clause string
		switch node.Action {
		case SET:
			val := t.translateNode(node.Value)
			clause = fmt.Sprintf("%s = %s", path, val)
		case ADD:
			val := t.translateNode(node.Value)
			clause = fmt.Sprintf("%s %s", path, val)
		case REMOVE:
			clause = path
		case DELETE:
			val := t.translateNode(node.Value)
			clause = fmt.Sprintf("%s %s", path, val)
		}
		if _, seen := groups[node.Action]; !seen {
			order = append(order, node.Action)
		}
		groups[node.Action] = append(groups[node.Action], clause)
	}

	var parts []string
	for _, action := range order {
		clauses := groups[action]
		parts = append(parts, fmt.Sprintf("%s %s", action, strings.Join(clauses, ", ")))
	}
	return strings.Join(parts, " ")
}

// translateProjection translates a ProjectionNode to comma-separated aliased paths.
func (t *translator) translateProjection(p ProjectionNode) string {
	paths := make([]string, len(p.Paths))
	for i, path := range p.Paths {
		paths[i] = t.translatePath(path)
	}
	return strings.Join(paths, ", ")
}

// ToDynamo translates an AST node into a DynamoDB expression string,
// ExpressionAttributeNames, and ExpressionAttributeValues maps.
//
// For a slice of UpdateNodes, pass them individually and combine; or use
// ToDynamoUpdates for grouped update translation.
func ToDynamo(node Node) (string, map[string]string, map[string]dynago.AttributeValue) {
	t := newTranslator()
	expr := t.translateNode(node)
	return expr, t.names, t.values
}

// ToDynamoUpdates translates multiple UpdateNodes into a single grouped
// update expression string with shared name/value maps.
func ToDynamoUpdates(nodes []UpdateNode) (string, map[string]string, map[string]dynago.AttributeValue) {
	t := newTranslator()
	expr := t.translateUpdate(nodes)
	return expr, t.names, t.values
}

// ToDynamoProjection translates a ProjectionNode into a projection expression
// string with name aliases.
func ToDynamoProjection(node ProjectionNode) (string, map[string]string) {
	t := newTranslator()
	expr := t.translateProjection(node)
	return expr, t.names
}
