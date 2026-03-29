// Package expr defines the expression AST used to represent DynamoDB
// condition, filter, projection, and update expressions.
package expr

import "github.com/danielmensah/dynago"

// Node is the interface implemented by all AST node types.
// The unexported marker method prevents external packages from implementing it.
type Node interface {
	node() // private marker
}

// CompareOp identifies a comparison operator.
type CompareOp int

const (
	EQ CompareOp = iota + 1
	NE
	LT
	LE
	GT
	GE
)

// String returns the DynamoDB operator symbol.
func (op CompareOp) String() string {
	switch op {
	case EQ:
		return "="
	case NE:
		return "<>"
	case LT:
		return "<"
	case LE:
		return "<="
	case GT:
		return ">"
	case GE:
		return ">="
	default:
		return "?"
	}
}

// LogicalOp identifies a logical operator.
type LogicalOp int

const (
	AND LogicalOp = iota + 1
	OR
	NOT
)

// String returns the DynamoDB keyword.
func (op LogicalOp) String() string {
	switch op {
	case AND:
		return "AND"
	case OR:
		return "OR"
	case NOT:
		return "NOT"
	default:
		return "?"
	}
}

// UpdateAction identifies a DynamoDB update action.
type UpdateAction int

const (
	SET UpdateAction = iota + 1
	ADD
	REMOVE
	DELETE
)

// String returns the action keyword.
func (a UpdateAction) String() string {
	switch a {
	case SET:
		return "SET"
	case ADD:
		return "ADD"
	case REMOVE:
		return "REMOVE"
	case DELETE:
		return "DELETE"
	default:
		return "?"
	}
}

// CompareNode represents a comparison expression: Left op Right.
type CompareNode struct {
	Left  Node
	Op    CompareOp
	Right Node
}

func (CompareNode) node() {}

// LogicalNode represents a logical expression. Right is nil for NOT.
type LogicalNode struct {
	Op    LogicalOp
	Left  Node
	Right Node // nil for NOT
}

func (LogicalNode) node() {}

// FuncNode represents a function call such as begins_with, contains, etc.
type FuncNode struct {
	Name string
	Args []Node
}

func (FuncNode) node() {}

// PathNode represents an attribute path, potentially nested (e.g. Address.City).
type PathNode struct {
	Parts []string
}

func (PathNode) node() {}

// ValueNode holds a concrete attribute value.
type ValueNode struct {
	Value dynago.AttributeValue
}

func (ValueNode) node() {}

// UpdateNode represents a single update action (SET, ADD, REMOVE, DELETE).
type UpdateNode struct {
	Action UpdateAction
	Path   PathNode
	Value  Node // nil for REMOVE
}

func (UpdateNode) node() {}

// ProjectionNode represents a projection expression (list of paths).
type ProjectionNode struct {
	Paths []PathNode
}

func (ProjectionNode) node() {}
