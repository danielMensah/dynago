package dynago

import "testing"

func TestUnmarshalPolymorphic_User(t *testing.T) {
	r := NewRegistry("_type")
	r.Register(userEntity{})
	r.Register(orderEntity{})

	item := map[string]AttributeValue{
		"_type": {Type: TypeS, S: "USER"},
		"PK":    {Type: TypeS, S: "USER#1"},
		"SK":    {Type: TypeS, S: "PROFILE"},
		"Name":  {Type: TypeS, S: "Alice"},
	}

	v, err := unmarshalPolymorphic(item, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u, ok := v.(userEntity)
	if !ok {
		t.Fatalf("expected userEntity, got %T", v)
	}
	if u.Name != "Alice" {
		t.Fatalf("expected Alice, got %q", u.Name)
	}
	if u.PK != "USER#1" {
		t.Fatalf("expected USER#1, got %q", u.PK)
	}
}

func TestUnmarshalPolymorphic_Order(t *testing.T) {
	r := NewRegistry("_type")
	r.Register(userEntity{})
	r.Register(orderEntity{})

	item := map[string]AttributeValue{
		"_type":  {Type: TypeS, S: "ORDER"},
		"PK":     {Type: TypeS, S: "USER#1"},
		"SK":     {Type: TypeS, S: "ORDER#001"},
		"Amount": {Type: TypeN, N: "42"},
	}

	v, err := unmarshalPolymorphic(item, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	o, ok := v.(orderEntity)
	if !ok {
		t.Fatalf("expected orderEntity, got %T", v)
	}
	if o.Amount != 42 {
		t.Fatalf("expected 42, got %d", o.Amount)
	}
}

func TestUnmarshalPolymorphic_MissingDiscriminator(t *testing.T) {
	r := NewRegistry("_type")
	r.Register(userEntity{})

	item := map[string]AttributeValue{
		"PK": {Type: TypeS, S: "USER#1"},
	}

	_, err := unmarshalPolymorphic(item, r)
	if err == nil {
		t.Fatal("expected error for missing discriminator")
	}
}

func TestUnmarshalPolymorphic_UnregisteredDiscriminator(t *testing.T) {
	r := NewRegistry("_type")
	r.Register(userEntity{})

	item := map[string]AttributeValue{
		"_type": {Type: TypeS, S: "UNKNOWN"},
		"PK":    {Type: TypeS, S: "X"},
	}

	_, err := unmarshalPolymorphic(item, r)
	if err == nil {
		t.Fatal("expected error for unregistered discriminator")
	}
}
