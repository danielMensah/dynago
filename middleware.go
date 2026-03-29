package dynago

// Middleware wraps a Backend to add cross-cutting concerns such as logging,
// tracing, or metrics. See [LoggingMiddleware] and the dynagotel package.
type Middleware func(Backend) Backend

// WithMiddleware returns an [Option] that wraps the DB's backend with the given
// middleware. Middleware is applied in order: the last middleware becomes the
// outermost wrapper.
func WithMiddleware(m ...Middleware) Option {
	return func(db *DB) {
		for _, mw := range m {
			db.backend = mw(db.backend)
		}
	}
}
