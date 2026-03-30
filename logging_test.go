package dynago

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// captureHandler is a slog.Handler that stores all records for inspection.
type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(_ string) slog.Handler      { return h }

func (h *captureHandler) getRecords() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	dst := make([]slog.Record, len(h.records))
	copy(dst, h.records)
	return dst
}

func TestLoggingMiddleware_LogsDebug(t *testing.T) {
	handler := &captureHandler{}
	logger := slog.New(handler)

	noop := &noopBackend{
		getResp:   &GetItemResponse{},
		putResp:   &PutItemResponse{},
		queryResp: &QueryResponse{},
	}

	mw := LoggingMiddleware(logger)
	backend := mw(noop)

	ctx := context.Background()
	backend.GetItem(ctx, &GetItemRequest{TableName: "users"})
	backend.PutItem(ctx, &PutItemRequest{TableName: "orders"})

	records := handler.getRecords()
	if len(records) != 2 {
		t.Fatalf("expected 2 log records, got %d", len(records))
	}

	// Both should be DEBUG level.
	for i, r := range records {
		if r.Level != slog.LevelDebug {
			t.Errorf("record %d: expected DEBUG, got %s", i, r.Level)
		}
	}

	// Check attributes on first record.
	var gotOp, gotTable string
	records[0].Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "operation":
			gotOp = a.Value.String()
		case "table":
			gotTable = a.Value.String()
		}
		return true
	})
	if gotOp != "GetItem" {
		t.Errorf("expected operation=GetItem, got %q", gotOp)
	}
	if gotTable != "users" {
		t.Errorf("expected table=users, got %q", gotTable)
	}
}

// slowBackend sleeps for a configurable duration on GetItem.
type slowBackend struct {
	noopBackend
	delay time.Duration
}

func (s *slowBackend) GetItem(ctx context.Context, req *GetItemRequest) (*GetItemResponse, error) {
	time.Sleep(s.delay)
	return &GetItemResponse{}, nil
}

func TestLoggingMiddleware_SlowThreshold(t *testing.T) {
	handler := &captureHandler{}
	logger := slog.New(handler)

	slow := &slowBackend{delay: 50 * time.Millisecond}
	mw := LoggingMiddleware(logger, LogSlowOperations(10*time.Millisecond))
	backend := mw(slow)

	ctx := context.Background()
	backend.GetItem(ctx, &GetItemRequest{TableName: "slow-table"})

	records := handler.getRecords()
	// Should have both DEBUG and WARN records.
	if len(records) != 2 {
		t.Fatalf("expected 2 log records (DEBUG + WARN), got %d", len(records))
	}
	if records[0].Level != slog.LevelDebug {
		t.Errorf("first record: expected DEBUG, got %s", records[0].Level)
	}
	if records[1].Level != slog.LevelWarn {
		t.Errorf("second record: expected WARN, got %s", records[1].Level)
	}
}

func TestLoggingMiddleware_AllOperations(t *testing.T) {
	handler := &captureHandler{}
	logger := slog.New(handler)

	noop := &noopBackend{
		getResp:   &GetItemResponse{},
		putResp:   &PutItemResponse{},
		queryResp: &QueryResponse{},
	}

	mw := LoggingMiddleware(logger)
	backend := mw(noop)

	ctx := context.Background()
	backend.GetItem(ctx, &GetItemRequest{TableName: "t"})
	backend.PutItem(ctx, &PutItemRequest{TableName: "t"})
	backend.DeleteItem(ctx, &DeleteItemRequest{TableName: "t"})
	backend.UpdateItem(ctx, &UpdateItemRequest{TableName: "t"})
	backend.Query(ctx, &QueryRequest{TableName: "t"})
	backend.Scan(ctx, &ScanRequest{TableName: "t"})
	backend.BatchGetItem(ctx, &BatchGetItemRequest{})
	backend.BatchWriteItem(ctx, &BatchWriteItemRequest{})
	backend.TransactGetItems(ctx, &TransactGetItemsRequest{})
	backend.TransactWriteItems(ctx, &TransactWriteItemsRequest{})

	records := handler.getRecords()
	if len(records) != 10 {
		t.Fatalf("expected 10 log records, got %d", len(records))
	}

	expected := []string{
		"GetItem", "PutItem", "DeleteItem", "UpdateItem",
		"Query", "Scan", "BatchGetItem", "BatchWriteItem",
		"TransactGetItems", "TransactWriteItems",
	}
	for i, r := range records {
		var gotOp string
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "operation" {
				gotOp = a.Value.String()
			}
			return true
		})
		if gotOp != expected[i] {
			t.Errorf("record %d: expected operation=%q, got %q", i, expected[i], gotOp)
		}
	}
}
