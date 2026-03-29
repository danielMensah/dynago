package expr

import (
	"fmt"
	"strings"

	"github.com/danielmensah/dynago"
)

// ParseCondition parses a placeholder expression like "Active = ? AND Age > ?"
// with the given argument values and returns an AST Node.
func ParseCondition(expression string, args ...any) (Node, error) {
	p := &parser{
		tokens: tokenize(expression),
		args:   args,
	}
	node, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.tokens) {
		return nil, fmt.Errorf("expr: unexpected token %q at position %d", p.tokens[p.pos], p.pos)
	}
	if p.argIdx < len(p.args) {
		return nil, fmt.Errorf("expr: %d unused arguments", len(p.args)-p.argIdx)
	}
	return node, nil
}

// BuildSet returns an UpdateNode for a SET action.
func BuildSet(attr string, val any) UpdateNode {
	return UpdateNode{
		Action: SET,
		Path:   parsePath(attr),
		Value:  ValueNode{Value: goToAttributeValue(val)},
	}
}

// BuildAdd returns an UpdateNode for an ADD action.
func BuildAdd(attr string, val any) UpdateNode {
	return UpdateNode{
		Action: ADD,
		Path:   parsePath(attr),
		Value:  ValueNode{Value: goToAttributeValue(val)},
	}
}

// BuildRemove returns an UpdateNode for a REMOVE action.
func BuildRemove(attr string) UpdateNode {
	return UpdateNode{
		Action: REMOVE,
		Path:   parsePath(attr),
	}
}

// BuildDelete returns an UpdateNode for a DELETE action (set removal).
func BuildDelete(attr string, val any) UpdateNode {
	return UpdateNode{
		Action: DELETE,
		Path:   parsePath(attr),
		Value:  ValueNode{Value: goToAttributeValue(val)},
	}
}

// parsePath splits a dotted path string into a PathNode.
func parsePath(s string) PathNode {
	// Handle #name placeholders: strip the # prefix.
	s = strings.TrimPrefix(s, "#")
	parts := strings.Split(s, ".")
	return PathNode{Parts: parts}
}

// goToAttributeValue converts a Go value to a dynago.AttributeValue.
func goToAttributeValue(v any) dynago.AttributeValue {
	switch val := v.(type) {
	case string:
		return dynago.AttributeValue{Type: dynago.TypeS, S: val}
	case bool:
		return dynago.AttributeValue{Type: dynago.TypeBOOL, BOOL: val}
	case int:
		return dynago.AttributeValue{Type: dynago.TypeN, N: fmt.Sprintf("%d", val)}
	case int8:
		return dynago.AttributeValue{Type: dynago.TypeN, N: fmt.Sprintf("%d", val)}
	case int16:
		return dynago.AttributeValue{Type: dynago.TypeN, N: fmt.Sprintf("%d", val)}
	case int32:
		return dynago.AttributeValue{Type: dynago.TypeN, N: fmt.Sprintf("%d", val)}
	case int64:
		return dynago.AttributeValue{Type: dynago.TypeN, N: fmt.Sprintf("%d", val)}
	case uint:
		return dynago.AttributeValue{Type: dynago.TypeN, N: fmt.Sprintf("%d", val)}
	case uint8:
		return dynago.AttributeValue{Type: dynago.TypeN, N: fmt.Sprintf("%d", val)}
	case uint16:
		return dynago.AttributeValue{Type: dynago.TypeN, N: fmt.Sprintf("%d", val)}
	case uint32:
		return dynago.AttributeValue{Type: dynago.TypeN, N: fmt.Sprintf("%d", val)}
	case uint64:
		return dynago.AttributeValue{Type: dynago.TypeN, N: fmt.Sprintf("%d", val)}
	case float32:
		return dynago.AttributeValue{Type: dynago.TypeN, N: fmt.Sprintf("%g", val)}
	case float64:
		return dynago.AttributeValue{Type: dynago.TypeN, N: fmt.Sprintf("%g", val)}
	case []byte:
		return dynago.AttributeValue{Type: dynago.TypeB, B: val}
	case []string:
		return dynago.AttributeValue{Type: dynago.TypeSS, SS: val}
	case []int:
		ns := make([]string, len(val))
		for i, n := range val {
			ns[i] = fmt.Sprintf("%d", n)
		}
		return dynago.AttributeValue{Type: dynago.TypeNS, NS: ns}
	case dynago.AttributeValue:
		return val
	case nil:
		return dynago.AttributeValue{Type: dynago.TypeNULL, NULL: true}
	default:
		panic(fmt.Sprintf("expr: unsupported value type %T", v))
	}
}

// --- Tokenizer ---

// tokenize splits an expression string into tokens. Tokens are:
// words/identifiers, operators (=, <>, <, <=, >, >=), ?, (, ), comma, dot.
func tokenize(s string) []string {
	var tokens []string
	i := 0
	for i < len(s) {
		ch := s[i]

		// Skip whitespace.
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			i++
			continue
		}

		// Two-character operators.
		if i+1 < len(s) {
			two := s[i : i+2]
			if two == "<>" || two == "<=" || two == ">=" {
				tokens = append(tokens, two)
				i += 2
				continue
			}
		}

		// Single-character tokens.
		switch ch {
		case '?', '(', ')', ',', '=', '<', '>':
			tokens = append(tokens, string(ch))
			i++
			continue
		}

		// Identifier or keyword (including #name and dotted paths like Address.City).
		if isIdentStart(ch) || ch == '#' {
			start := i
			i++
			for i < len(s) && (isIdentPart(s[i]) || s[i] == '.') {
				i++
			}
			tokens = append(tokens, s[start:i])
			continue
		}

		// Numbers (for literal numbers in expressions).
		if ch >= '0' && ch <= '9' {
			start := i
			for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || s[i] == '.') {
				i++
			}
			tokens = append(tokens, s[start:i])
			continue
		}

		// Unknown character - skip it.
		i++
	}
	return tokens
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || (ch >= '0' && ch <= '9')
}

// --- Parser ---

type parser struct {
	tokens []string
	pos    int
	args   []any
	argIdx int
}

func (p *parser) peek() string {
	if p.pos >= len(p.tokens) {
		return ""
	}
	return p.tokens[p.pos]
}

func (p *parser) advance() string {
	tok := p.tokens[p.pos]
	p.pos++
	return tok
}

func (p *parser) expect(tok string) error {
	if p.pos >= len(p.tokens) {
		return fmt.Errorf("expr: expected %q but reached end of expression", tok)
	}
	if p.tokens[p.pos] != tok {
		return fmt.Errorf("expr: expected %q but got %q", tok, p.tokens[p.pos])
	}
	p.pos++
	return nil
}

// parseOr handles OR (lowest precedence).
func (p *parser) parseOr() (Node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for strings.EqualFold(p.peek(), "OR") {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = LogicalNode{Op: OR, Left: left, Right: right}
	}
	return left, nil
}

// parseAnd handles AND.
func (p *parser) parseAnd() (Node, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for strings.EqualFold(p.peek(), "AND") {
		p.advance()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = LogicalNode{Op: AND, Left: left, Right: right}
	}
	return left, nil
}

// parseNot handles NOT (highest precedence among logical ops).
func (p *parser) parseNot() (Node, error) {
	if strings.EqualFold(p.peek(), "NOT") {
		p.advance()
		operand, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return LogicalNode{Op: NOT, Left: operand}, nil
	}
	return p.parseComparison()
}

// parseComparison handles comparison operators and function calls.
func (p *parser) parseComparison() (Node, error) {
	if p.peek() == "(" {
		p.advance()
		inner, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if err := p.expect(")"); err != nil {
			return nil, err
		}
		return inner, nil
	}

	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	// Check for comparison operator.
	op, ok := parseCompareOp(p.peek())
	if ok {
		p.advance()
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return CompareNode{Left: left, Op: op, Right: right}, nil
	}

	// It might be a standalone function call (e.g., attribute_exists(Email))
	// which was already parsed in parsePrimary as a FuncNode.
	return left, nil
}

// parsePrimary parses a value placeholder, path, or function call.
func (p *parser) parsePrimary() (Node, error) {
	tok := p.peek()
	if tok == "" {
		return nil, fmt.Errorf("expr: unexpected end of expression")
	}

	// Value placeholder.
	if tok == "?" {
		p.advance()
		if p.argIdx >= len(p.args) {
			return nil, fmt.Errorf("expr: not enough arguments for placeholder")
		}
		val := p.args[p.argIdx]
		p.argIdx++
		return ValueNode{Value: goToAttributeValue(val)}, nil
	}

	// Identifier - could be a function call or a path.
	if isIdentStart(tok[0]) || tok[0] == '#' {
		p.advance()

		// Check if this is a function call.
		if p.peek() == "(" {
			return p.parseFuncCall(tok)
		}

		// It's a path.
		return parsePathNode(tok), nil
	}

	return nil, fmt.Errorf("expr: unexpected token %q", tok)
}

// parseFuncCall parses a function call. The function name has already been consumed.
func (p *parser) parseFuncCall(name string) (Node, error) {
	if err := p.expect("("); err != nil {
		return nil, err
	}

	var args []Node
	for p.peek() != ")" {
		if len(args) > 0 {
			if err := p.expect(","); err != nil {
				return nil, err
			}
		}
		arg, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}
	if err := p.expect(")"); err != nil {
		return nil, err
	}

	return FuncNode{Name: name, Args: args}, nil
}

// parsePathNode converts a possibly dotted string to a PathNode.
func parsePathNode(tok string) PathNode {
	// Strip # prefix for reserved word placeholders.
	tok = strings.TrimPrefix(tok, "#")
	parts := strings.Split(tok, ".")
	return PathNode{Parts: parts}
}

// parseCompareOp converts a token to a CompareOp.
func parseCompareOp(tok string) (CompareOp, bool) {
	switch tok {
	case "=":
		return EQ, true
	case "<>":
		return NE, true
	case "<":
		return LT, true
	case "<=":
		return LE, true
	case ">":
		return GT, true
	case ">=":
		return GE, true
	default:
		return 0, false
	}
}
