package memdb

import (
	"context"
	"errors"
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

// --- US-304: Query ---

func seedQueryData(t *testing.T, m *MemoryBackend) {
	t.Helper()
	ctx := context.Background()
	items := []map[string]dynago.AttributeValue{
		{"PK": strAV("user#1"), "SK": strAV("order#001"), "Status": strAV("active"), "Amount": numAV("100")},
		{"PK": strAV("user#1"), "SK": strAV("order#002"), "Status": strAV("active"), "Amount": numAV("200")},
		{"PK": strAV("user#1"), "SK": strAV("order#003"), "Status": strAV("cancelled"), "Amount": numAV("50")},
		{"PK": strAV("user#1"), "SK": strAV("profile"), "Status": strAV("active"), "Amount": numAV("0")},
		{"PK": strAV("user#2"), "SK": strAV("order#001"), "Status": strAV("active"), "Amount": numAV("300")},
	}
	for _, item := range items {
		_, err := m.PutItem(ctx, &dynago.PutItemRequest{TableName: "users", Item: item})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestQueryBasic(t *testing.T) {
	m := newTestBackend()
	seedQueryData(t, m)
	ctx := context.Background()

	resp, err := m.Query(ctx, &dynago.QueryRequest{
		TableName:                 "users",
		KeyConditionExpression:    "#pk = :pk0",
		ExpressionAttributeNames:  map[string]string{"#pk": "PK"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("user#1")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 4 {
		t.Fatalf("expected 4 items, got %d", resp.Count)
	}
	// Items should be sorted by SK ascending.
	if resp.Items[0]["SK"].S != "order#001" {
		t.Fatalf("expected first item SK=order#001, got %q", resp.Items[0]["SK"].S)
	}
	if resp.Items[3]["SK"].S != "profile" {
		t.Fatalf("expected last item SK=profile, got %q", resp.Items[3]["SK"].S)
	}
}

func TestQuerySortKeyBeginsWith(t *testing.T) {
	m := newTestBackend()
	seedQueryData(t, m)
	ctx := context.Background()

	resp, err := m.Query(ctx, &dynago.QueryRequest{
		TableName:                 "users",
		KeyConditionExpression:    "#pk = :pk0 AND begins_with(#sk, :sk0)",
		ExpressionAttributeNames:  map[string]string{"#pk": "PK", "#sk": "SK"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("user#1"), ":sk0": strAV("order#")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 {
		t.Fatalf("expected 3 items, got %d", resp.Count)
	}
}

func TestQuerySortKeyEquals(t *testing.T) {
	m := newTestBackend()
	seedQueryData(t, m)
	ctx := context.Background()

	resp, err := m.Query(ctx, &dynago.QueryRequest{
		TableName:                 "users",
		KeyConditionExpression:    "#pk = :pk0 AND #sk = :sk0",
		ExpressionAttributeNames:  map[string]string{"#pk": "PK", "#sk": "SK"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("user#1"), ":sk0": strAV("order#002")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 1 {
		t.Fatalf("expected 1 item, got %d", resp.Count)
	}
	if resp.Items[0]["Amount"].N != "200" {
		t.Fatalf("expected Amount=200, got %q", resp.Items[0]["Amount"].N)
	}
}

func TestQuerySortKeyBetween(t *testing.T) {
	m := newTestBackend()
	seedQueryData(t, m)
	ctx := context.Background()

	resp, err := m.Query(ctx, &dynago.QueryRequest{
		TableName:                 "users",
		KeyConditionExpression:    "#pk = :pk0 AND #sk BETWEEN :lo AND :hi",
		ExpressionAttributeNames:  map[string]string{"#pk": "PK", "#sk": "SK"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("user#1"), ":lo": strAV("order#001"), ":hi": strAV("order#002")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Fatalf("expected 2 items, got %d", resp.Count)
	}
}

func TestQuerySortKeyComparisons(t *testing.T) {
	m := newTestBackend()
	seedQueryData(t, m)
	ctx := context.Background()

	// Greater than
	resp, err := m.Query(ctx, &dynago.QueryRequest{
		TableName:                 "users",
		KeyConditionExpression:    "#pk = :pk0 AND #sk > :sk0",
		ExpressionAttributeNames:  map[string]string{"#pk": "PK", "#sk": "SK"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("user#1"), ":sk0": strAV("order#002")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 { // order#003, profile
		t.Fatalf("expected 2 items for >, got %d", resp.Count)
	}

	// Less than or equal
	resp, err = m.Query(ctx, &dynago.QueryRequest{
		TableName:                 "users",
		KeyConditionExpression:    "#pk = :pk0 AND #sk <= :sk0",
		ExpressionAttributeNames:  map[string]string{"#pk": "PK", "#sk": "SK"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("user#1"), ":sk0": strAV("order#002")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 { // order#001, order#002
		t.Fatalf("expected 2 items for <=, got %d", resp.Count)
	}
}

func TestQueryDescending(t *testing.T) {
	m := newTestBackend()
	seedQueryData(t, m)
	ctx := context.Background()

	scanForward := false
	resp, err := m.Query(ctx, &dynago.QueryRequest{
		TableName:                 "users",
		KeyConditionExpression:    "#pk = :pk0 AND begins_with(#sk, :sk0)",
		ExpressionAttributeNames:  map[string]string{"#pk": "PK", "#sk": "SK"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("user#1"), ":sk0": strAV("order#")},
		ScanIndexForward:          &scanForward,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 {
		t.Fatalf("expected 3 items, got %d", resp.Count)
	}
	// Descending order.
	if resp.Items[0]["SK"].S != "order#003" {
		t.Fatalf("expected first item SK=order#003, got %q", resp.Items[0]["SK"].S)
	}
	if resp.Items[2]["SK"].S != "order#001" {
		t.Fatalf("expected last item SK=order#001, got %q", resp.Items[2]["SK"].S)
	}
}

func TestQueryFilter(t *testing.T) {
	m := newTestBackend()
	seedQueryData(t, m)
	ctx := context.Background()

	resp, err := m.Query(ctx, &dynago.QueryRequest{
		TableName:                 "users",
		KeyConditionExpression:    "#pk = :pk0",
		FilterExpression:          "#status = :status",
		ExpressionAttributeNames:  map[string]string{"#pk": "PK", "#status": "Status"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("user#1"), ":status": strAV("active")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 { // order#001, order#002, profile (all active)
		t.Fatalf("expected 3 items, got %d", resp.Count)
	}
	if resp.ScannedCount != 4 { // all 4 items were scanned
		t.Fatalf("expected ScannedCount=4, got %d", resp.ScannedCount)
	}
}

func TestQueryProjection(t *testing.T) {
	m := newTestBackend()
	seedQueryData(t, m)
	ctx := context.Background()

	resp, err := m.Query(ctx, &dynago.QueryRequest{
		TableName:                 "users",
		KeyConditionExpression:    "#pk = :pk0 AND #sk = :sk0",
		ProjectionExpression:      "#amount",
		ExpressionAttributeNames:  map[string]string{"#pk": "PK", "#sk": "SK", "#amount": "Amount"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("user#1"), ":sk0": strAV("order#001")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 1 {
		t.Fatalf("expected 1 item, got %d", resp.Count)
	}
	if len(resp.Items[0]) != 1 {
		t.Fatalf("expected 1 attribute in projection, got %d", len(resp.Items[0]))
	}
	if resp.Items[0]["Amount"].N != "100" {
		t.Fatalf("expected Amount=100, got %q", resp.Items[0]["Amount"].N)
	}
}

func TestQueryLimit(t *testing.T) {
	m := newTestBackend()
	seedQueryData(t, m)
	ctx := context.Background()

	resp, err := m.Query(ctx, &dynago.QueryRequest{
		TableName:                 "users",
		KeyConditionExpression:    "#pk = :pk0",
		ExpressionAttributeNames:  map[string]string{"#pk": "PK"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("user#1")},
		Limit:                     2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Fatalf("expected 2 items, got %d", resp.Count)
	}
	if resp.ScannedCount != 2 {
		t.Fatalf("expected ScannedCount=2, got %d", resp.ScannedCount)
	}
	if len(resp.LastEvaluatedKey) == 0 {
		t.Fatal("expected LastEvaluatedKey for pagination")
	}
}

func TestQueryPagination(t *testing.T) {
	m := newTestBackend()
	seedQueryData(t, m)
	ctx := context.Background()

	var allItems []map[string]dynago.AttributeValue
	var startKey map[string]dynago.AttributeValue

	for {
		resp, err := m.Query(ctx, &dynago.QueryRequest{
			TableName:                 "users",
			KeyConditionExpression:    "#pk = :pk0",
			ExpressionAttributeNames:  map[string]string{"#pk": "PK"},
			ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("user#1")},
			Limit:                     2,
			ExclusiveStartKey:         startKey,
		})
		if err != nil {
			t.Fatal(err)
		}
		allItems = append(allItems, resp.Items...)
		if len(resp.LastEvaluatedKey) == 0 {
			break
		}
		startKey = resp.LastEvaluatedKey
	}

	if len(allItems) != 4 {
		t.Fatalf("expected 4 total items across pages, got %d", len(allItems))
	}
	// Verify ordering preserved across pages.
	expectedSKs := []string{"order#001", "order#002", "order#003", "profile"}
	for i, item := range allItems {
		if item["SK"].S != expectedSKs[i] {
			t.Fatalf("item %d: expected SK=%s, got %s", i, expectedSKs[i], item["SK"].S)
		}
	}
}

func TestQueryEmptyPartition(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	resp, err := m.Query(ctx, &dynago.QueryRequest{
		TableName:                 "users",
		KeyConditionExpression:    "#pk = :pk0",
		ExpressionAttributeNames:  map[string]string{"#pk": "PK"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("nonexistent")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 0 {
		t.Fatalf("expected 0 items, got %d", resp.Count)
	}
}

func TestQueryGSI(t *testing.T) {
	m := newTestBackendWithGSI()
	ctx := context.Background()

	// Seed data with GSI attributes.
	items := []map[string]dynago.AttributeValue{
		{"PK": strAV("user#1"), "SK": strAV("profile"), "Email": strAV("alice@example.com"), "Status": strAV("active"), "CreatedAt": strAV("2024-01-01"), "Name": strAV("Alice")},
		{"PK": strAV("user#2"), "SK": strAV("profile"), "Email": strAV("bob@example.com"), "Status": strAV("active"), "CreatedAt": strAV("2024-01-02"), "Name": strAV("Bob")},
		{"PK": strAV("user#3"), "SK": strAV("profile"), "Email": strAV("charlie@example.com"), "Status": strAV("inactive"), "CreatedAt": strAV("2024-01-03"), "Name": strAV("Charlie")},
	}
	for _, item := range items {
		_, err := m.PutItem(ctx, &dynago.PutItemRequest{TableName: "users", Item: item})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Query GSI with hash key only.
	resp, err := m.Query(ctx, &dynago.QueryRequest{
		TableName:                 "users",
		IndexName:                 "status-created-index",
		KeyConditionExpression:    "#pk = :pk0",
		ExpressionAttributeNames:  map[string]string{"#pk": "Status"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("active")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Fatalf("expected 2 active items from GSI, got %d", resp.Count)
	}

	// Query GSI with sort key condition.
	resp, err = m.Query(ctx, &dynago.QueryRequest{
		TableName:                 "users",
		IndexName:                 "status-created-index",
		KeyConditionExpression:    "#pk = :pk0 AND #sk >= :sk0",
		ExpressionAttributeNames:  map[string]string{"#pk": "Status", "#sk": "CreatedAt"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("active"), ":sk0": strAV("2024-01-02")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 1 {
		t.Fatalf("expected 1 item from GSI with sort key, got %d", resp.Count)
	}
}

func TestQueryTableNotFound(t *testing.T) {
	m := New()
	ctx := context.Background()

	_, err := m.Query(ctx, &dynago.QueryRequest{
		TableName:                 "nonexistent",
		KeyConditionExpression:    "#pk = :pk0",
		ExpressionAttributeNames:  map[string]string{"#pk": "PK"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("x")},
	})
	if !errors.Is(err, dynago.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

// --- US-305: Scan ---

func TestScanBasic(t *testing.T) {
	m := newTestBackend()
	seedQueryData(t, m)
	ctx := context.Background()

	resp, err := m.Scan(ctx, &dynago.ScanRequest{TableName: "users"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 5 { // all items from seedQueryData
		t.Fatalf("expected 5 items, got %d", resp.Count)
	}
	if resp.ScannedCount != 5 {
		t.Fatalf("expected ScannedCount=5, got %d", resp.ScannedCount)
	}
}

func TestScanFilter(t *testing.T) {
	m := newTestBackend()
	seedQueryData(t, m)
	ctx := context.Background()

	resp, err := m.Scan(ctx, &dynago.ScanRequest{
		TableName:                 "users",
		FilterExpression:          "#status = :status",
		ExpressionAttributeNames:  map[string]string{"#status": "Status"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":status": strAV("cancelled")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 1 {
		t.Fatalf("expected 1 cancelled item, got %d", resp.Count)
	}
	if resp.ScannedCount != 5 {
		t.Fatalf("expected ScannedCount=5, got %d", resp.ScannedCount)
	}
}

func TestScanProjection(t *testing.T) {
	m := newTestBackend()
	seedQueryData(t, m)
	ctx := context.Background()

	resp, err := m.Scan(ctx, &dynago.ScanRequest{
		TableName:                "users",
		ProjectionExpression:     "#pk",
		ExpressionAttributeNames: map[string]string{"#pk": "PK"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range resp.Items {
		if len(item) != 1 {
			t.Fatalf("expected 1 attribute in projection, got %d", len(item))
		}
		if _, ok := item["PK"]; !ok {
			t.Fatal("expected PK in projected item")
		}
	}
}

func TestScanLimit(t *testing.T) {
	m := newTestBackend()
	seedQueryData(t, m)
	ctx := context.Background()

	resp, err := m.Scan(ctx, &dynago.ScanRequest{
		TableName: "users",
		Limit:     3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 {
		t.Fatalf("expected 3 items, got %d", resp.Count)
	}
	if len(resp.LastEvaluatedKey) == 0 {
		t.Fatal("expected LastEvaluatedKey for pagination")
	}
}

func TestScanPagination(t *testing.T) {
	m := newTestBackend()
	seedQueryData(t, m)
	ctx := context.Background()

	var allItems []map[string]dynago.AttributeValue
	var startKey map[string]dynago.AttributeValue

	for {
		resp, err := m.Scan(ctx, &dynago.ScanRequest{
			TableName:         "users",
			Limit:             2,
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			t.Fatal(err)
		}
		allItems = append(allItems, resp.Items...)
		if len(resp.LastEvaluatedKey) == 0 {
			break
		}
		startKey = resp.LastEvaluatedKey
	}

	if len(allItems) != 5 {
		t.Fatalf("expected 5 total items across pages, got %d", len(allItems))
	}
}

func TestScanGSI(t *testing.T) {
	m := newTestBackendWithGSI()
	ctx := context.Background()

	items := []map[string]dynago.AttributeValue{
		{"PK": strAV("user#1"), "SK": strAV("profile"), "Email": strAV("alice@example.com"), "Status": strAV("active"), "CreatedAt": strAV("2024-01-01")},
		{"PK": strAV("user#2"), "SK": strAV("profile"), "Email": strAV("bob@example.com"), "Status": strAV("active"), "CreatedAt": strAV("2024-01-02")},
	}
	for _, item := range items {
		_, err := m.PutItem(ctx, &dynago.PutItemRequest{TableName: "users", Item: item})
		if err != nil {
			t.Fatal(err)
		}
	}

	resp, err := m.Scan(ctx, &dynago.ScanRequest{
		TableName: "users",
		IndexName: "email-index",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Fatalf("expected 2 items in GSI scan, got %d", resp.Count)
	}
}

func TestScanEmpty(t *testing.T) {
	m := newTestBackend()
	ctx := context.Background()

	resp, err := m.Scan(ctx, &dynago.ScanRequest{TableName: "users"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 0 {
		t.Fatalf("expected 0 items, got %d", resp.Count)
	}
}

func TestScanTableNotFound(t *testing.T) {
	m := New()
	ctx := context.Background()

	_, err := m.Scan(ctx, &dynago.ScanRequest{TableName: "nonexistent"})
	if !errors.Is(err, dynago.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

// --- Unimplemented stubs ---

func TestUnimplementedStubs(t *testing.T) {
	m := New()
	ctx := context.Background()

	_, err := m.BatchGetItem(ctx, &dynago.BatchGetItemRequest{})
	if !errors.Is(err, dynago.ErrValidation) {
		t.Fatal("expected ErrValidation from BatchGetItem stub")
	}

	_, err = m.BatchWriteItem(ctx, &dynago.BatchWriteItemRequest{})
	if !errors.Is(err, dynago.ErrValidation) {
		t.Fatal("expected ErrValidation from BatchWriteItem stub")
	}

	_, err = m.TransactGetItems(ctx, &dynago.TransactGetItemsRequest{})
	if !errors.Is(err, dynago.ErrValidation) {
		t.Fatal("expected ErrValidation from TransactGetItems stub")
	}

	_, err = m.TransactWriteItems(ctx, &dynago.TransactWriteItemsRequest{})
	if !errors.Is(err, dynago.ErrValidation) {
		t.Fatal("expected ErrValidation from TransactWriteItems stub")
	}
}
