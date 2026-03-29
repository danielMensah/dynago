package dynago

import "testing"

func TestTable_WithRegistry(t *testing.T) {
	r := NewRegistry("_type")
	r.Register(userEntity{})

	sb := &putDeleteStub{}
	db := New(sb)
	tbl := db.Table("MyTable", WithRegistry(r))

	if tbl.Registry() != r {
		t.Fatal("expected registry to be set")
	}
	if tbl.Registry().DiscriminatorAttr() != "_type" {
		t.Fatalf("expected _type, got %q", tbl.Registry().DiscriminatorAttr())
	}
}

func TestTable_NoRegistry(t *testing.T) {
	sb := &putDeleteStub{}
	db := New(sb)
	tbl := db.Table("MyTable")

	if tbl.Registry() != nil {
		t.Fatal("expected nil registry")
	}
}

func TestTable_NameAndBackend(t *testing.T) {
	sb := &putDeleteStub{}
	db := New(sb)
	tbl := db.Table("TestTable")

	if tbl.Name() != "TestTable" {
		t.Fatalf("expected TestTable, got %q", tbl.Name())
	}
	if tbl.Backend() != sb {
		t.Fatal("expected backend to match")
	}
}
