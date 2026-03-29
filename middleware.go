package dynago

// Middleware wraps a Backend to add cross-cutting concerns.
type Middleware func(Backend) Backend

// WithMiddleware adds middleware to the DB. Applied in order: last is outermost.
func WithMiddleware(m ...Middleware) Option {
	return func(db *DB) {
		for _, mw := range m {
			db.backend = mw(db.backend)
		}
	}
}
