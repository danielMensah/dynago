package dynago

import (
	"fmt"
	"strings"
)

// conditionExpr holds the translated components of a condition expression
// ready to be applied to a request.
type conditionExpr struct {
	expression string
	names      map[string]string
	values     map[string]AttributeValue
}

// buildCondition parses a simple condition expression with ? placeholders and
// translates it into DynamoDB expression components. This is a lightweight
// implementation that avoids importing internal/expr (which would create an
// import cycle since internal/expr imports dynago for AttributeValue).
func buildCondition(expression string, vals ...any) (*conditionExpr, error) {
	b := &condBuilder{
		names:  make(map[string]string),
		values: make(map[string]AttributeValue),
	}

	result, err := b.translate(expression, vals)
	if err != nil {
		return nil, err
	}

	return &conditionExpr{
		expression: result,
		names:      b.names,
		values:     b.values,
	}, nil
}

// buildAttrNotExists builds a condition expression for attribute_not_exists.
func buildAttrNotExists(attr string) *conditionExpr {
	alias := "#" + attr
	return &conditionExpr{
		expression: fmt.Sprintf("attribute_not_exists(%s)", alias),
		names:      map[string]string{alias: attr},
		values:     map[string]AttributeValue{},
	}
}

// mergeConditions combines two condition expressions with AND.
func mergeConditions(a, b *conditionExpr) *conditionExpr {
	merged := &conditionExpr{
		expression: fmt.Sprintf("(%s) AND (%s)", a.expression, b.expression),
		names:      make(map[string]string, len(a.names)+len(b.names)),
		values:     make(map[string]AttributeValue, len(a.values)+len(b.values)),
	}
	for k, v := range a.names {
		merged.names[k] = v
	}
	for k, v := range b.names {
		merged.names[k] = v
	}
	for k, v := range a.values {
		merged.values[k] = v
	}
	for k, v := range b.values {
		merged.values[k] = v
	}
	return merged
}

type condBuilder struct {
	names    map[string]string
	values   map[string]AttributeValue
	valueIdx int
}

// translate processes an expression string, replacing attribute names with
// aliases and ? placeholders with value references.
func (cb *condBuilder) translate(expression string, vals []any) (string, error) {
	tokens := tokenizeExpr(expression)
	valIdx := 0
	var out []string

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]

		// Keywords pass through.
		upper := strings.ToUpper(tok)
		if upper == "AND" || upper == "OR" || upper == "NOT" {
			out = append(out, upper)
			continue
		}

		// Operators pass through.
		if isOperator(tok) {
			out = append(out, tok)
			continue
		}

		// Parentheses and commas pass through.
		if tok == "(" || tok == ")" || tok == "," {
			out = append(out, tok)
			continue
		}

		// Value placeholder.
		if tok == "?" {
			if valIdx >= len(vals) {
				return "", fmt.Errorf("dynago: not enough values for placeholder at position %d", i)
			}
			placeholder := fmt.Sprintf(":v%d", cb.valueIdx)
			cb.valueIdx++
			cb.values[placeholder] = anyToAttributeValue(vals[valIdx])
			valIdx++
			out = append(out, placeholder)
			continue
		}

		// Function name: check if next token is '('.
		if i+1 < len(tokens) && tokens[i+1] == "(" {
			out = append(out, tok)
			continue
		}

		// Attribute name: alias it.
		alias := "#" + tok
		cb.names[alias] = tok
		out = append(out, alias)
	}

	if valIdx < len(vals) {
		return "", fmt.Errorf("dynago: %d unused values", len(vals)-valIdx)
	}

	return strings.Join(out, " "), nil
}

func isOperator(tok string) bool {
	switch tok {
	case "=", "<>", "<", "<=", ">", ">=":
		return true
	}
	return false
}

// tokenizeExpr splits an expression string into tokens.
func tokenizeExpr(s string) []string {
	var tokens []string
	i := 0
	for i < len(s) {
		ch := s[i]

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

		switch ch {
		case '?', '(', ')', ',', '=', '<', '>':
			tokens = append(tokens, string(ch))
			i++
			continue
		}

		// Identifier (including dotted paths).
		if isCondIdentStart(ch) || ch == '#' {
			start := i
			i++
			for i < len(s) && (isCondIdentPart(s[i]) || s[i] == '.' || s[i] == '_') {
				i++
			}
			tokens = append(tokens, s[start:i])
			continue
		}

		// Numbers.
		if ch >= '0' && ch <= '9' {
			start := i
			for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || s[i] == '.') {
				i++
			}
			tokens = append(tokens, s[start:i])
			continue
		}

		i++
	}
	return tokens
}

func isCondIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isCondIdentPart(ch byte) bool {
	return isCondIdentStart(ch) || (ch >= '0' && ch <= '9')
}

// anyToAttributeValue converts a Go value to an AttributeValue for use in
// condition expressions.
func anyToAttributeValue(v any) AttributeValue {
	switch val := v.(type) {
	case string:
		return AttributeValue{Type: TypeS, S: val}
	case bool:
		return AttributeValue{Type: TypeBOOL, BOOL: val}
	case int:
		return AttributeValue{Type: TypeN, N: fmt.Sprintf("%d", val)}
	case int8:
		return AttributeValue{Type: TypeN, N: fmt.Sprintf("%d", val)}
	case int16:
		return AttributeValue{Type: TypeN, N: fmt.Sprintf("%d", val)}
	case int32:
		return AttributeValue{Type: TypeN, N: fmt.Sprintf("%d", val)}
	case int64:
		return AttributeValue{Type: TypeN, N: fmt.Sprintf("%d", val)}
	case uint:
		return AttributeValue{Type: TypeN, N: fmt.Sprintf("%d", val)}
	case uint8:
		return AttributeValue{Type: TypeN, N: fmt.Sprintf("%d", val)}
	case uint16:
		return AttributeValue{Type: TypeN, N: fmt.Sprintf("%d", val)}
	case uint32:
		return AttributeValue{Type: TypeN, N: fmt.Sprintf("%d", val)}
	case uint64:
		return AttributeValue{Type: TypeN, N: fmt.Sprintf("%d", val)}
	case float32:
		return AttributeValue{Type: TypeN, N: fmt.Sprintf("%g", val)}
	case float64:
		return AttributeValue{Type: TypeN, N: fmt.Sprintf("%g", val)}
	case []byte:
		return AttributeValue{Type: TypeB, B: val}
	case []string:
		return AttributeValue{Type: TypeSS, SS: val}
	case AttributeValue:
		return val
	case nil:
		return AttributeValue{Type: TypeNULL, NULL: true}
	default:
		panic(fmt.Sprintf("dynago: unsupported condition value type %T", v))
	}
}
