package dynago

import (
	"context"
	"log/slog"
	"time"
)

// LogOption configures the logging middleware.
type LogOption func(*logConfig)

type logConfig struct {
	slowThreshold time.Duration
}

// LogSlowOperations sets the threshold above which operations are logged at WARN level.
func LogSlowOperations(threshold time.Duration) LogOption {
	return func(c *logConfig) {
		c.slowThreshold = threshold
	}
}

// LoggingMiddleware returns a Middleware that logs every Backend operation using the provided slog.Logger.
// All operations are logged at DEBUG level. If a slow threshold is set via LogSlowOperations,
// operations exceeding that threshold are additionally logged at WARN level.
func LoggingMiddleware(logger *slog.Logger, opts ...LogOption) Middleware {
	cfg := &logConfig{}
	for _, o := range opts {
		o(cfg)
	}
	return func(inner Backend) Backend {
		return &loggingBackend{inner: inner, logger: logger, cfg: cfg}
	}
}

type loggingBackend struct {
	inner  Backend
	logger *slog.Logger
	cfg    *logConfig
}

func (l *loggingBackend) log(operation, table string, dur time.Duration, err error) {
	attrs := []slog.Attr{
		slog.String("operation", operation),
		slog.String("table", table),
		slog.Duration("duration", dur),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}

	l.logger.LogAttrs(context.Background(), slog.LevelDebug, "dynago operation", attrs...)

	if l.cfg.slowThreshold > 0 && dur >= l.cfg.slowThreshold {
		l.logger.LogAttrs(context.Background(), slog.LevelWarn, "slow dynago operation", attrs...)
	}
}

func (l *loggingBackend) GetItem(ctx context.Context, req *GetItemRequest) (*GetItemResponse, error) {
	start := time.Now()
	resp, err := l.inner.GetItem(ctx, req)
	l.log("GetItem", req.TableName, time.Since(start), err)
	return resp, err
}

func (l *loggingBackend) PutItem(ctx context.Context, req *PutItemRequest) (*PutItemResponse, error) {
	start := time.Now()
	resp, err := l.inner.PutItem(ctx, req)
	l.log("PutItem", req.TableName, time.Since(start), err)
	return resp, err
}

func (l *loggingBackend) DeleteItem(ctx context.Context, req *DeleteItemRequest) (*DeleteItemResponse, error) {
	start := time.Now()
	resp, err := l.inner.DeleteItem(ctx, req)
	l.log("DeleteItem", req.TableName, time.Since(start), err)
	return resp, err
}

func (l *loggingBackend) UpdateItem(ctx context.Context, req *UpdateItemRequest) (*UpdateItemResponse, error) {
	start := time.Now()
	resp, err := l.inner.UpdateItem(ctx, req)
	l.log("UpdateItem", req.TableName, time.Since(start), err)
	return resp, err
}

func (l *loggingBackend) Query(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
	start := time.Now()
	resp, err := l.inner.Query(ctx, req)
	l.log("Query", req.TableName, time.Since(start), err)
	return resp, err
}

func (l *loggingBackend) Scan(ctx context.Context, req *ScanRequest) (*ScanResponse, error) {
	start := time.Now()
	resp, err := l.inner.Scan(ctx, req)
	l.log("Scan", req.TableName, time.Since(start), err)
	return resp, err
}

func (l *loggingBackend) BatchGetItem(ctx context.Context, req *BatchGetItemRequest) (*BatchGetItemResponse, error) {
	start := time.Now()
	resp, err := l.inner.BatchGetItem(ctx, req)
	l.log("BatchGetItem", "", time.Since(start), err)
	return resp, err
}

func (l *loggingBackend) BatchWriteItem(ctx context.Context, req *BatchWriteItemRequest) (*BatchWriteItemResponse, error) {
	start := time.Now()
	resp, err := l.inner.BatchWriteItem(ctx, req)
	l.log("BatchWriteItem", "", time.Since(start), err)
	return resp, err
}

func (l *loggingBackend) TransactGetItems(ctx context.Context, req *TransactGetItemsRequest) (*TransactGetItemsResponse, error) {
	start := time.Now()
	resp, err := l.inner.TransactGetItems(ctx, req)
	l.log("TransactGetItems", "", time.Since(start), err)
	return resp, err
}

func (l *loggingBackend) TransactWriteItems(ctx context.Context, req *TransactWriteItemsRequest) (*TransactWriteItemsResponse, error) {
	start := time.Now()
	resp, err := l.inner.TransactWriteItems(ctx, req)
	l.log("TransactWriteItems", "", time.Since(start), err)
	return resp, err
}
