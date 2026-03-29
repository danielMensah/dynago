package dynago

import (
	"context"
	"fmt"
)

// Collection holds a heterogeneous set of items unmarshaled via a Registry.
type Collection struct {
	items []any
}

// QueryCollection executes a DynamoDB Query and unmarshals items
// polymorphically using the table's Registry. Items with unrecognized
// discriminators are silently skipped. Returns an error if the table has no
// registry.
func QueryCollection(ctx context.Context, t *Table, key KeyCondition, opts ...QueryOption) (*Collection, error) {
	if t.registry == nil {
		return nil, fmt.Errorf("dynago: QueryCollection requires a table with a registry")
	}

	var cfg queryConfig
	for _, o := range opts {
		o(&cfg)
	}

	var totalLimit int32
	if cfg.limit > 0 {
		totalLimit = cfg.limit
	}

	var items []any
	var startKey map[string]AttributeValue

	for {
		req := buildQueryRequest(t.Name(), key, &cfg, startKey)
		resp, err := t.Backend().Query(ctx, req)
		if err != nil {
			return nil, err
		}

		for _, item := range resp.Items {
			v, err := unmarshalPolymorphic(item, t.registry)
			if err != nil {
				// Silently skip unrecognized discriminators.
				continue
			}
			items = append(items, v)

			if totalLimit > 0 && int32(len(items)) >= totalLimit {
				return &Collection{items: items}, nil
			}
		}

		if len(resp.LastEvaluatedKey) == 0 {
			break
		}
		startKey = resp.LastEvaluatedKey

		if totalLimit > 0 {
			remaining := totalLimit - int32(len(items))
			req.Limit = remaining
		}
	}

	return &Collection{items: items}, nil
}

// ItemsOf returns all items in the Collection that are of type T.
func ItemsOf[T any](c *Collection) []T {
	var result []T
	for _, item := range c.items {
		if v, ok := item.(T); ok {
			result = append(result, v)
		}
	}
	return result
}
