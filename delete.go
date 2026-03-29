package dynago

import (
	"context"
	"errors"
	"fmt"
)

// DeleteOption configures a Delete operation.
type DeleteOption func(*deleteConfig)

type deleteConfig struct {
	condition *conditionExpr
}

// DeleteCondition adds an arbitrary condition expression to the Delete operation.
// Placeholders (?) in the expression are replaced with the provided values.
func DeleteCondition(expression string, vals ...any) DeleteOption {
	return func(c *deleteConfig) {
		cond, err := buildCondition(expression, vals...)
		if err != nil {
			panic(fmt.Sprintf("dynago.DeleteCondition: %v", err))
		}
		if c.condition != nil {
			c.condition = mergeConditions(c.condition, cond)
		} else {
			c.condition = cond
		}
	}
}

// Delete removes the item identified by the given key from the table. Options
// such as DeleteCondition can be used to add condition expressions.
func (t *Table) Delete(ctx context.Context, key KeyValue, opts ...DeleteOption) error {
	var cfg deleteConfig
	for _, o := range opts {
		o(&cfg)
	}

	req := &DeleteItemRequest{
		TableName: t.name,
		Key:       key.Map(),
	}

	if cfg.condition != nil {
		req.ConditionExpression = cfg.condition.expression
		req.ExpressionAttributeNames = cfg.condition.names
		req.ExpressionAttributeValues = cfg.condition.values
	}

	_, err := t.backend.DeleteItem(ctx, req)
	if err != nil {
		if errors.Is(err, ErrConditionFailed) {
			return err
		}
		return err
	}
	return nil
}
