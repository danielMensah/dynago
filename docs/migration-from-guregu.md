# Migrating from guregu/dynamo to DynaGo

This guide helps you migrate an existing codebase from
[guregu/dynamo](https://github.com/guregu/dynamo) to
[DynaGo](https://github.com/danielmensah/dynago). DynaGo is a modern,
generics-first DynamoDB library for Go 1.23+ that replaces method-chaining
with type-safe free functions, swaps the AWS `dynamodbiface` mock pattern for
a clean `Backend` interface, and adds first-class support for single-table
design, iterators, and OpenTelemetry.

---

## Table of Contents

1. [Setup and Initialization](#setup-and-initialization)
2. [Side-by-Side CRUD Comparison](#side-by-side-crud-comparison)
3. [Struct Tag Migration](#struct-tag-migration)
4. [Testing Migration](#testing-migration)
5. [Error Handling Differences](#error-handling-differences)
6. [What's New in DynaGo](#whats-new-in-dynago)

---

## Setup and Initialization

### guregu/dynamo

```go
import (
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/guregu/dynamo"
)

sess := session.Must(session.NewSession())
db := dynamo.New(sess)
table := db.Table("Users")
```

### DynaGo

```go
import (
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/danielmensah/dynago"
    dynagoaws "github.com/danielmensah/dynago/aws"
)

cfg, _ := config.LoadDefaultConfig(ctx)
client := dynamodb.NewFromConfig(cfg)
backend := dynagoaws.NewAWSBackend(client)
db := dynago.New(backend)
table := db.Table("Users")
```

DynaGo uses AWS SDK v2 and separates the backend from the library core. The
`Backend` interface has no AWS SDK types -- all request/response types are
library-owned.

---

## Side-by-Side CRUD Comparison

### Put

**guregu/dynamo**

```go
type User struct {
    ID   string `dynamo:"ID,hash"`
    Name string `dynamo:"Name"`
    Age  int    `dynamo:"Age"`
}

err := table.Put(User{ID: "123", Name: "Alice", Age: 30}).Run()
```

**DynaGo**

```go
type User struct {
    ID   string `dynamo:"ID,hash"`
    Name string `dynamo:"Name"`
    Age  int    `dynamo:"Age"`
}

err := table.Put(ctx, User{ID: "123", Name: "Alice", Age: 30})
```

Key differences:
- `Put` is a method on `*Table` that takes a `context.Context` as the first argument.
- No `.Run()` -- the operation executes immediately.
- Conditional puts use functional options: `dynago.IfNotExists("ID")` or
  `dynago.PutCondition("attribute_not_exists(#Email)", ...)`.

### Get

**guregu/dynamo**

```go
var user User
err := table.Get("ID", "123").One(&user)
```

**DynaGo**

```go
user, err := dynago.Get[User](ctx, table, dynago.Key("ID", "123"))
```

Key differences:
- Generic free function -- the return type is inferred from `[User]`.
- Key is built with `dynago.Key("attr", value)` (2 args for hash-only, 4 for
  hash+range).
- Options like `dynago.ConsistentRead()` and `dynago.Project("Name", "Age")`
  are passed as trailing arguments.

### Query

**guregu/dynamo**

```go
var users []User
err := table.Get("OrgID", "org-1").
    Range("SK", dynamo.BeginsWith, "USER#").
    Filter("Age > ?", 25).
    All(&users)
```

**DynaGo**

```go
users, err := dynago.Query[User](ctx, table,
    dynago.Partition("OrgID", "org-1").SortBeginsWith("SK", "USER#"),
    dynago.QueryFilter("Age > ?", 25),
)
```

Key differences:
- `Partition()` builds a `KeyCondition` with fluent sort-key methods:
  `.SortEquals()`, `.SortBeginsWith()`, `.SortBetween()`, `.SortGreaterThan()`,
  `.SortLessThan()`, `.SortGreaterOrEqual()`, `.SortLessOrEqual()`.
- Filter uses `?` placeholders, same as guregu.
- Additional options: `dynago.QueryLimit(n)`, `dynago.ScanForward(bool)`,
  `dynago.QueryIndex("gsi-name")`, `dynago.QueryProject("attr1", "attr2")`,
  `dynago.QueryConsistentRead()`.

**Iterator variant (lazy pagination):**

```go
for user, err := range dynago.QueryIter[User](ctx, table,
    dynago.Partition("OrgID", "org-1").SortBeginsWith("SK", "USER#"),
) {
    if err != nil {
        return err
    }
    fmt.Println(user.Name)
}
```

### Scan

**guregu/dynamo**

```go
var users []User
err := table.Scan().Filter("Age > ?", 25).All(&users)
```

**DynaGo**

```go
users, err := dynago.Scan[User](ctx, table,
    dynago.ScanFilter("Age > ?", 25),
)
```

Options: `dynago.ScanLimit(n)`, `dynago.ScanIndex("gsi-name")`,
`dynago.ScanProject("attr1")`, `dynago.ScanConsistentRead()`.

**Iterator variant:**

```go
for user, err := range dynago.ScanIter[User](ctx, table,
    dynago.ScanFilter("Age > ?", 25),
) {
    if err != nil {
        return err
    }
    process(user)
}
```

### Delete

**guregu/dynamo**

```go
err := table.Delete("ID", "123").Run()
```

**DynaGo**

```go
err := table.Delete(ctx, dynago.Key("ID", "123"))
```

Conditional delete:

```go
err := table.Delete(ctx, dynago.Key("ID", "123"),
    dynago.DeleteCondition("attribute_exists(#ID)"),
)
```

### Update

**guregu/dynamo**

```go
err := table.Update("ID", "123").
    Set("Name", "Bob").
    Add("LoginCount", 1).
    Remove("TempField").
    Run()
```

**DynaGo**

```go
err := table.Update(ctx, dynago.Key("ID", "123"),
    dynago.Set("Name", "Bob"),
    dynago.Add("LoginCount", 1),
    dynago.Remove("TempField"),
)
```

To get the updated item back:

```go
updated, err := dynago.UpdateReturning[User](ctx, table, dynago.Key("ID", "123"),
    dynago.Set("Name", "Bob"),
    dynago.ReturnNew(),
)
```

Options: `dynago.Set()`, `dynago.Add()`, `dynago.Remove()`, `dynago.Delete()`,
`dynago.IfCondition("expression", vals...)`, `dynago.ReturnNew()`,
`dynago.ReturnOld()`.

### BatchGet

**guregu/dynamo**

```go
var users []User
keys := []dynamo.Keyed{
    dynamo.Keys{"123"},
    dynamo.Keys{"456"},
}
err := table.Batch("ID").Get(keys...).All(&users)
```

**DynaGo**

```go
users, err := dynago.BatchGet[User](ctx, table, []dynago.KeyValue{
    dynago.Key("ID", "123"),
    dynago.Key("ID", "456"),
})
```

Items are automatically chunked into groups of 100 with retry for unprocessed
keys. Optional: `dynago.BatchGetProject("Name", "Age")`.

### BatchWrite (Put + Delete)

**guregu/dynamo**

```go
wrote, err := table.Batch("ID").
    Write().
    Put(user1, user2).
    Delete(dynamo.Keys{"789"}).
    Run()
```

**DynaGo**

```go
// Batch put
err := table.BatchPut(ctx, []any{user1, user2})

// Batch delete
err := table.BatchDelete(ctx, []dynago.KeyValue{
    dynago.Key("ID", "789"),
})
```

Options: `dynago.OnProgress(func(completed, total int) { ... })`,
`dynago.MaxConcurrency(4)`. Items are automatically chunked into groups of 25
with exponential backoff retry for unprocessed items.

---

## Struct Tag Migration

Both libraries use the `dynamo` struct tag. Most tags are directly compatible.

| Tag | guregu/dynamo | DynaGo | Notes |
|-----|-------------|--------|-------|
| Attribute name | `dynamo:"MyAttr"` | `dynamo:"MyAttr"` | Identical |
| Hash key | `dynamo:",hash"` | `dynamo:",hash"` | Identical |
| Range key | `dynamo:",range"` | `dynamo:",range"` | Identical |
| Omit empty | `dynamo:",omitempty"` | `dynamo:",omitempty"` | Identical |
| Skip field | `dynamo:"-"` | `dynamo:"-"` | Identical |
| Set encoding | `dynamo:",set"` | `dynamo:",set"` | Identical |
| Unix timestamp | `dynamo:",unixtime"` | `dynamo:",unixtime"` | Identical |
| GSI key | N/A | `dynamo:",gsi:IndexName"` | DynaGo-only: marks a field as a GSI key |

### Example

```go
// This struct works with both libraries (minus the gsi tag)
type Order struct {
    PK        string    `dynamo:"PK,hash"`
    SK        string    `dynamo:"SK,range"`
    GSI1PK    string    `dynamo:"GSI1PK,gsi:GSI1"`  // DynaGo-only
    Total     float64   `dynamo:"Total"`
    Tags      []string  `dynamo:"Tags,set"`
    CreatedAt time.Time `dynamo:"CreatedAt,unixtime"`
    Internal  string    `dynamo:"-"`
}
```

In most cases, your existing struct tags will work without changes. The `gsi:`
tag option is new in DynaGo and optional -- it is used for schema metadata and
does not affect marshaling behavior.

---

## Testing Migration

This is one of the biggest improvements in DynaGo. Instead of mocking the AWS
SDK interface, you use an in-memory backend.

### guregu/dynamo (old approach)

```go
import (
    "github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
    "github.com/guregu/dynamo"
)

// You had to create a mock that implements dozens of methods
type mockDynamoDB struct {
    dynamodbiface.DynamoDBAPI
    getItemOutput *dynamodb.GetItemOutput
    // ... one field per operation you want to mock
}

func (m *mockDynamoDB) GetItemWithContext(ctx aws.Context, input *dynamodb.GetItemInput, opts ...request.Option) (*dynamodb.GetItemOutput, error) {
    return m.getItemOutput, nil
}

// Repeat for PutItem, Query, Scan, DeleteItem, UpdateItem, BatchGetItem, ...
// Tests are brittle and coupled to AWS SDK types.
```

### DynaGo (new approach)

```go
import (
    "context"
    "testing"

    "github.com/danielmensah/dynago"
    "github.com/danielmensah/dynago/memdb"
)

func TestGetUser(t *testing.T) {
    // Create an in-memory backend
    backend := memdb.New()
    backend.CreateTable("Users", memdb.TableSchema{
        HashKey: memdb.KeyDef{Name: "ID", Type: memdb.StringKey},
    })

    db := dynago.New(backend)
    table := db.Table("Users")
    ctx := context.Background()

    // Seed test data using real Put operations
    err := table.Put(ctx, User{ID: "123", Name: "Alice", Age: 30})
    if err != nil {
        t.Fatal(err)
    }

    // Test your code
    user, err := dynago.Get[User](ctx, table, dynago.Key("ID", "123"))
    if err != nil {
        t.Fatal(err)
    }
    if user.Name != "Alice" {
        t.Errorf("got %q, want Alice", user.Name)
    }
}
```

Benefits of the memdb approach:
- **No mocks** -- real operations run against an in-memory store.
- **Full feature support** -- queries, scans, filters, conditions, batch
  operations, and transactions all work.
- **GSI support** -- define GSIs in the schema and query them.
- **No AWS credentials needed** -- tests run offline and instantly.
- **Realistic behavior** -- condition checks, key conflicts, and pagination
  work as they would against real DynamoDB.

### Composite keys with memdb

```go
backend := memdb.New()
backend.CreateTable("Events", memdb.TableSchema{
    HashKey:  memdb.KeyDef{Name: "PK", Type: memdb.StringKey},
    RangeKey: &memdb.KeyDef{Name: "SK", Type: memdb.StringKey},
    GSIs: []memdb.GSISchema{
        {
            Name:     "GSI1",
            HashKey:  memdb.KeyDef{Name: "GSI1PK", Type: memdb.StringKey},
            RangeKey: &memdb.KeyDef{Name: "GSI1SK", Type: memdb.StringKey},
        },
    },
})
```

### Custom Backend for advanced mocking

If you need fine-grained control (e.g., simulating throttling or specific error
responses), implement the `dynago.Backend` interface directly:

```go
type slowBackend struct {
    dynago.Backend // embed a real backend
}

func (s *slowBackend) GetItem(ctx context.Context, req *dynago.GetItemRequest) (*dynago.GetItemResponse, error) {
    time.Sleep(100 * time.Millisecond) // simulate latency
    return s.Backend.GetItem(ctx, req)
}
```

The `Backend` interface has 10 methods (vs. 50+ in `dynamodbiface.DynamoDBAPI`),
making custom implementations much simpler.

---

## Error Handling Differences

### guregu/dynamo

```go
import "github.com/guregu/dynamo"

err := table.Get("ID", "missing").One(&user)
if err == dynamo.ErrNotFound {
    // not found
}

err = table.Put(item).If("attribute_not_exists(ID)").Run()
if dynamo.IsCondCheckFailed(err) {
    // condition failed
}
```

### DynaGo

```go
import (
    "errors"
    "github.com/danielmensah/dynago"
)

user, err := dynago.Get[User](ctx, table, dynago.Key("ID", "missing"))
if errors.Is(err, dynago.ErrNotFound) {
    // not found
}
// Or use the convenience helper:
if dynago.IsNotFound(err) {
    // not found
}

err = table.Put(ctx, item, dynago.IfNotExists("ID"))
if errors.Is(err, dynago.ErrConditionFailed) {
    // condition failed
}
// Or:
if dynago.IsCondCheckFailed(err) {
    // condition failed
}
```

### Sentinel errors

| Error | DynaGo | Description |
|-------|--------|-------------|
| `dynago.ErrNotFound` | `dynago.IsNotFound(err)` | Item does not exist |
| `dynago.ErrConditionFailed` | `dynago.IsCondCheckFailed(err)` | Condition expression failed |
| `dynago.ErrValidation` | `dynago.IsValidation(err)` | Client-side validation error |
| `dynago.ErrTransactionCancelled` | `dynago.IsTxCancelled(err)` | Transaction was cancelled |

All DynaGo errors support `errors.Is()` and `errors.As()`. Transaction errors
provide per-operation failure reasons:

```go
err := dynago.WriteTx(ctx, db).
    Put(table, user1, dynago.IfNotExists("PK")).
    Put(table, user2, dynago.IfNotExists("PK")).
    Run()

if dynago.IsTxCancelled(err) {
    reasons := dynago.TxCancelReasons(err)
    for i, r := range reasons {
        fmt.Printf("operation %d: code=%s message=%s\n", i, r.Code, r.Message)
    }
}
```

---

## What's New in DynaGo

These features have no equivalent in guregu/dynamo.

### 1. Type-Safe Generics

Every read operation returns typed values instead of requiring `&result`
out-parameters:

```go
// No type assertions, no interface{} -- compile-time type safety
user, err := dynago.Get[User](ctx, table, dynago.Key("ID", "123"))
users, err := dynago.Query[User](ctx, table, key)
users, err := dynago.Scan[User](ctx, table)
items, err := dynago.BatchGet[User](ctx, table, keys)
updated, err := dynago.UpdateReturning[User](ctx, table, key, dynago.Set("Name", "Bob"), dynago.ReturnNew())
```

### 2. Go 1.23 Iterators (iter.Seq2)

Lazy pagination via standard Go range loops -- no need to collect all results
into memory:

```go
for user, err := range dynago.QueryIter[User](ctx, table, key) {
    if err != nil {
        return err
    }
    // Process one item at a time; pages are fetched on demand.
    // Break early to stop pagination.
}
```

Also available: `dynago.ScanIter[T]` and `dynago.CollectionIter` for
polymorphic queries.

### 3. Single-Table Design Support

DynaGo has built-in support for polymorphic queries in single-table designs
via a `Registry`:

```go
// Define entity types
type User struct {
    PK   string `dynamo:"PK,hash"`
    SK   string `dynamo:"SK,range"`
    Name string `dynamo:"Name"`
}

func (u User) DynagoEntity() dynago.EntityInfo {
    return dynago.EntityInfo{Discriminator: "USER"}
}

type Order struct {
    PK    string  `dynamo:"PK,hash"`
    SK    string  `dynamo:"SK,range"`
    Total float64 `dynamo:"Total"`
}

func (o Order) DynagoEntity() dynago.EntityInfo {
    return dynago.EntityInfo{Discriminator: "ORDER"}
}

// Register types
reg := dynago.NewRegistry("_type")
reg.Register(User{})
reg.Register(Order{})

table := db.Table("MainTable", dynago.WithRegistry(reg))

// Query returns mixed types, automatically unmarshaled
coll, err := dynago.QueryCollection(ctx, table,
    dynago.Partition("PK", "CUSTOMER#123"),
)

users := dynago.ItemsOf[User](coll)   // type-safe filtering
orders := dynago.ItemsOf[Order](coll)
```

### 4. In-Memory Backend for Testing

The `memdb` package provides a complete DynamoDB-compatible in-memory backend:

```go
import "github.com/danielmensah/dynago/memdb"

backend := memdb.New()
backend.CreateTable("MyTable", memdb.TableSchema{
    HashKey:  memdb.KeyDef{Name: "PK", Type: memdb.StringKey},
    RangeKey: &memdb.KeyDef{Name: "SK", Type: memdb.StringKey},
})

db := dynago.New(backend)
// Use db exactly as you would in production
```

No DynamoDB Local, no Docker, no network. Tests run in microseconds.

### 5. OpenTelemetry Instrumentation

Drop-in tracing and metrics via middleware -- no code changes to your
business logic:

```go
import "github.com/danielmensah/dynago/dynagotel"

otelMW := dynagotel.NewMiddleware(
    dynagotel.WithTracer(tracer),
    dynagotel.WithMeter(meter),
)

db := dynago.New(backend, dynago.WithMiddleware(otelMW))
// Every DynamoDB operation now emits:
//   - Trace spans with db.system=dynamodb, db.operation=GetItem, db.name=TableName
//   - Counter: dynago.operations.total
//   - Histogram: dynago.latency (ms)
```

### 6. Middleware Architecture

DynaGo supports composable middleware that wraps the `Backend` interface:

```go
// Middleware type: func(Backend) Backend
db := dynago.New(backend, dynago.WithMiddleware(
    loggingMiddleware,
    dynagotel.NewMiddleware(dynagotel.WithTracer(tracer)),
))
```

This enables cross-cutting concerns like logging, metrics, retries, and
circuit breakers without touching application code.

### 7. Transactions

First-class transaction builders with a fluent API:

```go
// Write transaction (up to 100 operations)
err := dynago.WriteTx(ctx, db).
    Put(table, user, dynago.IfNotExists("PK")).
    Update(table, dynago.Key("PK", "ORG#1", "SK", "META"), dynago.Add("UserCount", 1)).
    Delete(table, dynago.Key("PK", "TEMP#1", "SK", "DATA")).
    Check(table, dynago.Key("PK", "ORG#1", "SK", "META"), "attribute_exists(PK)").
    Run()

// Read transaction
result, err := dynago.ReadTx(ctx, db).
    Get(table, dynago.Key("PK", "USER#1", "SK", "PROFILE")).
    Get(table, dynago.Key("PK", "USER#2", "SK", "PROFILE")).
    Run()

user1, err := dynago.GetAs[User](result, 0)
user2, err := dynago.GetAs[User](result, 1)
```

---

## Quick Reference: API Mapping

| guregu/dynamo | DynaGo |
|---|---|
| `table.Put(item).Run()` | `table.Put(ctx, item)` |
| `table.Put(item).If("...").Run()` | `table.Put(ctx, item, dynago.PutCondition("..."))` |
| `table.Get("K", v).One(&out)` | `out, err := dynago.Get[T](ctx, table, dynago.Key("K", v))` |
| `table.Get("K", v).Consistent(true).One(&out)` | `out, err := dynago.Get[T](ctx, table, dynago.Key("K", v), dynago.ConsistentRead())` |
| `table.Get("PK", v).Range("SK", op, v2).All(&out)` | `out, err := dynago.Query[T](ctx, table, dynago.Partition("PK", v).Sort*(SK, v2))` |
| `table.Get("PK", v).Filter("...").All(&out)` | `out, err := dynago.Query[T](ctx, table, key, dynago.QueryFilter("..."))` |
| `table.Scan().Filter("...").All(&out)` | `out, err := dynago.Scan[T](ctx, table, dynago.ScanFilter("..."))` |
| `table.Delete("K", v).Run()` | `table.Delete(ctx, dynago.Key("K", v))` |
| `table.Update("K", v).Set("A", x).Run()` | `table.Update(ctx, dynago.Key("K", v), dynago.Set("A", x))` |
| `table.Batch("K").Get(keys...).All(&out)` | `out, err := dynago.BatchGet[T](ctx, table, keys)` |
| `table.Batch("K").Write().Put(items...).Run()` | `table.BatchPut(ctx, items)` |
| `dynamodbiface.DynamoDBAPI` mock | `memdb.New()` or implement `dynago.Backend` |
