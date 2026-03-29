package dynago

import (
	"context"
	"errors"
	"fmt"
)

// PutOption configures a Put operation.
type PutOption func(*putConfig)

type putConfig struct {
	condition *conditionExpr
}

// IfNotExists adds a condition that the item must not already exist. The attr
// parameter is the attribute name to check (typically the partition key).
func IfNotExists(attr string) PutOption {
	return func(c *putConfig) {
		cond := buildAttrNotExists(attr)
		if c.condition != nil {
			c.condition = mergeConditions(c.condition, cond)
		} else {
			c.condition = cond
		}
	}
}

// PutCondition adds an arbitrary condition expression to the Put operation.
// Placeholders (?) in the expression are replaced with the provided values.
func PutCondition(expression string, vals ...any) PutOption {
	return func(c *putConfig) {
		cond, err := buildCondition(expression, vals...)
		if err != nil {
			panic(fmt.Sprintf("dynago.PutCondition: %v", err))
		}
		if c.condition != nil {
			c.condition = mergeConditions(c.condition, cond)
		} else {
			c.condition = cond
		}
	}
}

// Put marshals the given item and stores it in the table. Options such as
// IfNotExists and PutCondition can be used to add condition expressions.
func (t *Table) Put(ctx context.Context, item any, opts ...PutOption) error {
	av, err := Marshal(item)
	if err != nil {
		return err
	}

	// Auto-set discriminator attribute if the table has a registry and the
	// item implements Entity.
	if e, ok := item.(Entity); ok && t.registry != nil {
		info := e.DynagoEntity()
		av[t.registry.DiscriminatorAttr()] = AttributeValue{Type: TypeS, S: info.Discriminator}
	}

	var cfg putConfig
	for _, o := range opts {
		o(&cfg)
	}

	req := &PutItemRequest{
		TableName: t.name,
		Item:      av,
	}

	if cfg.condition != nil {
		req.ConditionExpression = cfg.condition.expression
		req.ExpressionAttributeNames = cfg.condition.names
		req.ExpressionAttributeValues = cfg.condition.values
	}

	_, err = t.backend.PutItem(ctx, req)
	if err != nil {
		if errors.Is(err, ErrConditionFailed) {
			return err
		}
		return err
	}
	return nil
}
