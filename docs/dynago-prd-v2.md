# Product Requirements Document: DynaGo

## A Generics-First, Single-Table-Native DynamoDB Library for Go

**Author:** Daniel
**Reviewed by:** Staff Engineer (Go Libraries)
**Version:** 2.0
**Date:** March 2026
**Status:** Draft — Revised

---

## Revision Summary

This revision incorporates a staff-level Go engineering review focused on idiomatic library design, API discoverability, and reducing coupling. Key changes from v1.0:

| Area | v1.0 Proposal | v2.0 Revision | Rationale |
|---|---|---|---|
| Operation API surface | All operations as package-level generic free functions | Methods on `*Table` where type params aren't needed; free functions only for `Get`, `Query`, `Scan`, `Update` (return-type generics) | Discoverability via `table.` autocompletion; free functions only where Go's type system demands them |
| Backend interface | Uses AWS SDK input/output types directly | Library-owned request/response types with an AWS adapter | Decouples in-memory backend from AWS SDK; simplifies memdb implementation |
| Entity registry | Generic `Register[T]` free function on mutable registry | Interface-based `Entity` contract with `DynagoEntity()` method | Compile-time enforcement via interfaces; idiomatic Go polymorphism |
| Key composition | `text/template`-based `KeyTemplate` | Plain Go functions for construction; `SplitKey` utility for parsing | Zero-allocation, compile-time checked, no template parsing errors |
| Query API | Required key conditions mixed into variadic options | `KeyCondition` as a required parameter, options for everything else | Self-documenting API; compiler enforces required arguments |
| Expression evaluation | Parse DynamoDB expression strings in memdb | Build internal AST from option calls; translate to DynamoDB strings for AWS, evaluate AST directly in memdb | Avoids writing a full DynamoDB expression parser; single source of truth |
| Phase ordering | Expression work deferred to Phase 3 | Expression AST design starts in Phase 1 | The AST shapes the entire architecture; delaying it creates rework |
| Struct tag | `dynago` custom tag | `dynamo` tag (compatible with guregu naming) | Reduces migration friction; familiar to existing DynamoDB Go developers |
| Go version | 1.22+ | 1.23+ | Enables `range over func` for idiomatic iterators from day one |
| Benchmarking and CI | "Performance benchmarks" mentioned in Phase 4; CI deferred to Phase 5 | Four-tier GitHub Actions CI pipeline with benchmarks on every push, conformance with DynamoDB Local, weekly comparative benchmarks, pre-release integration tests, and nightly fuzz testing (Section 11) | Benchmarks run continuously, not as an afterthought; regression detection is automated |

---

## 1. Executive Summary

DynaGo is a Go library for Amazon DynamoDB that provides a type-safe, generics-first API with first-class support for single-table design patterns. It targets experienced Go developers building production DynamoDB-backed services who are frustrated by the verbosity of the AWS SDK and the lack of single-table design tooling in existing libraries like guregu/dynamo.

The library differentiates on four axes: compile-time type safety via Go generics, native support for polymorphic single-table design, an in-memory DynamoDB backend for testing without Docker, and built-in OpenTelemetry observability.

---

## 2. Problem Statement

### 2.1 The DynamoDB Developer Experience in Go Is Poor

The official AWS SDK for Go v2 requires developers to manually construct expression builders, marshal/unmarshal attribute values, handle pagination, manage retries, and deal with deeply nested request/response structs. Simple operations routinely require 20-40 lines of boilerplate.

### 2.2 Existing Wrappers Don't Solve the Right Problems

**guregu/dynamo v2** is the most popular wrapper. After thorough analysis of its codebase, the following structural limitations are evident:

- **No compile-time type safety.** Every public API method accepts and returns `interface{}`. The `Iter` interface, `Put`, `Get`, `Query.One`, `Query.All`, `Scan.All`, `BatchGet.All` — all use `interface{}`. The internal `unmarshalAppendTo` function uses `reflect.New(membert)` to create slice elements at runtime. Errors that should be caught at compile time surface only at runtime.

- **No single-table design support.** The marshaling pipeline resolves a single `typedef` per Go type. `Query.All(ctx, &results)` requires a homogeneous typed slice. There is no discriminator-based polymorphic unmarshaling, no key composition helpers, no GSI overloading abstractions, and no heterogeneous collection query support.

- **Testing requires Docker.** The test suite depends on DynamoDB Local via Docker Compose. The `dynamodbiface.DynamoDBAPI` interface exposes 15+ methods, making manual mocking impractical. There is no in-memory backend.

- **No observability.** `ConsumedCapacity` tracking is functional but there is no tracing, no metrics emission, and no integration with OpenTelemetry or any observability framework.

### 2.3 Single-Table Design Is Mainstream but Tooling Is Missing

Single-table design (popularised by Rick Houlihan and the DynamoDB community) is now the recommended approach for most DynamoDB workloads. It involves storing multiple entity types in a single table, using composite keys (e.g., `USER#123`, `ORDER#2024-01-15#abc`), overloading GSIs, and querying heterogeneous item collections. No Go library provides first-class abstractions for these patterns.

---

## 3. Target Users

### 3.1 Primary Persona: Backend / Platform Engineer

- Writes Go services backed by DynamoDB in production
- Works in a team with multiple services sharing DynamoDB tables
- Familiar with single-table design or actively adopting it
- Values type safety, testability, and observability
- Frustrated by AWS SDK verbosity and runtime type errors
- Likely current user of guregu/dynamo or raw AWS SDK

### 3.2 Secondary Persona: Solo Developer / Startup Engineer

- Building new services from scratch on DynamoDB
- Wants to adopt best practices (single-table design) without deep DynamoDB expertise
- Values clear documentation and examples over exhaustive API surface
- Wants fast test feedback loops without Docker dependencies

---

## 4. Goals and Non-Goals

### 4.1 Goals

1. **Eliminate runtime type errors** for common DynamoDB operations through Go generics.
2. **Make single-table design idiomatic** with key composition, polymorphic queries, and entity discrimination.
3. **Enable fast, reliable unit testing** without external dependencies (no Docker, no network).
4. **Provide production observability out of the box** via OpenTelemetry integration.
5. **Remain a thin, composable layer** over the AWS SDK v2 — not a framework.

### 4.2 Non-Goals

- **ORM or migration framework.** DynaGo does not manage schema evolution, seed data, or table lifecycle beyond basic `CreateTable`.
- **Support for DynamoDB Streams.** Streams processing is a distinct concern better served by dedicated libraries or AWS Lambda integrations.
- **Full compatibility with guregu/dynamo struct tag options.** DynaGo uses the `dynamo` struct tag name for familiarity, but individual tag options (e.g., set handling, GSI declarations) may differ. Migration helpers or documentation may be provided, but full tag-level compatibility is not a goal.
- **Support for Go versions below 1.23.** Generics-first design requires modern Go.

---

## 5. Core Features

### 5.1 Generics-First API

#### 5.1.1 API Surface Design: Methods + Free Functions

The API uses **methods on `*Table`** for operations where the type parameter is inferred from the argument (Put, Delete), and **package-level generic free functions** only for operations where the type parameter appears in the return type (Get, Query, Scan, Update with return).

This design provides editor discoverability via `table.` autocompletion for common write operations, while using free functions only where Go's type system requires the caller to specify the result type.

```go
// Put — method on Table. Type is inferred from the argument.
// No type parameter needed; the compiler knows `user` is a User.
err := table.Put(ctx, user)
err := table.Put(ctx, user, dynago.IfNotExists("PK"))

// Delete — method on Table. No return type to parameterise.
err := table.Delete(ctx, dynago.Key("PK", "USER#123", "SK", "PROFILE"))

// Get — free function. The caller must specify the return type.
user, err := dynago.Get[User](ctx, table, dynago.Key("PK", "USER#123", "SK", "PROFILE"))

// Query — free function. Returns typed slice.
users, err := dynago.Query[User](ctx, table,
    dynago.Partition("PK", "ORG#456"),
    dynago.SortBeginsWith("SK", "USER#"),
)

// Scan — free function. Returns typed slice.
active, err := dynago.Scan[User](ctx, table,
    dynago.Filter("Active = ?", true),
)
```

**Design rationale:** `table.Put(ctx, user)` is immediately discoverable — developers type `table.` and see all available operations. `dynago.Get[User](...)` uses a free function because Go does not allow type parameters on interface or struct methods when the type parameter only appears in the return position. This split follows the principle of *using generics only where they provide value that methods cannot*.

#### 5.1.2 Type-Safe Iterators

Iterators are parameterised by result type, eliminating the `interface{}` out parameter.

```go
iter := dynago.QueryIter[User](ctx, table,
    dynago.Partition("PK", "ORG#456"),
    dynago.SortBeginsWith("SK", "USER#"),
)
for user, ok := iter.Next(ctx); ok; user, ok = iter.Next(ctx) {
    // user is already typed as User — no assertion needed
    fmt.Println(user.Name)
}
if err := iter.Err(); err != nil {
    return err
}
```

#### 5.1.3 Type-Safe Updates

Update operations use a free function (return-type generic) with a fluent option API.

```go
updated, err := dynago.Update[User](ctx, table,
    dynago.Key("PK", "USER#123", "SK", "PROFILE"),
    dynago.Set("Name", "New Name"),
    dynago.Add("LoginCount", 1),
    dynago.Remove("TempField"),
    dynago.IfCondition("Version = ?", currentVersion),
    dynago.ReturnNew(), // returns the updated User
)
```

#### 5.1.4 Struct Tag Design

DynaGo uses the `dynamo` struct tag with a clean, minimal syntax.

```go
type User struct {
    PK        string    `dynago:"PK,hash"`
    SK        string    `dynago:"SK,range"`
    GSI1PK    string    `dynago:"GSI1PK,gsi:GSI1,hash"`
    GSI1SK    string    `dynago:"GSI1SK,gsi:GSI1,range"`
    Name      string    `dynago:"Name"`
    Email     string    `dynago:"Email,omitempty"`
    Tags      []string  `dynago:"Tags,set"`
    ExpiresAt time.Time `dynago:"ExpiresAt,unixtime"`
    Internal  string    `dynago:"-"`
}
```

### 5.2 Single-Table Design Support

#### 5.2.1 Entity Interface and Registry

Types participate in single-table design by implementing the `Entity` interface, which provides compile-time enforcement that every registered type declares its discriminator value.

```go
// Entity is the interface types implement to participate in polymorphic queries.
// This provides compile-time safety: forgetting to implement the method is a
// compile error at registration, not a runtime surprise.
type Entity interface {
    DynagoEntity() EntityInfo
}

type EntityInfo struct {
    Discriminator string // e.g., "USER", "ORDER"
}

// User implements Entity
type User struct {
    PK   string `dynago:"PK,hash"`
    SK   string `dynago:"SK,range"`
    Name string `dynago:"Name"`
}

func (u User) DynagoEntity() EntityInfo {
    return EntityInfo{Discriminator: "USER"}
}

// Order implements Entity
type Order struct {
    PK    string  `dynago:"PK,hash"`
    SK    string  `dynago:"SK,range"`
    Total float64 `dynago:"Total"`
}

func (o Order) DynagoEntity() EntityInfo {
    return EntityInfo{Discriminator: "ORDER"}
}
```

The registry maps discriminator values to Go types for unmarshaling. Registration is explicit and uses the interface constraint for type safety.

```go
// Create a registry with the discriminator attribute name
registry := dynago.NewRegistry("EntityType")

// Register types — compile error if type doesn't implement Entity
registry.Register(User{})
registry.Register(Order{})
registry.Register(Product{})

// Create a table handle bound to this registry
table := db.Table("MyApp", dynago.WithRegistry(registry))
```

**Design rationale:** Using an interface for polymorphism is idiomatic Go. The `Entity` interface serves as a compile-time contract — if a developer forgets to implement `DynagoEntity()` on a type they try to register, the code won't compile. This is strictly stronger than the v1.0 approach where `Register[User](registry, "USER")` provided cosmetic generics but stored a `reflect.Type` internally with no compile-time guarantee of correctness.

#### 5.2.2 Key Composition

Key construction uses plain Go functions. This is zero-allocation, compile-time checked, and immediately readable. No template parsing, no runtime errors.

```go
// Define key constructors as plain functions
func UserPK(userID string) string   { return "USER#" + userID }
func UserSK() string               { return "PROFILE" }
func OrderPK(userID string) string  { return "USER#" + userID }
func OrderSK(date, orderID string) string {
    return "ORDER#" + date + "#" + orderID
}

// Usage — type-safe, zero-allocation key construction
user, err := dynago.Get[User](ctx, table,
    dynago.Key("PK", UserPK("123"), "SK", UserSK()),
)

// For key parsing, use a delimiter-based utility
parts := dynago.SplitKey("ORDER#2024-01-15#abc", "#")
// parts[0] = "ORDER", parts[1] = "2024-01-15", parts[2] = "abc"

// Or use a structured parser for named components
parsed := dynago.ParseKey("ORDER#2024-01-15#abc", "ORDER", "OrderDate", "OrderID")
// parsed["OrderDate"] = "2024-01-15", parsed["OrderID"] = "abc"
```

**Design rationale (v1.0 change):** The v1.0 proposal used `text/template` syntax (`KeyTemplate("USER#{{.UserID}}")`). This pulled in `text/template` for string concatenation — a heavy, slow dependency that introduces runtime parsing errors for a trivial operation. Plain functions are compile-time checked, zero-allocation, and immediately understandable. Template syntax solves a problem that doesn't exist in key construction.

#### 5.2.3 Heterogeneous Collection Queries

Query a partition and receive results as different Go types based on the discriminator.

```go
// Query all items for a user (profile, orders, preferences, etc.)
collection, err := dynago.QueryCollection(ctx, table,
    dynago.Partition("PK", "USER#123"),
)

// Access typed results
users := dynago.ItemsOf[User](collection)       // []User
orders := dynago.ItemsOf[Order](collection)      // []Order
prefs := dynago.ItemsOf[Preferences](collection) // []Preferences

// Or process items as they arrive via iterator
iter := dynago.CollectionIter(ctx, table,
    dynago.Partition("PK", "USER#123"),
)
for item, ok := iter.Next(ctx); ok; item, ok = iter.Next(ctx) {
    switch v := item.(type) {
    case User:
        handleUser(v)
    case Order:
        handleOrder(v)
    }
}
```

#### 5.2.4 GSI Overloading Helpers

Support for the common pattern where GSI key attributes have different semantic meanings per entity type.

```go
// User: GSI1PK = "ORG#orgId", GSI1SK = "USER#userId"
// Order: GSI1PK = "STATUS#pending", GSI1SK = "ORDER#2024-01-15"

// Query GSI with typed results
pendingOrders, err := dynago.Query[Order](ctx, table,
    dynago.Index("GSI1"),
    dynago.Partition("GSI1PK", "STATUS#pending"),
    dynago.SortBetween("GSI1SK", "ORDER#2024-01-01", "ORDER#2024-12-31"),
)
```

### 5.3 In-Memory Testing Backend

#### 5.3.1 In-Memory DynamoDB Engine

A lightweight, pure-Go DynamoDB implementation that evaluates expressions, enforces key constraints, and simulates real DynamoDB behaviour — without Docker, network calls, or external processes.

The in-memory backend evaluates expressions from the library's internal AST representation directly, rather than parsing DynamoDB expression strings. This avoids the need to implement a full DynamoDB expression parser and ensures the in-memory backend and the real backend use the same expression semantics.

```go
func TestCreateUser(t *testing.T) {
    // Create an in-memory backend — no Docker, no network
    mem := dynago.NewMemoryBackend()
    db := dynago.New(mem)

    // Create table with schema
    mem.CreateTable("Users", dynago.TableSchema{
        HashKey:  dynago.KeyDef{Name: "PK", Type: dynago.StringType},
        RangeKey: dynago.KeyDef{Name: "SK", Type: dynago.StringType},
        GSIs: []dynago.GSISchema{
            {Name: "GSI1", HashKey: dynago.KeyDef{Name: "GSI1PK", Type: dynago.StringType}},
        },
    })

    table := db.Table("Users")

    // Test your actual code — expressions are evaluated, conditions are checked
    err := table.Put(ctx, User{PK: "USER#1", SK: "PROFILE", Name: "Alice"})
    assert.NoError(t, err)

    // Condition expressions work
    err = table.Put(ctx, User{PK: "USER#1", SK: "PROFILE", Name: "Bob"},
        dynago.IfNotExists("PK"),
    )
    assert.True(t, dynago.IsCondCheckFailed(err)) // correctly fails
}
```

#### 5.3.2 Concurrency Model

The in-memory backend uses `sync.RWMutex` for safe concurrent access. This is required, not optional — Go developers routinely use `t.Parallel()`, and a testing backend that corrupts data or panics under concurrent access is unusable.

Read operations (GetItem, Query, Scan, BatchGetItem, TransactGetItems) acquire a read lock. Write operations (PutItem, UpdateItem, DeleteItem, BatchWriteItem, TransactWriteItems) acquire a write lock. Transactions hold the write lock for the entire transaction to ensure atomicity.

#### 5.3.3 Supported Operations

The in-memory backend supports the following operations with expression evaluation:

| Operation | Expression Support |
|---|---|
| GetItem | Projection expressions |
| PutItem | Condition expressions |
| UpdateItem | Update expressions, condition expressions |
| DeleteItem | Condition expressions |
| Query | Key conditions, filter expressions, projection |
| Scan | Filter expressions, projection |
| BatchGetItem | Projection |
| BatchWriteItem | Put and delete operations |
| TransactGetItems | Projection |
| TransactWriteItems | Condition expressions, all write types |

#### 5.3.4 Fixture and Seed Helpers

```go
func TestOrderQueries(t *testing.T) {
    mem := dynago.NewMemoryBackend()
    db := dynago.New(mem)

    // Seed from a slice
    dynago.Seed(ctx, db.Table("App"), []any{
        User{PK: "USER#1", SK: "PROFILE", Name: "Alice"},
        Order{PK: "USER#1", SK: "ORDER#2024-01-15#abc", Total: 99.99},
        Order{PK: "USER#1", SK: "ORDER#2024-02-20#def", Total: 149.99},
    })

    // Seed from JSON fixtures
    dynago.SeedFromJSON(ctx, db.Table("App"), "testdata/fixtures.json")

    // Your test logic here
}
```

#### 5.3.5 Assertions

```go
// Assert item exists with specific attributes
dynago.AssertItemExists(t, table, dynago.Key("PK", "USER#1", "SK", "PROFILE"),
    dynago.HasAttribute("Name", "Alice"),
    dynago.HasAttribute("Active", true),
)

// Assert item count
dynago.AssertCount(t, table,
    dynago.Partition("PK", "USER#1"),
    dynago.SortBeginsWith("SK", "ORDER#"),
    dynago.Equals(3),
)
```

### 5.4 Transactions and Batch Operations

#### 5.4.1 Transactional Closures

A higher-level API for read-modify-write patterns with automatic optimistic locking.

```go
// Read-modify-write with automatic condition checking
err := dynago.ReadModifyWrite[User](ctx, table,
    dynago.Key("PK", "USER#123", "SK", "PROFILE"),
    func(user *User) error {
        user.LoginCount++
        user.LastLogin = time.Now()
        return nil // return error to abort
    },
    dynago.OptimisticLock("Version"), // auto-increments and conditions on Version
)
```

#### 5.4.2 Fluent Transaction Builder

```go
result, err := dynago.WriteTx(ctx, db).
    Put(table, newOrder,
        dynago.IfNotExists("PK"),
    ).
    Update(table,
        dynago.Key("PK", "USER#123", "SK", "PROFILE"),
        dynago.Add("OrderCount", 1),
    ).
    Check(table,
        dynago.Key("PK", "INVENTORY#SKU-789", "SK", "STOCK"),
        dynago.Condition("Quantity >= ?", orderQty),
    ).
    Run()

// Structured error handling for partial failures
if dynago.IsTxCancelled(err) {
    reasons := dynago.TxCancelReasons(err)
    for i, reason := range reasons {
        fmt.Printf("Operation %d failed: %s\n", i, reason.Code)
        if reason.Item != nil {
            // Access the current item that caused the failure
        }
    }
}
```

#### 5.4.3 Batch Operations with Progress

```go
// Batch write with automatic chunking and progress callback
err := table.BatchPut(ctx, thousandItems,
    dynago.OnProgress(func(completed, total int) {
        log.Printf("Progress: %d/%d", completed, total)
    }),
    dynago.MaxConcurrency(4),
)

// Batch get with type safety
results, err := dynago.BatchGet[User](ctx, table, keys)
```

### 5.5 Observability

#### 5.5.1 OpenTelemetry Integration

All operations automatically emit OpenTelemetry spans and metrics when a tracer/meter provider is configured.

```go
db := dynago.New(cfg,
    dynago.WithTracer(otel.Tracer("dynago")),
    dynago.WithMeter(otel.Meter("dynago")),
)
```

**Spans emitted per operation:**

| Attribute | Description |
|---|---|
| `db.system` | `dynamodb` |
| `db.operation` | `GetItem`, `Query`, `PutItem`, etc. |
| `db.name` | Table name |
| `dynago.index` | Index name (if applicable) |
| `dynago.consumed_capacity.total` | Total consumed capacity units |
| `dynago.consumed_capacity.read` | Read capacity consumed |
| `dynago.consumed_capacity.write` | Write capacity consumed |
| `dynago.retry_count` | Number of retries for this operation |
| `dynago.items_count` | Number of items returned/written |
| `dynago.scanned_count` | Number of items scanned (for queries/scans) |

**Metrics emitted:**

| Metric | Type | Description |
|---|---|---|
| `dynago.operations.total` | Counter | Total operations by type and table |
| `dynago.consumed_capacity` | Counter | Consumed capacity units by table |
| `dynago.latency` | Histogram | Operation latency in milliseconds |
| `dynago.retries` | Counter | Retry count by table and operation |
| `dynago.errors` | Counter | Error count by type and table |

#### 5.5.2 Structured Logging Hook

```go
db := dynago.New(cfg,
    dynago.WithLogger(slog.Default()),
    dynago.LogSlowOperations(100 * time.Millisecond),
)
// Logs: level=WARN msg="slow DynamoDB operation" operation=Query table=MyApp duration=234ms consumed_rcu=15.5
```

---

## 6. Architecture

### 6.1 Package Structure

The root `dynago` package contains everything needed for basic operations. Sub-packages are reserved for opt-in features. Users should never need to import more than `dynago` for standard CRUD.

```
dynago/
├── dynago.go          # DB, Table, Backend interface — core entry points
├── option.go          # All option types (PutOption, GetOption, QueryOption, etc.)
├── key.go             # Key, KeyCondition, SplitKey, ParseKey
├── encode.go          # Generics-aware marshal/unmarshal
├── tag.go             # Struct tag parsing with sync.Map cache
├── errors.go          # Sentinel errors, typed error extractors
├── entity.go          # Entity interface, Registry, polymorphic dispatch
├── collection.go      # Heterogeneous collection queries and iterators
├── iter.go            # Generic iterator types
├── tx.go              # Transaction builders
├── batch.go           # Batch operation builders
├── memdb/             # In-memory DynamoDB backend
│   ├── backend.go     # Core in-memory store with sync.RWMutex
│   ├── table.go       # Table with indexes
│   ├── query.go       # Query evaluation (operates on internal AST)
│   └── tx.go          # Transaction support
├── otel/              # OpenTelemetry integration (separate sub-package)
│   ├── tracing.go
│   └── metrics.go
├── dynagotest/        # Test helpers (separate sub-package)
│   ├── assert.go
│   ├── seed.go
│   └── fixtures.go
└── internal/
    ├── expr/          # Expression AST, builder, and evaluator
    │   ├── ast.go     # AST node types
    │   ├── build.go   # Build AST from option calls
    │   ├── eval.go    # Evaluate AST (used by memdb)
    │   └── dynamo.go  # Translate AST to DynamoDB expression strings
    ├── codec/         # Codec internals (reflect-based, cached)
    └── reserved.go    # DynamoDB reserved words
```

### 6.2 Backend Interface

The library defines its own request/response types for the backend interface, decoupling implementations from AWS SDK version details. An AWS adapter translates to/from SDK types at the boundary.

```go
// Backend defines the operations the library needs.
// Types are library-owned, not AWS SDK types.
type Backend interface {
    GetItem(ctx context.Context, req *GetItemRequest) (*GetItemResponse, error)
    PutItem(ctx context.Context, req *PutItemRequest) (*PutItemResponse, error)
    DeleteItem(ctx context.Context, req *DeleteItemRequest) (*DeleteItemResponse, error)
    UpdateItem(ctx context.Context, req *UpdateItemRequest) (*UpdateItemResponse, error)
    Query(ctx context.Context, req *QueryRequest) (*QueryResponse, error)
    Scan(ctx context.Context, req *ScanRequest) (*ScanResponse, error)
    BatchGetItem(ctx context.Context, req *BatchGetItemRequest) (*BatchGetItemResponse, error)
    BatchWriteItem(ctx context.Context, req *BatchWriteItemRequest) (*BatchWriteItemResponse, error)
    TransactGetItems(ctx context.Context, req *TransactGetItemsRequest) (*TransactGetItemsResponse, error)
    TransactWriteItems(ctx context.Context, req *TransactWriteItemsRequest) (*TransactWriteItemsResponse, error)
}

// Request/response types use library-owned attribute value types.
// These are thin wrappers, not a full reimplementation.
type GetItemRequest struct {
    TableName       string
    Key             map[string]AttributeValue
    ProjectionExprs []expr.Node // internal AST
    ConsistentRead  bool
}

type GetItemResponse struct {
    Item             map[string]AttributeValue
    ConsumedCapacity *ConsumedCapacity
}
```

The AWS adapter translates these to SDK calls:

```go
// AWSBackend adapts the Backend interface to the AWS SDK v2 DynamoDB client.
// This is the only file that imports "github.com/aws/aws-sdk-go-v2/service/dynamodb".
type AWSBackend struct {
    client *dynamodb.Client
}

func NewAWSBackend(cfg aws.Config, opts ...func(*dynamodb.Options)) *AWSBackend {
    return &AWSBackend{client: dynamodb.NewFromConfig(cfg, opts...)}
}

func (b *AWSBackend) GetItem(ctx context.Context, req *GetItemRequest) (*GetItemResponse, error) {
    // Translate library types → AWS SDK types
    input := &dynamodb.GetItemInput{
        TableName:      &req.TableName,
        Key:            toAWSKey(req.Key),
        ConsistentRead: &req.ConsistentRead,
    }
    if len(req.ProjectionExprs) > 0 {
        input.ProjectionExpression, input.ExpressionAttributeNames = expr.ToDynamo(req.ProjectionExprs)
    }
    // Call AWS, translate response back
    out, err := b.client.GetItem(ctx, input)
    if err != nil {
        return nil, err
    }
    return &GetItemResponse{
        Item:             fromAWSItem(out.Item),
        ConsumedCapacity: fromAWSCapacity(out.ConsumedCapacity),
    }, nil
}
```

**Design rationale (v1.0 change):** The v1.0 backend interface used AWS SDK input/output types directly (`*dynamodb.GetItemInput`, `*dynamodb.GetItemOutput`). This forced the in-memory backend to construct and return full AWS SDK response objects — tedious and tightly coupled to SDK internals. Library-owned types keep the in-memory backend simple (it only deals with maps and the internal expression AST), and isolate AWS SDK version changes to the single adapter file.

### 6.3 Expression Architecture

Expressions (conditions, filters, projections, updates) are represented internally as an AST. This AST is the single source of truth for expression semantics.

```
User calls dynago.Filter("Active = ?", true)
    ↓
Option function builds expr.Node (AST)
    ↓
    ├── AWS path: expr.ToDynamo(node) → DynamoDB expression string + attribute maps
    │     → sent to real DynamoDB
    │
    └── memdb path: expr.Eval(node, item) → bool
          → evaluated in-memory against the item's attribute map
```

This architecture means the in-memory backend never needs to parse DynamoDB expression strings. It evaluates the same AST that was used to generate the DynamoDB expression. This is a massive simplification — DynamoDB's expression grammar has non-trivial precedence rules, function calls, nested paths, and list indexing. Building a correct parser for it is a multi-week effort that we avoid entirely.

```go
// internal/expr/ast.go

// Node represents an expression AST node.
type Node interface {
    nodeType() nodeKind
}

type CompareNode struct {
    Left  Node
    Op    CompareOp // EQ, NE, LT, LE, GT, GE
    Right Node
}

type FuncNode struct {
    Name string    // "attribute_exists", "begins_with", "contains", "size"
    Args []Node
}

type PathNode struct {
    Parts []string // e.g., ["Address", "City"] for nested access
}

type ValueNode struct {
    Value AttributeValue
}

type LogicalNode struct {
    Op    LogicalOp // AND, OR, NOT
    Left  Node
    Right Node      // nil for NOT
}
```

### 6.4 Encoding Pipeline

The encoding layer leverages generics to eliminate `reflect`-based type resolution at call sites while maintaining a cached `reflect`-based codec internally for marshaling struct fields.

```
User struct → typeCodec (cached via sync.Map) → encodeItem() → map[string]AttributeValue
map[string]AttributeValue → typeCodec (cached via sync.Map) → decodeItem() → User struct
```

Key difference from guregu: the public API never exposes `interface{}`. The internal codec uses reflection, but the boundary between library and caller is always typed.

---

## 7. API Design Principles

1. **Methods for writes, free functions for typed reads.** `table.Put(ctx, item)` and `table.Delete(ctx, key)` are methods because their types are inferred. `dynago.Get[T](ctx, table, key)` and `dynago.Query[T](ctx, table, ...)` are free functions because the caller must specify the return type. This maximises discoverability while using generics only where necessary.

2. **Required parameters are explicit; options are variadic.** Key conditions are required function parameters, not options mixed into the variadic list. The compiler enforces that you provide a key for `Get` and a partition for `Query`.

3. **Options over chaining.** Operations accept functional options rather than builder-chain methods. This is more composable and avoids the "forgotten terminal method" bug class.

4. **Errors are values, not panics.** All operations return errors. Condition check failures, not-found, and transaction cancellations have typed sentinel errors and extractor functions.

5. **Zero-config for simple cases.** A basic `Put`/`Get` should require no configuration beyond table name and item struct. Advanced features (registry, tracing, custom encoding) are opt-in.

6. **AWS SDK types at the boundary.** DynaGo accepts `aws.Config` for initialisation. Users can access the underlying AWS client when needed for operations the library doesn't wrap.

7. **Internal AST, not string parsing.** Expression semantics are defined by the internal AST. DynamoDB expression strings are a serialisation format used only by the AWS adapter. The in-memory backend evaluates the AST directly.

---

## 8. Query and Option API Design

### 8.1 Separating Required Parameters from Options

Query operations separate required parameters (key conditions) from optional modifiers (filters, projections, limits). This makes the API self-documenting and prevents compile-time-catchable mistakes from becoming runtime errors.

```go
// KeyCondition is a required first argument to Query, not a variadic option.
type KeyCondition struct {
    partitionAttr string
    partitionVal  AttributeValue
    sortExpr      *SortExpression // nil when querying partition only
}

// Constructors
func Partition(attr string, val any) KeyCondition { ... }

// SortExpression modifiers — these return a modified KeyCondition
func (kc KeyCondition) SortEquals(attr string, val any) KeyCondition { ... }
func (kc KeyCondition) SortBeginsWith(attr string, prefix string) KeyCondition { ... }
func (kc KeyCondition) SortBetween(attr string, lo, hi any) KeyCondition { ... }
func (kc KeyCondition) SortGreaterThan(attr string, val any) KeyCondition { ... }

// Query signature — partition is required, options are optional
func Query[T any](ctx context.Context, t *Table, key KeyCondition, opts ...QueryOption) ([]T, error)

// Usage
users, err := dynago.Query[User](ctx, table,
    dynago.Partition("PK", "ORG#456").SortBeginsWith("SK", "USER#"),
    dynago.Filter("Active = ?", true),  // QueryOption
    dynago.Limit(10),                   // QueryOption
)
```

### 8.2 Expression Placeholder Syntax

DynaGo uses `?` for value placeholders, consistent with SQL conventions and guregu/dynamo. Attribute name placeholders use `#name` syntax when referencing DynamoDB reserved words.

```go
// ? for values (familiar from SQL and guregu)
dynago.Filter("Active = ? AND Age > ?", true, 21)

// #name for reserved word attributes
dynago.Filter("#Status = ?", "active")

// The library automatically handles ExpressionAttributeNames and
// ExpressionAttributeValues when translating to DynamoDB expressions.
```

---

## 9. Migration Path from guregu/dynamo

While API compatibility is not a goal, a migration guide will be provided covering:

| guregu/dynamo | DynaGo |
|---|---|
| `table.Put(item).Run(ctx)` | `table.Put(ctx, item)` |
| `table.Get("ID", 42).One(ctx, &out)` | `out, err := dynago.Get[T](ctx, table, key)` |
| `table.Get("ID", 42).Range("SK", dynamo.Equal, sk).One(ctx, &out)` | `out, err := dynago.Get[T](ctx, table, dynago.Key("ID", 42, "SK", sk))` |
| `table.Scan().Filter("X = ?", v).All(ctx, &results)` | `results, err := dynago.Scan[T](ctx, table, dynago.Filter("X = ?", v))` |
| `dynamo` struct tag | `dynamo` struct tag (compatible syntax) |
| `dynamodbiface.DynamoDBAPI` for mocking | `dynago.Backend` interface + `memdb` package |

---

## 10. Phased Delivery Plan

### Phase 1: Foundation + Expression AST

**Deliverables:**
- Core types: `DB`, `Table`, `Backend` interface with library-owned request/response types
- AWS adapter (`AWSBackend`) translating library types to/from AWS SDK
- Expression AST (`internal/expr/ast.go`) with builder and DynamoDB string translator
- Encoding/decoding pipeline with struct tags (`dynamo` tag)
- `Table.Put`, `Table.Delete` as methods
- `Get[T]` as free function
- `Query[T]` and `Scan[T]` as free functions with filter/projection support
- `KeyCondition` as required parameter for Query
- Basic `Update[T]` builder
- Typed iterators with pagination support
- Error types and sentinel values
- Tier 1 benchmarks: encoding/decoding, expression build, full round-trip, key construction (see Section 11.2)
- Unit tests using standard Go testing
- `README.md` with examples

**Exit criteria:** Can perform all basic CRUD operations with type safety. Encoding/decoding parity with guregu/dynamo for common types (strings, numbers, bools, time.Time, slices, maps, sets, nested structs, custom marshalers). Expression AST can build and evaluate simple conditions, and translate them to DynamoDB expression strings.

**Note (v1.0 change):** Expression AST design starts in Phase 1, not Phase 3. The AST is the single source of truth for expression semantics and shapes both the AWS adapter and the in-memory backend. Deferring it creates rework.

### Phase 2: Single-Table Design

**Deliverables:**
- `Entity` interface and `Registry` with discriminator-based type resolution
- Key construction helpers (plain functions) and `SplitKey`/`ParseKey` utilities
- Heterogeneous collection queries and iterators
- GSI query support
- Polymorphic `Put` (auto-sets discriminator attribute)
- Documentation: "Single-Table Design with DynaGo" guide

**Exit criteria:** Can model and query a real-world single-table design (e.g., e-commerce with Users, Orders, Products) with type-safe polymorphic results.

### Phase 3: Testing Backend

**Deliverables:**
- In-memory backend (`memdb` package) implementing `Backend` with library-owned types
- Expression evaluator operating on internal AST (no DynamoDB string parsing)
- `sync.RWMutex` concurrency support
- Table schema enforcement (hash/range keys, GSIs)
- Test helper package (`dynagotest`) with assertions and seed utilities
- Tier 1 memdb benchmarks: expression evaluation, query at scale, concurrency (see Section 11.3)
- Tier 2 conformance test suite running against both memdb and DynamoDB Local (see Section 11.4)
- Nightly fuzz testing for encoding round-trips and expression evaluation (see Section 11.4.3)

**Exit criteria:** Can run a full application test suite against the in-memory backend with identical behaviour to DynamoDB Local for supported operations. Tier 2 conformance suite passes against both memdb and DynamoDB Local. Memdb query of 100 items completes in under 1ms.

### Phase 4: Transactions, Batches, and Observability

**Deliverables:**
- Transaction builders (read and write transactions)
- `ReadModifyWrite` with optimistic locking
- Batch get/write with automatic chunking and progress
- Transaction error decomposition helpers
- OpenTelemetry tracing and metrics (`otel` sub-package)
- Structured logging hook
- Comparative benchmarks (Tier 3) vs guregu/dynamo and raw AWS SDK
- Observability overhead benchmarks (Tier 4)

**Exit criteria:** Feature-complete for v0.1 release. Tier 3 comparative benchmarks show no more than 10% ns/op overhead and no more than 2x allocs/op vs guregu/dynamo for equivalent operations. Otel noop provider adds less than 2% overhead.

### Phase 5: Polish and Launch

**Deliverables:**
- API review and stabilisation
- Comprehensive documentation (GoDoc, guides, examples)
- Migration guide from guregu/dynamo
- Full CI/CD pipeline in GitHub Actions (all four tiers from Section 11.1: core tests and benchmarks on every push, conformance with DynamoDB Local on every push, weekly comparative benchmarks with regression alerts, pre-release integration tests against real DynamoDB via OIDC, nightly fuzz testing)
- Benchmark results published to GitHub Pages with historical tracking via `benchmark-action/github-action-benchmark`
- v0.1.0 release on GitHub
- Blog post / announcement

---

## 11. Benchmarking and Validation Strategy

Benchmarking covers three dimensions: raw library overhead, in-memory backend correctness, and comparative performance against alternatives. All benchmarks run in GitHub Actions CI with tiered frequency based on cost and runtime.

### 11.1 CI Pipeline Architecture (GitHub Actions)

The CI pipeline uses four workflow tiers triggered at different frequencies. All workflows use `go test -bench` and `go test -fuzz` with results stored as GitHub Actions artifacts for historical comparison.

| Tier | Trigger | Runner | DynamoDB Local | Estimated Duration |
|---|---|---|---|---|
| **Tier 1: Core** | Every push / PR | `ubuntu-latest` | No | ~2 minutes |
| **Tier 2: Conformance** | Every push / PR | `ubuntu-latest` | Yes (Docker service container) | ~5 minutes |
| **Tier 3: Comparative** | Weekly (scheduled) + pre-release tags | `ubuntu-latest` | No | ~10 minutes |
| **Tier 4: Integration** | Pre-release tags only | `ubuntu-latest` | Real DynamoDB (via OIDC role) | ~15 minutes |

**Tier 1 workflow (every push):**
```yaml
# .github/workflows/ci.yml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - run: go test ./... -race -count=1
      - run: go test ./... -bench=. -benchmem -run=^$ -count=3 | tee bench.txt
      - uses: actions/upload-artifact@v4
        with:
          name: benchmarks
          path: bench.txt
```

**Tier 2 workflow (conformance with DynamoDB Local):**
```yaml
# .github/workflows/conformance.yml
name: Conformance
on: [push, pull_request]
jobs:
  conformance:
    runs-on: ubuntu-latest
    services:
      dynamodb-local:
        image: amazon/dynamodb-local:latest
        ports:
          - 8000:8000
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - run: go test ./... -tags=conformance -race
        env:
          DYNAMODB_LOCAL_ENDPOINT: http://localhost:8000
```

**Tier 3 workflow (weekly comparative benchmarks):**
```yaml
# .github/workflows/benchmarks-weekly.yml
name: Comparative Benchmarks
on:
  schedule:
    - cron: '0 6 * * 1' # Monday 6am UTC
  push:
    tags: ['v*']
jobs:
  compare:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - run: go test ./benchmarks/... -bench=. -benchmem -count=5 -timeout=30m | tee compare.txt
      - uses: benchmark-action/github-action-benchmark@v1
        with:
          tool: 'go'
          output-file-path: compare.txt
          github-token: ${{ secrets.GITHUB_TOKEN }}
          auto-push: true
          alert-threshold: '120%'
          comment-on-alert: true
```

**Tier 4 workflow (pre-release integration against real DynamoDB):**
```yaml
# .github/workflows/integration.yml
name: Integration
on:
  push:
    tags: ['v*']
jobs:
  integration:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_INTEGRATION_ROLE_ARN }}
          aws-region: us-east-1
      - run: go test ./... -tags=integration -race -timeout=30m
```

### 11.2 Library Overhead Benchmarks (Tier 1 — every push)

These measure what DynaGo adds on top of the AWS SDK, isolated from network latency using a no-op backend.

#### 11.2.1 Encoding / Decoding

The codec is the hottest library path — every Put encodes, every Get decodes. Benchmark with `b.ReportAllocs()` across struct complexity levels.

```go
func BenchmarkEncode_Flat(b *testing.B) {
    user := User{PK: "USER#123", SK: "PROFILE", Name: "Alice", Email: "alice@example.com"}
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := dynago.Marshal(user)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkEncode_Nested(b *testing.B)    { /* struct with nested structs, slices, maps */ }
func BenchmarkEncode_Large(b *testing.B)     { /* struct with 30+ fields */ }
func BenchmarkEncode_WithSets(b *testing.B)  { /* string sets, number sets */ }
func BenchmarkDecode_Flat(b *testing.B)      { /* reverse of encode benchmarks */ }
func BenchmarkDecode_Nested(b *testing.B)
func BenchmarkDecode_Large(b *testing.B)

// Codec cache effectiveness: first call vs cached call
func BenchmarkEncode_ColdCache(b *testing.B) { /* new type each iteration */ }
func BenchmarkEncode_WarmCache(b *testing.B) { /* same type, codec already cached */ }
```

#### 11.2.2 Expression AST Construction and Translation

Every Query, Scan, and conditional Put builds an AST and translates it to a DynamoDB expression string.

```go
func BenchmarkExprBuild_SimpleFilter(b *testing.B) {
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        _ = dynago.Filter("Active = ? AND Age > ?", true, 21)
    }
}

func BenchmarkExprBuild_ComplexCondition(b *testing.B)  { /* 5-6 conditions ANDed with nested paths */ }
func BenchmarkExprTranslate_ToDynamo(b *testing.B)      { /* pre-built AST to DynamoDB expression string */ }
```

#### 11.2.3 Full Round-Trip (No-Op Backend)

Measures total library overhead per operation end-to-end, excluding network. Uses a no-op backend that returns a pre-built response immediately.

```go
type noopBackend struct {
    response *GetItemResponse
}

func (b *noopBackend) GetItem(ctx context.Context, req *GetItemRequest) (*GetItemResponse, error) {
    return b.response, nil
}

func BenchmarkGetRoundTrip(b *testing.B) {
    backend := &noopBackend{response: prebuiltUserResponse()}
    db := dynago.New(backend)
    table := db.Table("Bench")
    ctx := context.Background()

    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := dynago.Get[User](ctx, table, dynago.Key("PK", "USER#1", "SK", "PROFILE"))
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkPutRoundTrip(b *testing.B)   { /* same pattern */ }
func BenchmarkQueryRoundTrip(b *testing.B) { /* 100-item pre-built response */ }
```

#### 11.2.4 Key Construction

Validates that plain function key construction is near-zero cost.

```go
func BenchmarkKeyConstruction(b *testing.B) {
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        _ = dynago.Key("PK", UserPK("123"), "SK", OrderSK("2024-01-15", "abc"))
    }
}
```

### 11.3 In-Memory Backend Benchmarks (Tier 1 — every push)

#### 11.3.1 Expression Evaluation

The hot path in memdb — every filter, condition, and projection is evaluated against every scanned item.

```go
func BenchmarkMemDB_EvalSimpleFilter(b *testing.B) {
    node := buildFilter("Active = ?", true)
    item := map[string]AttributeValue{"Active": boolVal(true), "Name": strVal("Alice")}

    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = expr.Eval(node, item)
    }
}

func BenchmarkMemDB_EvalComplexFilter(b *testing.B)  { /* 5+ conditions */ }
func BenchmarkMemDB_EvalNestedPath(b *testing.B)      { /* Address.City = ? */ }
func BenchmarkMemDB_EvalFunctionCall(b *testing.B)    { /* begins_with, contains, size */ }
```

#### 11.3.2 Query and Scan at Scale

Benchmark memdb with realistic data volumes to ensure users' test suites stay fast.

```go
func BenchmarkMemDB_Query_100Items(b *testing.B)    { benchQuery(b, 100) }
func BenchmarkMemDB_Query_1000Items(b *testing.B)   { benchQuery(b, 1000) }
func BenchmarkMemDB_Query_10000Items(b *testing.B)  { benchQuery(b, 10000) }

func BenchmarkMemDB_ScanWithFilter_10000Items(b *testing.B) {
    // 10k items in table, filter matches 10%
}
```

**Performance target:** Query of 100 items from a 10k-item table should complete in under 1ms. A developer's test suite with 500 test cases using memdb should run in under 10 seconds.

#### 11.3.3 Concurrency Under Load

Validates that `sync.RWMutex` doesn't bottleneck under `t.Parallel()` usage.

```go
func BenchmarkMemDB_ConcurrentReads(b *testing.B) {
    mem := setupTableWithNItems(1000)
    ctx := context.Background()

    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _, _ = mem.GetItem(ctx, &GetItemRequest{
                TableName: "Bench",
                Key:       randomKey(),
            })
        }
    })
}

func BenchmarkMemDB_ConcurrentMixedReadWrite(b *testing.B) {
    // 80% reads, 20% writes — simulates realistic test workload
}
```

### 11.4 Conformance / Correctness Suite (Tier 2 — every push with DynamoDB Local)

A single test suite runs identical assertions against both the in-memory backend and DynamoDB Local. Every test must produce identical results on both backends.

#### 11.4.1 Dual-Target Test Runner

```go
// conformance_test.go — build tag: conformance

func backends(t *testing.T) []dynago.Backend {
    t.Helper()
    backends := []dynago.Backend{dynago.NewMemoryBackend()}

    if endpoint := os.Getenv("DYNAMODB_LOCAL_ENDPOINT"); endpoint != "" {
        cfg, _ := config.LoadDefaultConfig(context.Background(),
            config.WithEndpointResolver(...),
        )
        backends = append(backends, dynago.NewAWSBackend(cfg))
    }
    return backends
}

func TestConformance_PutGetRoundTrip(t *testing.T) {
    for _, backend := range backends(t) {
        t.Run(backendName(backend), func(t *testing.T) {
            // identical test logic runs against both backends
        })
    }
}
```

#### 11.4.2 Edge Case Coverage Matrix

The conformance suite targets known DynamoDB behavioural subtleties:

| Category | Test Cases |
|---|---|
| **Key types** | String keys, number keys, binary keys, empty string rejection |
| **Condition expressions** | `attribute_exists` on nested path, `attribute_not_exists` on missing vs null, `size()` on strings vs lists vs maps |
| **Update expressions** | SET nested path, ADD to number, ADD to set, REMOVE from list by index, DELETE from set |
| **Query edge cases** | Empty partition (0 results, no error), `SortBetween` with equal bounds, `Limit(1)` with pagination token |
| **Batch operations** | Exactly 25 items (max batch), 26 items (requires chunking), duplicate keys in batch, unprocessed items retry |
| **Transactions** | Exactly 100 items (max transaction), condition failure on item N of M, idempotency token behaviour |
| **Type marshaling** | `time.Time` as unixtime vs ISO, `[]byte` as binary, nil pointer vs zero value, empty string with `omitempty` |
| **Capacity reporting** | `ConsumedCapacity` returned with correct RCU/WCU for each operation type |

#### 11.4.3 Fuzz Testing (Nightly)

Uses Go's built-in fuzz testing to discover encoding/decoding round-trip bugs and expression evaluation edge cases. Runs nightly on a scheduled GitHub Actions workflow with a 30-minute fuzz duration.

```go
func FuzzEncodeDecodeRoundTrip(f *testing.F) {
    f.Add("hello", int64(42), true)
    f.Fuzz(func(t *testing.T, s string, n int64, b bool) {
        item := SimpleStruct{Name: s, Count: n, Active: b}
        encoded, err := dynago.Marshal(item)
        if err != nil {
            t.Skip()
        }
        var decoded SimpleStruct
        err = dynago.Unmarshal(encoded, &decoded)
        if err != nil {
            t.Fatalf("encode succeeded but decode failed: %v", err)
        }
        if decoded != item {
            t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded, item)
        }
    })
}

func FuzzExpressionEval(f *testing.F) {
    // Fuzz the AST evaluator with random attribute values
    // to surface panics, nil pointer dereferences, and logic errors
}
```

```yaml
# .github/workflows/fuzz.yml
name: Fuzz
on:
  schedule:
    - cron: '0 2 * * *' # 2am UTC nightly
jobs:
  fuzz:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - run: go test ./... -fuzz=Fuzz -fuzztime=30m
```

### 11.5 Comparative Benchmarks (Tier 3 — weekly + pre-release)

Identical operations run through DynaGo, guregu/dynamo, and the raw AWS SDK to answer "is DynaGo faster, slower, or the same?"

#### 11.5.1 Benchmark Matrix

```go
func BenchmarkComparison(b *testing.B) {
    structs := []struct {
        name   string
        fields int
    }{
        {"Small_5fields", 5},
        {"Medium_15fields", 15},
        {"Large_30fields", 30},
        {"Nested_3levels", 15},
        {"WithSets", 10},
    }

    for _, s := range structs {
        b.Run(s.name+"/DynaGo_Encode", func(b *testing.B) { /* dynago.Marshal */ })
        b.Run(s.name+"/Guregu_Encode", func(b *testing.B) { /* dynamo.MarshalItem */ })
        b.Run(s.name+"/AWSSDK_Encode", func(b *testing.B) { /* attributevalue.MarshalMap */ })
        b.Run(s.name+"/DynaGo_Decode", func(b *testing.B) { /* dynago.Unmarshal */ })
        b.Run(s.name+"/Guregu_Decode", func(b *testing.B) { /* dynamo.UnmarshalItem */ })
        b.Run(s.name+"/AWSSDK_Decode", func(b *testing.B) { /* attributevalue.UnmarshalMap */ })
    }
}
```

#### 11.5.2 Reporting

All comparative benchmarks report three columns: **ns/op**, **bytes/op**, **allocs/op**. Results are tracked over time using `benchmark-action/github-action-benchmark` and published to GitHub Pages. The CI workflow comments on PRs when any benchmark regresses by more than 20%.

**Performance target:** DynaGo must be within 10% of guregu/dynamo ns/op for equivalent operations, and must not exceed 2x the allocs/op of the raw AWS SDK encoder.

### 11.6 Integration Benchmarks (Tier 4 — pre-release only)

These run against real DynamoDB via an OIDC-authenticated AWS role to validate production-like behaviour.

#### 11.6.1 Latency Distribution

Measures p50, p95, p99 latency per operation type and compares against raw AWS SDK doing the same operations.

```go
func TestIntegration_LatencyDistribution(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    // Run 1000 Gets, record latency histogram
    // Compare DynaGo vs raw AWS SDK for the same 1000 Gets
    // Report p50, p95, p99 delta
}
```

**Performance target:** DynaGo p99 latency must not exceed 5% above raw AWS SDK p99 for the same operation.

#### 11.6.2 Batch and Transaction Throughput

Measures items/second for batch writes across concurrency levels.

```go
func BenchmarkIntegration_BatchPut(b *testing.B) {
    concurrencyLevels := []int{1, 2, 4, 8}
    itemCounts := []int{100, 500, 1000, 5000}

    for _, c := range concurrencyLevels {
        for _, n := range itemCounts {
            b.Run(fmt.Sprintf("concurrency_%d/items_%d", c, n), func(b *testing.B) {
                // measure items/second throughput
            })
        }
    }
}
```

#### 11.6.3 Observability Overhead

Measures the cost of OpenTelemetry tracing and metrics when enabled vs disabled.

```go
func BenchmarkOtel_Disabled(b *testing.B)      { /* no tracer/meter configured */ }
func BenchmarkOtel_NoopProvider(b *testing.B)   { /* noop tracer/meter */ }
func BenchmarkOtel_RealProvider(b *testing.B)   { /* actual otel SDK with in-memory exporter */ }
```

**Performance target:** Otel with a noop provider must add less than 2% overhead. Otel with a real provider must add less than 5%.

### 11.7 Benchmark Summary

| Category | What It Validates | CI Tier | Frequency | Performance Target |
|---|---|---|---|---|
| Encoding / decoding | Codec performance, allocs, cache effectiveness | Tier 1 | Every push | Within 10% of guregu ns/op |
| Expression build + translate | AST construction overhead | Tier 1 | Every push | < 500ns for simple filter |
| Full round-trip (noop backend) | Total per-operation library cost | Tier 1 | Every push | < 1µs for Get |
| Key construction | Key helper overhead | Tier 1 | Every push | Near-zero (< 50ns) |
| MemDB query at scale | Test suite speed for users | Tier 1 | Every push | 100-item query < 1ms |
| MemDB concurrency | `t.Parallel()` safety | Tier 1 | Every push | No degradation at 8 goroutines |
| Conformance (dual-target) | memdb correctness vs DynamoDB Local | Tier 2 | Every push | 100% pass rate on both backends |
| Fuzz testing | Edge case discovery | Nightly | Nightly | No panics or round-trip failures |
| Comparative (vs guregu, AWS SDK) | Competitive positioning | Tier 3 | Weekly + pre-release | < 10% overhead, < 2x allocs |
| Integration latency | Production readiness | Tier 4 | Pre-release | p99 within 5% of raw SDK |
| Batch/tx throughput | Chunking and concurrency | Tier 4 | Pre-release | Linear scaling to 4 goroutines |
| Otel overhead | Observability cost | Tier 4 | Pre-release | < 2% noop, < 5% real |

---

## 12. Success Metrics

| Metric | Target (6 months post-launch) | Target (12 months) |
|---|---|---|
| GitHub stars | 200 | 1,000 |
| Weekly downloads (pkg.go.dev proxy) | 500 | 2,500 |
| Contributors | 3 | 10 |
| Open issues response time | < 48 hours | < 48 hours |
| Test coverage | > 85% | > 90% |
| Benchmark regressions in CI | 0 unresolved | 0 unresolved |
| Conformance suite pass rate (memdb vs DynamoDB Local) | 100% | 100% |
| Known correctness bugs | 0 critical | 0 critical |

---

## 13. Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
|---|---|---|---|
| In-memory backend has subtle behavioural differences vs real DynamoDB | Users write tests that pass locally but fail in production | High | Maintain a conformance test suite that runs against both the in-memory backend and DynamoDB Local. Document known limitations explicitly. |
| Generics API design doesn't feel idiomatic | Low adoption, API churn | Medium | Study established generics-first Go libraries (e.g., `samber/lo`, `hashicorp/go-set`). Get API feedback from Go community before v1.0. Use methods where possible; free functions only where generics require them. |
| Single-table design abstractions are too opinionated | Alienates users with different data modeling preferences | Medium | Keep single-table features opt-in (registry is optional). Ensure the library works well for simple one-entity-per-table use cases without any registry setup. |
| AWS SDK v2 introduces breaking changes | Library stops compiling | Low | Library-owned backend types isolate SDK changes to the adapter. Pin AWS SDK dependencies. Monitor AWS SDK releases. |
| Scope creep delays initial release | Library never ships | Medium | Phase 1 is deliberately minimal. Ship a usable library with just typed CRUD before tackling advanced features. |
| guregu/dynamo adopts generics in v3 | Eliminates primary differentiator | Low | Single-table design, in-memory backend, and observability are durable differentiators that won't be easily replicated. |
| Backend interface abstraction adds overhead | Performance regression vs direct SDK calls | Low | Benchmark the adapter translation layer. The cost of one struct copy per operation is negligible compared to network latency. |

---

## 14. Resolved Design Decisions

These questions were open in v1.0 and are now resolved:

| Question | Decision | Rationale |
|---|---|---|
| Expression syntax | `?` for values, `#name` for reserved-word attributes | Familiar from SQL and guregu/dynamo. Low learning curve. |
| Key template syntax | Plain Go functions | Zero-allocation, compile-time checked, no runtime parsing errors. Templates are overengineered for string concatenation. |
| Entity registry scope | Per-table (via `WithRegistry` option) | Most flexible. Different tables may have different entity sets. The registry is wired once at table creation and immutable thereafter. |
| In-memory backend concurrency | `sync.RWMutex` with read/write lock separation | Required for `t.Parallel()` support. Not optional. |
| Module path | `github.com/<org>/dynago` | Use GitHub path. No vanity path until adoption warrants it. Verify no conflicts on pkg.go.dev before publishing. |
| Go version requirement | 1.23+ | Generics support required. Go 1.23 includes `range over func` which enables idiomatic iterator patterns. |

---

## 15. Appendix: Competitive Landscape

| Feature | AWS SDK v2 | guregu/dynamo v2 | DynaGo (proposed) |
|---|---|---|---|
| Type-safe operations | No | No (`interface{}`) | Yes (generics) |
| Expression helpers | Basic (builder) | Yes (placeholder syntax) | Yes (placeholder syntax + internal AST) |
| Single-table design | No | No | Yes (first-class) |
| Polymorphic queries | No | No | Yes (entity registry) |
| Key composition | No | No | Yes (function helpers) |
| In-memory testing | No | No | Yes |
| Mock interface | No | Yes (15+ methods) | Yes (10 methods, library-owned types) |
| Test assertions | No | No | Yes |
| OpenTelemetry | No | No | Yes |
| Struct tag encoding | `dynamodbav` | `dynamo` | `dynamo` |
| Custom marshaler support | Yes | Yes | Yes |
| AWS SDK compatibility | Native | Yes (`AWSEncoding`) | Yes (adapter pattern) |
| Batch auto-chunking | No | Yes | Yes |
| Transaction support | Native | Yes | Yes (enhanced) |
| Parallel scan | No | Yes | Yes |
| Go version requirement | 1.20+ | 1.23+ | 1.23+ |
