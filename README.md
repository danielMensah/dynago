# DynaGo

[![CI](https://github.com/danielMensah/dynago/actions/workflows/ci.yml/badge.svg)](https://github.com/danielMensah/dynago/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/danielmensah/dynago.svg)](https://pkg.go.dev/github.com/danielmensah/dynago)
[![Go Report Card](https://goreportcard.com/badge/github.com/danielmensah/dynago)](https://goreportcard.com/report/github.com/danielmensah/dynago)

DynaGo is a type-safe DynamoDB library for Go that leverages generics to provide a clean, expressive API for single-table and multi-table designs. It ships with an in-memory backend for fast, deterministic tests and optional OpenTelemetry middleware for production observability.

## Requirements

- Go 1.23+

## Installation

```bash
go get github.com/danielmensah/dynago
```

For OpenTelemetry instrumentation (separate module):

```bash
go get github.com/danielmensah/dynago/dynagotel
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"

	"github.com/danielmensah/dynago"
	"github.com/danielmensah/dynago/memdb"
)

type User struct {
	PK    string `dynamodbav:"PK"`
	SK    string `dynamodbav:"SK"`
	Name  string `dynamodbav:"Name"`
	Email string `dynamodbav:"Email"`
}

func main() {
	ctx := context.Background()

	// Create an in-memory backend and table.
	backend := memdb.New()
	backend.CreateTable("MyTable", memdb.TableSchema{
		HashKey:  memdb.KeyDef{Name: "PK", Type: memdb.StringKey},
		RangeKey: &memdb.KeyDef{Name: "SK", Type: memdb.StringKey},
	})

	db := dynago.New(backend)
	table := db.Table("MyTable")

	// Put an item.
	user := User{PK: "USER#123", SK: "PROFILE", Name: "Alice", Email: "alice@example.com"}
	_ = table.Put(ctx, user)

	// Get an item.
	got, _ := dynago.Get[User](ctx, table, dynago.Key("PK", "USER#123", "SK", "PROFILE"))
	fmt.Println(got.Name) // Alice

	// Query items by partition key.
	users, _ := dynago.Query[User](ctx, table, dynago.Partition("PK", "USER#123"))
	fmt.Println(len(users))
}
```

## Features

- **Type-safe generics API** -- `Get[T]`, `Put`, `Query[T]`, and `Scan[T]` work with your Go structs directly. No manual marshaling required.
- **Single-table design** -- A `Registry` maps discriminator values to concrete types, enabling polymorphic queries with `QueryCollection` and `ItemsOf[T]`.
- **In-memory testing** -- The `memdb` package provides a complete in-memory DynamoDB backend with support for GSIs, condition expressions, filter expressions, and pagination.
- **OpenTelemetry observability** -- The `dynagotel` middleware adds tracing spans and latency/count metrics to every operation.
- **Middleware architecture** -- Wrap the backend with custom middleware for logging, retries, caching, or any cross-cutting concern.

## Examples

### Basic CRUD

```go
// Put
user := User{PK: "USER#1", SK: "PROFILE", Name: "Bob", Email: "bob@example.com"}
err := table.Put(ctx, user)

// Get
user, err := dynago.Get[User](ctx, table, dynago.Key("PK", "USER#1", "SK", "PROFILE"))

// Get with consistent read and projection
user, err := dynago.Get[User](ctx, table,
	dynago.Key("PK", "USER#1", "SK", "PROFILE"),
	dynago.ConsistentRead(),
	dynago.Project("Name", "Email"),
)

// Query all items under a partition key
users, err := dynago.Query[User](ctx, table, dynago.Partition("PK", "USER#1"))

// Scan with limit
users, err := dynago.Scan[User](ctx, table, dynago.ScanLimit(100))
```

### Query with Sort Key Conditions and Filters

```go
// Query with begins_with on sort key
orders, err := dynago.Query[Order](ctx, table,
	dynago.Partition("PK", "USER#1").SortBeginsWith("SK", "ORDER#"),
)

// Query with sort key range
orders, err := dynago.Query[Order](ctx, table,
	dynago.Partition("PK", "USER#1").SortBetween("SK", "ORDER#2024-01", "ORDER#2024-12"),
)

// Query with filter expression, limit, and descending order
orders, err := dynago.Query[Order](ctx, table,
	dynago.Partition("PK", "USER#1").SortBeginsWith("SK", "ORDER#"),
	dynago.QueryFilter("Amount > ?", 100),
	dynago.QueryLimit(10),
	dynago.ScanForward(false),
)

// Query a secondary index
orders, err := dynago.Query[Order](ctx, table,
	dynago.Partition("GSI1PK", "STATUS#shipped"),
	dynago.QueryIndex("GSI1"),
)
```

### Conditional Writes

```go
// Put only if item does not already exist
err := table.Put(ctx, user, dynago.IfNotExists("PK"))

// Put with custom condition expression
err := table.Put(ctx, user, dynago.PutCondition("Version = ?", 3))
```

### Single-Table Design with Registry

```go
// Define entity types that implement the Entity interface.
type User struct {
	PK    string `dynamodbav:"PK"`
	SK    string `dynamodbav:"SK"`
	Name  string `dynamodbav:"Name"`
}

func (u User) DynagoEntity() dynago.EntityInfo {
	return dynago.EntityInfo{Discriminator: "USER"}
}

type Order struct {
	PK     string  `dynamodbav:"PK"`
	SK     string  `dynamodbav:"SK"`
	Amount float64 `dynamodbav:"Amount"`
}

func (o Order) DynagoEntity() dynago.EntityInfo {
	return dynago.EntityInfo{Discriminator: "ORDER"}
}

// Create a registry and attach it to the table.
reg := dynago.NewRegistry("EntityType")
reg.Register(User{})
reg.Register(Order{})

table := db.Table("MyTable", dynago.WithRegistry(reg))

// Query all entities under a partition key.
coll, err := dynago.QueryCollection(ctx, table, dynago.Partition("PK", "USER#1"))

// Extract typed slices from the collection.
users := dynago.ItemsOf[User](coll)
orders := dynago.ItemsOf[Order](coll)

// Or iterate lazily with Go 1.23+ range-over-func.
for item, err := range dynago.CollectionIter(ctx, table, dynago.Partition("PK", "USER#1")) {
	if err != nil {
		log.Fatal(err)
	}
	switch v := item.(type) {
	case User:
		fmt.Println("User:", v.Name)
	case Order:
		fmt.Println("Order:", v.Amount)
	}
}
```

### Testing with memdb

```go
package myapp_test

import (
	"context"
	"testing"

	"github.com/danielmensah/dynago"
	"github.com/danielmensah/dynago/dynagotest"
	"github.com/danielmensah/dynago/memdb"
)

func TestUserService(t *testing.T) {
	backend := memdb.New()
	backend.CreateTable("MyTable", memdb.TableSchema{
		HashKey:  memdb.KeyDef{Name: "PK", Type: memdb.StringKey},
		RangeKey: &memdb.KeyDef{Name: "SK", Type: memdb.StringKey},
	})

	db := dynago.New(backend)
	table := db.Table("MyTable")
	ctx := context.Background()

	// Seed test data.
	err := dynagotest.Seed(ctx, table, []any{
		User{PK: "USER#1", SK: "PROFILE", Name: "Alice"},
		User{PK: "USER#2", SK: "PROFILE", Name: "Bob"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Or seed from a DynamoDB JSON file.
	// err = dynagotest.SeedFromJSON(ctx, table, "testdata/fixtures.json")

	// Assert items exist with expected attributes.
	dynagotest.AssertItemExists(t, table,
		dynago.Key("PK", "USER#1", "SK", "PROFILE"),
		dynagotest.HasAttribute("Name", "Alice"),
	)

	// Assert an item does not exist.
	dynagotest.AssertItemNotExists(t, table,
		dynago.Key("PK", "USER#999", "SK", "PROFILE"),
	)

	// Assert query result count.
	dynagotest.AssertCount(t, table, dynago.Partition("PK", "USER#1"), 1)
}
```

### OpenTelemetry Setup

```go
import (
	"github.com/danielmensah/dynago"
	"github.com/danielmensah/dynago/dynagotel"
	"go.opentelemetry.io/otel"
)

tracer := otel.Tracer("myapp")
meter := otel.Meter("myapp")

otelMiddleware := dynagotel.NewMiddleware(
	dynagotel.WithTracer(tracer),
	dynagotel.WithMeter(meter),
)

db := dynago.New(backend, dynago.WithMiddleware(otelMiddleware))
```

This wraps every DynamoDB operation with:
- A trace span (`db.system=dynamodb`, `db.operation=GetItem`, etc.)
- An operation counter (`dynago.operations.total`)
- A latency histogram (`dynago.latency` in milliseconds)

### Custom Middleware

```go
func loggingMiddleware(inner dynago.Backend) dynago.Backend {
	// Return a Backend implementation that logs before/after delegating to inner.
	// ...
}

db := dynago.New(backend, dynago.WithMiddleware(loggingMiddleware))
```

## API Reference

Full API documentation is available on [pkg.go.dev](https://pkg.go.dev/github.com/danielmensah/dynago).

### Packages

| Package | Description |
|---------|-------------|
| [`dynago`](https://pkg.go.dev/github.com/danielmensah/dynago) | Core library: DB, Table, Get, Put, Query, Scan, Registry, Collection |
| [`memdb`](https://pkg.go.dev/github.com/danielmensah/dynago/memdb) | In-memory Backend for testing |
| [`dynagotest`](https://pkg.go.dev/github.com/danielmensah/dynago/dynagotest) | Test helpers: Seed, SeedFromJSON, AssertItemExists, AssertCount |
| [`dynagotel`](https://pkg.go.dev/github.com/danielmensah/dynago/dynagotel) | OpenTelemetry tracing and metrics middleware |

## License

MIT
