---
title: "Single-Table Design"
weight: 1
---

# Single-Table Design with DynaGo

This guide shows how to model multiple entity types in a single DynamoDB table
using DynaGo's entity, registry, and collection APIs. If you are new to
single-table design, read the concept section first; otherwise skip ahead to the
code.

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
string.

```go
package main

import "github.com/danielmensah/dynago"

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
	PK       string  `dynamo:"PK"`
	SK       string  `dynamo:"SK"`
	Amount   float64 `dynamo:"Amount"`
	Status   string  `dynamo:"Status"`
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
```

---

## Creating a Registry

A `Registry` maps discriminator strings to concrete Go types:

```go
reg := dynago.NewRegistry("_type")
reg.Register(User{})
reg.Register(Order{})
reg.Register(Product{})
```

---

## Binding a Registry to a Table

```go
db := dynago.New(backend)
table := db.Table("ECommerceTable", dynago.WithRegistry(reg))
```

Once bound, `Put` automatically sets the discriminator attribute, and
`QueryCollection` / `CollectionIter` use the registry to unmarshal items into
the correct Go types.

---

## Writing Items

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
```

Each item is stored with `_type` set to the entity's discriminator value.

---

## Querying Heterogeneous Collections

`QueryCollection` runs a DynamoDB Query and uses the registry to unmarshal each
item into its correct Go type:

```go
coll, err := dynago.QueryCollection(ctx, table,
	dynago.Partition("PK", "USER#u1"),
)

users := dynago.ItemsOf[User](coll)
orders := dynago.ItemsOf[Order](coll)

fmt.Printf("User: %s\n", users[0].Name)
fmt.Printf("Orders: %d\n", len(orders))
```

### Filtering and Limiting

```go
coll, err := dynago.QueryCollection(ctx, table,
	dynago.Partition("PK", "USER#u1").SortBeginsWith("SK", "ORDER#"),
	dynago.QueryLimit(10),
	dynago.ScanForward(false),
)
```

---

## Streaming with CollectionIter

For large result sets, use Go 1.23+ iterators that lazily page through results:

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
	}
}
```

Breaking out of the loop stops pagination early.

---

## GSI Overloading

Store different access patterns in the same Global Secondary Index:

| Entity  | GSI1PK            | GSI1SK           | Access Pattern         |
|---------|--------------------|------------------|------------------------|
| Order   | `STATUS#SHIPPED`   | `2024-01-15`     | Orders by status+date  |
| Product | `CAT#BOOKS`        | `PRODUCT#p1`     | Products by category   |

Query the GSI:

```go
coll, err := dynago.QueryCollection(ctx, table,
	dynago.Partition("GSI1PK", "STATUS#SHIPPED"),
	dynago.QueryIndex("GSI1"),
	dynago.ScanForward(false),
)
orders := dynago.ItemsOf[Order](coll)
```

---

## Parsing Composite Keys with SplitKey

```go
parts := dynago.SplitKey("ORDER#o1", "#")
// parts = ["ORDER", "o1"]
orderID := parts[1]
```

---

## Testing with memdb

```go
func TestSingleTableDesign(t *testing.T) {
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

	reg := dynago.NewRegistry("_type")
	reg.Register(User{})
	reg.Register(Order{})

	db := dynago.New(backend)
	table := db.Table("ECommerce", dynago.WithRegistry(reg))
	ctx := context.Background()

	// Write, query, and assert as shown above...
}
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
