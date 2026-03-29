package memdb

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/danielmensah/dynago"
)

func newTestBackend() *MemoryBackend {
	m := New()
	m.CreateTable("users", TableSchema{
		HashKey: KeyDef{Name: "PK", Type: StringKey},
		RangeKey: &KeyDef{Name: "SK", Type: StringKey},
	})
	return m
}

func newTestBackendWithGSI() *MemoryBackend {
	m := New()
	m.CreateTable("users", TableSchema{
		HashKey:  KeyDef{Name: "PK", Type: StringKey},
		RangeKey: &KeyDef{Name: "SK", Type: StringKey},
		GSIs: []GSISchema{
			{
				Name:    "email-index",
				HashKey: KeyDef{Name: "Email", Type: StringKey},
			},
			{
				Name:     "status-created-index",
				HashKey:  KeyDef{Name: "Status", Type: StringKey},
				RangeKey: &KeyDef{Name: "CreatedAt", Type: StringKey},
			},
		},
	})
	return m
}

func strAV(s string) dynago.AttributeValue {
	return dynago.AttributeValue{Type: dynago.TypeS, S: s}
}

func numAV(n string) dynago.AttributeValue {
	return dynago.AttributeValue{Type: dynago.TypeN, N: n}
}

// --- US-300: CreateTable and interface compliance ---

func TestNew(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatal("New() returned nil")
	}
}

func TestCreateTable(t *testing.T) {
	m := New()
	m.CreateTable("test", TableSchema{
		HashKey: KeyDef{Name: "id", Type: StringKey},
	})
	// Should work fine
}

func TestCreateTableDuplicate(t *testing.T) {
	m := New()
	m.CreateTable("test", TableSchema{HashKey: KeyDef{Name: "id", Type: StringKey}})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for duplicate table")
		}
	}()
	m.CreateTable("test", TableSchema{HashKey: KeyDef{Name: "id", Type: StringKey}})
}

func TestBackendInterface(t *testing.T) {
	var _ dynago.Backend = (*MemoryBackend)(nil)
}

func TestTableNotFound(t *testing.T) {
	m := New()
	_, err := m.GetItem(context.Background(), &dynago.GetItemRequest{
		TableName: "nonexistent",
		Key:       map[string]dynago.AttributeValue{"PK": strAV("1")},
	})
	if err == nil {
		t.Fatal("expected error for nonexistent table")
	}
	if !errors.Is(err, dynago.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

// --- US-301: GetItem and PutItem ---

func TestPutAndGetItem(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	item := map[string]dynago.AttributeValue{
		"PK":   strAV("user#1"),
		"SK":   strAV("profile"),
		"Name": strAV("Alice"),
		"Age":  numAV("30"),
	}

	_, err := m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      item,
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := m.GetItem(ctx, &dynago.GetItemRequest{
		TableName: "users",
		Key: map[string]dynago.AttributeValue{
			"PK": strAV("user#1"),
			"SK": strAV("profile"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Item == nil {
		t.Fatal("expected item, got nil")
	}
	if resp.Item["Name"].S != "Alice" {
		t.Fatalf("expected Name=Alice, got %q", resp.Item["Name"].S)
	}
	if resp.Item["Age"].N != "30" {
		t.Fatalf("expected Age=30, got %q", resp.Item["Age"].N)
	}
}

func TestGetItemNotFound(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	resp, err := m.GetItem(ctx, &dynago.GetItemRequest{
		TableName: "users",
		Key: map[string]dynago.AttributeValue{
			"PK": strAV("nope"),
			"SK": strAV("nope"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Item != nil {
		t.Fatal("expected nil item for not found")
	}
}

func TestPutItemOverwrite(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	key := map[string]dynago.AttributeValue{
		"PK": strAV("user#1"),
		"SK": strAV("profile"),
	}

	_, err := m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item: map[string]dynago.AttributeValue{
			"PK":   strAV("user#1"),
			"SK":   strAV("profile"),
			"Name": strAV("Alice"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item: map[string]dynago.AttributeValue{
			"PK":   strAV("user#1"),
			"SK":   strAV("profile"),
			"Name": strAV("Bob"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := m.GetItem(ctx, &dynago.GetItemRequest{TableName: "users", Key: key})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Item["Name"].S != "Bob" {
		t.Fatalf("expected Bob, got %q", resp.Item["Name"].S)
	}
}

func TestPutItemDeepCopy(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	item := map[string]dynago.AttributeValue{
		"PK":   strAV("user#1"),
		"SK":   strAV("profile"),
		"Name": strAV("Alice"),
	}

	_, err := m.PutItem(ctx, &dynago.PutItemRequest{TableName: "users", Item: item})
	if err != nil {
		t.Fatal(err)
	}

	// Mutate original - should not affect stored item
	item["Name"] = strAV("Mutated")

	resp, err := m.GetItem(ctx, &dynago.GetItemRequest{
		TableName: "users",
		Key: map[string]dynago.AttributeValue{
			"PK": strAV("user#1"),
			"SK": strAV("profile"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Item["Name"].S != "Alice" {
		t.Fatal("stored item was mutated")
	}
}

func TestGetItemDeepCopy(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, err := m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item: map[string]dynago.AttributeValue{
			"PK":   strAV("user#1"),
			"SK":   strAV("profile"),
			"Name": strAV("Alice"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	key := map[string]dynago.AttributeValue{
		"PK": strAV("user#1"),
		"SK": strAV("profile"),
	}

	resp, err := m.GetItem(ctx, &dynago.GetItemRequest{TableName: "users", Key: key})
	if err != nil {
		t.Fatal(err)
	}

	// Mutate returned item
	resp.Item["Name"] = strAV("Mutated")

	// Fetch again - should be unchanged
	resp2, err := m.GetItem(ctx, &dynago.GetItemRequest{TableName: "users", Key: key})
	if err != nil {
		t.Fatal(err)
	}
	if resp2.Item["Name"].S != "Alice" {
		t.Fatal("stored item was mutated through returned reference")
	}
}

func TestPutItemConditionExpression(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	// Put with attribute_not_exists condition should succeed for new item
	_, err := m.PutItem(ctx, &dynago.PutItemRequest{
		TableName:           "users",
		Item:                map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice")},
		ConditionExpression: "attribute_not_exists(#pk)",
		ExpressionAttributeNames: map[string]string{"#pk": "PK"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Put again with same condition should fail
	_, err = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName:           "users",
		Item:                map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Bob")},
		ConditionExpression: "attribute_not_exists(#pk)",
		ExpressionAttributeNames: map[string]string{"#pk": "PK"},
	})
	if !errors.Is(err, dynago.ErrConditionFailed) {
		t.Fatalf("expected ErrConditionFailed, got %v", err)
	}
}

func TestPutItemConditionWithValues(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, err := m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item: map[string]dynago.AttributeValue{
			"PK": strAV("user#1"), "SK": strAV("profile"),
			"Age": numAV("30"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Condition: Age = :val (should pass)
	_, err = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item: map[string]dynago.AttributeValue{
			"PK": strAV("user#1"), "SK": strAV("profile"),
			"Age": numAV("31"),
		},
		ConditionExpression:       "#age = :val",
		ExpressionAttributeNames:  map[string]string{"#age": "Age"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":val": numAV("30")},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetItemProjection(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, err := m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item: map[string]dynago.AttributeValue{
			"PK": strAV("user#1"), "SK": strAV("profile"),
			"Name": strAV("Alice"), "Age": numAV("30"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := m.GetItem(ctx, &dynago.GetItemRequest{
		TableName: "users",
		Key: map[string]dynago.AttributeValue{
			"PK": strAV("user#1"), "SK": strAV("profile"),
		},
		ProjectionExpression:     "#n",
		ExpressionAttributeNames: map[string]string{"#n": "Name"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Item) != 1 {
		t.Fatalf("expected 1 attribute in projection, got %d", len(resp.Item))
	}
	if resp.Item["Name"].S != "Alice" {
		t.Fatalf("expected Name=Alice, got %q", resp.Item["Name"].S)
	}
}

func TestPutItemMissingKey(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, err := m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1")},
	})
	if !errors.Is(err, dynago.ErrValidation) {
		t.Fatalf("expected ErrValidation for missing range key, got %v", err)
	}
}

// --- US-302: DeleteItem ---

func TestDeleteItem(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, err := m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item: map[string]dynago.AttributeValue{
			"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = m.DeleteItem(ctx, &dynago.DeleteItemRequest{
		TableName: "users",
		Key: map[string]dynago.AttributeValue{
			"PK": strAV("user#1"), "SK": strAV("profile"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := m.GetItem(ctx, &dynago.GetItemRequest{
		TableName: "users",
		Key: map[string]dynago.AttributeValue{
			"PK": strAV("user#1"), "SK": strAV("profile"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Item != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestDeleteItemNotExist(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	// Delete non-existent item should succeed (no-op)
	_, err := m.DeleteItem(ctx, &dynago.DeleteItemRequest{
		TableName: "users",
		Key: map[string]dynago.AttributeValue{
			"PK": strAV("nope"), "SK": strAV("nope"),
		},
	})
	if err != nil {
		t.Fatal("delete of non-existent item should not error")
	}
}

func TestDeleteItemConditionFails(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	// Delete with condition on non-existent item fails
	_, err := m.DeleteItem(ctx, &dynago.DeleteItemRequest{
		TableName:           "users",
		Key:                 map[string]dynago.AttributeValue{"PK": strAV("nope"), "SK": strAV("nope")},
		ConditionExpression: "attribute_exists(#pk)",
		ExpressionAttributeNames: map[string]string{"#pk": "PK"},
	})
	if !errors.Is(err, dynago.ErrConditionFailed) {
		t.Fatalf("expected ErrConditionFailed, got %v", err)
	}
}

func TestDeleteItemConditionPasses(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice")},
	})

	_, err := m.DeleteItem(ctx, &dynago.DeleteItemRequest{
		TableName:           "users",
		Key:                 map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
		ConditionExpression: "#name = :val",
		ExpressionAttributeNames:  map[string]string{"#name": "Name"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":val": strAV("Alice")},
	})
	if err != nil {
		t.Fatal(err)
	}
}

// --- US-303: UpdateItem ---

func TestUpdateItemSetAttribute(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice")},
	})

	resp, err := m.UpdateItem(ctx, &dynago.UpdateItemRequest{
		TableName:        "users",
		Key:              map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
		UpdateExpression: "SET #name = :val",
		ExpressionAttributeNames:  map[string]string{"#name": "Name"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":val": strAV("Bob")},
		ReturnValues:     "ALL_NEW",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Attributes["Name"].S != "Bob" {
		t.Fatalf("expected Name=Bob, got %q", resp.Attributes["Name"].S)
	}
}

func TestUpdateItemUpsert(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	resp, err := m.UpdateItem(ctx, &dynago.UpdateItemRequest{
		TableName:        "users",
		Key:              map[string]dynago.AttributeValue{"PK": strAV("user#new"), "SK": strAV("profile")},
		UpdateExpression: "SET #name = :val",
		ExpressionAttributeNames:  map[string]string{"#name": "Name"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":val": strAV("Charlie")},
		ReturnValues:     "ALL_NEW",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Attributes["Name"].S != "Charlie" {
		t.Fatalf("expected Name=Charlie, got %q", resp.Attributes["Name"].S)
	}
	if resp.Attributes["PK"].S != "user#new" {
		t.Fatal("expected key to be present in upserted item")
	}
}

func TestUpdateItemReturnValuesOLD(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice")},
	})

	resp, err := m.UpdateItem(ctx, &dynago.UpdateItemRequest{
		TableName:        "users",
		Key:              map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
		UpdateExpression: "SET #name = :val",
		ExpressionAttributeNames:  map[string]string{"#name": "Name"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":val": strAV("Bob")},
		ReturnValues:     "ALL_OLD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Attributes["Name"].S != "Alice" {
		t.Fatalf("expected old Name=Alice, got %q", resp.Attributes["Name"].S)
	}
}

func TestUpdateItemConditionFails(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice")},
	})

	_, err := m.UpdateItem(ctx, &dynago.UpdateItemRequest{
		TableName:           "users",
		Key:                 map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
		UpdateExpression:    "SET #name = :newval",
		ConditionExpression: "#name = :oldval",
		ExpressionAttributeNames:  map[string]string{"#name": "Name"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":newval": strAV("Bob"), ":oldval": strAV("Wrong")},
	})
	if !errors.Is(err, dynago.ErrConditionFailed) {
		t.Fatalf("expected ErrConditionFailed, got %v", err)
	}
}

func TestUpdateItemRemove(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice"), "Age": numAV("30")},
	})

	resp, err := m.UpdateItem(ctx, &dynago.UpdateItemRequest{
		TableName:        "users",
		Key:              map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
		UpdateExpression: "REMOVE #age",
		ExpressionAttributeNames: map[string]string{"#age": "Age"},
		ReturnValues:     "ALL_NEW",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := resp.Attributes["Age"]; ok {
		t.Fatal("expected Age to be removed")
	}
	if resp.Attributes["Name"].S != "Alice" {
		t.Fatal("expected Name to still be Alice")
	}
}

func TestUpdateItemAdd(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Counter": numAV("5")},
	})

	resp, err := m.UpdateItem(ctx, &dynago.UpdateItemRequest{
		TableName:        "users",
		Key:              map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
		UpdateExpression: "ADD #counter :val",
		ExpressionAttributeNames:  map[string]string{"#counter": "Counter"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":val": numAV("3")},
		ReturnValues:     "ALL_NEW",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Attributes["Counter"].N != "8" {
		t.Fatalf("expected Counter=8, got %q", resp.Attributes["Counter"].N)
	}
}

func TestUpdateItemMultipleActions(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item: map[string]dynago.AttributeValue{
			"PK": strAV("user#1"), "SK": strAV("profile"),
			"Name": strAV("Alice"), "Age": numAV("30"), "Old": strAV("remove-me"),
		},
	})

	resp, err := m.UpdateItem(ctx, &dynago.UpdateItemRequest{
		TableName:        "users",
		Key:              map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
		UpdateExpression: "SET #name = :name REMOVE #old",
		ExpressionAttributeNames:  map[string]string{"#name": "Name", "#old": "Old"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":name": strAV("Bob")},
		ReturnValues:     "ALL_NEW",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Attributes["Name"].S != "Bob" {
		t.Fatalf("expected Name=Bob, got %q", resp.Attributes["Name"].S)
	}
	if _, ok := resp.Attributes["Old"]; ok {
		t.Fatal("expected Old to be removed")
	}
}

func TestUpdateItemArithmetic(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Counter": numAV("10")},
	})

	resp, err := m.UpdateItem(ctx, &dynago.UpdateItemRequest{
		TableName:        "users",
		Key:              map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
		UpdateExpression: "SET #counter = #counter + :val",
		ExpressionAttributeNames:  map[string]string{"#counter": "Counter"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":val": numAV("5")},
		ReturnValues:     "ALL_NEW",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Attributes["Counter"].N != "15" {
		t.Fatalf("expected Counter=15, got %q", resp.Attributes["Counter"].N)
	}
}

// --- US-306: GSI Maintenance ---

func TestGSIPutItem(t *testing.T) {
	m := newTestBackendWithGSI()
	ctx := context.Background()

	_, err := m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item: map[string]dynago.AttributeValue{
			"PK": strAV("user#1"), "SK": strAV("profile"),
			"Email":     strAV("alice@example.com"),
			"Status":    strAV("active"),
			"CreatedAt": strAV("2024-01-01"),
			"Name":      strAV("Alice"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify GSI data
	td := m.tables["users"]
	td.mu.RLock()
	defer td.mu.RUnlock()

	emailGSI := td.gsis["email-index"]
	hashKey := keyString(strAV("alice@example.com"))
	if _, ok := emailGSI.items[hashKey]; !ok {
		t.Fatal("expected item in email-index GSI")
	}

	statusGSI := td.gsis["status-created-index"]
	statusHash := keyString(strAV("active"))
	createdRange := keyString(strAV("2024-01-01"))
	if rng, ok := statusGSI.items[statusHash]; !ok {
		t.Fatal("expected item in status-created-index GSI")
	} else if _, ok := rng[createdRange]; !ok {
		t.Fatal("expected item at correct range key in GSI")
	}
}

func TestGSIUpdateItem(t *testing.T) {
	m := newTestBackendWithGSI()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item: map[string]dynago.AttributeValue{
			"PK": strAV("user#1"), "SK": strAV("profile"),
			"Email":  strAV("alice@example.com"),
			"Status": strAV("active"),
		},
	})

	_, err := m.UpdateItem(ctx, &dynago.UpdateItemRequest{
		TableName:        "users",
		Key:              map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
		UpdateExpression: "SET #email = :email",
		ExpressionAttributeNames:  map[string]string{"#email": "Email"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":email": strAV("bob@example.com")},
	})
	if err != nil {
		t.Fatal(err)
	}

	td := m.tables["users"]
	td.mu.RLock()
	defer td.mu.RUnlock()

	emailGSI := td.gsis["email-index"]
	// Old entry should be gone
	oldHash := keyString(strAV("alice@example.com"))
	if rng, ok := emailGSI.items[oldHash]; ok && len(rng) > 0 {
		t.Fatal("old GSI entry should be removed")
	}
	// New entry should exist
	newHash := keyString(strAV("bob@example.com"))
	if _, ok := emailGSI.items[newHash]; !ok {
		t.Fatal("expected new GSI entry")
	}
}

func TestGSIDeleteItem(t *testing.T) {
	m := newTestBackendWithGSI()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item: map[string]dynago.AttributeValue{
			"PK": strAV("user#1"), "SK": strAV("profile"),
			"Email":  strAV("alice@example.com"),
			"Status": strAV("active"),
		},
	})

	_, err := m.DeleteItem(ctx, &dynago.DeleteItemRequest{
		TableName: "users",
		Key:       map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
	})
	if err != nil {
		t.Fatal(err)
	}

	td := m.tables["users"]
	td.mu.RLock()
	defer td.mu.RUnlock()

	emailGSI := td.gsis["email-index"]
	hashKey := keyString(strAV("alice@example.com"))
	if rng, ok := emailGSI.items[hashKey]; ok && len(rng) > 0 {
		t.Fatal("expected GSI entry to be removed after delete")
	}
}

func TestGSIMissingKeyNotIndexed(t *testing.T) {
	m := newTestBackendWithGSI()
	ctx := context.Background()

	// Item without Email should not appear in email-index GSI
	_, err := m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item: map[string]dynago.AttributeValue{
			"PK":   strAV("user#1"),
			"SK":   strAV("profile"),
			"Name": strAV("Alice"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	td := m.tables["users"]
	td.mu.RLock()
	defer td.mu.RUnlock()

	emailGSI := td.gsis["email-index"]
	if len(emailGSI.items) != 0 {
		t.Fatal("expected no items in email-index GSI when Email is missing")
	}
}

func TestGSIStoresFullItem(t *testing.T) {
	m := newTestBackendWithGSI()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item: map[string]dynago.AttributeValue{
			"PK":    strAV("user#1"),
			"SK":    strAV("profile"),
			"Email": strAV("alice@example.com"),
			"Name":  strAV("Alice"),
			"Age":   numAV("30"),
		},
	})

	td := m.tables["users"]
	td.mu.RLock()
	defer td.mu.RUnlock()

	emailGSI := td.gsis["email-index"]
	hashKey := keyString(strAV("alice@example.com"))
	rngMap := emailGSI.items[hashKey]
	for _, item := range rngMap {
		if item["Name"].S != "Alice" {
			t.Fatal("GSI should store full item copy")
		}
		if item["Age"].N != "30" {
			t.Fatal("GSI should store all attributes")
		}
	}
}

// --- Hash-only table tests ---

func TestHashOnlyTable(t *testing.T) {
	m := New()
	m.CreateTable("simple", TableSchema{
		HashKey: KeyDef{Name: "ID", Type: StringKey},
	})
	ctx := context.Background()

	_, err := m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "simple",
		Item:      map[string]dynago.AttributeValue{"ID": strAV("1"), "Name": strAV("Test")},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := m.GetItem(ctx, &dynago.GetItemRequest{
		TableName: "simple",
		Key:       map[string]dynago.AttributeValue{"ID": strAV("1")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Item["Name"].S != "Test" {
		t.Fatal("expected Name=Test")
	}
}

// --- Unimplemented stubs ---

func TestUnimplementedStubs(t *testing.T) {
	m := New()
	ctx := context.Background()

	_, err := m.Query(ctx, &dynago.QueryRequest{})
	if !errors.Is(err, dynago.ErrValidation) {
		t.Fatal("expected ErrValidation from Query stub")
	}

	_, err = m.Scan(ctx, &dynago.ScanRequest{})
	if !errors.Is(err, dynago.ErrValidation) {
		t.Fatal("expected ErrValidation from Scan stub")
	}
}

// ---------------------------------------------------------------------------
// US-406: Transaction Support
// ---------------------------------------------------------------------------

func TestTransactWriteItems_PutMultiple(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, err := m.TransactWriteItems(ctx, &dynago.TransactWriteItemsRequest{
		TransactItems: []dynago.TransactWriteItem{
			{Put: &dynago.TransactPut{
				TableName: "users",
				Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice")},
			}},
			{Put: &dynago.TransactPut{
				TableName: "users",
				Item:      map[string]dynago.AttributeValue{"PK": strAV("user#2"), "SK": strAV("profile"), "Name": strAV("Bob")},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify both items were written
	resp, _ := m.GetItem(ctx, &dynago.GetItemRequest{
		TableName: "users",
		Key:       map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
	})
	if resp.Item["Name"].S != "Alice" {
		t.Fatalf("expected Alice, got %q", resp.Item["Name"].S)
	}

	resp, _ = m.GetItem(ctx, &dynago.GetItemRequest{
		TableName: "users",
		Key:       map[string]dynago.AttributeValue{"PK": strAV("user#2"), "SK": strAV("profile")},
	})
	if resp.Item["Name"].S != "Bob" {
		t.Fatalf("expected Bob, got %q", resp.Item["Name"].S)
	}
}

func TestTransactWriteItems_ConditionFails_NoWrites(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	// Pre-populate user#1
	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice")},
	})

	// Transaction: Put user#2 + condition check on user#1 that fails
	_, err := m.TransactWriteItems(ctx, &dynago.TransactWriteItemsRequest{
		TransactItems: []dynago.TransactWriteItem{
			{Put: &dynago.TransactPut{
				TableName: "users",
				Item:      map[string]dynago.AttributeValue{"PK": strAV("user#2"), "SK": strAV("profile"), "Name": strAV("Bob")},
			}},
			{ConditionCheck: &dynago.TransactConditionCheck{
				TableName:                 "users",
				Key:                       map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
				ConditionExpression:       "#name = :val",
				ExpressionAttributeNames:  map[string]string{"#name": "Name"},
				ExpressionAttributeValues: map[string]dynago.AttributeValue{":val": strAV("Wrong")},
			}},
		},
	})
	if !errors.Is(err, dynago.ErrTransactionCancelled) {
		t.Fatalf("expected ErrTransactionCancelled, got %v", err)
	}

	// user#2 should NOT have been written (atomicity)
	resp, _ := m.GetItem(ctx, &dynago.GetItemRequest{
		TableName: "users",
		Key:       map[string]dynago.AttributeValue{"PK": strAV("user#2"), "SK": strAV("profile")},
	})
	if resp.Item != nil {
		t.Fatal("expected user#2 to not be written when transaction fails")
	}
}

func TestTransactWriteItems_ReturnsReasons(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, err := m.TransactWriteItems(ctx, &dynago.TransactWriteItemsRequest{
		TransactItems: []dynago.TransactWriteItem{
			{Put: &dynago.TransactPut{
				TableName: "users",
				Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice")},
			}},
			{Put: &dynago.TransactPut{
				TableName:                 "users",
				Item:                      map[string]dynago.AttributeValue{"PK": strAV("user#2"), "SK": strAV("profile"), "Name": strAV("Bob")},
				ConditionExpression:       "attribute_exists(#pk)",
				ExpressionAttributeNames:  map[string]string{"#pk": "PK"},
			}},
			{ConditionCheck: &dynago.TransactConditionCheck{
				TableName:                 "users",
				Key:                       map[string]dynago.AttributeValue{"PK": strAV("user#3"), "SK": strAV("profile")},
				ConditionExpression:       "attribute_exists(#pk)",
				ExpressionAttributeNames:  map[string]string{"#pk": "PK"},
			}},
		},
	})

	reasons := dynago.TxCancelReasons(err)
	if reasons == nil {
		t.Fatal("expected TxCancelledError with reasons")
	}
	if len(reasons) != 3 {
		t.Fatalf("expected 3 reasons, got %d", len(reasons))
	}
	// First op has no condition, so reason should be empty
	if reasons[0].Code != "" {
		t.Fatalf("expected empty reason for op 0, got %q", reasons[0].Code)
	}
	// Second and third ops should have ConditionalCheckFailed
	if reasons[1].Code != "ConditionalCheckFailed" {
		t.Fatalf("expected ConditionalCheckFailed for op 1, got %q", reasons[1].Code)
	}
	if reasons[2].Code != "ConditionalCheckFailed" {
		t.Fatalf("expected ConditionalCheckFailed for op 2, got %q", reasons[2].Code)
	}
}

func TestTransactWriteItems_Delete(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice")},
	})

	_, err := m.TransactWriteItems(ctx, &dynago.TransactWriteItemsRequest{
		TransactItems: []dynago.TransactWriteItem{
			{Delete: &dynago.TransactDelete{
				TableName: "users",
				Key:       map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, _ := m.GetItem(ctx, &dynago.GetItemRequest{
		TableName: "users",
		Key:       map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
	})
	if resp.Item != nil {
		t.Fatal("expected item to be deleted")
	}
}

func TestTransactWriteItems_Update(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice"), "Counter": numAV("5")},
	})

	_, err := m.TransactWriteItems(ctx, &dynago.TransactWriteItemsRequest{
		TransactItems: []dynago.TransactWriteItem{
			{Update: &dynago.TransactUpdate{
				TableName:                 "users",
				Key:                       map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
				UpdateExpression:          "SET #counter = #counter + :inc",
				ExpressionAttributeNames:  map[string]string{"#counter": "Counter"},
				ExpressionAttributeValues: map[string]dynago.AttributeValue{":inc": numAV("3")},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, _ := m.GetItem(ctx, &dynago.GetItemRequest{
		TableName: "users",
		Key:       map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
	})
	if resp.Item["Counter"].N != "8" {
		t.Fatalf("expected Counter=8, got %q", resp.Item["Counter"].N)
	}
}

func TestTransactWriteItems_UpdateUpsert(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, err := m.TransactWriteItems(ctx, &dynago.TransactWriteItemsRequest{
		TransactItems: []dynago.TransactWriteItem{
			{Update: &dynago.TransactUpdate{
				TableName:                 "users",
				Key:                       map[string]dynago.AttributeValue{"PK": strAV("user#new"), "SK": strAV("profile")},
				UpdateExpression:          "SET #name = :val",
				ExpressionAttributeNames:  map[string]string{"#name": "Name"},
				ExpressionAttributeValues: map[string]dynago.AttributeValue{":val": strAV("Charlie")},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, _ := m.GetItem(ctx, &dynago.GetItemRequest{
		TableName: "users",
		Key:       map[string]dynago.AttributeValue{"PK": strAV("user#new"), "SK": strAV("profile")},
	})
	if resp.Item["Name"].S != "Charlie" {
		t.Fatalf("expected Name=Charlie, got %q", resp.Item["Name"].S)
	}
}

func TestTransactWriteItems_MixedOps(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice")},
	})

	_, err := m.TransactWriteItems(ctx, &dynago.TransactWriteItemsRequest{
		TransactItems: []dynago.TransactWriteItem{
			{Put: &dynago.TransactPut{
				TableName: "users",
				Item:      map[string]dynago.AttributeValue{"PK": strAV("user#2"), "SK": strAV("profile"), "Name": strAV("Bob")},
			}},
			{Delete: &dynago.TransactDelete{
				TableName: "users",
				Key:       map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
			}},
			{ConditionCheck: &dynago.TransactConditionCheck{
				TableName:                 "users",
				Key:                       map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
				ConditionExpression:       "attribute_exists(#pk)",
				ExpressionAttributeNames:  map[string]string{"#pk": "PK"},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// user#1 should be deleted
	resp, _ := m.GetItem(ctx, &dynago.GetItemRequest{
		TableName: "users",
		Key:       map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
	})
	if resp.Item != nil {
		t.Fatal("expected user#1 to be deleted")
	}

	// user#2 should exist
	resp, _ = m.GetItem(ctx, &dynago.GetItemRequest{
		TableName: "users",
		Key:       map[string]dynago.AttributeValue{"PK": strAV("user#2"), "SK": strAV("profile")},
	})
	if resp.Item["Name"].S != "Bob" {
		t.Fatal("expected user#2 to be created")
	}
}

func TestTransactWriteItems_ExceedsMax(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	items := make([]dynago.TransactWriteItem, 101)
	for i := range items {
		items[i] = dynago.TransactWriteItem{
			Put: &dynago.TransactPut{
				TableName: "users",
				Item:      map[string]dynago.AttributeValue{"PK": strAV(fmt.Sprintf("u#%d", i)), "SK": strAV("p")},
			},
		}
	}

	_, err := m.TransactWriteItems(ctx, &dynago.TransactWriteItemsRequest{TransactItems: items})
	if !errors.Is(err, dynago.ErrValidation) {
		t.Fatalf("expected ErrValidation for >100 ops, got %v", err)
	}
}

func TestTransactWriteItems_PutWithConditionPass(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, err := m.TransactWriteItems(ctx, &dynago.TransactWriteItemsRequest{
		TransactItems: []dynago.TransactWriteItem{
			{Put: &dynago.TransactPut{
				TableName:                 "users",
				Item:                      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice")},
				ConditionExpression:       "attribute_not_exists(#pk)",
				ExpressionAttributeNames:  map[string]string{"#pk": "PK"},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTransactWriteItems_DeleteWithConditionFail(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, err := m.TransactWriteItems(ctx, &dynago.TransactWriteItemsRequest{
		TransactItems: []dynago.TransactWriteItem{
			{Delete: &dynago.TransactDelete{
				TableName:                 "users",
				Key:                       map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
				ConditionExpression:       "attribute_exists(#pk)",
				ExpressionAttributeNames:  map[string]string{"#pk": "PK"},
			}},
		},
	})
	if !errors.Is(err, dynago.ErrTransactionCancelled) {
		t.Fatalf("expected ErrTransactionCancelled, got %v", err)
	}
	// Should also match ErrConditionFailed via Is()
	if !errors.Is(err, dynago.ErrConditionFailed) {
		t.Fatal("expected TxCancelledError to match ErrConditionFailed")
	}
}

func TestTransactWriteItems_UpdateWithConditionFail(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice")},
	})

	_, err := m.TransactWriteItems(ctx, &dynago.TransactWriteItemsRequest{
		TransactItems: []dynago.TransactWriteItem{
			{Update: &dynago.TransactUpdate{
				TableName:                 "users",
				Key:                       map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
				UpdateExpression:          "SET #name = :newval",
				ConditionExpression:       "#name = :oldval",
				ExpressionAttributeNames:  map[string]string{"#name": "Name"},
				ExpressionAttributeValues: map[string]dynago.AttributeValue{":newval": strAV("Bob"), ":oldval": strAV("Wrong")},
			}},
		},
	})
	if !errors.Is(err, dynago.ErrTransactionCancelled) {
		t.Fatalf("expected ErrTransactionCancelled, got %v", err)
	}

	// Verify item was NOT modified
	resp, _ := m.GetItem(ctx, &dynago.GetItemRequest{
		TableName: "users",
		Key:       map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
	})
	if resp.Item["Name"].S != "Alice" {
		t.Fatal("expected item to be unchanged after failed transaction")
	}
}

func TestTransactWriteItems_TableNotFound(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, err := m.TransactWriteItems(ctx, &dynago.TransactWriteItemsRequest{
		TransactItems: []dynago.TransactWriteItem{
			{Put: &dynago.TransactPut{
				TableName: "nonexistent",
				Item:      map[string]dynago.AttributeValue{"PK": strAV("u#1"), "SK": strAV("p")},
			}},
		},
	})
	if !errors.Is(err, dynago.ErrValidation) {
		t.Fatalf("expected ErrValidation for unknown table, got %v", err)
	}
}

// --- TransactGetItems ---

func TestTransactGetItems_Basic(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice")},
	})
	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      map[string]dynago.AttributeValue{"PK": strAV("user#2"), "SK": strAV("profile"), "Name": strAV("Bob")},
	})

	resp, err := m.TransactGetItems(ctx, &dynago.TransactGetItemsRequest{
		TransactItems: []dynago.TransactGetItem{
			{TableName: "users", Key: map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")}},
			{TableName: "users", Key: map[string]dynago.AttributeValue{"PK": strAV("user#2"), "SK": strAV("profile")}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.Responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(resp.Responses))
	}
	if resp.Responses[0]["Name"].S != "Alice" {
		t.Fatalf("expected Alice at index 0, got %q", resp.Responses[0]["Name"].S)
	}
	if resp.Responses[1]["Name"].S != "Bob" {
		t.Fatalf("expected Bob at index 1, got %q", resp.Responses[1]["Name"].S)
	}
}

func TestTransactGetItems_MissingItem(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice")},
	})

	resp, err := m.TransactGetItems(ctx, &dynago.TransactGetItemsRequest{
		TransactItems: []dynago.TransactGetItem{
			{TableName: "users", Key: map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")}},
			{TableName: "users", Key: map[string]dynago.AttributeValue{"PK": strAV("user#missing"), "SK": strAV("profile")}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.Responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(resp.Responses))
	}
	if resp.Responses[0] == nil {
		t.Fatal("expected non-nil at index 0")
	}
	if resp.Responses[1] != nil {
		t.Fatal("expected nil at index 1 for missing item")
	}
}

func TestTransactGetItems_WithProjection(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, _ = m.PutItem(ctx, &dynago.PutItemRequest{
		TableName: "users",
		Item:      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile"), "Name": strAV("Alice"), "Age": numAV("30")},
	})

	resp, err := m.TransactGetItems(ctx, &dynago.TransactGetItemsRequest{
		TransactItems: []dynago.TransactGetItem{
			{
				TableName:                "users",
				Key:                      map[string]dynago.AttributeValue{"PK": strAV("user#1"), "SK": strAV("profile")},
				ProjectionExpression:     "#n",
				ExpressionAttributeNames: map[string]string{"#n": "Name"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Responses[0]) != 1 {
		t.Fatalf("expected 1 projected attribute, got %d", len(resp.Responses[0]))
	}
	if resp.Responses[0]["Name"].S != "Alice" {
		t.Fatal("expected projected Name=Alice")
	}
}

func TestTransactGetItems_ExceedsMax(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	items := make([]dynago.TransactGetItem, 101)
	for i := range items {
		items[i] = dynago.TransactGetItem{
			TableName: "users",
			Key:       map[string]dynago.AttributeValue{"PK": strAV(fmt.Sprintf("u#%d", i)), "SK": strAV("p")},
		}
	}

	_, err := m.TransactGetItems(ctx, &dynago.TransactGetItemsRequest{TransactItems: items})
	if !errors.Is(err, dynago.ErrValidation) {
		t.Fatalf("expected ErrValidation for >100 ops, got %v", err)
	}
}

func TestTransactGetItems_TableNotFound(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	_, err := m.TransactGetItems(ctx, &dynago.TransactGetItemsRequest{
		TransactItems: []dynago.TransactGetItem{
			{TableName: "nonexistent", Key: map[string]dynago.AttributeValue{"PK": strAV("u#1"), "SK": strAV("p")}},
		},
	})
	if !errors.Is(err, dynago.ErrValidation) {
		t.Fatalf("expected ErrValidation for unknown table, got %v", err)
	}
}

func TestTransactWriteItems_GSIMaintenance(t *testing.T) {
	m := newTestBackendWithGSI()
	ctx := context.Background()

	_, err := m.TransactWriteItems(ctx, &dynago.TransactWriteItemsRequest{
		TransactItems: []dynago.TransactWriteItem{
			{Put: &dynago.TransactPut{
				TableName: "users",
				Item: map[string]dynago.AttributeValue{
					"PK": strAV("user#1"), "SK": strAV("profile"),
					"Email": strAV("alice@example.com"),
				},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	td := m.tables["users"]
	td.mu.RLock()
	defer td.mu.RUnlock()

	emailGSI := td.gsis["email-index"]
	hashKey := keyString(strAV("alice@example.com"))
	if _, ok := emailGSI.items[hashKey]; !ok {
		t.Fatal("expected GSI to be updated by transactional put")
	}
}
