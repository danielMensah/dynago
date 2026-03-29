package dynagotest

import (
	"context"
	"fmt"
	"testing"

	"github.com/danielmensah/dynago"
)

// AssertOption configures assertion checks on an item.
type AssertOption func(*assertConfig)

type assertConfig struct {
	attrChecks []attrCheck
}

type attrCheck struct {
	name     string
	expected any
}

// HasAttribute adds a check that the item has the given attribute with the
// expected value. The expected value is converted to an AttributeValue for
// comparison.
func HasAttribute(name string, expected any) AssertOption {
	return func(c *assertConfig) {
		c.attrChecks = append(c.attrChecks, attrCheck{name: name, expected: expected})
	}
}

// AssertItemExists fetches the item by key and fails the test if it does not
// exist. Additional AssertOption checks can be applied to verify attributes.
func AssertItemExists(t testing.TB, table *dynago.Table, key dynago.KeyValue, opts ...AssertOption) {
	t.Helper()

	resp, err := table.Backend().GetItem(context.Background(), &dynago.GetItemRequest{
		TableName: table.Name(),
		Key:       key.Map(),
	})
	if err != nil {
		t.Fatalf("AssertItemExists: GetItem failed: %v", err)
	}
	if len(resp.Item) == 0 {
		t.Fatalf("AssertItemExists: item not found for key %v", key.Map())
	}

	var cfg assertConfig
	for _, o := range opts {
		o(&cfg)
	}

	for _, check := range cfg.attrChecks {
		av, ok := resp.Item[check.name]
		if !ok {
			t.Errorf("AssertItemExists: attribute %q not found in item", check.name)
			continue
		}
		expectedAV := anyToAV(check.expected)
		if !avEqual(av, expectedAV) {
			t.Errorf("AssertItemExists: attribute %q: got %v, want %v", check.name, av, expectedAV)
		}
	}
}

// AssertItemNotExists fetches the item by key and fails the test if it exists.
func AssertItemNotExists(t testing.TB, table *dynago.Table, key dynago.KeyValue) {
	t.Helper()

	resp, err := table.Backend().GetItem(context.Background(), &dynago.GetItemRequest{
		TableName: table.Name(),
		Key:       key.Map(),
	})
	if err != nil {
		t.Fatalf("AssertItemNotExists: GetItem failed: %v", err)
	}
	if len(resp.Item) > 0 {
		t.Fatalf("AssertItemNotExists: item unexpectedly found for key %v", key.Map())
	}
}

// countItem is a minimal struct used internally for counting query results.
type countItem struct {
	// empty struct - we only care about the count
}

// AssertCount queries the table with the given key condition and asserts the
// result count matches expected.
func AssertCount(t testing.TB, table *dynago.Table, key dynago.KeyCondition, expected int) {
	t.Helper()

	results, err := dynago.Query[countItem](context.Background(), table, key)
	if err != nil {
		t.Fatalf("AssertCount: Query failed: %v", err)
	}
	if len(results) != expected {
		t.Fatalf("AssertCount: got %d items, want %d", len(results), expected)
	}
}

// anyToAV converts a Go value to an AttributeValue for comparison.
func anyToAV(v any) dynago.AttributeValue {
	switch val := v.(type) {
	case string:
		return dynago.AttributeValue{Type: dynago.TypeS, S: val}
	case int:
		return dynago.AttributeValue{Type: dynago.TypeN, N: fmt.Sprintf("%d", val)}
	case int64:
		return dynago.AttributeValue{Type: dynago.TypeN, N: fmt.Sprintf("%d", val)}
	case float64:
		return dynago.AttributeValue{Type: dynago.TypeN, N: fmt.Sprintf("%g", val)}
	case bool:
		return dynago.AttributeValue{Type: dynago.TypeBOOL, BOOL: val}
	case dynago.AttributeValue:
		return val
	default:
		return dynago.AttributeValue{}
	}
}

// avEqual checks if two AttributeValues are equal.
func avEqual(a, b dynago.AttributeValue) bool {
	if a.Type != b.Type {
		return false
	}
	switch a.Type {
	case dynago.TypeS:
		return a.S == b.S
	case dynago.TypeN:
		return a.N == b.N
	case dynago.TypeBOOL:
		return a.BOOL == b.BOOL
	case dynago.TypeNULL:
		return true
	default:
		return false
	}
}
