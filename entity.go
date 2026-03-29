package dynago

// EntityInfo holds metadata about an entity type for single-table design.
type EntityInfo struct {
	Discriminator string
}

// Entity is the interface that entity types must implement for polymorphic support.
type Entity interface {
	DynagoEntity() EntityInfo
}
