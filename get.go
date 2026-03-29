package dynago

import (
	"context"
	"strings"
)

// GetOption configures a Get[T] call.
type GetOption func(*getConfig)

type getConfig struct {
	consistentRead bool
	projection     []string
}

// ConsistentRead requests a strongly consistent read.
func ConsistentRead() GetOption {
	return func(c *getConfig) {
		c.consistentRead = true
	}
}

// Project specifies which attributes to retrieve. Each attr is a
// top-level attribute name; nested paths are not yet supported here.
func Project(attrs ...string) GetOption {
	return func(c *getConfig) {
		c.projection = append(c.projection, attrs...)
	}
}

// buildProjection builds a ProjectionExpression and ExpressionAttributeNames
// from a list of attribute names. Each attribute gets an #alias entry.
func buildProjection(attrs []string) (string, map[string]string) {
	names := make(map[string]string, len(attrs))
	aliases := make([]string, len(attrs))
	for i, attr := range attrs {
		alias := "#" + attr
		names[alias] = attr
		aliases[i] = alias
	}
	return strings.Join(aliases, ", "), names
}

// Get retrieves a single item from DynamoDB by primary key and unmarshals
// it into T. It returns ErrNotFound when the item does not exist.
func Get[T any](ctx context.Context, t *Table, key KeyValue, opts ...GetOption) (T, error) {
	var zero T
	var cfg getConfig
	for _, o := range opts {
		o(&cfg)
	}

	req := &GetItemRequest{
		TableName:      t.Name(),
		Key:            key.Map(),
		ConsistentRead: cfg.consistentRead,
	}

	if len(cfg.projection) > 0 {
		projExpr, names := buildProjection(cfg.projection)
		req.ProjectionExpression = projExpr
		req.ExpressionAttributeNames = names
	}

	resp, err := t.Backend().GetItem(ctx, req)
	if err != nil {
		return zero, err
	}

	if len(resp.Item) == 0 {
		return zero, ErrNotFound
	}

	var result T
	if err := Unmarshal(resp.Item, &result); err != nil {
		return zero, err
	}
	return result, nil
}
