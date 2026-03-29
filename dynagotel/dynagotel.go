// Package dynagotel provides OpenTelemetry instrumentation middleware for dynago.
package dynagotel

import (
	"context"
	"time"

	"github.com/danielmensah/dynago"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Option configures the OpenTelemetry middleware.
type Option func(*config)

type config struct {
	tracer  trace.Tracer
	meter   metric.Meter
	counter metric.Int64Counter
	latency metric.Float64Histogram
}

// WithTracer sets the tracer used for creating spans.
func WithTracer(t trace.Tracer) Option {
	return func(c *config) {
		c.tracer = t
	}
}

// WithMeter sets the meter used for recording metrics.
func WithMeter(m metric.Meter) Option {
	return func(c *config) {
		c.meter = m
	}
}

// NewMiddleware returns a dynago.Middleware that adds OpenTelemetry tracing and metrics.
// If no tracer or meter is provided, those aspects are skipped (noop-safe).
func NewMiddleware(opts ...Option) dynago.Middleware {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}

	// Initialize metric instruments if a meter was provided.
	if cfg.meter != nil {
		var err error
		cfg.counter, err = cfg.meter.Int64Counter("dynago.operations.total",
			metric.WithDescription("Total number of dynago operations"),
			metric.WithUnit("{operation}"),
		)
		if err != nil {
			cfg.counter = nil
		}
		cfg.latency, err = cfg.meter.Float64Histogram("dynago.latency",
			metric.WithDescription("Latency of dynago operations"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			cfg.latency = nil
		}
	}

	return func(inner dynago.Backend) dynago.Backend {
		return &otelBackend{inner: inner, cfg: cfg}
	}
}

type otelBackend struct {
	inner dynago.Backend
	cfg   *config
}

func (o *otelBackend) instrument(ctx context.Context, operation, table string) (context.Context, func(error)) {
	attrs := []attribute.KeyValue{
		attribute.String("db.system", "dynamodb"),
		attribute.String("db.operation", operation),
	}
	if table != "" {
		attrs = append(attrs, attribute.String("db.name", table))
	}

	if o.cfg.tracer != nil {
		ctx, _ = o.cfg.tracer.Start(ctx, operation,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(attrs...),
		)
	}

	start := time.Now()
	metricAttrs := metric.WithAttributes(attrs...)

	return ctx, func(err error) {
		elapsed := float64(time.Since(start).Milliseconds())

		if o.cfg.tracer != nil {
			span := trace.SpanFromContext(ctx)
			if err != nil {
				span.RecordError(err)
			}
			span.End()
		}

		if o.cfg.counter != nil {
			o.cfg.counter.Add(ctx, 1, metricAttrs)
		}
		if o.cfg.latency != nil {
			o.cfg.latency.Record(ctx, elapsed, metricAttrs)
		}
	}
}

func (o *otelBackend) GetItem(ctx context.Context, req *dynago.GetItemRequest) (*dynago.GetItemResponse, error) {
	ctx, done := o.instrument(ctx, "GetItem", req.TableName)
	resp, err := o.inner.GetItem(ctx, req)
	done(err)
	return resp, err
}

func (o *otelBackend) PutItem(ctx context.Context, req *dynago.PutItemRequest) (*dynago.PutItemResponse, error) {
	ctx, done := o.instrument(ctx, "PutItem", req.TableName)
	resp, err := o.inner.PutItem(ctx, req)
	done(err)
	return resp, err
}

func (o *otelBackend) DeleteItem(ctx context.Context, req *dynago.DeleteItemRequest) (*dynago.DeleteItemResponse, error) {
	ctx, done := o.instrument(ctx, "DeleteItem", req.TableName)
	resp, err := o.inner.DeleteItem(ctx, req)
	done(err)
	return resp, err
}

func (o *otelBackend) UpdateItem(ctx context.Context, req *dynago.UpdateItemRequest) (*dynago.UpdateItemResponse, error) {
	ctx, done := o.instrument(ctx, "UpdateItem", req.TableName)
	resp, err := o.inner.UpdateItem(ctx, req)
	done(err)
	return resp, err
}

func (o *otelBackend) Query(ctx context.Context, req *dynago.QueryRequest) (*dynago.QueryResponse, error) {
	ctx, done := o.instrument(ctx, "Query", req.TableName)
	resp, err := o.inner.Query(ctx, req)
	done(err)
	return resp, err
}

func (o *otelBackend) Scan(ctx context.Context, req *dynago.ScanRequest) (*dynago.ScanResponse, error) {
	ctx, done := o.instrument(ctx, "Scan", req.TableName)
	resp, err := o.inner.Scan(ctx, req)
	done(err)
	return resp, err
}

func (o *otelBackend) BatchGetItem(ctx context.Context, req *dynago.BatchGetItemRequest) (*dynago.BatchGetItemResponse, error) {
	ctx, done := o.instrument(ctx, "BatchGetItem", "")
	resp, err := o.inner.BatchGetItem(ctx, req)
	done(err)
	return resp, err
}

func (o *otelBackend) BatchWriteItem(ctx context.Context, req *dynago.BatchWriteItemRequest) (*dynago.BatchWriteItemResponse, error) {
	ctx, done := o.instrument(ctx, "BatchWriteItem", "")
	resp, err := o.inner.BatchWriteItem(ctx, req)
	done(err)
	return resp, err
}

func (o *otelBackend) TransactGetItems(ctx context.Context, req *dynago.TransactGetItemsRequest) (*dynago.TransactGetItemsResponse, error) {
	ctx, done := o.instrument(ctx, "TransactGetItems", "")
	resp, err := o.inner.TransactGetItems(ctx, req)
	done(err)
	return resp, err
}

func (o *otelBackend) TransactWriteItems(ctx context.Context, req *dynago.TransactWriteItemsRequest) (*dynago.TransactWriteItemsResponse, error) {
	ctx, done := o.instrument(ctx, "TransactWriteItems", "")
	resp, err := o.inner.TransactWriteItems(ctx, req)
	done(err)
	return resp, err
}
