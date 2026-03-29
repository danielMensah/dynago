# Single-Table Design with DynaGo

This guide shows how to model multiple entity types in a single DynamoDB table
using DynaGo's entity, registry, and collection APIs. If you are new to
single-table design, read the concept section first; otherwise skip ahead to the
code.

## Table of Contents

- [Why Single-Table Design?](#why-single-table-design)
- [Core Concepts](#core-concepts)
- [Defining Entities](#defining-entities)
- [Composite Key Construction](#composite-key-construction)
- [Creating a Registry](#creating-a-registry)
- [Binding a Registry to a Table](#binding-a-registry-to-a-table)
- [Writing Items](#writing-items)
- [Querying Heterogeneous Collections](#querying-heterogeneous-collections)
- [Streaming with CollectionIter](#streaming-with-collectioniter)
- [GSI Overloading](#gsi-overloading)
- [Parsing Composite Keys with SplitKey](#parsing-composite-keys-with-splitkey)
- [Testing with memdb](#testing-with-memdb)
- [Complete Example](#complete-example)

---

## Why Single-Table Design?

DynamoDB charges per table and performs best when related data lives together.
Single-table design stores multiple entity types (users, orders, products, etc.)
in one table. Items are distinguished by a **discriminator attribute** (e.g.
`_type`) and addressed by **composite keys** that encode the entity type and
relationship into the partition key (PK) and sort key (SK).

Benefits:

- Fetch related entities in a single Query (e.g. a user and all their orders).
- Reduce the number of tables to manage.
- Model 1:N and M:N relationships with sort key patterns.

## Core Concepts

| Concept               | DynaGo API                    | Purpose                                              |
|-----------------------|-------------------------------|------------------------------------------------------|
| Entity interface      | `DynagoEntity() EntityInfo`   | Declares a struct as a table entity with a discriminator value |
| Registry              | `NewRegistry`, `Register`     | Maps discriminator strings to Go types               |
| Table binding         | `WithRegistry(r)`             | Attaches a registry to a table for polymorphic operations |
| Collection query      | `QueryCollection`             | Queries and unmarshals mixed entity types             |
| Typed extraction      | `ItemsOf[T]`                  | Filters a collection down to a single Go type         |
| Streaming iterator    | `CollectionIter`              | Lazily pages through mixed results with a range loop  |
| Composite key parsing | `SplitKey`                    | Splits `"USER#123"` into `["USER", "123"]`            |

---

## Defining Entities

Every entity type implements the `Entity` interface by providing a
`DynagoEntity()` method that returns an `EntityInfo` with a unique discriminator
string. The discriminator is the value stored in the table's discriminator
attribute (e.g. `_type = "USER"`).

Struct fields are tagged with `dynamo:"..."` (the same tag used by guregu/dynamo,
so existing structs often work unchanged).

```go
package main

import "github.com/danielmensah/dynago"

// User represents a customer profile.
type User struct {
	PK    string `dynamo:"PK"`
	SK    string `dynamo:"SK"`
	Name  string `dynamo:"Name"`
	Email string `dynamo:"Email"`
}

func (u User) DynagoEntity() dynago.EntityInfo {
	return dynago.EntityInfo{Discriminator: "USER"}
}

// Order represents a purchase by a user.
type Order struct {
	PK       string  `dynamo:"PK"`
	SK       string  `dynamo:"SK"`
	Amount   float64 `dynamo:"Amount"`
	Status   string  `dynamo:"Status"`
}

func (o Order) DynagoEntity() dynago.EntityInfo {
	return dynago.EntityInfo{Discriminator: "ORDER"}
}

// Product represents an item in the catalog.
type Product struct {
	PK       string  `dynamo:"PK"`
	SK       string  `dynamo:"SK"`
	Title    string  `dynamo:"Title"`
	Price    float64 `dynamo:"Price"`
	Category string  `dynamo:"Category"`
}

func (p Product) DynagoEntity() dynago.EntityInfo {
	return dynago.EntityInfo{Discriminator: "PRODUCT"}
}
```

The discriminator values (`"USER"`, `"ORDER"`, `"PRODUCT"`) must be unique across
all entity types registered in the same registry.

---

## Composite Key Construction

In single-table design, PK and SK values encode entity type and identity:

| Access Pattern              | PK            | SK                    |
|-----------------------------|---------------|-----------------------|
| Get user profile            | `USER#<uid>`  | `PROFILE`             |
| Get user's orders           | `USER#<uid>`  | `ORDER#<oid>`         |
| Get product                 | `PRODUCT#<id>`| `DETAIL`              |
| Get orders by product (GSI) | `PRODUCT#<id>`| `ORDER#<oid>`         |

Build keys with plain Go string formatting and the `Key` helper:

```go
func userKey(uid string) dynago.KeyValue {
	return dynago.Key("PK", "USER#"+uid, "SK", "PROFILE")
}

func orderKey(uid, oid string) dynago.KeyValue {
	return dynago.Key("PK", "USER#"+uid, "SK", "ORDER#"+oid)
}

func productKey(pid string) dynago.KeyValue {
	return dynago.Key("PK", "PRODUCT#"+pid, "SK", "DETAIL")
}
```

`Key` accepts 2 arguments (partition key only) or 4 arguments (partition + sort
key). Values can be `string`, `int`, `int64`, `float64`, `uint`, `uint64`, or
`[]byte`.

---

## Creating a Registry

A `Registry` maps discriminator strings to concrete Go types. Create one with
`NewRegistry`, passing the name of the DynamoDB attribute that holds the
discriminator (commonly `"_type"` or `"EntityType"`).

```go
reg := dynago.NewRegistry("_type")
reg.Register(User{})
reg.Register(Order{})
reg.Register(Product{})
```

`Register` reads the discriminator from the struct's `DynagoEntity()` method and
stores the mapping. It panics if two types share the same discriminator.

---

## Binding a Registry to a Table

Pass the registry when creating a table reference with `WithRegistry`:

```go
db := dynago.New(backend)
table := db.Table("ECommerceTable", dynago.WithRegistry(reg))
```

Once bound, `Put` automatically sets the discriminator attribute on items that
implement `Entity`, and `QueryCollection` / `CollectionIter` use the registry to
unmarshal items back into the correct Go types.

---

## Writing Items

Use the standard `Table.Put` method. When the table has a registry and the item
implements `Entity`, the discriminator attribute is set automatically -- you do
not need to include it in your struct or set it manually.

```go
ctx := context.Background()

err := table.Put(ctx, User{
	PK:    "USER#u1",
	SK:    "PROFILE",
	Name:  "Alice",
	Email: "alice@example.com",
})

err = table.Put(ctx, Order{
	PK:     "USER#u1",
	SK:     "ORDER#o1",
	Amount: 99.99,
	Status: "SHIPPED",
})

err = table.Put(ctx, Product{
	PK:       "PRODUCT#p1",
	SK:       "DETAIL",
	Title:    "Go Programming Book",
	Price:    39.99,
	Category: "BOOKS",
})
```

Each item is stored with `_type` set to the entity's discriminator value.

---

## Querying Heterogeneous Collections

`QueryCollection` runs a DynamoDB Query and uses the table's registry to
unmarshal each item into its correct Go type. The result is a `Collection`
containing a mixed bag of entities.

```go
coll, err := dynago.QueryCollection(ctx, table,
	dynago.Partition("PK", "USER#u1"),
)
if err != nil {
	log.Fatal(err)
}
```

Extract typed slices with `ItemsOf[T]`:

```go
users := dynago.ItemsOf[User](coll)
orders := dynago.ItemsOf[Order](coll)

fmt.Printf("User: %s\n", users[0].Name)            // Alice
fmt.Printf("Orders: %d\n", len(orders))             // 1
fmt.Printf("First order amount: %.2f\n", orders[0].Amount) // 99.99
```

Items with unrecognized discriminator values are silently skipped, so adding a
new entity type does not break existing queries.

### Filtering and Limiting

`QueryCollection` accepts the same options as `Query[T]`:

```go
coll, err := dynago.QueryCollection(ctx, table,
	dynago.Partition("PK", "USER#u1").SortBeginsWith("SK", "ORDER#"),
	dynago.QueryLimit(10),
	dynago.ScanForward(false), // newest first
)
```

---

## Streaming with CollectionIter

For large result sets, `CollectionIter` returns a Go 1.23+ iterator that lazily
pages through results. Use a type switch to handle each entity:

```go
for item, err := range dynago.CollectionIter(ctx, table,
	dynago.Partition("PK", "USER#u1"),
) {
	if err != nil {
		log.Fatal(err)
	}
	switch v := item.(type) {
	case User:
		fmt.Printf("User: %s\n", v.Name)
	case Order:
		fmt.Printf("Order: %s, Amount: %.2f\n", v.SK, v.Amount)
	default:
		fmt.Printf("Unknown type: %T\n", v)
	}
}
```

Breaking out of the loop stops pagination early -- no wasted reads.

---

## GSI Overloading

GSI overloading stores different access patterns in the same Global Secondary
Index by giving each entity type its own meaning for the GSI keys. For example,
use `GSI1PK` / `GSI1SK` attributes:

| Entity  | GSI1PK            | GSI1SK           | Access Pattern         |
|---------|--------------------|------------------|------------------------|
| Order   | `STATUS#SHIPPED`   | `2024-01-15`     | Orders by status+date  |
| Product | `CAT#BOOKS`        | `PRODUCT#p1`     | Products by category   |

Add GSI key attributes to the relevant structs:

```go
type Order struct {
	PK     string  `dynamo:"PK"`
	SK     string  `dynamo:"SK"`
	Amount float64 `dynamo:"Amount"`
	Status string  `dynamo:"Status"`
	GSI1PK string  `dynamo:"GSI1PK"`
	GSI1SK string  `dynamo:"GSI1SK"`
}

type Product struct {
	PK       string  `dynamo:"PK"`
	SK       string  `dynamo:"SK"`
	Title    string  `dynamo:"Title"`
	Price    float64 `dynamo:"Price"`
	Category string  `dynamo:"Category"`
	GSI1PK   string  `dynamo:"GSI1PK"`
	GSI1SK   string  `dynamo:"GSI1SK"`
}
```

Query the GSI with `QueryIndex`:

```go
// All shipped orders, newest first.
coll, err := dynago.QueryCollection(ctx, table,
	dynago.Partition("GSI1PK", "STATUS#SHIPPED"),
	dynago.QueryIndex("GSI1"),
	dynago.ScanForward(false),
)
orders := dynago.ItemsOf[Order](coll)

// All products in the BOOKS category.
coll, err = dynago.QueryCollection(ctx, table,
	dynago.Partition("GSI1PK", "CAT#BOOKS"),
	dynago.QueryIndex("GSI1"),
)
products := dynago.ItemsOf[Product](coll)
```

Because the registry handles unmarshaling, mixed entity types in a single GSI
partition work seamlessly.

---

## Parsing Composite Keys with SplitKey

`SplitKey` splits a composite key string by a delimiter. Use it to extract IDs
from sort keys or partition keys:

```go
parts := dynago.SplitKey("ORDER#o1", "#")
// parts = ["ORDER", "o1"]

orderID := parts[1] // "o1"
```

The delimiter is a parameter so you can use whatever separator your schema uses
(`"#"`, `"|"`, `"::"`, etc.).

---

## Testing with memdb

The `memdb` package provides a full in-memory DynamoDB backend. It supports
table creation with key schemas, GSI definitions, queries, scans, and all CRUD
operations -- perfect for testing single-table designs without network calls.

```go
package ecommerce_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/danielmensah/dynago"
	"github.com/danielmensah/dynago/memdb"
)

// Entity types (User, Order, Product) defined as shown above.

type User struct {
	PK    string `dynamo:"PK"`
	SK    string `dynamo:"SK"`
	Name  string `dynamo:"Name"`
	Email string `dynamo:"Email"`
}

func (u User) DynagoEntity() dynago.EntityInfo {
	return dynago.EntityInfo{Discriminator: "USER"}
}

type Order struct {
	PK     string  `dynamo:"PK"`
	SK     string  `dynamo:"SK"`
	Amount float64 `dynamo:"Amount"`
	Status string  `dynamo:"Status"`
	GSI1PK string  `dynamo:"GSI1PK"`
	GSI1SK string  `dynamo:"GSI1SK"`
}

func (o Order) DynagoEntity() dynago.EntityInfo {
	return dynago.EntityInfo{Discriminator: "ORDER"}
}

type Product struct {
	PK       string  `dynamo:"PK"`
	SK       string  `dynamo:"SK"`
	Title    string  `dynamo:"Title"`
	Price    float64 `dynamo:"Price"`
	Category string  `dynamo:"Category"`
	GSI1PK   string  `dynamo:"GSI1PK"`
	GSI1SK   string  `dynamo:"GSI1SK"`
}

func (p Product) DynagoEntity() dynago.EntityInfo {
	return dynago.EntityInfo{Discriminator: "PRODUCT"}
}

func TestSingleTableDesign(t *testing.T) {
	// 1. Create the in-memory backend and table schema.
	backend := memdb.New()
	rangeKey := memdb.KeyDef{Name: "SK", Type: memdb.StringKey}
	backend.CreateTable("ECommerce", memdb.TableSchema{
		HashKey:  memdb.KeyDef{Name: "PK", Type: memdb.StringKey},
		RangeKey: &rangeKey,
		GSIs: []memdb.GSISchema{
			{
				Name:    "GSI1",
				HashKey: memdb.KeyDef{Name: "GSI1PK", Type: memdb.StringKey},
				RangeKey: &memdb.KeyDef{Name: "GSI1SK", Type: memdb.StringKey},
			},
		},
	})

	// 2. Create a registry and bind it to the table.
	reg := dynago.NewRegistry("_type")
	reg.Register(User{})
	reg.Register(Order{})
	reg.Register(Product{})

	db := dynago.New(backend)
	table := db.Table("ECommerce", dynago.WithRegistry(reg))
	ctx := context.Background()

	// 3. Write entities.
	if err := table.Put(ctx, User{
		PK: "USER#u1", SK: "PROFILE",
		Name: "Alice", Email: "alice@example.com",
	}); err != nil {
		t.Fatal(err)
	}

	if err := table.Put(ctx, Order{
		PK: "USER#u1", SK: "ORDER#o1",
		Amount: 99.99, Status: "SHIPPED",
		GSI1PK: "STATUS#SHIPPED", GSI1SK: "2024-01-15",
	}); err != nil {
		t.Fatal(err)
	}

	if err := table.Put(ctx, Order{
		PK: "USER#u1", SK: "ORDER#o2",
		Amount: 24.50, Status: "PENDING",
		GSI1PK: "STATUS#PENDING", GSI1SK: "2024-01-16",
	}); err != nil {
		t.Fatal(err)
	}

	// 4. Query all items for a user (profile + orders).
	coll, err := dynago.QueryCollection(ctx, table,
		dynago.Partition("PK", "USER#u1"),
	)
	if err != nil {
		t.Fatal(err)
	}

	users := dynago.ItemsOf[User](coll)
	if len(users) != 1 || users[0].Name != "Alice" {
		t.Fatalf("expected 1 user named Alice, got %+v", users)
	}

	orders := dynago.ItemsOf[Order](coll)
	if len(orders) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(orders))
	}

	// 5. Query only orders using a sort key prefix.
	coll, err = dynago.QueryCollection(ctx, table,
		dynago.Partition("PK", "USER#u1").SortBeginsWith("SK", "ORDER#"),
	)
	if err != nil {
		t.Fatal(err)
	}
	orders = dynago.ItemsOf[Order](coll)
	if len(orders) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(orders))
	}

	// 6. Query a GSI for shipped orders.
	coll, err = dynago.QueryCollection(ctx, table,
		dynago.Partition("GSI1PK", "STATUS#SHIPPED"),
		dynago.QueryIndex("GSI1"),
	)
	if err != nil {
		t.Fatal(err)
	}
	shipped := dynago.ItemsOf[Order](coll)
	if len(shipped) != 1 || shipped[0].Amount != 99.99 {
		t.Fatalf("expected 1 shipped order, got %+v", shipped)
	}

	// 7. Stream with CollectionIter and a type switch.
	var userCount, orderCount int
	for item, err := range dynago.CollectionIter(ctx, table,
		dynago.Partition("PK", "USER#u1"),
	) {
		if err != nil {
			t.Fatal(err)
		}
		switch item.(type) {
		case User:
			userCount++
		case Order:
			orderCount++
		}
	}
	if userCount != 1 || orderCount != 2 {
		t.Fatalf("expected 1 user and 2 orders, got %d users %d orders",
			userCount, orderCount)
	}

	// 8. Parse a composite key.
	parts := dynago.SplitKey("ORDER#o1", "#")
	if parts[0] != "ORDER" || parts[1] != "o1" {
		t.Fatalf("unexpected split: %v", parts)
	}

	fmt.Println("All single-table design tests passed.")
}
```

---

## Complete Example

Putting it all together, here is the typical flow for a single-table design with
DynaGo:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/danielmensah/dynago"
	"github.com/danielmensah/dynago/memdb"
)

// -- Entity definitions --

type User struct {
	PK    string `dynamo:"PK"`
	SK    string `dynamo:"SK"`
	Name  string `dynamo:"Name"`
	Email string `dynamo:"Email"`
}

func (u User) DynagoEntity() dynago.EntityInfo {
	return dynago.EntityInfo{Discriminator: "USER"}
}

type Order struct {
	PK     string  `dynamo:"PK"`
	SK     string  `dynamo:"SK"`
	Amount float64 `dynamo:"Amount"`
	Status string  `dynamo:"Status"`
}

func (o Order) DynagoEntity() dynago.EntityInfo {
	return dynago.EntityInfo{Discriminator: "ORDER"}
}

type Product struct {
	PK       string  `dynamo:"PK"`
	SK       string  `dynamo:"SK"`
	Title    string  `dynamo:"Title"`
	Price    float64 `dynamo:"Price"`
	Category string  `dynamo:"Category"`
}

func (p Product) DynagoEntity() dynago.EntityInfo {
	return dynago.EntityInfo{Discriminator: "PRODUCT"}
}

// -- Key helpers --

func userKey(uid string) dynago.KeyValue {
	return dynago.Key("PK", "USER#"+uid, "SK", "PROFILE")
}

func orderKey(uid, oid string) dynago.KeyValue {
	return dynago.Key("PK", "USER#"+uid, "SK", "ORDER#"+oid)
}

// -- Main --

func main() {
	// Set up an in-memory backend for demonstration. Replace with
	// awsbackend.NewFromConfig(cfg) for production use.
	backend := memdb.New()
	rangeKey := memdb.KeyDef{Name: "SK", Type: memdb.StringKey}
	backend.CreateTable("Shop", memdb.TableSchema{
		HashKey:  memdb.KeyDef{Name: "PK", Type: memdb.StringKey},
		RangeKey: &rangeKey,
	})

	// Create registry and table.
	reg := dynago.NewRegistry("_type")
	reg.Register(User{})
	reg.Register(Order{})
	reg.Register(Product{})

	db := dynago.New(backend)
	table := db.Table("Shop", dynago.WithRegistry(reg))
	ctx := context.Background()

	// Write data.
	_ = table.Put(ctx, User{PK: "USER#u1", SK: "PROFILE", Name: "Alice", Email: "alice@example.com"})
	_ = table.Put(ctx, Order{PK: "USER#u1", SK: "ORDER#o1", Amount: 49.99, Status: "SHIPPED"})
	_ = table.Put(ctx, Order{PK: "USER#u1", SK: "ORDER#o2", Amount: 12.00, Status: "PENDING"})

	// Fetch everything for user u1.
	coll, err := dynago.QueryCollection(ctx, table, dynago.Partition("PK", "USER#u1"))
	if err != nil {
		log.Fatal(err)
	}

	for _, u := range dynago.ItemsOf[User](coll) {
		fmt.Printf("User: %s (%s)\n", u.Name, u.Email)
	}
	for _, o := range dynago.ItemsOf[Order](coll) {
		id := dynago.SplitKey(o.SK, "#")[1]
		fmt.Printf("  Order %s: $%.2f [%s]\n", id, o.Amount, o.Status)
	}

	// Fetch a single item by key.
	user, err := dynago.Get[User](ctx, table, userKey("u1"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Fetched user: %s\n", user.Name)
}
```

Output:

```
User: Alice (alice@example.com)
  Order o1: $49.99 [SHIPPED]
  Order o2: $12.00 [PENDING]
Fetched user: Alice
```

---

## Summary

| Step                  | API                                            |
|-----------------------|------------------------------------------------|
| Define entity         | Implement `DynagoEntity() EntityInfo`          |
| Register types        | `NewRegistry("_type")` + `Register(T{})`       |
| Bind to table         | `db.Table("name", WithRegistry(reg))`          |
| Write items           | `table.Put(ctx, item)` -- discriminator is auto-set |
| Query mixed types     | `QueryCollection(ctx, table, key, opts...)`    |
| Extract typed slice   | `ItemsOf[User](coll)`                          |
| Stream results        | `CollectionIter(ctx, table, key, opts...)`     |
| Query a GSI           | Add `QueryIndex("GSI1")` option                |
| Parse composite keys  | `SplitKey("ORDER#o1", "#")`                    |
| Test without AWS      | `memdb.New()` + `CreateTable(...)`             |
