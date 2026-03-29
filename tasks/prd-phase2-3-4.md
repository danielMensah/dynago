# PRD: Phases 2, 3 & 4 — Single-Table Design, Testing Backend, Transactions & Observability

## Introduction

This PRD covers the execution plan for DynaGo Phases 2 through 4, building on the foundation established in Phase 1 (core types, Backend interface, encoding/decoding, expression AST, CRUD operations). These three phases have real dependencies between them: the Entity interface from Phase 2 shapes what the memdb backend in Phase 3 must support, and transactions in Phase 4 need memdb support from Phase 3. This single document makes those cross-phase dependencies explicit.

By the end of Phase 4, DynaGo is feature-complete for a v0.1 release: type-safe single-table design with polymorphic queries, an in-memory testing backend with conformance validation against DynamoDB Local, transactions, batch operations, and a pluggable instrumentation middleware.

## Goals

### Phase 2: Single-Table Design
- Define the `Entity` interface and `EntityInfo` type as a compile-time contract for polymorphic types
- Implement a `Registry` that maps discriminator values to Go types for unmarshaling
- Wire polymorphic unmarshaling into the decode pipeline so collection queries return typed results
- Provide `SplitKey` utility for composite key parsing (defer `ParseKey` to post-launch unless users request it)
- Support heterogeneous collection queries and typed iterators over mixed-entity result sets
- Enable GSI queries through the existing `Query[T]` API with an `Index` option
- Auto-set discriminator attributes on `Put` for registered entity types

### Phase 3: Testing Backend
- Implement an in-memory backend (`memdb` package) for core operations: GetItem, PutItem, DeleteItem, UpdateItem, Query, Scan
- Evaluate expressions from the internal AST directly (no DynamoDB expression string parsing)
- Enforce table schema (hash/range keys, GSIs) with proper key validation
- Support `sync.RWMutex` concurrency for safe `t.Parallel()` usage
- Build a test helper package (`dynagotest`) with assertion and seed utilities
- Establish Tier 1 memdb benchmarks validating the 100-item query < 1ms target
- Build a Tier 2 conformance test suite running identical assertions against memdb and DynamoDB Local

### Phase 4: Transactions, Batches & Observability
- Implement transaction builders (read and write transactions) with structured error decomposition
- Build `ReadModifyWrite` with optimistic locking
- Implement batch get/write with automatic chunking and progress callbacks
- Extend memdb to support BatchGetItem, BatchWriteItem, TransactGetItems, TransactWriteItems
- Build a pluggable instrumentation middleware (Backend-wrapping decorator) that any observability provider can plug into
- Ship a `dynagotel` module as a reference OpenTelemetry implementation (separate `go.mod` to isolate otel dependencies)
- Establish Tier 3 comparative benchmarks vs guregu/dynamo and raw AWS SDK

## User Stories

---

### Phase 2: Single-Table Design

---

### US-200: Entity Interface and EntityInfo Type

**Description:** As a developer using single-table design, I need a compile-time contract that ensures every entity type declares its discriminator value, so that missing discriminators are caught at compile time rather than runtime.

**Acceptance Criteria:**
- [ ] `Entity` interface defined: `DynagoEntity() EntityInfo`
- [ ] `EntityInfo` struct defined with `Discriminator string` field
- [ ] Interface is exported from the `dynago` package
- [ ] A type that does not implement `DynagoEntity()` cannot be passed where `Entity` is required (compile error)
- [ ] Unit test: a concrete type implementing `Entity` returns correct `EntityInfo`
- [ ] Typecheck and lint pass

---

### US-201: Registry Implementation

**Description:** As a developer, I need a registry that maps discriminator values to Go types so the library can resolve the correct type during polymorphic unmarshaling.

**Acceptance Criteria:**
- [ ] `NewRegistry(discriminatorAttr string) *Registry` constructor
- [ ] `Registry.Register(e Entity)` method — accepts any type implementing `Entity`, stores the mapping from discriminator string to `reflect.Type`
- [ ] `Registry.Register` panics if two types register the same discriminator value (programmer error)
- [ ] `Registry.Lookup(discriminator string) (reflect.Type, bool)` returns the registered type
- [ ] `Registry.DiscriminatorAttr() string` returns the attribute name used for discrimination
- [ ] Registry is safe for concurrent reads after initialization (no writes after setup)
- [ ] Unit tests: register multiple types, lookup by discriminator, panic on duplicate, lookup miss returns false
- [ ] Typecheck and lint pass

---

### US-202: Table-Level Registry Binding

**Description:** As a developer, I need to bind a registry to a table so that polymorphic operations know which discriminator attribute and type mappings to use.

**Acceptance Criteria:**
- [ ] `WithRegistry(r *Registry) TableOption` functional option
- [ ] `Table` stores an optional `*Registry` reference
- [ ] `Table.Registry() *Registry` accessor (returns nil if no registry bound)
- [ ] Creating a table without `WithRegistry` works as before (no registry, no polymorphism)
- [ ] Unit test: create table with and without registry, verify accessor
- [ ] Typecheck and lint pass

---

### US-203: Polymorphic Unmarshaling Integration

**Description:** As a developer, I need the decode pipeline to use the registry's discriminator attribute to resolve the correct Go type when unmarshaling items from collection queries.

**Acceptance Criteria:**
- [ ] `unmarshalPolymorphic(item map[string]AttributeValue, registry *Registry) (any, error)` function
- [ ] Reads the discriminator attribute from the item, looks up the type in the registry, unmarshals into that type
- [ ] Returns a descriptive error if the discriminator attribute is missing from the item
- [ ] Returns a descriptive error if the discriminator value is not registered
- [ ] Works with all existing field types (strings, numbers, bools, time.Time, slices, maps, nested structs)
- [ ] Unit tests: unmarshal User vs Order based on discriminator, missing discriminator error, unknown discriminator error
- [ ] Typecheck and lint pass

---

### US-204: Polymorphic Put (Auto-Set Discriminator)

**Description:** As a developer, I want `table.Put` to automatically set the discriminator attribute when the table has a registry and the item implements `Entity`, so I don't have to manually add the discriminator field to every struct.

**Acceptance Criteria:**
- [ ] When `table.Put` is called with a value implementing `Entity` and the table has a registry, the discriminator attribute is automatically added to the marshaled item
- [ ] The discriminator value comes from calling `DynagoEntity().Discriminator` on the item
- [ ] If the item already has the discriminator attribute set in the struct, the auto-set value takes precedence (ensures consistency)
- [ ] If the table has no registry, `Put` behaves exactly as before (no discriminator logic)
- [ ] If the item does not implement `Entity` but the table has a registry, `Put` proceeds without setting a discriminator (supports mixed usage)
- [ ] Unit tests: put with auto-discriminator, put without registry, put with non-Entity type on registry table
- [ ] Typecheck and lint pass

---

### US-205: SplitKey Utility

**Description:** As a developer working with composite keys, I need a utility to split delimited key strings into components so I can extract semantic values from keys like `ORDER#2024-01-15#abc`.

**Design decision:** Ship `SplitKey` only. `ParseKey` (named component mapping) is deferred — most key parsing in single-table designs is one-off splitting in application code. A smaller API surface in Phase 2 is safer; `ParseKey` can be added later based on real user demand.

**Acceptance Criteria:**
- [ ] `SplitKey(key string, delimiter string) []string` — splits a composite key by the given delimiter
- [ ] Delimiter is a parameter (not hardcoded to `#`) — teams use `#`, `|`, `::`, etc.
- [ ] Zero-allocation (uses `strings.Split` which is already optimized)
- [ ] Unit tests: split `#`-delimited key, split `|`-delimited key, split with no delimiter match returns single-element slice, empty string input
- [ ] Typecheck and lint pass

---

### US-206: Heterogeneous Collection Query

**Description:** As a developer using single-table design, I need to query a partition and receive results grouped by entity type, so I can access typed slices of Users, Orders, etc. from a single query.

**Acceptance Criteria:**
- [ ] `Collection` type that holds a list of polymorphically unmarshaled items
- [ ] `QueryCollection(ctx, table, key KeyCondition, opts ...QueryOption) (*Collection, error)` free function
- [ ] Uses the table's registry to unmarshal each item into its correct Go type
- [ ] Returns an error if the table has no registry
- [ ] `ItemsOf[T](collection *Collection) []T` generic function that filters and returns items of type T
- [ ] Items with unrecognized discriminators are silently skipped (not an error — allows schema evolution)
- [ ] Unit tests: query partition with mixed User/Order items, extract typed slices, empty partition returns empty collection
- [ ] Typecheck and lint pass

---

### US-207: Heterogeneous Collection Iterator (range over func)

**Description:** As a developer, I need an iterator over heterogeneous collection results so I can process mixed-type items one at a time using a type switch, especially for large result sets.

**Design decision:** Use `iter.Seq2[any, error]` (Go 1.23 `range over func`) for collection iteration. This is the same pattern used for all iterators in the library — Phase 1's `QueryIter` and `ScanIter` are also migrated to `range over func` in US-209. A non-nil error signals both the problem and the end of iteration (no separate `.Err()` check needed).

**Acceptance Criteria:**
- [ ] `CollectionIter(ctx, table, key KeyCondition, opts ...QueryOption) iter.Seq2[any, error]` free function
- [ ] Yields `(value any, err error)` pairs — each value is the concrete registered type
- [ ] Usable with `for item, err := range CollectionIter(...)` syntax
- [ ] A non-nil error is yielded as the final value; iteration stops after error
- [ ] Supports pagination (fetches next page automatically when current page exhausted)
- [ ] Items with unrecognized discriminators are skipped during iteration
- [ ] Unit tests: iterate mixed items with type switch, pagination across pages, error yields correctly, range-for usage
- [ ] Typecheck and lint pass

---

### US-208: GSI Query Support

**Description:** As a developer, I need to query Global Secondary Indexes through the existing `Query[T]` API so I can use GSI overloading patterns for single-table design.

**Acceptance Criteria:**
- [ ] `Index(name string) QueryOption` functional option
- [ ] When `Index` is set, the `QueryRequest` includes the `IndexName` field
- [ ] Works with both homogeneous `Query[T]` and heterogeneous `QueryCollection`
- [ ] The key condition attributes can differ from the table's primary key (GSI key attributes)
- [ ] `Index` also works as a `ScanOption` for GSI scans
- [ ] Unit tests: query GSI with typed results, query GSI with collection, scan GSI
- [ ] AWS adapter passes `IndexName` through to the SDK correctly
- [ ] Typecheck and lint pass

---

### US-209: Migrate Iterators to range over func

**Description:** As a developer, I want all iterators in the library to use Go 1.23's `range over func` pattern so there is one consistent iteration idiom across the entire API.

**Design decision:** The PRD committed to Go 1.23+ specifically for `range over func`. Phase 1's `QueryIter` and `ScanIter` used an explicit `.Next()` pattern in design sketches. This story unifies all iterators to `iter.Seq2[T, error]` before Phase 3 builds on them. A non-nil error is yielded as the final pair; no separate `.Err()` method is needed.

**Acceptance Criteria:**
- [ ] `QueryIter[T](ctx, table, key KeyCondition, opts ...QueryOption) iter.Seq2[T, error]` — returns a range-able iterator
- [ ] `ScanIter[T](ctx, table, opts ...ScanOption) iter.Seq2[T, error]` — returns a range-able iterator
- [ ] Both support automatic pagination (fetch next page when current exhausted)
- [ ] Usable with `for item, err := range QueryIter[T](...)` syntax
- [ ] Error on final yield signals end of iteration; breaking out of the loop early is safe (no resource leak)
- [ ] `Query[T]` and `Scan[T]` convenience functions internally consume the iterator (collect all)
- [ ] Remove old `.Next()` / `.Err()` iterator types if they exist
- [ ] Update all existing iterator tests to use range-for pattern
- [ ] Unit tests: range-for iteration, early break, pagination, error propagation
- [ ] Typecheck and lint pass

---

### Phase 3: Testing Backend

---

### US-300: MemDB Core Structure and Table Management

**Description:** As a developer writing tests, I need an in-memory DynamoDB backend that can create tables with key schema so I can run tests without Docker or network.

**Acceptance Criteria:**
- [ ] `memdb` package under the project (not `internal` — users import it directly)
- [ ] `NewMemoryBackend() *MemoryBackend` constructor
- [ ] `MemoryBackend` implements the `dynago.Backend` interface (stub/error for unimplemented methods initially)
- [ ] `CreateTable(name string, schema dynago.TableSchema)` method for test setup
- [ ] `TableSchema` struct with `HashKey`, `RangeKey` (optional), and `GSIs` slice
- [ ] `KeyDef` struct with `Name string` and `Type KeyType` (String, Number, Binary)
- [ ] `GSISchema` struct with `Name`, `HashKey`, `RangeKey` (optional)
- [ ] Items stored as `map[string]dynago.AttributeValue` internally, indexed by hash key (and range key if present)
- [ ] `CreateTable` panics if table already exists (test setup error)
- [ ] Concurrency: `sync.RWMutex` per table for safe `t.Parallel()` usage
- [ ] Unit tests: create table, create table with range key, create table with GSI, panic on duplicate
- [ ] Typecheck and lint pass

---

### US-301: MemDB GetItem and PutItem

**Description:** As a developer writing tests, I need GetItem and PutItem to work in the in-memory backend so I can test basic CRUD operations.

**Acceptance Criteria:**
- [ ] `PutItem` stores items indexed by their hash key (and range key if table has one)
- [ ] `PutItem` validates that the item contains the required hash key (and range key if defined)
- [ ] `PutItem` returns an error if required key attributes are missing or wrong type
- [ ] `PutItem` supports condition expressions — evaluates the AST from the request against the existing item (if any)
- [ ] `PutItem` returns `ConditionalCheckFailedException`-equivalent error when condition fails
- [ ] `GetItem` returns the item matching the key, or an empty response (not error) if not found
- [ ] `GetItem` supports projection expressions — evaluates the AST to return only requested attributes
- [ ] `GetItem` supports `ConsistentRead` (accepted but no behavioral difference in memdb)
- [ ] Items are stored by value (mutations to the returned item don't affect stored data)
- [ ] Write operations acquire write lock; read operations acquire read lock
- [ ] Unit tests: put and get round-trip, put with condition (success and failure), get with projection, get missing item, concurrent put/get
- [ ] Typecheck and lint pass

---

### US-302: MemDB DeleteItem

**Description:** As a developer writing tests, I need DeleteItem in the in-memory backend so I can test delete operations with condition expressions.

**Acceptance Criteria:**
- [ ] `DeleteItem` removes the item matching the key
- [ ] `DeleteItem` supports condition expressions (evaluated before deletion)
- [ ] `DeleteItem` returns the old item if `ReturnValues` is set to ALL_OLD
- [ ] `DeleteItem` is a no-op (not an error) if the item doesn't exist (matches DynamoDB behavior)
- [ ] `DeleteItem` with a condition on a non-existent item fails the condition check (matches DynamoDB)
- [ ] Unit tests: delete existing item, delete with condition, delete non-existent item, delete with return old values
- [ ] Typecheck and lint pass

---

### US-303: MemDB UpdateItem

**Description:** As a developer writing tests, I need UpdateItem in the in-memory backend so I can test update expressions (SET, ADD, REMOVE, DELETE) against in-memory data.

**Acceptance Criteria:**
- [ ] `UpdateItem` applies update expression AST nodes to the stored item
- [ ] Supports SET (overwrite attribute), ADD (increment number or add to set), REMOVE (delete attribute), DELETE (remove elements from set)
- [ ] Supports condition expressions evaluated before the update is applied
- [ ] Creates the item if it doesn't exist (upsert behavior, matching DynamoDB)
- [ ] Returns the updated item if `ReturnValues` is set to ALL_NEW
- [ ] Returns the old item if `ReturnValues` is set to ALL_OLD
- [ ] Validates that required key attributes are present
- [ ] Uses the expression evaluator from `internal/expr` (specifically `EvalUpdate`)
- [ ] Unit tests: SET field, ADD to number, ADD to set, REMOVE field, DELETE from set, upsert, condition check failure, return values
- [ ] Typecheck and lint pass

---

### US-304: MemDB Query

**Description:** As a developer writing tests, I need Query in the in-memory backend so I can test partition queries with sort key conditions and filter expressions.

**Acceptance Criteria:**
- [ ] Query scans the table (or GSI) for items matching the partition key
- [ ] Applies sort key condition: equals, begins_with, between, less_than, greater_than, less_than_or_equal, greater_than_or_equal
- [ ] Applies filter expression AST to post-filter results
- [ ] Applies projection expression to limit returned attributes
- [ ] Supports `ScanIndexForward` (ascending/descending sort)
- [ ] Supports `Limit` — returns at most N items (after key condition, before or after filter depending on DynamoDB semantics: limit applies to items read, not items returned)
- [ ] Returns `LastEvaluatedKey` when limit is reached (pagination support)
- [ ] Accepts `ExclusiveStartKey` for pagination continuation
- [ ] When querying a GSI, reads from the GSI's projected items (for now, full projection — all attributes copied to GSI)
- [ ] Returns `ScannedCount` and `Count` in response
- [ ] Unit tests: basic partition query, sort key conditions (each type), filter expression, pagination with limit, descending order, GSI query, empty results
- [ ] Typecheck and lint pass

---

### US-305: MemDB Scan

**Description:** As a developer writing tests, I need Scan in the in-memory backend so I can test full-table scans with filter expressions.

**Acceptance Criteria:**
- [ ] Scan iterates all items in the table (or GSI)
- [ ] Applies filter expression AST to each item
- [ ] Applies projection expression to limit returned attributes
- [ ] Supports `Limit` and pagination (`LastEvaluatedKey` / `ExclusiveStartKey`)
- [ ] Supports `IndexName` for GSI scans
- [ ] Returns `ScannedCount` and `Count`
- [ ] Unit tests: full scan, scan with filter, scan with limit and pagination, GSI scan
- [ ] Typecheck and lint pass

---

### US-306: MemDB GSI Maintenance

**Description:** As a developer writing tests, I need GSI indexes to be automatically maintained when items are written, so that GSI queries return correct results.

**Acceptance Criteria:**
- [ ] When an item is put, updated, or deleted, all GSI indexes on the table are updated
- [ ] GSI entries are keyed by the GSI's hash key (and range key if defined)
- [ ] Items missing the GSI hash key attribute are not indexed in that GSI (matches DynamoDB)
- [ ] GSI items store all attributes (full projection for simplicity)
- [ ] Unit tests: put item and query via GSI, update item and verify GSI reflects change, delete item and verify GSI entry removed, item missing GSI key not in index
- [ ] Typecheck and lint pass

---

### US-307: Test Helper Package — Seed Utilities

**Description:** As a developer writing tests, I need seed utilities to quickly populate the in-memory backend with test data so I can focus on testing behavior rather than setup.

**Acceptance Criteria:**
- [ ] `dynagotest` package (separate sub-package, importable as `dynago/dynagotest`)
- [ ] `Seed(ctx, table *dynago.Table, items []any) error` — marshals and puts each item
- [ ] `SeedFromJSON(ctx, table *dynago.Table, path string) error` — reads a JSON file in DynamoDB JSON format and seeds items
- [ ] DynamoDB JSON format only (with type descriptors: `{"S": "value"}`, `{"N": "42"}`, `{"SS": ["a","b"]}`, etc.) — no flat JSON with type inference (deferred; add later if users request it)
- [ ] File format: JSON array of objects, each object uses DynamoDB's AttributeValue type descriptors
- [ ] Rationale: DynamoDB JSON is unambiguous (distinguishes string sets from lists, numbers from number-strings) and compatible with `aws dynamodb get-item` output for easy fixture creation
- [ ] `Seed` returns an error if any put fails (wraps the first error with item index context)
- [ ] Unit tests: seed from slice, seed from JSON file, seed with invalid item
- [ ] Typecheck and lint pass

---

### US-308: Test Helper Package — Assertions

**Description:** As a developer writing tests, I need assertion helpers that check item state in the backend so I can write expressive, readable test assertions.

**Acceptance Criteria:**
- [ ] `AssertItemExists(t testing.TB, table *dynago.Table, key dynago.KeyValue, opts ...AssertOption)` — fails the test if item doesn't exist
- [ ] `AssertItemNotExists(t testing.TB, table *dynago.Table, key dynago.KeyValue)` — fails the test if item exists
- [ ] `HasAttribute(name string, expected any) AssertOption` — asserts a specific attribute value
- [ ] `AssertCount(t testing.TB, table *dynago.Table, key dynago.KeyCondition, expected int)` — asserts the number of items matching a query
- [ ] All assertions use `t.Helper()` so failure messages point to the caller
- [ ] All assertions produce clear failure messages including expected vs actual values
- [ ] Unit tests for each assertion function (positive and negative cases)
- [ ] Typecheck and lint pass

---

### US-309: Tier 1 MemDB Benchmarks

**Description:** As a library maintainer, I need benchmarks for the in-memory backend to validate that memdb query performance meets the < 1ms target for 100 items and to catch regressions.

**Acceptance Criteria:**
- [ ] `BenchmarkMemDB_EvalSimpleFilter` — single equality check against an item
- [ ] `BenchmarkMemDB_EvalComplexFilter` — 5+ conditions with AND/OR
- [ ] `BenchmarkMemDB_EvalNestedPath` — path traversal (e.g., `Address.City`)
- [ ] `BenchmarkMemDB_EvalFunctionCall` — `begins_with`, `contains`, `size`
- [ ] `BenchmarkMemDB_Query_100Items` — query returning 100 items from a 1000-item table
- [ ] `BenchmarkMemDB_Query_1000Items` — query returning 1000 items
- [ ] `BenchmarkMemDB_ScanWithFilter_10000Items` — 10k items, filter matches 10%
- [ ] `BenchmarkMemDB_ConcurrentReads` — parallel GetItem on shared table
- [ ] `BenchmarkMemDB_ConcurrentMixedReadWrite` — 80% reads, 20% writes
- [ ] All benchmarks use `b.ReportAllocs()`
- [ ] 100-item query benchmark completes in under 1ms per operation
- [ ] Typecheck and lint pass

---

### US-310: Tier 2 Conformance Test Suite

**Description:** As a library maintainer, I need a conformance test suite that runs identical assertions against both the in-memory backend and DynamoDB Local, ensuring memdb correctly simulates DynamoDB behavior.

**Acceptance Criteria:**
- [ ] Tests gated behind `//go:build conformance` build tag
- [ ] `backends(t)` helper returns memdb always, plus AWS backend when `DYNAMODB_LOCAL_ENDPOINT` is set
- [ ] Each test runs against all available backends via `t.Run(backendName, ...)`
- [ ] Conformance tests cover: put/get round-trip, condition expressions (attribute_exists, attribute_not_exists), update expressions (SET, ADD, REMOVE, DELETE), query with sort key conditions (equals, begins_with, between), query with filter, query pagination, scan with filter, scan pagination, delete with condition, type marshaling (strings, numbers, bools, time.Time, slices, maps, nested structs, sets)
- [ ] Empty partition query returns empty slice (not error)
- [ ] `SortBetween` with equal bounds returns items with that exact sort key
- [ ] `Limit(1)` returns pagination token
- [ ] All conformance tests pass against memdb
- [ ] Typecheck and lint pass

---

### US-311: Conformance CI Workflow

**Description:** As a library maintainer, I need a CI workflow that runs the conformance suite against DynamoDB Local on every push so regressions are caught automatically.

**Acceptance Criteria:**
- [ ] `.github/workflows/conformance.yml` workflow file
- [ ] Triggers on push and pull_request
- [ ] Uses `amazon/dynamodb-local:latest` as a Docker service container on port 8000
- [ ] Sets `DYNAMODB_LOCAL_ENDPOINT=http://localhost:8000` environment variable
- [ ] Runs `go test ./... -tags=conformance -race`
- [ ] Uses Go 1.23
- [ ] Typecheck and lint pass (if workflow includes a lint step)

---

### US-312: Fuzz Testing for Encoding and Expression Evaluation

**Description:** As a library maintainer, I need fuzz tests to discover edge cases in encoding round-trips and expression evaluation that unit tests miss.

**Acceptance Criteria:**
- [ ] `FuzzEncodeDecodeRoundTrip` — fuzzes string, int64, bool fields through marshal/unmarshal and asserts equality
- [ ] `FuzzExpressionEval` — fuzzes attribute values through expression evaluator and asserts no panics
- [ ] Fuzz tests are in appropriate `_test.go` files
- [ ] Fuzz tests run successfully with `go test -fuzz=Fuzz -fuzztime=10s` (short sanity check)
- [ ] `.github/workflows/fuzz.yml` nightly workflow running `go test ./... -fuzz=Fuzz -fuzztime=30m`
- [ ] Typecheck and lint pass

---

### Phase 4: Transactions, Batches & Observability

---

### US-400: Write Transaction Builder

**Description:** As a developer, I need a fluent transaction builder for write transactions so I can compose multi-item atomic operations (put, update, delete, condition check) in a type-safe way.

**Acceptance Criteria:**
- [ ] `WriteTx(ctx context.Context, db *DB) *WriteTxBuilder` constructor
- [ ] `WriteTxBuilder.Put(table *Table, item any, opts ...PutOption) *WriteTxBuilder` — adds a put operation
- [ ] `WriteTxBuilder.Update(table *Table, key KeyValue, opts ...UpdateOption) *WriteTxBuilder` — adds an update operation
- [ ] `WriteTxBuilder.Delete(table *Table, key KeyValue, opts ...DeleteOption) *WriteTxBuilder` — adds a delete operation
- [ ] `WriteTxBuilder.Check(table *Table, key KeyValue, condition ConditionOption) *WriteTxBuilder` — adds a condition check
- [ ] `WriteTxBuilder.Run() error` — executes the transaction via `Backend.TransactWriteItems`
- [ ] Maximum 100 operations per transaction (return error if exceeded before calling backend)
- [ ] Each operation builds its own expression AST (conditions, updates) independently
- [ ] Unit tests: build a transaction with mixed operations, exceed 100 ops error, empty transaction
- [ ] Typecheck and lint pass

---

### US-401: Read Transaction Builder

**Description:** As a developer, I need a read transaction builder for consistent reads across multiple items so I can get a point-in-time snapshot of several items atomically.

**Acceptance Criteria:**
- [ ] `ReadTx(ctx context.Context, db *DB) *ReadTxBuilder` constructor
- [ ] `ReadTxBuilder.Get(table *Table, key KeyValue, opts ...GetOption) *ReadTxBuilder` — adds a get operation
- [ ] `ReadTxBuilder.Run() (*ReadTxResult, error)` — executes via `Backend.TransactGetItems`
- [ ] `ReadTxResult.Item(index int) (map[string]AttributeValue, bool)` — returns the item at position index
- [ ] `GetAs[T](result *ReadTxResult, index int) (T, error)` — generic helper to unmarshal item at index into type T
- [ ] Maximum 100 operations per transaction
- [ ] Unit tests: read transaction with multiple gets, unmarshal results, empty result for missing item
- [ ] Typecheck and lint pass

---

### US-402: Transaction Error Decomposition

**Description:** As a developer, I need structured error handling for transaction failures so I can determine which operation failed and why, rather than parsing error strings.

**Acceptance Criteria:**
- [ ] `IsTxCancelled(err error) bool` — checks if the error is a transaction cancellation
- [ ] `TxCancelReasons(err error) []TxCancelReason` — extracts per-operation failure reasons
- [ ] `TxCancelReason` struct with `Code string` (e.g., "ConditionalCheckFailed", "ItemCollectionSizeLimitExceeded") and `Message string`
- [ ] Works with errors returned from both the AWS adapter and memdb
- [ ] `TxCancelledError` type that wraps the individual reasons
- [ ] Unit tests: parse cancellation with multiple reasons, non-tx error returns nil reasons
- [ ] Typecheck and lint pass

---

### US-403: ReadModifyWrite with Optimistic Locking

**Description:** As a developer, I need a read-modify-write helper with automatic optimistic locking so I can safely update items without manual version checking.

**Acceptance Criteria:**
- [ ] `ReadModifyWrite[T any](ctx, table, key KeyValue, fn func(*T) error, opts ...RMWOption) error`
- [ ] Reads the item, calls `fn` with a pointer to the decoded item, then puts the modified item back
- [ ] `OptimisticLock(versionAttr string) RMWOption` — auto-increments the version field and adds a condition expression checking the old version
- [ ] Retries automatically on condition check failure (optimistic lock contention), up to a configurable max (default 3)
- [ ] `MaxRetries(n int) RMWOption` — configures retry count
- [ ] Returns the condition check error if all retries exhausted
- [ ] If `fn` returns an error, the write is aborted and the error is returned
- [ ] Unit tests: successful RMW, concurrent RMW with retries, fn returns error, max retries exceeded
- [ ] Typecheck and lint pass

---

### US-404: Batch Write with Auto-Chunking

**Description:** As a developer, I need batch write operations that automatically chunk requests into DynamoDB's 25-item limit and handle unprocessed items.

**Acceptance Criteria:**
- [ ] `table.BatchPut(ctx, items []any, opts ...BatchOption) error` — method on Table
- [ ] `table.BatchDelete(ctx, keys []KeyValue, opts ...BatchOption) error` — method on Table
- [ ] Automatically chunks into groups of 25 (DynamoDB batch limit)
- [ ] Retries unprocessed items with exponential backoff
- [ ] `OnProgress(fn func(completed, total int)) BatchOption` — progress callback
- [ ] `MaxConcurrency(n int) BatchOption` — limits concurrent chunk requests (default 1, sequential)
- [ ] Returns a combined error if any chunk fails after retries
- [ ] Unit tests: batch put < 25 items, batch put > 25 items (chunking), progress callback invoked correctly, batch delete
- [ ] Typecheck and lint pass

---

### US-405: Batch Get with Type Safety

**Description:** As a developer, I need a type-safe batch get that retrieves multiple items by key and returns them as a typed slice.

**Acceptance Criteria:**
- [ ] `BatchGet[T any](ctx, table *Table, keys []KeyValue, opts ...BatchGetOption) ([]T, error)` free function
- [ ] Automatically chunks into groups of 100 (DynamoDB BatchGetItem limit)
- [ ] Retries unprocessed keys with exponential backoff
- [ ] Returns results in arbitrary order (matches DynamoDB behavior — no ordering guarantee)
- [ ] Supports projection expressions via options
- [ ] Unit tests: batch get < 100 keys, batch get > 100 keys, unprocessed key retry, projection
- [ ] Typecheck and lint pass

---

### US-406: MemDB Transaction Support

**Description:** As a developer writing tests, I need the in-memory backend to support TransactWriteItems and TransactGetItems so I can test transaction logic without DynamoDB.

**Acceptance Criteria:**
- [ ] `TransactWriteItems` — executes all write operations atomically (all-or-nothing)
- [ ] Evaluates condition expressions for each operation; if any condition fails, the entire transaction is cancelled
- [ ] Returns a `TxCancelledError` with per-operation reasons on failure
- [ ] `TransactGetItems` — reads multiple items in a single atomic snapshot
- [ ] Holds write lock for the entire transaction duration (atomicity)
- [ ] Supports Put, Update, Delete, and ConditionCheck operations within a transaction
- [ ] Maximum 100 operations enforced
- [ ] Unit tests: successful write tx, condition failure cancels all, read tx returns consistent snapshot, 100-op limit
- [ ] Typecheck and lint pass

---

### US-407: MemDB Batch Support

**Description:** As a developer writing tests, I need the in-memory backend to support BatchGetItem and BatchWriteItem so I can test batch operations.

**Acceptance Criteria:**
- [ ] `BatchWriteItem` — executes put and delete operations in a batch
- [ ] Does not support condition expressions (matches DynamoDB BatchWriteItem behavior)
- [ ] Maximum 25 operations per request enforced
- [ ] `BatchGetItem` — retrieves multiple items by key
- [ ] Maximum 100 keys per request enforced
- [ ] Returns all items found; missing items are silently omitted (matches DynamoDB)
- [ ] No `UnprocessedItems`/`UnprocessedKeys` in memdb (everything succeeds immediately)
- [ ] Unit tests: batch write, batch get, batch get with missing items, size limit enforcement
- [ ] Typecheck and lint pass

---

### US-408: Instrumentation Middleware

**Description:** As a developer, I need a pluggable instrumentation middleware that wraps the Backend interface so I can add observability (tracing, metrics, logging) without modifying the core library.

**Acceptance Criteria:**
- [ ] `Middleware func(Backend) Backend` type definition
- [ ] `DB` accepts middleware via `WithMiddleware(m ...Middleware) Option`
- [ ] Middleware is applied in order: the last middleware in the list is the outermost wrapper
- [ ] Each middleware wraps all 10 Backend methods
- [ ] `OperationInfo` struct available in middleware context: `TableName`, `OperationName` (e.g., "GetItem", "Query"), `IndexName` (if applicable)
- [ ] Unit test: middleware that counts calls, verify it intercepts all operations
- [ ] Unit test: multiple middleware applied in correct order
- [ ] Typecheck and lint pass

---

### US-409: OpenTelemetry Reference Implementation

**Description:** As a developer using OpenTelemetry, I need a reference middleware implementation that emits spans and metrics so I can monitor DynamoDB operations in production.

**Acceptance Criteria:**
- [ ] `dynagotel` package in `dynagotel/` directory with its own `go.mod` (`github.com/<org>/dynago/dynagotel`) — separate module so otel dependencies are never transitive for core library users
- [ ] `dynagotel/go.mod` requires `github.com/<org>/dynago` at a compatible version
- [ ] `NewMiddleware(opts ...Option) dynago.Middleware` constructor
- [ ] `WithTracer(tracer trace.Tracer) Option`
- [ ] `WithMeter(meter metric.Meter) Option`
- [ ] Emits a span per operation with attributes: `db.system=dynamodb`, `db.operation`, `db.name` (table)
- [ ] Records `dynago.consumed_capacity.total` on spans when available
- [ ] Emits `dynago.operations.total` counter metric
- [ ] Emits `dynago.latency` histogram metric
- [ ] Noop-safe: if tracer/meter is nil, middleware is a passthrough (no panics)
- [ ] Unit tests with in-memory span/metric exporters to verify emitted telemetry
- [ ] Tagged independently from core module (e.g., `dynagotel/v0.1.0`) — only needs a new release when its own code or the core `Backend`/`Middleware` interface changes
- [ ] Typecheck and lint pass

---

### US-410: Structured Logging Middleware

**Description:** As a developer, I need a logging middleware that logs slow operations using `slog` so I can identify performance issues without full tracing.

**Acceptance Criteria:**
- [ ] `WithLogger(logger *slog.Logger) dynago.Middleware` function (in root `dynago` package, not `otel` sub-package)
- [ ] `LogSlowOperations(threshold time.Duration) Option` for the logging middleware
- [ ] Logs operations exceeding the threshold at WARN level
- [ ] Log attributes: `operation`, `table`, `duration`, `consumed_rcu` (if available), `consumed_wcu` (if available)
- [ ] All operations are logged at DEBUG level regardless of duration
- [ ] Unit test: verify slow operations logged at WARN, fast operations at DEBUG
- [ ] Typecheck and lint pass

---

### US-411: Tier 3 Comparative Benchmarks

**Description:** As a library maintainer, I need comparative benchmarks against guregu/dynamo and the raw AWS SDK to validate that DynaGo's performance overhead is within acceptable bounds.

**Acceptance Criteria:**
- [ ] Benchmark module in `benchmarks/` directory (separate `go.mod` to isolate guregu/dynamo dependency)
- [ ] Compares encoding and decoding across struct complexity levels: Small (5 fields), Medium (15 fields), Large (30 fields), Nested (3 levels), WithSets
- [ ] Each size tested with DynaGo `Marshal`/`Unmarshal`, guregu `MarshalItem`/`UnmarshalItem`, AWS SDK `MarshalMap`/`UnmarshalMap`
- [ ] All benchmarks report ns/op, bytes/op, allocs/op via `b.ReportAllocs()`
- [ ] `.github/workflows/benchmarks-weekly.yml` running weekly + on version tags
- [ ] Uses `benchmark-action/github-action-benchmark` for tracking and regression alerts at 120% threshold
- [ ] Typecheck and lint pass

---

### US-412: Observability Overhead Benchmarks

**Description:** As a library maintainer, I need benchmarks measuring the overhead of the instrumentation middleware to ensure observability doesn't degrade performance.

**Acceptance Criteria:**
- [ ] `BenchmarkMiddleware_Disabled` — no middleware configured
- [ ] `BenchmarkMiddleware_NoopProvider` — otel middleware with noop tracer/meter
- [ ] `BenchmarkMiddleware_RealProvider` — otel middleware with in-memory exporter
- [ ] `BenchmarkMiddleware_LoggingOnly` — logging middleware with slog
- [ ] All benchmarks use a noop backend to isolate middleware overhead
- [ ] Noop provider adds less than 2% overhead compared to disabled
- [ ] All benchmarks report ns/op and allocs/op
- [ ] Typecheck and lint pass

---

## Functional Requirements

- FR-1: Types implementing `Entity` interface declare discriminator via `DynagoEntity() EntityInfo`
- FR-2: `Registry` maps discriminator strings to `reflect.Type` for polymorphic unmarshaling
- FR-3: `WithRegistry` binds a registry to a `Table` for polymorphic operations
- FR-4: `table.Put` auto-sets discriminator attribute for `Entity` types when registry is bound
- FR-5: `QueryCollection` returns heterogeneous results grouped by entity type
- FR-6: `ItemsOf[T]` extracts a typed slice from a `Collection`
- FR-7: `CollectionIter` provides streaming access to heterogeneous results via `iter.Seq2[any, error]`
- FR-8: `Index` option enables GSI queries through existing `Query[T]` and `Scan[T]` APIs
- FR-9: `SplitKey` splits composite key strings by a configurable delimiter
- FR-9a: All iterators (`QueryIter`, `ScanIter`, `CollectionIter`) use Go 1.23 `range over func` pattern (`iter.Seq2`)
- FR-10: `MemoryBackend` implements `Backend` for GetItem, PutItem, DeleteItem, UpdateItem, Query, Scan (Phase 3), plus BatchGetItem, BatchWriteItem, TransactGetItems, TransactWriteItems (Phase 4)
- FR-11: MemDB evaluates expressions from the internal AST directly (no DynamoDB expression string parsing)
- FR-12: MemDB maintains GSI indexes automatically on write operations
- FR-13: MemDB uses `sync.RWMutex` per table for concurrent access safety
- FR-14: `dynagotest` package provides `Seed`, `SeedFromJSON`, `AssertItemExists`, `AssertItemNotExists`, `AssertCount`
- FR-15: Conformance test suite runs identical assertions against memdb and DynamoDB Local
- FR-16: `WriteTx` builder composes up to 100 put/update/delete/check operations atomically
- FR-17: `ReadTx` builder reads up to 100 items in an atomic snapshot
- FR-18: `IsTxCancelled` and `TxCancelReasons` decompose transaction failures per-operation
- FR-19: `ReadModifyWrite[T]` provides read-modify-write with automatic optimistic locking and retry
- FR-20: `BatchPut`, `BatchDelete`, and `BatchGet[T]` auto-chunk and retry unprocessed items
- FR-21: `Middleware` type wraps `Backend` for pluggable instrumentation
- FR-22: `dynagotel` module (separate `go.mod`) provides a reference OpenTelemetry middleware
- FR-23: Logging middleware logs slow operations via `slog`

## Non-Goals

- **Full DynamoDB expression string parser.** The memdb evaluates the internal AST directly. It never parses DynamoDB expression syntax.
- **DynamoDB Streams support.** Stream processing is out of scope for DynaGo.
- **PartiQL support.** DynaGo uses its own expression AST, not PartiQL.
- **DynamoDB Local as a required dev dependency.** DynamoDB Local is used only in CI for conformance testing. All local development and testing uses the in-memory backend.
- **Full otel span attribute coverage in Phase 4.** The otel sub-package ships as a reference implementation with core attributes. Full span attribute coverage (retry count, scanned count, index name) is Phase 5 polish.
- **Parallel scan.** Not included in these phases. Can be added later without breaking changes.
- **Batch operation concurrency > 1 by default.** Concurrent chunking is opt-in via `MaxConcurrency`.

## Technical Considerations

- **Import cycle avoidance:** `internal/expr` imports `dynago` for `AttributeValue`. The `dynago` package must NOT import `internal/expr`. The memdb package will need to import both — ensure this doesn't create cycles by having memdb import `internal/expr` directly and `dynago` types via the Backend interface.
- **Memdb package location:** `memdb/` at the project root (not `internal/`) so users can import it directly for tests.
- **GSI full projection:** For simplicity, memdb GSIs store all item attributes (full projection). Projected attribute subsets are a future optimization.
- **Transaction atomicity in memdb:** Write transactions hold the write lock for the entire transaction. This is simple and correct but means transactions block all other writes. Acceptable for a testing backend.
- **otel dependency isolation:** `dynagotel/` is a separate Go module with its own `go.mod`. This is the established Go pattern (used by `go.opentelemetry.io/contrib`, `connectrpc.com/connect`, `entgo.io/ent`) for isolating heavyweight optional dependencies. Build tags don't actually isolate dependencies — if `go.opentelemetry.io/otel` is in `go.mod`, every user downloads it regardless of tags. Tagged independently: `dynagotel/v0.1.0`.
- **Package naming:** Use `dynagotel` (flat name), not `otel` as a subdirectory. A sub-directory named `otel` confuses tooling and humans into thinking it *is* the otel package. `dynagotel` is unambiguous in import paths and `grep`.
- **Benchmark module isolation:** Comparative benchmarks in `benchmarks/` use a separate `go.mod` to isolate guregu/dynamo and AWS SDK dependencies from the core library.

## Success Metrics

- All Phase 2 acceptance criteria pass — can model and query e-commerce single-table design
- All Phase 3 conformance tests pass against both memdb and DynamoDB Local
- MemDB 100-item query completes in < 1ms
- All Phase 4 acceptance criteria pass — transactions, batches, and instrumentation work end-to-end
- Otel noop middleware adds < 2% overhead
- Comparative benchmarks show < 10% ns/op overhead vs guregu/dynamo

## Resolved Design Decisions

| Question | Decision | Rationale |
|---|---|---|
| `ParseKey` error handling | Deferred entirely — ship `SplitKey` only in Phase 2 | Most key parsing is one-off splitting in app code. `ParseKey` can be added later based on real user demand. If added, it returns an error on mismatch (silent partial results are a footgun). |
| `CollectionIter` iteration pattern | `range over func` (`iter.Seq2`) for all iterators | Go 1.23+ was chosen specifically for this. Phase 1 `.Next()` iterators are migrated in US-209. One iteration idiom across the entire API. Non-nil error as final yield signals end. |
| `SeedFromJSON` format | DynamoDB JSON (with type descriptors) only | Unambiguous type resolution (distinguishes SS from L, N from S). Compatible with `aws dynamodb get-item` output. Flat JSON deferred — type inference rules are surprising design work. |
| `otel` module structure | Separate `go.mod` in `dynagotel/` directory | Build tags don't isolate dependencies from `go mod download`. Separate module is the established Go pattern. Tagged independently (`dynagotel/v0.1.0`). |
| `otel` package naming | `dynagotel` (flat), not `dynago/otel` | Avoids confusion with the actual `otel` package. Unambiguous in import paths and grep. |
| Key delimiter | Parameter on `SplitKey`, not hardcoded `#` | Teams use `#`, `|`, `::`, etc. Parameterizing costs nothing and avoids a breaking change. |
