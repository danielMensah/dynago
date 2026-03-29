package dynago

import (
	"reflect"
	"testing"
)

func TestRegistry_RegisterAndLookup(t *testing.T) {
	r := NewRegistry("_type")

	r.Register(userEntity{})
	r.Register(orderEntity{})

	typ, ok := r.Lookup("USER")
	if !ok {
		t.Fatal("expected USER to be registered")
	}
	if typ != reflect.TypeOf(userEntity{}) {
		t.Fatalf("expected userEntity type, got %v", typ)
	}

	typ, ok = r.Lookup("ORDER")
	if !ok {
		t.Fatal("expected ORDER to be registered")
	}
	if typ != reflect.TypeOf(orderEntity{}) {
		t.Fatalf("expected orderEntity type, got %v", typ)
	}
}

func TestRegistry_LookupMiss(t *testing.T) {
	r := NewRegistry("_type")
	r.Register(userEntity{})

	_, ok := r.Lookup("UNKNOWN")
	if ok {
		t.Fatal("expected lookup miss for UNKNOWN")
	}
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	r := NewRegistry("_type")
	r.Register(userEntity{})

	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected panic on duplicate discriminator")
		}
	}()

	r.Register(userEntity{}) // same discriminator "USER"
}

func TestRegistry_DiscriminatorAttr(t *testing.T) {
	r := NewRegistry("entityType")
	if r.DiscriminatorAttr() != "entityType" {
		t.Fatalf("expected entityType, got %q", r.DiscriminatorAttr())
	}
}
