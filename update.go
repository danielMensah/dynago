package dynago

import (
	"context"
	"fmt"
	"strings"
)

// UpdateOption configures an Update or UpdateReturning call.
type UpdateOption func(*updateConfig)

// updateAction identifies a DynamoDB update action type.
type updateAction int

const (
	actionSET updateAction = iota + 1
	actionADD
	actionREMOVE
	actionDELETE
)

func (a updateAction) String() string {
	switch a {
	case actionSET:
		return "SET"
	case actionADD:
		return "ADD"
	case actionREMOVE:
		return "REMOVE"
	case actionDELETE:
		return "DELETE"
	default:
		return "?"
	}
}

// updateClause represents a single update action (e.g., SET #Name = :v0).
type updateClause struct {
	action updateAction
	attr   string
	val    any  // nil for REMOVE
	hasVal bool // false for REMOVE
}

type updateConfig struct {
	clauses    []updateClause
	condition  *conditionExpr
	returnMode string // "ALL_NEW", "ALL_OLD", or ""
}

// Set adds a SET action that assigns val to the given attribute.
func Set(attr string, val any) UpdateOption {
	return func(c *updateConfig) {
		c.clauses = append(c.clauses, updateClause{
			action: actionSET,
			attr:   attr,
			val:    val,
			hasVal: true,
		})
	}
}

// Add adds an ADD action. For numbers this increments; for sets this unions.
func Add(attr string, val any) UpdateOption {
	return func(c *updateConfig) {
		c.clauses = append(c.clauses, updateClause{
			action: actionADD,
			attr:   attr,
			val:    val,
			hasVal: true,
		})
	}
}

// Remove adds a REMOVE action that deletes the given attribute.
func Remove(attr string) UpdateOption {
	return func(c *updateConfig) {
		c.clauses = append(c.clauses, updateClause{
			action: actionREMOVE,
			attr:   attr,
			hasVal: false,
		})
	}
}

// Delete adds a DELETE action that removes elements from a set attribute.
func Delete(attr string, val any) UpdateOption {
	return func(c *updateConfig) {
		c.clauses = append(c.clauses, updateClause{
			action: actionDELETE,
			attr:   attr,
			val:    val,
			hasVal: true,
		})
	}
}

// IfCondition adds a condition expression for the update. Placeholders (?)
// in the expression are replaced with the provided values.
func IfCondition(expression string, vals ...any) UpdateOption {
	return func(c *updateConfig) {
		cond, err := buildCondition(expression, vals...)
		if err != nil {
			panic(fmt.Sprintf("dynago.IfCondition: %v", err))
		}
		if c.condition != nil {
			c.condition = mergeConditions(c.condition, cond)
		} else {
			c.condition = cond
		}
	}
}

// ReturnNew requests that the updated item (after the update) is returned.
func ReturnNew() UpdateOption {
	return func(c *updateConfig) {
		c.returnMode = "ALL_NEW"
	}
}

// ReturnOld requests that the item (before the update) is returned.
func ReturnOld() UpdateOption {
	return func(c *updateConfig) {
		c.returnMode = "ALL_OLD"
	}
}

// Update applies update expressions to the item identified by key. It does
// not return the updated item; use UpdateReturning[T] if you need the result.
func (t *Table) Update(ctx context.Context, key KeyValue, opts ...UpdateOption) error {
	var cfg updateConfig
	for _, o := range opts {
		o(&cfg)
	}

	if len(cfg.clauses) == 0 {
		return newError(ErrValidation, "dynago.Update: at least one update action is required")
	}

	req := buildUpdateRequest(t.name, key, &cfg)
	req.ReturnValues = "NONE"

	_, err := t.backend.UpdateItem(ctx, req)
	return err
}

// UpdateReturning applies update expressions to the item identified by key
// and returns the item (either before or after the update depending on
// ReturnNew/ReturnOld). At least one of ReturnNew() or ReturnOld() must be
// provided; otherwise ErrValidation is returned.
func UpdateReturning[T any](ctx context.Context, t *Table, key KeyValue, opts ...UpdateOption) (T, error) {
	var zero T
	var cfg updateConfig
	for _, o := range opts {
		o(&cfg)
	}

	if len(cfg.clauses) == 0 {
		return zero, newError(ErrValidation, "dynago.UpdateReturning: at least one update action is required")
	}

	if cfg.returnMode == "" {
		return zero, newError(ErrValidation, "dynago.UpdateReturning: ReturnNew() or ReturnOld() option is required")
	}

	req := buildUpdateRequest(t.Name(), key, &cfg)
	req.ReturnValues = cfg.returnMode

	resp, err := t.Backend().UpdateItem(ctx, req)
	if err != nil {
		return zero, err
	}

	if len(resp.Attributes) == 0 {
		return zero, nil
	}

	var result T
	if err := Unmarshal(resp.Attributes, &result); err != nil {
		return zero, err
	}
	return result, nil
}

// buildUpdateRequest constructs an UpdateItemRequest from the config.
func buildUpdateRequest(tableName string, key KeyValue, cfg *updateConfig) *UpdateItemRequest {
	names := make(map[string]string)
	values := make(map[string]AttributeValue)
	valueIdx := 0

	// Group clauses by action, preserving insertion order.
	type actionGroup struct {
		action updateAction
		parts  []string
	}
	var groups []actionGroup
	groupMap := make(map[updateAction]int)

	for _, cl := range cfg.clauses {
		alias := "#" + cl.attr
		names[alias] = cl.attr

		var part string
		switch cl.action {
		case actionSET:
			placeholder := fmt.Sprintf(":v%d", valueIdx)
			valueIdx++
			values[placeholder] = anyToAttributeValue(cl.val)
			part = fmt.Sprintf("%s = %s", alias, placeholder)
		case actionADD:
			placeholder := fmt.Sprintf(":v%d", valueIdx)
			valueIdx++
			values[placeholder] = anyToAttributeValue(cl.val)
			part = fmt.Sprintf("%s %s", alias, placeholder)
		case actionREMOVE:
			part = alias
		case actionDELETE:
			placeholder := fmt.Sprintf(":v%d", valueIdx)
			valueIdx++
			values[placeholder] = anyToAttributeValue(cl.val)
			part = fmt.Sprintf("%s %s", alias, placeholder)
		}

		idx, ok := groupMap[cl.action]
		if !ok {
			idx = len(groups)
			groupMap[cl.action] = idx
			groups = append(groups, actionGroup{action: cl.action})
		}
		groups[idx].parts = append(groups[idx].parts, part)
	}

	// Build the update expression.
	var sections []string
	for _, g := range groups {
		sections = append(sections, fmt.Sprintf("%s %s", g.action, strings.Join(g.parts, ", ")))
	}
	updateExpr := strings.Join(sections, " ")

	req := &UpdateItemRequest{
		TableName:                 tableName,
		Key:                       key.Map(),
		UpdateExpression:          updateExpr,
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	}

	if cfg.condition != nil {
		req.ConditionExpression = cfg.condition.expression
		for k, v := range cfg.condition.names {
			req.ExpressionAttributeNames[k] = v
		}
		for k, v := range cfg.condition.values {
			req.ExpressionAttributeValues[k] = v
		}
	}

	return req
}
