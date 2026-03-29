package dynago

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	// maxBatchWriteItems is the DynamoDB limit for BatchWriteItem.
	maxBatchWriteItems = 25
	// maxBatchGetItems is the DynamoDB limit for BatchGetItem.
	maxBatchGetItems = 100

	// backoffBase is the initial retry delay.
	backoffBase = 50 * time.Millisecond
	// maxRetries is the maximum number of retries for unprocessed items.
	maxRetries = 5
)

// ---------------------------------------------------------------------------
// Batch Write
// ---------------------------------------------------------------------------

// BatchOption configures batch write operations.
type BatchOption func(*batchConfig)

type batchConfig struct {
	onProgress     func(completed, total int)
	maxConcurrency int
}

func defaultBatchConfig() batchConfig {
	return batchConfig{maxConcurrency: 1}
}

// OnProgress sets a callback that is invoked after each chunk completes.
// The callback receives the cumulative number of completed items and the total.
func OnProgress(fn func(completed, total int)) BatchOption {
	return func(c *batchConfig) {
		c.onProgress = fn
	}
}

// MaxConcurrency sets the maximum number of concurrent chunk requests.
// Default is 1 (sequential).
func MaxConcurrency(n int) BatchOption {
	return func(c *batchConfig) {
		if n < 1 {
			n = 1
		}
		c.maxConcurrency = n
	}
}

// BatchPut writes multiple items in batches of 25. It retries unprocessed
// items with exponential backoff and reports progress via OnProgress.
func (t *Table) BatchPut(ctx context.Context, items []any, opts ...BatchOption) error {
	if len(items) == 0 {
		return nil
	}

	cfg := defaultBatchConfig()
	for _, o := range opts {
		o(&cfg)
	}

	// Marshal all items up front so we fail fast on bad input.
	writeRequests := make([]WriteRequest, len(items))
	for i, item := range items {
		av, err := Marshal(item)
		if err != nil {
			return fmt.Errorf("dynago.BatchPut: marshal item %d: %w", i, err)
		}
		writeRequests[i] = WriteRequest{PutItem: &PutRequest{Item: av}}
	}

	return t.executeBatchWrite(ctx, writeRequests, cfg)
}

// BatchDelete deletes multiple items by key in batches of 25.
func (t *Table) BatchDelete(ctx context.Context, keys []KeyValue, opts ...BatchOption) error {
	if len(keys) == 0 {
		return nil
	}

	cfg := defaultBatchConfig()
	for _, o := range opts {
		o(&cfg)
	}

	writeRequests := make([]WriteRequest, len(keys))
	for i, k := range keys {
		writeRequests[i] = WriteRequest{DeleteItem: &DeleteRequest{Key: k.Map()}}
	}

	return t.executeBatchWrite(ctx, writeRequests, cfg)
}

func (t *Table) executeBatchWrite(ctx context.Context, requests []WriteRequest, cfg batchConfig) error {
	total := len(requests)
	chunks := chunkWriteRequests(requests, maxBatchWriteItems)

	var (
		mu        sync.Mutex
		completed int
	)

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.maxConcurrency)

	for _, chunk := range chunks {
		chunk := chunk // capture
		g.Go(func() error {
			err := t.sendBatchWrite(gctx, chunk)
			if err != nil {
				return err
			}
			if cfg.onProgress != nil {
				mu.Lock()
				completed += len(chunk)
				c := completed
				mu.Unlock()
				cfg.onProgress(c, total)
			}
			return nil
		})
	}

	return g.Wait()
}

func (t *Table) sendBatchWrite(ctx context.Context, items []WriteRequest) error {
	pending := items
	for attempt := 0; ; attempt++ {
		req := &BatchWriteItemRequest{
			RequestItems: map[string][]WriteRequest{
				t.Name(): pending,
			},
		}

		resp, err := t.Backend().BatchWriteItem(ctx, req)
		if err != nil {
			return err
		}

		unprocessed := resp.UnprocessedItems[t.Name()]
		if len(unprocessed) == 0 {
			return nil
		}

		if attempt >= maxRetries {
			return fmt.Errorf("dynago.BatchWrite: %d unprocessed items after %d retries", len(unprocessed), maxRetries)
		}

		delay := backoffBase << uint(attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		pending = unprocessed
	}
}

func chunkWriteRequests(items []WriteRequest, size int) [][]WriteRequest {
	var chunks [][]WriteRequest
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks
}

// ---------------------------------------------------------------------------
// Batch Get
// ---------------------------------------------------------------------------

// BatchGetOption configures a BatchGet call.
type BatchGetOption func(*batchGetConfig)

type batchGetConfig struct {
	projection []string
}

// BatchGetProject specifies which attributes to retrieve in a BatchGet.
func BatchGetProject(attrs ...string) BatchGetOption {
	return func(c *batchGetConfig) {
		c.projection = append(c.projection, attrs...)
	}
}

// BatchGet retrieves multiple items by key and returns them as a typed slice.
// Items are returned in arbitrary order (matching DynamoDB behavior).
// Keys are automatically chunked into groups of 100.
func BatchGet[T any](ctx context.Context, t *Table, keys []KeyValue, opts ...BatchGetOption) ([]T, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	var cfg batchGetConfig
	for _, o := range opts {
		o(&cfg)
	}

	// Build the key maps.
	keyMaps := make([]map[string]AttributeValue, len(keys))
	for i, k := range keys {
		keyMaps[i] = k.Map()
	}

	// Build projection once.
	var projExpr string
	var projNames map[string]string
	if len(cfg.projection) > 0 {
		projExpr, projNames = buildProjection(cfg.projection)
	}

	var results []T

	// Process in chunks of 100.
	for i := 0; i < len(keyMaps); i += maxBatchGetItems {
		end := i + maxBatchGetItems
		if end > len(keyMaps) {
			end = len(keyMaps)
		}
		chunk := keyMaps[i:end]

		items, err := sendBatchGet(ctx, t, chunk, projExpr, projNames)
		if err != nil {
			return nil, err
		}

		for _, item := range items {
			var v T
			if err := Unmarshal(item, &v); err != nil {
				return nil, err
			}
			results = append(results, v)
		}
	}

	return results, nil
}

func sendBatchGet(ctx context.Context, t *Table, keys []map[string]AttributeValue, projExpr string, projNames map[string]string) ([]map[string]AttributeValue, error) {
	pending := keys
	var allItems []map[string]AttributeValue

	for attempt := 0; ; attempt++ {
		kp := KeysAndProjection{
			Keys:                     pending,
			ProjectionExpression:     projExpr,
			ExpressionAttributeNames: projNames,
		}

		req := &BatchGetItemRequest{
			RequestItems: map[string]KeysAndProjection{
				t.Name(): kp,
			},
		}

		resp, err := t.Backend().BatchGetItem(ctx, req)
		if err != nil {
			return nil, err
		}

		if items, ok := resp.Responses[t.Name()]; ok {
			allItems = append(allItems, items...)
		}

		unprocessed := resp.UnprocessedKeys[t.Name()]
		if len(unprocessed.Keys) == 0 {
			return allItems, nil
		}

		if attempt >= maxRetries {
			return nil, fmt.Errorf("dynago.BatchGet: %d unprocessed keys after %d retries", len(unprocessed.Keys), maxRetries)
		}

		delay := backoffBase << uint(attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}

		pending = unprocessed.Keys
	}
}
