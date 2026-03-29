package benchmarks

import (
	"context"
	"log/slog"
	"testing"

	"github.com/danielmensah/dynago"
	"github.com/danielmensah/dynago/dynagotel"
)

// noopBenchBackend implements dynago.Backend with zero-cost responses.
type noopBenchBackend struct{}

func (n *noopBenchBackend) GetItem(_ context.Context, _ *dynago.GetItemRequest) (*dynago.GetItemResponse, error) {
	return &dynago.GetItemResponse{}, nil
}
func (n *noopBenchBackend) PutItem(_ context.Context, _ *dynago.PutItemRequest) (*dynago.PutItemResponse, error) {
	return &dynago.PutItemResponse{}, nil
}
func (n *noopBenchBackend) DeleteItem(_ context.Context, _ *dynago.DeleteItemRequest) (*dynago.DeleteItemResponse, error) {
	return &dynago.DeleteItemResponse{}, nil
}
func (n *noopBenchBackend) UpdateItem(_ context.Context, _ *dynago.UpdateItemRequest) (*dynago.UpdateItemResponse, error) {
	return &dynago.UpdateItemResponse{}, nil
}
func (n *noopBenchBackend) Query(_ context.Context, _ *dynago.QueryRequest) (*dynago.QueryResponse, error) {
	return &dynago.QueryResponse{}, nil
}
func (n *noopBenchBackend) Scan(_ context.Context, _ *dynago.ScanRequest) (*dynago.ScanResponse, error) {
	return &dynago.ScanResponse{}, nil
}
func (n *noopBenchBackend) BatchGetItem(_ context.Context, _ *dynago.BatchGetItemRequest) (*dynago.BatchGetItemResponse, error) {
	return &dynago.BatchGetItemResponse{}, nil
}
func (n *noopBenchBackend) BatchWriteItem(_ context.Context, _ *dynago.BatchWriteItemRequest) (*dynago.BatchWriteItemResponse, error) {
	return &dynago.BatchWriteItemResponse{}, nil
}
func (n *noopBenchBackend) TransactGetItems(_ context.Context, _ *dynago.TransactGetItemsRequest) (*dynago.TransactGetItemsResponse, error) {
	return &dynago.TransactGetItemsResponse{}, nil
}
func (n *noopBenchBackend) TransactWriteItems(_ context.Context, _ *dynago.TransactWriteItemsRequest) (*dynago.TransactWriteItemsResponse, error) {
	return &dynago.TransactWriteItemsResponse{}, nil
}

// BenchmarkMiddleware_Disabled measures baseline: no middleware at all.
func BenchmarkMiddleware_Disabled(b *testing.B) {
	backend := &noopBenchBackend{}
	req := &dynago.GetItemRequest{TableName: "bench"}
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		backend.GetItem(ctx, req)
	}
}

// BenchmarkMiddleware_NoopProvider measures overhead of OTel middleware with nil tracer/meter.
func BenchmarkMiddleware_NoopProvider(b *testing.B) {
	mw := dynagotel.NewMiddleware() // no tracer, no meter
	backend := mw(&noopBenchBackend{})
	req := &dynago.GetItemRequest{TableName: "bench"}
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		backend.GetItem(ctx, req)
	}
}

// BenchmarkMiddleware_LoggingOnly measures overhead of logging middleware with discarding logger.
func BenchmarkMiddleware_LoggingOnly(b *testing.B) {
	// Use a logger that discards all output.
	logger := slog.New(discardHandler{})
	mw := dynago.LoggingMiddleware(logger)
	backend := mw(&noopBenchBackend{})
	req := &dynago.GetItemRequest{TableName: "bench"}
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		backend.GetItem(ctx, req)
	}
}

// discardHandler is a slog.Handler that discards all records.
type discardHandler struct{}

func (discardHandler) Enabled(_ context.Context, _ slog.Level) bool  { return false }
func (discardHandler) Handle(_ context.Context, _ slog.Record) error { return nil }
func (d discardHandler) WithAttrs(_ []slog.Attr) slog.Handler        { return d }
func (d discardHandler) WithGroup(_ string) slog.Handler              { return d }
