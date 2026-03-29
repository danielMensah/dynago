---
title: "Migrating from guregu/dynamo"
weight: 2
---

# Migrating from guregu/dynamo to DynaGo

This guide helps you migrate an existing codebase from
[guregu/dynamo](https://github.com/guregu/dynamo) to
[DynaGo](https://github.com/danielmensah/dynago). DynaGo is a modern,
generics-first DynamoDB library for Go 1.23+ that replaces method-chaining
with type-safe free functions, swaps the AWS `dynamodbiface` mock pattern for
a clean `Backend` interface, and adds first-class support for single-table
design, iterators, and OpenTelemetry.

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

DynaGo uses AWS SDK v2 and separates the backend from the library core.

---

## Side-by-Side CRUD Comparison

### Put

**guregu/dynamo**

```go
err := table.Put(User{ID: "123", Name: "Alice", Age: 30}).Run()
```

**DynaGo**

```go
err := table.Put(ctx, User{ID: "123", Name: "Alice", Age: 30})
```

No `.Run()` -- the operation executes immediately. Conditional puts use functional options: `dynago.IfNotExists("ID")`.

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

Generic free function -- the return type is inferred from `[User]`.

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

### Delete

**guregu/dynamo**

```go
err := table.Delete("ID", "123").Run()
```

**DynaGo**

```go
err := table.Delete(ctx, dynago.Key("ID", "123"))
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

### BatchGet

**guregu/dynamo**

```go
var users []User
keys := []dynamo.Keyed{dynamo.Keys{"123"}, dynamo.Keys{"456"}}
err := table.Batch("ID").Get(keys...).All(&users)
```

**DynaGo**

```go
users, err := dynago.BatchGet[User](ctx, table, []dynago.KeyValue{
    dynago.Key("ID", "123"),
    dynago.Key("ID", "456"),
})
```

### BatchWrite

**guregu/dynamo**

```go
wrote, err := table.Batch("ID").Write().Put(user1, user2).Delete(dynamo.Keys{"789"}).Run()
```

**DynaGo**

```go
err := table.BatchPut(ctx, []any{user1, user2})
err := table.BatchDelete(ctx, []dynago.KeyValue{dynago.Key("ID", "789")})
```

---

## Struct Tag Migration

Both libraries use the `dynamo` struct tag. Most tags are directly compatible:

| Tag | guregu/dynamo | DynaGo | Notes |
|-----|-------------|--------|-------|
| Attribute name | `dynamo:"MyAttr"` | `dynamo:"MyAttr"` | Identical |
| Hash key | `dynamo:",hash"` | `dynamo:",hash"` | Identical |
| Range key | `dynamo:",range"` | `dynamo:",range"` | Identical |
| Omit empty | `dynamo:",omitempty"` | `dynamo:",omitempty"` | Identical |
| Skip field | `dynamo:"-"` | `dynamo:"-"` | Identical |
| Set encoding | `dynamo:",set"` | `dynamo:",set"` | Identical |
| Unix timestamp | `dynamo:",unixtime"` | `dynamo:",unixtime"` | Identical |
| GSI key | N/A | `dynamo:",gsi:IndexName"` | DynaGo-only |

---

## Testing Migration

This is one of the biggest improvements in DynaGo. Instead of mocking the AWS
SDK interface, you use an in-memory backend.

### guregu/dynamo (old approach)

```go
// You had to create a mock that implements dozens of methods
type mockDynamoDB struct {
    dynamodbiface.DynamoDBAPI
    getItemOutput *dynamodb.GetItemOutput
}
// Repeat for PutItem, Query, Scan, DeleteItem, UpdateItem, BatchGetItem, ...
```

### DynaGo (new approach)

```go
func TestGetUser(t *testing.T) {
    backend := memdb.New()
    backend.CreateTable("Users", memdb.TableSchema{
        HashKey: memdb.KeyDef{Name: "ID", Type: memdb.StringKey},
    })

    db := dynago.New(backend)
    table := db.Table("Users")
    ctx := context.Background()

    _ = table.Put(ctx, User{ID: "123", Name: "Alice", Age: 30})

    user, err := dynago.Get[User](ctx, table, dynago.Key("ID", "123"))
    if err != nil {
        t.Fatal(err)
    }
    if user.Name != "Alice" {
        t.Errorf("got %q, want Alice", user.Name)
    }
}
```

Benefits:
- **No mocks** -- real operations run against an in-memory store
- **Full feature support** -- queries, scans, filters, conditions, batch operations, and transactions
- **No AWS credentials needed** -- tests run offline and instantly
- **Realistic behavior** -- condition checks, key conflicts, and pagination work as expected

---

## Error Handling Differences

### guregu/dynamo

```go
err := table.Get("ID", "missing").One(&user)
if err == dynamo.ErrNotFound { ... }

err = table.Put(item).If("attribute_not_exists(ID)").Run()
if dynamo.IsCondCheckFailed(err) { ... }
```

### DynaGo

```go
user, err := dynago.Get[User](ctx, table, dynago.Key("ID", "missing"))
if errors.Is(err, dynago.ErrNotFound) { ... }
// Or: dynago.IsNotFound(err)

err = table.Put(ctx, item, dynago.IfNotExists("ID"))
if errors.Is(err, dynago.ErrConditionFailed) { ... }
// Or: dynago.IsCondCheckFailed(err)
```

### Sentinel Errors

| Error | Helper | Description |
|-------|--------|-------------|
| `dynago.ErrNotFound` | `dynago.IsNotFound(err)` | Item does not exist |
| `dynago.ErrConditionFailed` | `dynago.IsCondCheckFailed(err)` | Condition expression failed |
| `dynago.ErrValidation` | `dynago.IsValidation(err)` | Client-side validation error |
| `dynago.ErrTransactionCancelled` | `dynago.IsTxCancelled(err)` | Transaction was cancelled |

---

## What's New in DynaGo

These features have no equivalent in guregu/dynamo:

1. **Type-Safe Generics** -- `Get[User]`, `Query[Order]`, `Scan[T]` with compile-time safety
2. **Go 1.23 Iterators** -- `QueryIter[T]`, `ScanIter[T]`, `CollectionIter` for lazy pagination
3. **Single-Table Design** -- Built-in `Registry`, `QueryCollection`, `ItemsOf[T]`
4. **In-Memory Backend** -- `memdb` package for fast, offline testing
5. **OpenTelemetry** -- Drop-in tracing and metrics via `dynagotel` middleware
6. **Middleware Architecture** -- Composable `func(Backend) Backend` wrappers
7. **Transactions** -- `WriteTx` and `ReadTx` builders with fluent API

---

## Quick Reference: API Mapping

| guregu/dynamo | DynaGo |
|---|---|
| `table.Put(item).Run()` | `table.Put(ctx, item)` |
| `table.Put(item).If("...").Run()` | `table.Put(ctx, item, dynago.PutCondition("..."))` |
| `table.Get("K", v).One(&out)` | `out, err := dynago.Get[T](ctx, table, dynago.Key("K", v))` |
| `table.Get("K", v).Range("SK", op, v2).All(&out)` | `out, err := dynago.Query[T](ctx, table, dynago.Partition("K", v).Sort*(SK, v2))` |
| `table.Scan().Filter("...").All(&out)` | `out, err := dynago.Scan[T](ctx, table, dynago.ScanFilter("..."))` |
| `table.Delete("K", v).Run()` | `table.Delete(ctx, dynago.Key("K", v))` |
| `table.Update("K", v).Set("A", x).Run()` | `table.Update(ctx, dynago.Key("K", v), dynago.Set("A", x))` |
| `table.Batch("K").Get(keys...).All(&out)` | `out, err := dynago.BatchGet[T](ctx, table, keys)` |
| `table.Batch("K").Write().Put(items...).Run()` | `table.BatchPut(ctx, items)` |
| `dynamodbiface.DynamoDBAPI` mock | `memdb.New()` or implement `dynago.Backend` |
