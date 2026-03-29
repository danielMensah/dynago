package dynagotest_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielmensah/dynago"
	"github.com/danielmensah/dynago/dynagotest"
	"github.com/danielmensah/dynago/memdb"
)

type testItem struct {
	PK   string `dynamo:"PK"`
	SK   string `dynamo:"SK"`
	Name string `dynamo:"Name"`
	Age  int    `dynamo:"Age"`
}

func newTestTable() *dynago.Table {
	m := memdb.New()
	m.CreateTable("test", memdb.TableSchema{
		HashKey:  memdb.KeyDef{Name: "PK", Type: memdb.StringKey},
		RangeKey: &memdb.KeyDef{Name: "SK", Type: memdb.StringKey},
	})
	db := dynago.New(m)
	return db.Table("test")
}

// --- US-307: Seed Tests ---

func TestSeed(t *testing.T) {
	table := newTestTable()
	ctx := context.Background()

	items := []any{
		testItem{PK: "user#1", SK: "profile", Name: "Alice", Age: 30},
		testItem{PK: "user#2", SK: "profile", Name: "Bob", Age: 25},
	}

	if err := dynagotest.Seed(ctx, table, items); err != nil {
		t.Fatal(err)
	}

	// Verify items were put.
	resp, err := table.Backend().GetItem(ctx, &dynago.GetItemRequest{
		TableName: "test",
		Key: map[string]dynago.AttributeValue{
			"PK": {Type: dynago.TypeS, S: "user#1"},
			"SK": {Type: dynago.TypeS, S: "profile"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Item["Name"].S != "Alice" {
		t.Fatalf("expected Name=Alice, got %q", resp.Item["Name"].S)
	}
}

func TestSeedError(t *testing.T) {
	// Use a table that doesn't exist to trigger error.
	m := memdb.New()
	db := dynago.New(m)
	table := db.Table("nonexistent")

	err := dynagotest.Seed(context.Background(), table, []any{
		testItem{PK: "user#1", SK: "profile"},
	})
	if err == nil {
		t.Fatal("expected error for nonexistent table")
	}
}

func TestSeedFromJSON(t *testing.T) {
	table := newTestTable()
	ctx := context.Background()

	// Create a temporary JSON file.
	jsonData := `[
		{
			"PK": {"S": "user#1"},
			"SK": {"S": "profile"},
			"Name": {"S": "Alice"},
			"Age": {"N": "30"}
		},
		{
			"PK": {"S": "user#2"},
			"SK": {"S": "profile"},
			"Name": {"S": "Bob"},
			"Active": {"BOOL": true}
		}
	]`

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "items.json")
	if err := os.WriteFile(path, []byte(jsonData), 0644); err != nil {
		t.Fatal(err)
	}

	if err := dynagotest.SeedFromJSON(ctx, table, path); err != nil {
		t.Fatal(err)
	}

	// Verify first item.
	resp, err := table.Backend().GetItem(ctx, &dynago.GetItemRequest{
		TableName: "test",
		Key: map[string]dynago.AttributeValue{
			"PK": {Type: dynago.TypeS, S: "user#1"},
			"SK": {Type: dynago.TypeS, S: "profile"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Item["Name"].S != "Alice" {
		t.Fatalf("expected Name=Alice, got %q", resp.Item["Name"].S)
	}
	if resp.Item["Age"].N != "30" {
		t.Fatalf("expected Age=30, got %q", resp.Item["Age"].N)
	}

	// Verify second item.
	resp, err = table.Backend().GetItem(ctx, &dynago.GetItemRequest{
		TableName: "test",
		Key: map[string]dynago.AttributeValue{
			"PK": {Type: dynago.TypeS, S: "user#2"},
			"SK": {Type: dynago.TypeS, S: "profile"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Item["Active"].BOOL != true {
		t.Fatal("expected Active=true")
	}
}

func TestSeedFromJSONFileNotFound(t *testing.T) {
	table := newTestTable()
	err := dynagotest.SeedFromJSON(context.Background(), table, "/nonexistent/path.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestSeedFromJSONNestedTypes(t *testing.T) {
	table := newTestTable()
	ctx := context.Background()

	jsonData := `[
		{
			"PK": {"S": "user#1"},
			"SK": {"S": "profile"},
			"Tags": {"SS": ["go", "rust"]},
			"Scores": {"NS": ["100", "200"]},
			"Address": {"M": {"City": {"S": "NYC"}, "Zip": {"S": "10001"}}},
			"Items": {"L": [{"S": "a"}, {"N": "42"}]}
		}
	]`

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nested.json")
	if err := os.WriteFile(path, []byte(jsonData), 0644); err != nil {
		t.Fatal(err)
	}

	if err := dynagotest.SeedFromJSON(ctx, table, path); err != nil {
		t.Fatal(err)
	}

	resp, err := table.Backend().GetItem(ctx, &dynago.GetItemRequest{
		TableName: "test",
		Key: map[string]dynago.AttributeValue{
			"PK": {Type: dynago.TypeS, S: "user#1"},
			"SK": {Type: dynago.TypeS, S: "profile"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Item["Tags"].SS) != 2 {
		t.Fatalf("expected 2 SS values, got %d", len(resp.Item["Tags"].SS))
	}
	if resp.Item["Address"].M["City"].S != "NYC" {
		t.Fatal("expected nested City=NYC")
	}
	if len(resp.Item["Items"].L) != 2 {
		t.Fatalf("expected 2 list items, got %d", len(resp.Item["Items"].L))
	}
}

// --- US-308: Assertion Tests ---

func TestAssertItemExists(t *testing.T) {
	table := newTestTable()
	ctx := context.Background()

	dynagotest.Seed(ctx, table, []any{
		testItem{PK: "user#1", SK: "profile", Name: "Alice", Age: 30},
	})

	dynagotest.AssertItemExists(t, table, dynago.Key("PK", "user#1", "SK", "profile"))
}

func TestAssertItemExistsWithAttributes(t *testing.T) {
	table := newTestTable()
	ctx := context.Background()

	dynagotest.Seed(ctx, table, []any{
		testItem{PK: "user#1", SK: "profile", Name: "Alice", Age: 30},
	})

	dynagotest.AssertItemExists(t, table, dynago.Key("PK", "user#1", "SK", "profile"),
		dynagotest.HasAttribute("Name", "Alice"),
		dynagotest.HasAttribute("Age", 30),
	)
}

func TestAssertItemNotExists(t *testing.T) {
	table := newTestTable()
	dynagotest.AssertItemNotExists(t, table, dynago.Key("PK", "nonexistent", "SK", "nope"))
}

func TestAssertCount(t *testing.T) {
	table := newTestTable()
	ctx := context.Background()

	dynagotest.Seed(ctx, table, []any{
		testItem{PK: "user#1", SK: "order#001", Name: "Order 1"},
		testItem{PK: "user#1", SK: "order#002", Name: "Order 2"},
		testItem{PK: "user#1", SK: "order#003", Name: "Order 3"},
		testItem{PK: "user#2", SK: "order#001", Name: "Other"},
	})

	dynagotest.AssertCount(t, table, dynago.Partition("PK", "user#1"), 3)
	dynagotest.AssertCount(t, table, dynago.Partition("PK", "user#2"), 1)
	dynagotest.AssertCount(t, table, dynago.Partition("PK", "nobody"), 0)
}
