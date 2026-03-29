package dynago

import (
	"context"
	"fmt"
)

// KeyCondition describes the partition key (required) and optional sort key
// condition for a Query operation.
type KeyCondition struct {
	partitionAttr string
	partitionVal  AttributeValue
	sortCond      *conditionExpr // nil when no sort condition
}

// Partition creates a KeyCondition requiring that attr equals val.
func Partition(attr string, val any) KeyCondition {
	return KeyCondition{
		partitionAttr: attr,
		partitionVal:  toAttributeValue(val),
	}
}

// SortEquals adds an equality condition on the sort key.
func (kc KeyCondition) SortEquals(attr string, val any) KeyCondition {
	kc.sortCond = sortCompare(attr, "=", val)
	return kc
}

// SortBeginsWith adds a begins_with condition on the sort key.
func (kc KeyCondition) SortBeginsWith(attr string, prefix any) KeyCondition {
	alias := "#" + attr
	kc.sortCond = &conditionExpr{
		expression: fmt.Sprintf("begins_with(%s, :sk0)", alias),
		names:      map[string]string{alias: attr},
		values:     map[string]AttributeValue{":sk0": toAttributeValue(prefix)},
	}
	return kc
}

// SortBetween adds a BETWEEN condition on the sort key.
func (kc KeyCondition) SortBetween(attr string, low, high any) KeyCondition {
	alias := "#" + attr
	kc.sortCond = &conditionExpr{
		expression: fmt.Sprintf("%s BETWEEN :sk0 AND :sk1", alias),
		names:      map[string]string{alias: attr},
		values: map[string]AttributeValue{
			":sk0": toAttributeValue(low),
			":sk1": toAttributeValue(high),
		},
	}
	return kc
}

// sortCompare builds a simple sort key comparison condition.
func sortCompare(attr string, op string, val any) *conditionExpr {
	alias := "#" + attr
	return &conditionExpr{
		expression: fmt.Sprintf("%s %s :sk0", alias, op),
		names:      map[string]string{alias: attr},
		values:     map[string]AttributeValue{":sk0": toAttributeValue(val)},
	}
}

// SortGreaterThan adds a > condition on the sort key.
func (kc KeyCondition) SortGreaterThan(attr string, val any) KeyCondition {
	kc.sortCond = sortCompare(attr, ">", val)
	return kc
}

// SortLessThan adds a < condition on the sort key.
func (kc KeyCondition) SortLessThan(attr string, val any) KeyCondition {
	kc.sortCond = sortCompare(attr, "<", val)
	return kc
}

// SortGreaterOrEqual adds a >= condition on the sort key.
func (kc KeyCondition) SortGreaterOrEqual(attr string, val any) KeyCondition {
	kc.sortCond = sortCompare(attr, ">=", val)
	return kc
}

// SortLessOrEqual adds a <= condition on the sort key.
func (kc KeyCondition) SortLessOrEqual(attr string, val any) KeyCondition {
	kc.sortCond = sortCompare(attr, "<=", val)
	return kc
}

// QueryOption configures a Query operation.
type QueryOption func(*queryConfig)

type queryConfig struct {
	filter         *conditionExpr
	limit          int32
	scanForward    *bool
	indexName      string
	projection     []string
	consistentRead bool
}

// QueryFilter adds a filter expression to the query.
func QueryFilter(expression string, vals ...any) QueryOption {
	return func(c *queryConfig) {
		cond, err := buildCondition(expression, vals...)
		if err != nil {
			panic(fmt.Sprintf("dynago.QueryFilter: %v", err))
		}
		if c.filter != nil {
			c.filter = mergeConditions(c.filter, cond)
		} else {
			c.filter = cond
		}
	}
}

// QueryLimit sets the maximum number of items to return across all pages.
func QueryLimit(n int) QueryOption {
	v := int32(n)
	return func(c *queryConfig) {
		c.limit = v
	}
}

// ScanForward sets the scan direction. True for ascending (default), false
// for descending.
func ScanForward(forward bool) QueryOption {
	return func(c *queryConfig) {
		c.scanForward = &forward
	}
}

// QueryIndex specifies a secondary index to query.
func QueryIndex(name string) QueryOption {
	return func(c *queryConfig) {
		c.indexName = name
	}
}

// QueryProject specifies which attributes to retrieve.
func QueryProject(attrs ...string) QueryOption {
	return func(c *queryConfig) {
		c.projection = append(c.projection, attrs...)
	}
}

// QueryConsistentRead requests a strongly consistent read.
func QueryConsistentRead() QueryOption {
	return func(c *queryConfig) {
		c.consistentRead = true
	}
}

// buildQueryRequest builds a QueryRequest from key condition and options.
func buildQueryRequest(tableName string, key KeyCondition, cfg *queryConfig, startKey map[string]AttributeValue) *QueryRequest {
	names := make(map[string]string)
	values := make(map[string]AttributeValue)

	// Build key condition expression.
	pkAlias := "#" + key.partitionAttr
	names[pkAlias] = key.partitionAttr
	values[":pk0"] = key.partitionVal
	keyExpr := fmt.Sprintf("%s = :pk0", pkAlias)

	if key.sortCond != nil {
		keyExpr = keyExpr + " AND " + key.sortCond.expression
		for k, v := range key.sortCond.names {
			names[k] = v
		}
		for k, v := range key.sortCond.values {
			values[k] = v
		}
	}

	req := &QueryRequest{
		TableName:                 tableName,
		KeyConditionExpression:    keyExpr,
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
		IndexName:                 cfg.indexName,
		ConsistentRead:            cfg.consistentRead,
		ScanIndexForward:          cfg.scanForward,
		ExclusiveStartKey:         startKey,
	}

	if cfg.limit > 0 {
		req.Limit = cfg.limit
	}

	if cfg.filter != nil {
		req.FilterExpression = cfg.filter.expression
		for k, v := range cfg.filter.names {
			req.ExpressionAttributeNames[k] = v
		}
		for k, v := range cfg.filter.values {
			req.ExpressionAttributeValues[k] = v
		}
	}

	if len(cfg.projection) > 0 {
		projExpr, projNames := buildProjection(cfg.projection)
		req.ProjectionExpression = projExpr
		for k, v := range projNames {
			req.ExpressionAttributeNames[k] = v
		}
	}

	return req
}

// Query executes a DynamoDB Query and unmarshals all results into a slice of T.
// It automatically paginates when the response contains a LastEvaluatedKey,
// collecting all items until done or the Limit is reached.
func Query[T any](ctx context.Context, t *Table, key KeyCondition, opts ...QueryOption) ([]T, error) {
	var cfg queryConfig
	for _, o := range opts {
		o(&cfg)
	}

	req := buildQueryRequest(t.Name(), key, &cfg, nil)

	var totalLimit int32
	if cfg.limit > 0 {
		totalLimit = cfg.limit
	}

	var results []T
	for {
		resp, err := t.Backend().Query(ctx, req)
		if err != nil {
			return nil, err
		}

		for _, item := range resp.Items {
			var v T
			if err := Unmarshal(item, &v); err != nil {
				return nil, err
			}
			results = append(results, v)

			if totalLimit > 0 && int32(len(results)) >= totalLimit {
				return results, nil
			}
		}

		if len(resp.LastEvaluatedKey) == 0 {
			break
		}

		req.ExclusiveStartKey = resp.LastEvaluatedKey

		if totalLimit > 0 {
			remaining := totalLimit - int32(len(results))
			req.Limit = remaining
		}
	}

	return results, nil
}
