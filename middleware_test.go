package dynago

import (
	"context"
	"testing"
)

// countingBackend wraps an inner Backend and counts all method calls.
type countingBackend struct {
	Backend // embed inner
	calls   int
}

func (c *countingBackend) GetItem(ctx context.Context, req *GetItemRequest) (*GetItemResponse, error) {
	c.calls++
	return c.Backend.GetItem(ctx, req)
}

func (c *countingBackend) PutItem(ctx context.Context, req *PutItemRequest) (*PutItemResponse, error) {
	c.calls++
	return c.Backend.PutItem(ctx, req)
}

func (c *countingBackend) DeleteItem(ctx context.Context, req *DeleteItemRequest) (*DeleteItemResponse, error) {
	c.calls++
	return c.Backend.DeleteItem(ctx, req)
}

func (c *countingBackend) UpdateItem(ctx context.Context, req *UpdateItemRequest) (*UpdateItemResponse, error) {
	c.calls++
	return c.Backend.UpdateItem(ctx, req)
}

func (c *countingBackend) Query(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
	c.calls++
	return c.Backend.Query(ctx, req)
}

func (c *countingBackend) Scan(ctx context.Context, req *ScanRequest) (*ScanResponse, error) {
	c.calls++
	return c.Backend.Scan(ctx, req)
}

func (c *countingBackend) BatchGetItem(ctx context.Context, req *BatchGetItemRequest) (*BatchGetItemResponse, error) {
	c.calls++
	return c.Backend.BatchGetItem(ctx, req)
}

func (c *countingBackend) BatchWriteItem(ctx context.Context, req *BatchWriteItemRequest) (*BatchWriteItemResponse, error) {
	c.calls++
	return c.Backend.BatchWriteItem(ctx, req)
}

func (c *countingBackend) TransactGetItems(ctx context.Context, req *TransactGetItemsRequest) (*TransactGetItemsResponse, error) {
	c.calls++
	return c.Backend.TransactGetItems(ctx, req)
}

func (c *countingBackend) TransactWriteItems(ctx context.Context, req *TransactWriteItemsRequest) (*TransactWriteItemsResponse, error) {
	c.calls++
	return c.Backend.TransactWriteItems(ctx, req)
}

func countingMiddleware(cb **countingBackend) Middleware {
	return func(inner Backend) Backend {
		b := &countingBackend{Backend: inner}
		*cb = b
		return b
	}
}

func TestWithMiddleware_CountsCalls(t *testing.T) {
	noop := &noopBackend{
		getResp:   &GetItemResponse{},
		putResp:   &PutItemResponse{},
		queryResp: &QueryResponse{},
	}

	var counter *countingBackend
	db := New(noop, WithMiddleware(countingMiddleware(&counter)))
	tbl := db.Table("test")

	ctx := context.Background()

	// Call all 10 operations.
	tbl.Backend().GetItem(ctx, &GetItemRequest{TableName: "test"})
	tbl.Backend().PutItem(ctx, &PutItemRequest{TableName: "test"})
	tbl.Backend().DeleteItem(ctx, &DeleteItemRequest{TableName: "test"})
	tbl.Backend().UpdateItem(ctx, &UpdateItemRequest{TableName: "test"})
	tbl.Backend().Query(ctx, &QueryRequest{TableName: "test"})
	tbl.Backend().Scan(ctx, &ScanRequest{TableName: "test"})
	tbl.Backend().BatchGetItem(ctx, &BatchGetItemRequest{})
	tbl.Backend().BatchWriteItem(ctx, &BatchWriteItemRequest{})
	tbl.Backend().TransactGetItems(ctx, &TransactGetItemsRequest{})
	tbl.Backend().TransactWriteItems(ctx, &TransactWriteItemsRequest{})

	if counter.calls != 10 {
		t.Errorf("expected 10 calls, got %d", counter.calls)
	}
}

func TestWithMiddleware_Order(t *testing.T) {
	// Track the order middleware is invoked.
	var order []string

	mkMiddleware := func(name string) Middleware {
		return func(inner Backend) Backend {
			order = append(order, name+"_wrap")
			return &orderBackend{Backend: inner, name: name, order: &order}
		}
	}

	noop := &noopBackend{getResp: &GetItemResponse{}}
	db := New(noop, WithMiddleware(mkMiddleware("A"), mkMiddleware("B")))

	// Applied in order: A wraps noop, B wraps A(noop). So B is outermost.
	if len(order) != 2 || order[0] != "A_wrap" || order[1] != "B_wrap" {
		t.Fatalf("expected wrap order [A_wrap, B_wrap], got %v", order)
	}

	// Reset and make a call: outermost (B) should be called first.
	order = order[:0]
	tbl := db.Table("test")
	tbl.Backend().GetItem(context.Background(), &GetItemRequest{TableName: "test"})

	if len(order) != 2 || order[0] != "B_call" || order[1] != "A_call" {
		t.Fatalf("expected call order [B_call, A_call], got %v", order)
	}
}

// orderBackend records call order for middleware ordering tests.
type orderBackend struct {
	Backend
	name  string
	order *[]string
}

func (o *orderBackend) GetItem(ctx context.Context, req *GetItemRequest) (*GetItemResponse, error) {
	*o.order = append(*o.order, o.name+"_call")
	return o.Backend.GetItem(ctx, req)
}
