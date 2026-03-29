package dynago

import (
	"fmt"
	"reflect"
)

// unmarshalPolymorphic reads the discriminator attribute from the item, looks
// up the concrete type in the registry, creates a new instance, and calls
// Unmarshal to populate it. It returns an error if the discriminator attribute
// is missing or the discriminator value is not registered.
func unmarshalPolymorphic(item map[string]AttributeValue, registry *Registry) (any, error) {
	attr := registry.DiscriminatorAttr()
	av, ok := item[attr]
	if !ok {
		return nil, fmt.Errorf("dynago: discriminator attribute %q not found in item", attr)
	}
	if av.Type != TypeS {
		return nil, fmt.Errorf("dynago: discriminator attribute %q must be type S, got %d", attr, av.Type)
	}

	typ, ok := registry.Lookup(av.S)
	if !ok {
		return nil, fmt.Errorf("dynago: unregistered discriminator %q", av.S)
	}

	ptr := reflect.New(typ).Interface()
	if err := Unmarshal(item, ptr); err != nil {
		return nil, err
	}
	return reflect.ValueOf(ptr).Elem().Interface(), nil
}
