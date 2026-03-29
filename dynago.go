package dynago

// Option configures a DB instance.
type Option func(*DB)

// DB is the top-level handle for interacting with DynamoDB through a Backend.
type DB struct {
	backend Backend
}

// New creates a new DB with the given backend and options.
func New(backend Backend, opts ...Option) *DB {
	db := &DB{backend: backend}
	for _, o := range opts {
		o(db)
	}
	return db
}

// TableOption configures a Table instance.
type TableOption func(*Table)

// Table represents a single DynamoDB table bound to the DB's backend.
type Table struct {
	name     string
	backend  Backend
	registry *Registry
}

// WithRegistry attaches a Registry to the table for polymorphic support.
func WithRegistry(r *Registry) TableOption {
	return func(t *Table) {
		t.registry = r
	}
}

// Registry returns the table's Registry, or nil if none is set.
func (t *Table) Registry() *Registry {
	return t.registry
}

// Table creates a Table reference that uses the DB's backend.
func (db *DB) Table(name string, opts ...TableOption) *Table {
	t := &Table{
		name:    name,
		backend: db.backend,
	}
	for _, o := range opts {
		o(t)
	}
	return t
}

// Name returns the table's DynamoDB table name.
func (t *Table) Name() string {
	return t.name
}

// Backend returns the table's backend. This is used internally by free
// functions such as Get[T] and Query[T].
func (t *Table) Backend() Backend {
	return t.backend
}
