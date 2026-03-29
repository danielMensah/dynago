---
title: "Getting Started"
weight: 1
bookCollapseSection: false
---

# Getting Started

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

## Basic CRUD

### Put

```go
user := User{PK: "USER#1", SK: "PROFILE", Name: "Bob", Email: "bob@example.com"}
err := table.Put(ctx, user)
```

### Get

```go
user, err := dynago.Get[User](ctx, table, dynago.Key("PK", "USER#1", "SK", "PROFILE"))

// With consistent read and projection
user, err := dynago.Get[User](ctx, table,
	dynago.Key("PK", "USER#1", "SK", "PROFILE"),
	dynago.ConsistentRead(),
	dynago.Project("Name", "Email"),
)
```

### Query

```go
// All items under a partition key
users, err := dynago.Query[User](ctx, table, dynago.Partition("PK", "USER#1"))

// With sort key condition
orders, err := dynago.Query[Order](ctx, table,
	dynago.Partition("PK", "USER#1").SortBeginsWith("SK", "ORDER#"),
)

// With filter, limit, and descending order
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

### Scan

```go
users, err := dynago.Scan[User](ctx, table, dynago.ScanLimit(100))
```

### Conditional Writes

```go
// Put only if item does not already exist
err := table.Put(ctx, user, dynago.IfNotExists("PK"))

// Put with custom condition expression
err := table.Put(ctx, user, dynago.PutCondition("Version = ?", 3))
```

## Testing with memdb

```go
func TestUserService(t *testing.T) {
	backend := memdb.New()
	backend.CreateTable("MyTable", memdb.TableSchema{
		HashKey:  memdb.KeyDef{Name: "PK", Type: memdb.StringKey},
		RangeKey: &memdb.KeyDef{Name: "SK", Type: memdb.StringKey},
	})

	db := dynago.New(backend)
	table := db.Table("MyTable")
	ctx := context.Background()

	// Seed test data
	err := dynagotest.Seed(ctx, table, []any{
		User{PK: "USER#1", SK: "PROFILE", Name: "Alice"},
		User{PK: "USER#2", SK: "PROFILE", Name: "Bob"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Assert items exist
	dynagotest.AssertItemExists(t, table,
		dynago.Key("PK", "USER#1", "SK", "PROFILE"),
		dynagotest.HasAttribute("Name", "Alice"),
	)

	// Assert query result count
	dynagotest.AssertCount(t, table, dynago.Partition("PK", "USER#1"), 1)
}
```

## OpenTelemetry Setup

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

## Custom Middleware

```go
func loggingMiddleware(inner dynago.Backend) dynago.Backend {
	// Return a Backend implementation that logs before/after delegating to inner.
	// ...
}

db := dynago.New(backend, dynago.WithMiddleware(loggingMiddleware))
```
