package dynago

import (
	"context"
	"fmt"
)

// ScanOption configures a Scan operation.
type ScanOption func(*scanConfig)

type scanConfig struct {
	filter         *conditionExpr
	limit          int32
	indexName      string
	projection     []string
	consistentRead bool
}

// ScanFilter adds a filter expression to the scan.
func ScanFilter(expression string, vals ...any) ScanOption {
	return func(c *scanConfig) {
		cond, err := buildCondition(expression, vals...)
		if err != nil {
			panic(fmt.Sprintf("dynago.ScanFilter: %v", err))
		}
		if c.filter != nil {
			c.filter = mergeConditions(c.filter, cond)
		} else {
			c.filter = cond
		}
	}
}

// ScanLimit sets the maximum number of items to return across all pages.
func ScanLimit(n int) ScanOption {
	v := int32(n)
	return func(c *scanConfig) {
		c.limit = v
	}
}

// ScanIndex specifies a secondary index to scan.
func ScanIndex(name string) ScanOption {
	return func(c *scanConfig) {
		c.indexName = name
	}
}

// ScanProject specifies which attributes to retrieve.
func ScanProject(attrs ...string) ScanOption {
	return func(c *scanConfig) {
		c.projection = append(c.projection, attrs...)
	}
}

// ScanConsistentRead requests a strongly consistent read.
func ScanConsistentRead() ScanOption {
	return func(c *scanConfig) {
		c.consistentRead = true
	}
}

// buildScanRequest builds a ScanRequest from options.
func buildScanRequest(tableName string, cfg *scanConfig, startKey map[string]AttributeValue) *ScanRequest {
	req := &ScanRequest{
		TableName:         tableName,
		IndexName:         cfg.indexName,
		ConsistentRead:    cfg.consistentRead,
		ExclusiveStartKey: startKey,
	}

	if cfg.limit > 0 {
		req.Limit = cfg.limit
	}

	if cfg.filter != nil {
		req.FilterExpression = cfg.filter.expression
		req.ExpressionAttributeNames = cfg.filter.names
		req.ExpressionAttributeValues = cfg.filter.values
	}

	if len(cfg.projection) > 0 {
		projExpr, projNames := buildProjection(cfg.projection)
		req.ProjectionExpression = projExpr
		if req.ExpressionAttributeNames == nil {
			req.ExpressionAttributeNames = projNames
		} else {
			for k, v := range projNames {
				req.ExpressionAttributeNames[k] = v
			}
		}
	}

	return req
}

// Scan executes a DynamoDB Scan and unmarshals all results into a slice of T.
// It automatically paginates when the response contains a LastEvaluatedKey,
// collecting all items until done or the Limit is reached.
func Scan[T any](ctx context.Context, t *Table, opts ...ScanOption) ([]T, error) {
	var cfg scanConfig
	for _, o := range opts {
		o(&cfg)
	}

	req := buildScanRequest(t.Name(), &cfg, nil)

	var totalLimit int32
	if cfg.limit > 0 {
		totalLimit = cfg.limit
	}

	var results []T
	for {
		resp, err := t.Backend().Scan(ctx, req)
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
