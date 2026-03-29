package dynago

import (
	"fmt"
	"reflect"
)

// Registry maps discriminator strings to concrete Go types for polymorphic
// unmarshaling in single-table designs.
type Registry struct {
	discriminatorAttr string
	types             map[string]reflect.Type
}

// NewRegistry creates a new Registry that uses the given attribute name as the
// discriminator field in DynamoDB items.
func NewRegistry(discriminatorAttr string) *Registry {
	return &Registry{
		discriminatorAttr: discriminatorAttr,
		types:             make(map[string]reflect.Type),
	}
}

// Register adds the Entity's concrete type to the registry, keyed by its
// discriminator value. It panics if another type is already registered with
// the same discriminator.
func (r *Registry) Register(e Entity) {
	info := e.DynagoEntity()
	if _, exists := r.types[info.Discriminator]; exists {
		panic(fmt.Sprintf("dynago: duplicate discriminator %q", info.Discriminator))
	}
	r.types[info.Discriminator] = reflect.TypeOf(e)
}

// Lookup returns the reflect.Type registered for the given discriminator.
// The second return value is false if no type is registered.
func (r *Registry) Lookup(discriminator string) (reflect.Type, bool) {
	t, ok := r.types[discriminator]
	return t, ok
}

// DiscriminatorAttr returns the attribute name used as the discriminator.
func (r *Registry) DiscriminatorAttr() string {
	return r.discriminatorAttr
}
