package memdb

import (
	"fmt"
	"strings"

	"github.com/danielmensah/dynago"
	"github.com/danielmensah/dynago/internal/expr"
)

// evalCondition parses and evaluates a DynamoDB condition expression string
// with #name and :value substitution against an item.
func evalCondition(expression string, names map[string]string, values map[string]dynago.AttributeValue, item map[string]dynago.AttributeValue) (bool, error) {
	if expression == "" {
		return true, nil
	}

	node, err := parseExpressionToAST(expression, names, values)
	if err != nil {
		return false, err
	}

	return expr.Eval(node, item)
}

// parseExpressionToAST converts a DynamoDB expression string with #name/#value
// placeholders into an AST that the expr package can evaluate.
func parseExpressionToAST(expression string, names map[string]string, values map[string]dynago.AttributeValue) (expr.Node, error) {
	p := &exprParser{
		tokens: tokenizeExpr(expression),
		names:  names,
		values: values,
	}
	node, err := p.parseOr()
	if err != nil {
		return nil, fmt.Errorf("memdb: expression parse error: %w", err)
	}
	if p.pos < len(p.tokens) {
		return nil, fmt.Errorf("memdb: unexpected token %q at position %d", p.tokens[p.pos], p.pos)
	}
	return node, nil
}

// tokenizeExpr splits a DynamoDB expression into tokens, preserving #name and :value placeholders.
func tokenizeExpr(s string) []string {
	var tokens []string
	i := 0
	for i < len(s) {
		ch := s[i]

		// Skip whitespace
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			i++
			continue
		}

		// Two-character operators
		if i+1 < len(s) {
			two := s[i : i+2]
			if two == "<>" || two == "<=" || two == ">=" {
				tokens = append(tokens, two)
				i += 2
				continue
			}
		}

		// Single-character tokens
		switch ch {
		case '(', ')', ',', '=', '<', '>', '+', '-':
			tokens = append(tokens, string(ch))
			i++
			continue
		}

		// :value placeholder
		if ch == ':' {
			start := i
			i++
			for i < len(s) && isIdentPart(s[i]) {
				i++
			}
			tokens = append(tokens, s[start:i])
			continue
		}

		// #name placeholder
		if ch == '#' {
			start := i
			i++
			for i < len(s) && isIdentPart(s[i]) {
				i++
			}
			tokens = append(tokens, s[start:i])
			continue
		}

		// Identifiers/keywords
		if isIdentStart(ch) {
			start := i
			i++
			for i < len(s) && (isIdentPart(s[i]) || s[i] == '.') {
				i++
			}
			tokens = append(tokens, s[start:i])
			continue
		}

		// Numbers
		if ch >= '0' && ch <= '9' {
			start := i
			i++
			for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || s[i] == '.') {
				i++
			}
			tokens = append(tokens, s[start:i])
			continue
		}

		i++ // skip unknown
	}
	return tokens
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || (ch >= '0' && ch <= '9')
}

// exprParser parses DynamoDB expression strings into AST nodes.
type exprParser struct {
	tokens []string
	pos    int
	names  map[string]string
	values map[string]dynago.AttributeValue
}

func (p *exprParser) peek() string {
	if p.pos >= len(p.tokens) {
		return ""
	}
	return p.tokens[p.pos]
}

func (p *exprParser) advance() string {
	tok := p.tokens[p.pos]
	p.pos++
	return tok
}

func (p *exprParser) expect(tok string) error {
	if p.pos >= len(p.tokens) {
		return fmt.Errorf("expected %q but reached end", tok)
	}
	if p.tokens[p.pos] != tok {
		return fmt.Errorf("expected %q but got %q", tok, p.tokens[p.pos])
	}
	p.pos++
	return nil
}

// parseOr handles OR (lowest precedence).
func (p *exprParser) parseOr() (expr.Node, error) {
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
		left = expr.LogicalNode{Op: expr.OR, Left: left, Right: right}
	}
	return left, nil
}

// parseAnd handles AND.
func (p *exprParser) parseAnd() (expr.Node, error) {
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
		left = expr.LogicalNode{Op: expr.AND, Left: left, Right: right}
	}
	return left, nil
}

// parseNot handles NOT.
func (p *exprParser) parseNot() (expr.Node, error) {
	if strings.EqualFold(p.peek(), "NOT") {
		p.advance()
		operand, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return expr.LogicalNode{Op: expr.NOT, Left: operand}, nil
	}
	return p.parseComparison()
}

// parseComparison handles comparison and function calls.
func (p *exprParser) parseComparison() (expr.Node, error) {
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

	// Check for BETWEEN
	if strings.EqualFold(p.peek(), "BETWEEN") {
		p.advance()
		low, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(p.peek(), "AND") {
			return nil, fmt.Errorf("expected AND in BETWEEN expression")
		}
		p.advance()
		high, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return expr.LogicalNode{
			Op:    expr.AND,
			Left:  expr.CompareNode{Left: left, Op: expr.GE, Right: low},
			Right: expr.CompareNode{Left: left, Op: expr.LE, Right: high},
		}, nil
	}

	// Check for IN
	if strings.EqualFold(p.peek(), "IN") {
		p.advance()
		if err := p.expect("("); err != nil {
			return nil, err
		}
		var node expr.Node
		first := true
		for p.peek() != ")" {
			if !first {
				if err := p.expect(","); err != nil {
					return nil, err
				}
			}
			val, err := p.parsePrimary()
			if err != nil {
				return nil, err
			}
			cmp := expr.CompareNode{Left: left, Op: expr.EQ, Right: val}
			if node == nil {
				node = cmp
			} else {
				node = expr.LogicalNode{Op: expr.OR, Left: node, Right: cmp}
			}
			first = false
		}
		if err := p.expect(")"); err != nil {
			return nil, err
		}
		if node == nil {
			return expr.CompareNode{
				Left:  expr.ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: ""}},
				Op:    expr.NE,
				Right: expr.ValueNode{Value: dynago.AttributeValue{Type: dynago.TypeS, S: ""}},
			}, nil
		}
		return node, nil
	}

	// Check for comparison operator
	op, ok := parseCompareOp(p.peek())
	if ok {
		p.advance()
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return expr.CompareNode{Left: left, Op: op, Right: right}, nil
	}

	// Standalone expression (e.g., function call)
	return left, nil
}

// parsePrimary parses a primary expression: value, path, or function call.
func (p *exprParser) parsePrimary() (expr.Node, error) {
	tok := p.peek()
	if tok == "" {
		return nil, fmt.Errorf("unexpected end of expression")
	}

	// :value placeholder
	if tok[0] == ':' {
		p.advance()
		v, ok := p.values[tok]
		if !ok {
			return nil, fmt.Errorf("unknown value placeholder %q", tok)
		}
		return expr.ValueNode{Value: v}, nil
	}

	// #name placeholder
	if tok[0] == '#' {
		p.advance()
		resolved, ok := p.names[tok]
		if !ok {
			return nil, fmt.Errorf("unknown name placeholder %q", tok)
		}
		// Check if this is a function call
		if p.peek() == "(" {
			return p.parseFuncCall(resolved)
		}
		return expr.PathNode{Parts: strings.Split(resolved, ".")}, nil
	}

	// Function or path identifier
	if isIdentStart(tok[0]) {
		p.advance()

		// Known function names
		lower := strings.ToLower(tok)
		switch lower {
		case "attribute_exists", "attribute_not_exists", "begins_with", "contains", "size", "attribute_type":
			if p.peek() == "(" {
				return p.parseFuncCall(tok)
			}
		}

		return expr.PathNode{Parts: strings.Split(tok, ".")}, nil
	}

	return nil, fmt.Errorf("unexpected token %q", tok)
}

func (p *exprParser) parseFuncCall(name string) (expr.Node, error) {
	if err := p.expect("("); err != nil {
		return nil, err
	}
	var args []expr.Node
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

	return expr.FuncNode{Name: strings.ToLower(name), Args: args}, nil
}

func parseCompareOp(tok string) (expr.CompareOp, bool) {
	switch tok {
	case "=":
		return expr.EQ, true
	case "<>":
		return expr.NE, true
	case "<":
		return expr.LT, true
	case "<=":
		return expr.LE, true
	case ">":
		return expr.GT, true
	case ">=":
		return expr.GE, true
	default:
		return 0, false
	}
}

// --- Update expression parsing ---

// updateValue is an interface for values in update expressions.
// Unlike expr.Node, these can be extended in this package.
type updateValue interface {
	resolveValue(item map[string]dynago.AttributeValue) (dynago.AttributeValue, error)
}

// literalValue wraps a concrete AttributeValue.
type literalValue struct {
	val dynago.AttributeValue
}

func (v literalValue) resolveValue(_ map[string]dynago.AttributeValue) (dynago.AttributeValue, error) {
	return v.val, nil
}

// pathValue looks up a path in the item.
type pathValue struct {
	parts []string
}

func (v pathValue) resolveValue(item map[string]dynago.AttributeValue) (dynago.AttributeValue, error) {
	av, ok := resolvePathValue(v.parts, item)
	if !ok {
		return dynago.AttributeValue{}, fmt.Errorf("memdb: path %v not found", v.parts)
	}
	return av, nil
}

// arithmeticValue performs + or - on two values.
type arithmeticValue struct {
	left  updateValue
	op    string
	right updateValue
}

func (v *arithmeticValue) resolveValue(item map[string]dynago.AttributeValue) (dynago.AttributeValue, error) {
	left, err := v.left.resolveValue(item)
	if err != nil {
		return dynago.AttributeValue{}, err
	}
	right, err := v.right.resolveValue(item)
	if err != nil {
		return dynago.AttributeValue{}, err
	}
	if left.Type != dynago.TypeN || right.Type != dynago.TypeN {
		return dynago.AttributeValue{}, fmt.Errorf("memdb: arithmetic requires number types")
	}
	var result string
	switch v.op {
	case "+":
		result, err = addNumbers(left.N, right.N)
	case "-":
		result, err = subtractNumbers(left.N, right.N)
	default:
		return dynago.AttributeValue{}, fmt.Errorf("memdb: unknown arithmetic op %q", v.op)
	}
	if err != nil {
		return dynago.AttributeValue{}, err
	}
	return dynago.AttributeValue{Type: dynago.TypeN, N: result}, nil
}

// ifNotExistsValue resolves to the path's value if it exists, or the default.
type ifNotExistsValue struct {
	path       []string
	defaultVal updateValue
}

func (v *ifNotExistsValue) resolveValue(item map[string]dynago.AttributeValue) (dynago.AttributeValue, error) {
	av, ok := resolvePathValue(v.path, item)
	if ok {
		return av, nil
	}
	return v.defaultVal.resolveValue(item)
}

// listAppendValue concatenates two lists.
type listAppendValue struct {
	left  updateValue
	right updateValue
}

func (v *listAppendValue) resolveValue(item map[string]dynago.AttributeValue) (dynago.AttributeValue, error) {
	left, err := v.left.resolveValue(item)
	if err != nil {
		return dynago.AttributeValue{}, err
	}
	right, err := v.right.resolveValue(item)
	if err != nil {
		return dynago.AttributeValue{}, err
	}
	if left.Type != dynago.TypeL || right.Type != dynago.TypeL {
		return dynago.AttributeValue{}, fmt.Errorf("memdb: list_append requires list types")
	}
	merged := make([]dynago.AttributeValue, 0, len(left.L)+len(right.L))
	merged = append(merged, left.L...)
	merged = append(merged, right.L...)
	return dynago.AttributeValue{Type: dynago.TypeL, L: merged}, nil
}

// updateOp represents a parsed update operation.
type updateOp struct {
	action expr.UpdateAction
	path   expr.PathNode
	value  updateValue // nil for REMOVE
}

// parseUpdateExpression parses a DynamoDB update expression string.
func parseUpdateExpression(expression string, names map[string]string, values map[string]dynago.AttributeValue) ([]updateOp, error) {
	tokens := tokenizeExpr(expression)
	p := &updateParser{
		tokens: tokens,
		names:  names,
		values: values,
	}
	return p.parse()
}

type updateParser struct {
	tokens []string
	pos    int
	names  map[string]string
	values map[string]dynago.AttributeValue
}

func (p *updateParser) peek() string {
	if p.pos >= len(p.tokens) {
		return ""
	}
	return p.tokens[p.pos]
}

func (p *updateParser) advance() string {
	tok := p.tokens[p.pos]
	p.pos++
	return tok
}

func (p *updateParser) parse() ([]updateOp, error) {
	var ops []updateOp
	for p.pos < len(p.tokens) {
		tok := p.peek()
		upper := strings.ToUpper(tok)
		switch upper {
		case "SET":
			p.advance()
			setOps, err := p.parseSetClauses()
			if err != nil {
				return nil, err
			}
			ops = append(ops, setOps...)
		case "ADD":
			p.advance()
			addOps, err := p.parseAddDeleteClauses(expr.ADD)
			if err != nil {
				return nil, err
			}
			ops = append(ops, addOps...)
		case "REMOVE":
			p.advance()
			removeOps, err := p.parseRemoveClauses()
			if err != nil {
				return nil, err
			}
			ops = append(ops, removeOps...)
		case "DELETE":
			p.advance()
			delOps, err := p.parseAddDeleteClauses(expr.DELETE)
			if err != nil {
				return nil, err
			}
			ops = append(ops, delOps...)
		default:
			return nil, fmt.Errorf("memdb: unexpected token %q in update expression", tok)
		}
	}
	return ops, nil
}

func (p *updateParser) isActionKeyword(tok string) bool {
	upper := strings.ToUpper(tok)
	return upper == "SET" || upper == "ADD" || upper == "REMOVE" || upper == "DELETE"
}

func (p *updateParser) resolvePath(tok string) expr.PathNode {
	if len(tok) > 0 && tok[0] == '#' {
		if resolved, ok := p.names[tok]; ok {
			return expr.PathNode{Parts: strings.Split(resolved, ".")}
		}
	}
	return expr.PathNode{Parts: strings.Split(tok, ".")}
}

func (p *updateParser) resolveVal(tok string) (updateValue, error) {
	if len(tok) > 0 && tok[0] == ':' {
		v, ok := p.values[tok]
		if !ok {
			return nil, fmt.Errorf("memdb: unknown value placeholder %q", tok)
		}
		return literalValue{val: v}, nil
	}
	if len(tok) > 0 && tok[0] == '#' {
		if resolved, ok := p.names[tok]; ok {
			return pathValue{parts: strings.Split(resolved, ".")}, nil
		}
		return nil, fmt.Errorf("memdb: unknown name placeholder %q", tok)
	}
	return pathValue{parts: strings.Split(tok, ".")}, nil
}

func (p *updateParser) parseSetClauses() ([]updateOp, error) {
	var ops []updateOp
	for {
		if p.peek() == "" || p.isActionKeyword(p.peek()) {
			break
		}
		pathTok := p.advance()
		path := p.resolvePath(pathTok)

		if p.peek() != "=" {
			return nil, fmt.Errorf("memdb: expected '=' after path in SET, got %q", p.peek())
		}
		p.advance()

		val, err := p.parseSetValue()
		if err != nil {
			return nil, err
		}

		ops = append(ops, updateOp{action: expr.SET, path: path, value: val})

		if p.peek() == "," {
			p.advance()
		}
	}
	return ops, nil
}

func (p *updateParser) parseSetValue() (updateValue, error) {
	first, err := p.parseSetAtom()
	if err != nil {
		return nil, err
	}

	// Check for arithmetic: path + :val, path - :val
	if p.peek() == "+" || p.peek() == "-" {
		op := p.advance()
		second, err := p.parseSetAtom()
		if err != nil {
			return nil, err
		}
		return &arithmeticValue{left: first, op: op, right: second}, nil
	}

	return first, nil
}

func (p *updateParser) parseSetAtom() (updateValue, error) {
	tok := p.peek()
	if tok == "" {
		return nil, fmt.Errorf("memdb: unexpected end of expression")
	}

	if tok[0] == ':' {
		p.advance()
		return p.resolveVal(tok)
	}

	if tok[0] == '#' {
		p.advance()
		// Could be a function call or a path reference
		if p.peek() == "(" {
			resolved := tok
			if r, ok := p.names[tok]; ok {
				resolved = r
			}
			return p.parseSetFunction(resolved)
		}
		if resolved, ok := p.names[tok]; ok {
			return pathValue{parts: strings.Split(resolved, ".")}, nil
		}
		return nil, fmt.Errorf("memdb: unknown name placeholder %q", tok)
	}

	if isIdentStart(tok[0]) {
		p.advance()
		if p.peek() == "(" {
			return p.parseSetFunction(tok)
		}
		return pathValue{parts: strings.Split(tok, ".")}, nil
	}

	return nil, fmt.Errorf("memdb: unexpected token %q in SET value", tok)
}

func (p *updateParser) parseSetFunction(name string) (updateValue, error) {
	p.advance() // consume (
	lower := strings.ToLower(name)

	switch lower {
	case "if_not_exists":
		pathTok := p.advance()
		path := p.resolvePath(pathTok)

		if p.peek() != "," {
			return nil, fmt.Errorf("memdb: expected ',' in if_not_exists")
		}
		p.advance()

		val, err := p.parseSetValue()
		if err != nil {
			return nil, err
		}
		if p.peek() != ")" {
			return nil, fmt.Errorf("memdb: expected ')' in if_not_exists")
		}
		p.advance()
		return &ifNotExistsValue{path: path.Parts, defaultVal: val}, nil

	case "list_append":
		first, err := p.parseSetValue()
		if err != nil {
			return nil, err
		}
		if p.peek() != "," {
			return nil, fmt.Errorf("memdb: expected ',' in list_append")
		}
		p.advance()
		second, err := p.parseSetValue()
		if err != nil {
			return nil, err
		}
		if p.peek() != ")" {
			return nil, fmt.Errorf("memdb: expected ')' in list_append")
		}
		p.advance()
		return &listAppendValue{left: first, right: second}, nil

	default:
		return nil, fmt.Errorf("memdb: unknown SET function %q", name)
	}
}

func (p *updateParser) parseAddDeleteClauses(action expr.UpdateAction) ([]updateOp, error) {
	var ops []updateOp
	for {
		if p.peek() == "" || p.isActionKeyword(p.peek()) {
			break
		}
		pathTok := p.advance()
		path := p.resolvePath(pathTok)

		valTok := p.peek()
		if valTok == "" || p.isActionKeyword(valTok) {
			return nil, fmt.Errorf("memdb: expected value after path in %s", action)
		}
		p.advance()
		val, err := p.resolveVal(valTok)
		if err != nil {
			return nil, err
		}

		ops = append(ops, updateOp{action: action, path: path, value: val})

		if p.peek() == "," {
			p.advance()
		}
	}
	return ops, nil
}

func (p *updateParser) parseRemoveClauses() ([]updateOp, error) {
	var ops []updateOp
	for {
		if p.peek() == "" || p.isActionKeyword(p.peek()) {
			break
		}
		pathTok := p.advance()
		path := p.resolvePath(pathTok)
		ops = append(ops, updateOp{action: expr.REMOVE, path: path})

		if p.peek() == "," {
			p.advance()
		}
	}
	return ops, nil
}

// evalUpdateNodes applies parsed update operations to an item.
func evalUpdateNodes(ops []updateOp, item map[string]dynago.AttributeValue) (map[string]dynago.AttributeValue, error) {
	result := deepCopyItem(item)

	for _, op := range ops {
		switch op.action {
		case expr.SET:
			val, err := op.value.resolveValue(result)
			if err != nil {
				return nil, err
			}
			setNestedPath(op.path.Parts, result, val)

		case expr.ADD:
			val, err := op.value.resolveValue(result)
			if err != nil {
				return nil, err
			}
			existing, exists := resolvePathValue(op.path.Parts, result)
			if !exists {
				setNestedPath(op.path.Parts, result, val)
			} else if existing.Type == dynago.TypeN && val.Type == dynago.TypeN {
				sum, err := addNumbers(existing.N, val.N)
				if err != nil {
					return nil, err
				}
				setNestedPath(op.path.Parts, result, dynago.AttributeValue{Type: dynago.TypeN, N: sum})
			} else if existing.Type == dynago.TypeSS && val.Type == dynago.TypeSS {
				merged := addToStringSet(existing.SS, val.SS)
				setNestedPath(op.path.Parts, result, dynago.AttributeValue{Type: dynago.TypeSS, SS: merged})
			} else if existing.Type == dynago.TypeNS && val.Type == dynago.TypeNS {
				merged := addToStringSet(existing.NS, val.NS)
				setNestedPath(op.path.Parts, result, dynago.AttributeValue{Type: dynago.TypeNS, NS: merged})
			} else {
				return nil, fmt.Errorf("memdb: ADD not supported for types %d and %d", existing.Type, val.Type)
			}

		case expr.REMOVE:
			removeNestedPath(op.path.Parts, result)

		case expr.DELETE:
			val, err := op.value.resolveValue(result)
			if err != nil {
				return nil, err
			}
			existing, exists := resolvePathValue(op.path.Parts, result)
			if !exists {
				continue
			}
			if existing.Type == dynago.TypeSS && val.Type == dynago.TypeSS {
				filtered := removeFromStringSet(existing.SS, val.SS)
				setNestedPath(op.path.Parts, result, dynago.AttributeValue{Type: dynago.TypeSS, SS: filtered})
			} else if existing.Type == dynago.TypeNS && val.Type == dynago.TypeNS {
				filtered := removeFromStringSet(existing.NS, val.NS)
				setNestedPath(op.path.Parts, result, dynago.AttributeValue{Type: dynago.TypeNS, NS: filtered})
			}
		}
	}

	return result, nil
}

// --- Helpers for path resolution and mutation ---

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

func setNestedPath(parts []string, item map[string]dynago.AttributeValue, val dynago.AttributeValue) {
	if len(parts) == 1 {
		item[parts[0]] = val
		return
	}
	cur, ok := item[parts[0]]
	if !ok {
		inner := make(map[string]dynago.AttributeValue)
		setNestedPath(parts[1:], inner, val)
		item[parts[0]] = dynago.AttributeValue{Type: dynago.TypeM, M: inner}
		return
	}
	if cur.Type != dynago.TypeM {
		return
	}
	newMap := make(map[string]dynago.AttributeValue, len(cur.M))
	for k, v := range cur.M {
		newMap[k] = v
	}
	setNestedPath(parts[1:], newMap, val)
	item[parts[0]] = dynago.AttributeValue{Type: dynago.TypeM, M: newMap}
}

func removeNestedPath(parts []string, item map[string]dynago.AttributeValue) {
	if len(parts) == 1 {
		delete(item, parts[0])
		return
	}
	cur, ok := item[parts[0]]
	if !ok || cur.Type != dynago.TypeM {
		return
	}
	newMap := make(map[string]dynago.AttributeValue, len(cur.M))
	for k, v := range cur.M {
		newMap[k] = v
	}
	removeNestedPath(parts[1:], newMap)
	item[parts[0]] = dynago.AttributeValue{Type: dynago.TypeM, M: newMap}
}

// --- Number arithmetic helpers ---

func addNumbers(a, b string) (string, error) {
	af, err := parseFloat(a)
	if err != nil {
		return "", err
	}
	bf, err := parseFloat(b)
	if err != nil {
		return "", err
	}
	return formatFloat(af + bf), nil
}

func subtractNumbers(a, b string) (string, error) {
	af, err := parseFloat(a)
	if err != nil {
		return "", err
	}
	bf, err := parseFloat(b)
	if err != nil {
		return "", err
	}
	return formatFloat(af - bf), nil
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%g", &f)
	return f, err
}

func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%g", f)
}

// --- Set helpers ---

func addToStringSet(existing, add []string) []string {
	set := make(map[string]struct{}, len(existing))
	for _, s := range existing {
		set[s] = struct{}{}
	}
	result := make([]string, len(existing))
	copy(result, existing)
	for _, s := range add {
		if _, ok := set[s]; !ok {
			result = append(result, s)
			set[s] = struct{}{}
		}
	}
	return result
}

func removeFromStringSet(existing, remove []string) []string {
	toRemove := make(map[string]struct{}, len(remove))
	for _, s := range remove {
		toRemove[s] = struct{}{}
	}
	var result []string
	for _, s := range existing {
		if _, ok := toRemove[s]; !ok {
			result = append(result, s)
		}
	}
	return result
}
