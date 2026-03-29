// Package dynagotest provides test helpers for the dynago DynamoDB library.
// It includes seed utilities for populating tables and assertion helpers for
// verifying table state in tests.
//
// Example:
//
//	func TestMyFeature(t *testing.T) {
//	    backend := memdb.New()
//	    backend.CreateTable("users", memdb.TableSchema{
//	        HashKey: memdb.KeyDef{Name: "PK", Type: memdb.StringKey},
//	    })
//	    db := dynago.New(backend)
//	    table := db.Table("users")
//
//	    // Seed test data
//	    dynagotest.Seed(ctx, table, []any{User{PK: "u#1", Name: "Alice"}})
//
//	    // Assert
//	    dynagotest.AssertItemExists(t, table, dynago.Key("PK", "u#1"),
//	        dynagotest.HasAttribute("Name", "Alice"),
//	    )
//	}
package dynagotest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/danielmensah/dynago"
)

// Seed marshals and puts each item into the given table. Items are marshaled
// using dynago.Marshal and put sequentially. On failure, the error includes the
// index of the item that failed.
func Seed(ctx context.Context, table *dynago.Table, items []any) error {
	for i, item := range items {
		if err := table.Put(ctx, item); err != nil {
			return fmt.Errorf("dynagotest.Seed: item %d: %w", i, err)
		}
	}
	return nil
}

// SeedFromJSON reads a JSON file containing items in DynamoDB JSON format
// (with type descriptors like {"S": "value"}) and puts them into the table.
// The file must contain a JSON array of objects.
func SeedFromJSON(ctx context.Context, table *dynago.Table, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("dynagotest.SeedFromJSON: %w", err)
	}

	var rawItems []map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawItems); err != nil {
		return fmt.Errorf("dynagotest.SeedFromJSON: %w", err)
	}

	for i, rawItem := range rawItems {
		item, err := parseDynamoDBJSON(rawItem)
		if err != nil {
			return fmt.Errorf("dynagotest.SeedFromJSON: item %d: %w", i, err)
		}
		_, err = table.Backend().PutItem(ctx, &dynago.PutItemRequest{
			TableName: table.Name(),
			Item:      item,
		})
		if err != nil {
			return fmt.Errorf("dynagotest.SeedFromJSON: item %d: %w", i, err)
		}
	}
	return nil
}

// parseDynamoDBJSON converts a DynamoDB JSON object to a map of AttributeValues.
func parseDynamoDBJSON(raw map[string]json.RawMessage) (map[string]dynago.AttributeValue, error) {
	result := make(map[string]dynago.AttributeValue, len(raw))
	for key, val := range raw {
		av, err := parseDynamoDBValue(val)
		if err != nil {
			return nil, fmt.Errorf("attribute %q: %w", key, err)
		}
		result[key] = av
	}
	return result, nil
}

// parseDynamoDBValue converts a single DynamoDB JSON value descriptor to an AttributeValue.
func parseDynamoDBValue(raw json.RawMessage) (dynago.AttributeValue, error) {
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return dynago.AttributeValue{}, fmt.Errorf("expected type descriptor object: %w", err)
	}

	if len(wrapper) != 1 {
		return dynago.AttributeValue{}, fmt.Errorf("expected exactly one type key, got %d", len(wrapper))
	}

	for typKey, typVal := range wrapper {
		switch typKey {
		case "S":
			var s string
			if err := json.Unmarshal(typVal, &s); err != nil {
				return dynago.AttributeValue{}, err
			}
			return dynago.AttributeValue{Type: dynago.TypeS, S: s}, nil

		case "N":
			var s string
			if err := json.Unmarshal(typVal, &s); err != nil {
				return dynago.AttributeValue{}, err
			}
			// Validate it's a valid number.
			if _, err := strconv.ParseFloat(s, 64); err != nil {
				return dynago.AttributeValue{}, fmt.Errorf("invalid number %q: %w", s, err)
			}
			return dynago.AttributeValue{Type: dynago.TypeN, N: s}, nil

		case "BOOL":
			var b bool
			if err := json.Unmarshal(typVal, &b); err != nil {
				return dynago.AttributeValue{}, err
			}
			return dynago.AttributeValue{Type: dynago.TypeBOOL, BOOL: b}, nil

		case "NULL":
			return dynago.AttributeValue{Type: dynago.TypeNULL, NULL: true}, nil

		case "L":
			var items []json.RawMessage
			if err := json.Unmarshal(typVal, &items); err != nil {
				return dynago.AttributeValue{}, err
			}
			list := make([]dynago.AttributeValue, len(items))
			for i, item := range items {
				av, err := parseDynamoDBValue(item)
				if err != nil {
					return dynago.AttributeValue{}, fmt.Errorf("list[%d]: %w", i, err)
				}
				list[i] = av
			}
			return dynago.AttributeValue{Type: dynago.TypeL, L: list}, nil

		case "M":
			var m map[string]json.RawMessage
			if err := json.Unmarshal(typVal, &m); err != nil {
				return dynago.AttributeValue{}, err
			}
			result, err := parseDynamoDBJSON(m)
			if err != nil {
				return dynago.AttributeValue{}, err
			}
			return dynago.AttributeValue{Type: dynago.TypeM, M: result}, nil

		case "SS":
			var ss []string
			if err := json.Unmarshal(typVal, &ss); err != nil {
				return dynago.AttributeValue{}, err
			}
			return dynago.AttributeValue{Type: dynago.TypeSS, SS: ss}, nil

		case "NS":
			var ns []string
			if err := json.Unmarshal(typVal, &ns); err != nil {
				return dynago.AttributeValue{}, err
			}
			return dynago.AttributeValue{Type: dynago.TypeNS, NS: ns}, nil

		default:
			return dynago.AttributeValue{}, fmt.Errorf("unknown type descriptor %q", typKey)
		}
	}

	return dynago.AttributeValue{}, fmt.Errorf("empty type descriptor")
}
