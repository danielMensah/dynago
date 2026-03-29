package dynago

import (
	"context"
	"iter"
)

// QueryIter returns a Go 1.23+ iterator that lazily pages through Query
// results. Each call to the yield function provides a single unmarshalled
// item of type T. On error, the iterator yields (zero, err) as the final
// element and stops. Breaking out of a range loop is safe and does not leak
// goroutines.
func QueryIter[T any](ctx context.Context, t *Table, key KeyCondition, opts ...QueryOption) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var cfg queryConfig
		for _, o := range opts {
			o(&cfg)
		}

		var startKey map[string]AttributeValue
		for {
			req := buildQueryRequest(t.Name(), key, &cfg, startKey)
			resp, err := t.Backend().Query(ctx, req)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}

			for _, item := range resp.Items {
				var v T
				if err := Unmarshal(item, &v); err != nil {
					var zero T
					yield(zero, err)
					return
				}
				if !yield(v, nil) {
					return
				}
			}

			if len(resp.LastEvaluatedKey) == 0 {
				return
			}
			startKey = resp.LastEvaluatedKey
		}
	}
}

// ScanIter returns a Go 1.23+ iterator that lazily pages through Scan
// results. Each call to the yield function provides a single unmarshalled
// item of type T. On error, the iterator yields (zero, err) as the final
// element and stops. Breaking out of a range loop is safe and does not leak
// goroutines.
func ScanIter[T any](ctx context.Context, t *Table, opts ...ScanOption) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var cfg scanConfig
		for _, o := range opts {
			o(&cfg)
		}

		var startKey map[string]AttributeValue
		for {
			req := buildScanRequest(t.Name(), &cfg, startKey)
			resp, err := t.Backend().Scan(ctx, req)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}

			for _, item := range resp.Items {
				var v T
				if err := Unmarshal(item, &v); err != nil {
					var zero T
					yield(zero, err)
					return
				}
				if !yield(v, nil) {
					return
				}
			}

			if len(resp.LastEvaluatedKey) == 0 {
				return
			}
			startKey = resp.LastEvaluatedKey
		}
	}
}
