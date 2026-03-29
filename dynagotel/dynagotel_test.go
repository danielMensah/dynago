package dynagotel

import (
	"context"
	"testing"

	"github.com/danielmensah/dynago"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// noopBackend implements dynago.Backend with no-op responses.
type noopTestBackend struct{}

func (n *noopTestBackend) GetItem(_ context.Context, _ *dynago.GetItemRequest) (*dynago.GetItemResponse, error) {
	return &dynago.GetItemResponse{}, nil
}
func (n *noopTestBackend) PutItem(_ context.Context, _ *dynago.PutItemRequest) (*dynago.PutItemResponse, error) {
	return &dynago.PutItemResponse{}, nil
}
func (n *noopTestBackend) DeleteItem(_ context.Context, _ *dynago.DeleteItemRequest) (*dynago.DeleteItemResponse, error) {
	return &dynago.DeleteItemResponse{}, nil
}
func (n *noopTestBackend) UpdateItem(_ context.Context, _ *dynago.UpdateItemRequest) (*dynago.UpdateItemResponse, error) {
	return &dynago.UpdateItemResponse{}, nil
}
func (n *noopTestBackend) Query(_ context.Context, _ *dynago.QueryRequest) (*dynago.QueryResponse, error) {
	return &dynago.QueryResponse{}, nil
}
func (n *noopTestBackend) Scan(_ context.Context, _ *dynago.ScanRequest) (*dynago.ScanResponse, error) {
	return &dynago.ScanResponse{}, nil
}
func (n *noopTestBackend) BatchGetItem(_ context.Context, _ *dynago.BatchGetItemRequest) (*dynago.BatchGetItemResponse, error) {
	return &dynago.BatchGetItemResponse{}, nil
}
func (n *noopTestBackend) BatchWriteItem(_ context.Context, _ *dynago.BatchWriteItemRequest) (*dynago.BatchWriteItemResponse, error) {
	return &dynago.BatchWriteItemResponse{}, nil
}
func (n *noopTestBackend) TransactGetItems(_ context.Context, _ *dynago.TransactGetItemsRequest) (*dynago.TransactGetItemsResponse, error) {
	return &dynago.TransactGetItemsResponse{}, nil
}
func (n *noopTestBackend) TransactWriteItems(_ context.Context, _ *dynago.TransactWriteItemsRequest) (*dynago.TransactWriteItemsResponse, error) {
	return &dynago.TransactWriteItemsResponse{}, nil
}

func TestMiddleware_CreatesSpans(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	tracer := tp.Tracer("test")

	mw := NewMiddleware(WithTracer(tracer))
	backend := mw(&noopTestBackend{})

	ctx := context.Background()

	backend.GetItem(ctx, &dynago.GetItemRequest{TableName: "users"})
	backend.PutItem(ctx, &dynago.PutItemRequest{TableName: "orders"})
	backend.Query(ctx, &dynago.QueryRequest{TableName: "events"})

	spans := exporter.GetSpans()
	if len(spans) != 3 {
		t.Fatalf("expected 3 spans, got %d", len(spans))
	}

	// Verify span names.
	expected := []string{"GetItem", "PutItem", "Query"}
	for i, s := range spans {
		if s.Name != expected[i] {
			t.Errorf("span %d: expected name %q, got %q", i, expected[i], s.Name)
		}
	}

	// Verify attributes on first span.
	attrMap := make(map[string]string)
	for _, a := range spans[0].Attributes {
		attrMap[string(a.Key)] = a.Value.Emit()
	}
	if v := attrMap["db.system"]; v != "dynamodb" {
		t.Errorf("expected db.system=dynamodb, got %q", v)
	}
	if v := attrMap["db.operation"]; v != "GetItem" {
		t.Errorf("expected db.operation=GetItem, got %q", v)
	}
	if v := attrMap["db.name"]; v != "users" {
		t.Errorf("expected db.name=users, got %q", v)
	}
}

func TestMiddleware_AllOperationsCreateSpans(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	tracer := tp.Tracer("test")
	mw := NewMiddleware(WithTracer(tracer))
	backend := mw(&noopTestBackend{})

	ctx := context.Background()

	backend.GetItem(ctx, &dynago.GetItemRequest{TableName: "t"})
	backend.PutItem(ctx, &dynago.PutItemRequest{TableName: "t"})
	backend.DeleteItem(ctx, &dynago.DeleteItemRequest{TableName: "t"})
	backend.UpdateItem(ctx, &dynago.UpdateItemRequest{TableName: "t"})
	backend.Query(ctx, &dynago.QueryRequest{TableName: "t"})
	backend.Scan(ctx, &dynago.ScanRequest{TableName: "t"})
	backend.BatchGetItem(ctx, &dynago.BatchGetItemRequest{})
	backend.BatchWriteItem(ctx, &dynago.BatchWriteItemRequest{})
	backend.TransactGetItems(ctx, &dynago.TransactGetItemsRequest{})
	backend.TransactWriteItems(ctx, &dynago.TransactWriteItemsRequest{})

	spans := exporter.GetSpans()
	if len(spans) != 10 {
		t.Fatalf("expected 10 spans, got %d", len(spans))
	}

	expectedOps := []string{
		"GetItem", "PutItem", "DeleteItem", "UpdateItem",
		"Query", "Scan", "BatchGetItem", "BatchWriteItem",
		"TransactGetItems", "TransactWriteItems",
	}
	for i, s := range spans {
		if s.Name != expectedOps[i] {
			t.Errorf("span %d: expected %q, got %q", i, expectedOps[i], s.Name)
		}
	}
}

func TestMiddleware_NoopSafe(t *testing.T) {
	// No tracer, no meter — should not panic.
	mw := NewMiddleware()
	backend := mw(&noopTestBackend{})

	ctx := context.Background()
	resp, err := backend.GetItem(ctx, &dynago.GetItemRequest{TableName: "t"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestMiddleware_WithMeter(t *testing.T) {
	// Use noop meter — should not panic, instruments are created successfully.
	meter := noop.NewMeterProvider().Meter("test")

	mw := NewMiddleware(WithMeter(meter))
	backend := mw(&noopTestBackend{})

	ctx := context.Background()
	_, err := backend.GetItem(ctx, &dynago.GetItemRequest{TableName: "t"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMiddleware_SpanAttributes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	tracer := tp.Tracer("test")
	mw := NewMiddleware(WithTracer(tracer))
	backend := mw(&noopTestBackend{})

	// BatchGetItem should not have db.name attribute (no single table).
	backend.BatchGetItem(context.Background(), &dynago.BatchGetItemRequest{})

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	for _, a := range spans[0].Attributes {
		if a.Key == attribute.Key("db.name") {
			t.Error("batch operation should not have db.name attribute")
		}
	}
}
