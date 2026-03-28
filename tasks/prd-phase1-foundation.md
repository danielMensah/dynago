# PRD: Phase 1 — Foundation + Expression AST

## Introduction

Phase 1 establishes the core abstractions of DynaGo: the `DB` and `Table` types, the `Backend` interface with library-owned request/response types, the encoding/decoding pipeline, the expression AST, and the first set of CRUD operations (`Put`, `Delete`, `Get`, `Query`, `Scan`, `Update`). It also includes the AWS adapter, typed iterators, error types, Tier 1 benchmarks, and a minimal README.

By the end of Phase 1, a developer can perform all basic DynamoDB CRUD operations with compile-time type safety, build and evaluate expressions via an internal AST, and run benchmarks to establish performance baselines.

## Goals

- Define the `Backend` interface with library-owned request/response types, decoupled from AWS SDK types
- Implement a full AWS adapter (`AWSBackend`) that translates library types to/from AWS SDK v2
- Build an encoding/decoding pipeline using `dynamo` struct tags with a cached reflect-based codec
- Design and implement the expression AST (`internal/expr`) with builder, evaluator, and DynamoDB string translator
- Ship `Table.Put`, `Table.Delete`, `Table.Update` as methods and `Get[T]`, `Query[T]`, `Scan[T]`, `UpdateReturning[T]` as generic free functions
- Implement `KeyCondition` as a required parameter for `Query` (not a variadic option)
- Provide typed iterators (`QueryIter`, `ScanIter`) with pagination support using `range over func` (Go 1.23+)
- Define error types and sentinel values for condition check failures, not-found, and validation errors
- Establish Tier 1 benchmark baselines for encoding, expression, round-trip, and key construction
- Publish a minimal README with project description and a Get/Put example

## User Stories

### US-001: Core Types and Backend Interface

**Description:** As a library developer, I need the foundational types (`DB`, `Table`, `Backend` interface, `AttributeValue`, and library-owned request/response structs) so that all subsequent operations have a stable base to build on.

**Acceptance Criteria:**
- [ ] `DB` struct exists with a `New(backend Backend, opts ...Option) *DB` constructor
- [ ] `Table` struct exists with `db.Table(name string, opts ...TableOption) *Table` method
- [ ] `Backend` interface defined with all 10 methods: `GetItem`, `PutItem`, `DeleteItem`, `UpdateItem`, `Query`, `Scan`, `BatchGetItem`, `BatchWriteItem`, `TransactGetItems`, `TransactWriteItems`
- [ ] All request/response types are library-owned (e.g., `GetItemRequest`, `GetItemResponse`) — no AWS SDK types in the interface
- [ ] `AttributeValue` type defined as a library-owned sum type (not `types.AttributeValue` from AWS SDK)
- [ ] `ConsumedCapacity` struct defined
- [ ] `KeyValue` type defined (opaque type that resolves to `map[string]AttributeValue` internally)
- [ ] `Key(pairs ...any) KeyValue` helper function implemented; accepts 2 args (hash only) or 4 args (hash + range)
- [ ] `Key()` panics on odd argument count, non-string attribute names, or unsupported value types (programmer error, same convention as `regexp.MustCompile`)
- [ ] Package compiles with `go build ./...`
- [ ] Unit tests for `Key()` construction with string, number, and binary key values, and panic on invalid input
- [ ] Typecheck and lint pass

### US-002: Struct Tag Parser

**Description:** As a library developer, I need a struct tag parser that reads the full `dynamo` tag syntax so that encoding/decoding and future phases (GSI metadata, key schema) all use a single cached parse result.

**Acceptance Criteria:**
- [ ] Parses `dynamo` struct tag (not `dynago`)
- [ ] Recognises all tag options: field name, `hash`, `range`, `gsi:<name>`, `omitempty`, `set`, `unixtime`, `-` (skip)
- [ ] Stores parsed result in a `fieldOptions` struct containing all metadata (including GSI/key info that Phase 1 won't act on)
- [ ] `typeCodec` struct caches parsed field metadata per struct type via `sync.Map`
- [ ] Untagged exported fields default to using the field name as the attribute name
- [ ] Unexported fields are always skipped
- [ ] `-` tag skips the field
- [ ] Tag parsing is tested for all recognised options including combinations (e.g., `dynamo:"GSI1PK,gsi:GSI1,hash"`)
- [ ] Benchmark: tag parsing of a 10-field struct is cached after first call
- [ ] Typecheck and lint pass

### US-003: Encoding Pipeline (Marshal)

**Description:** As a developer using DynaGo, I want to marshal Go structs to `map[string]AttributeValue` so that I can write items to DynamoDB without manual attribute construction.

**Acceptance Criteria:**
- [ ] `Marshal(v any) (map[string]AttributeValue, error)` function implemented
- [ ] Supports: `string`, `int/int64/float64` (as DynamoDB N), `bool`, `[]byte` (as B), `time.Time` (as N unixtime when tagged, ISO 8601 string otherwise)
- [ ] Supports: slices (as L), maps (as M), nested structs (as M)
- [ ] Supports: `[]string` / `[]int` / `[]float64` / `[][]byte` as sets (SS/NS/BS) when tagged with `set`
- [ ] Supports: pointer fields (nil pointer omitted)
- [ ] `omitempty` skips zero-value fields (empty string, 0, false, nil pointer, empty slice/map)
- [ ] Custom marshaler interface: types implementing `MarshalDynamo() (AttributeValue, error)` are used
- [ ] Uses cached `typeCodec` from US-002 — no re-parsing on repeated calls
- [ ] Unit tests for every supported type and tag option
- [ ] Typecheck and lint pass

### US-004: Decoding Pipeline (Unmarshal)

**Description:** As a developer using DynaGo, I want to unmarshal `map[string]AttributeValue` into typed Go structs so that I get compile-time safe results from DynamoDB reads.

**Acceptance Criteria:**
- [ ] `Unmarshal(item map[string]AttributeValue, out any) error` function implemented
- [ ] Supports all types listed in US-003 (reverse direction)
- [ ] `time.Time` decoded from unixtime (N) when tagged, ISO 8601 (S) otherwise
- [ ] Sets (SS/NS/BS) decoded into slices when field is tagged with `set`
- [ ] Pointer fields: missing attribute sets pointer to nil (does not allocate)
- [ ] Extra attributes in the map that don't match struct fields are silently ignored
- [ ] Custom unmarshaler interface: types implementing `UnmarshalDynamo(AttributeValue) error` are used
- [ ] Round-trip test: `Marshal` then `Unmarshal` produces identical struct for all supported types
- [ ] Typecheck and lint pass

### US-005: Expression AST — Core Nodes and Builder

**Description:** As a library developer, I need an internal expression AST so that expressions are built once and consumed by both the DynamoDB string translator (AWS path) and the evaluator (memdb path).

**Acceptance Criteria:**
- [ ] `internal/expr/ast.go` defines node types: `CompareNode`, `LogicalNode`, `FuncNode`, `PathNode`, `ValueNode`, `UpdateNode`, `ProjectionNode`
- [ ] `CompareOp` enum: `EQ`, `NE`, `LT`, `LE`, `GT`, `GE`
- [ ] `LogicalOp` enum: `AND`, `OR`, `NOT`
- [ ] `FuncNode` supports: `attribute_exists`, `attribute_not_exists`, `begins_with`, `contains`, `size`
- [ ] `PathNode` supports nested paths (e.g., `["Address", "City"]`)
- [ ] `UpdateNode` supports actions: `SET`, `ADD`, `REMOVE`, `DELETE`
- [ ] Builder functions parse `?` placeholder syntax: `Filter("Active = ? AND Age > ?", true, 21)` produces the correct AST
- [ ] Builder handles `#name` reserved-word attribute name placeholders
- [ ] `internal/reserved.go` contains DynamoDB reserved words list
- [ ] Unit tests for AST construction from placeholder expressions including edge cases (nested paths, multiple conditions, function calls)
- [ ] Typecheck and lint pass

### US-006: Expression AST — DynamoDB String Translator

**Description:** As a library developer, I need to translate the expression AST into DynamoDB expression strings and attribute name/value maps so that the AWS adapter can send expressions to real DynamoDB.

**Acceptance Criteria:**
- [ ] `expr.ToDynamo(node Node) (string, map[string]string, map[string]AttributeValue)` translates AST to expression string + `ExpressionAttributeNames` + `ExpressionAttributeValues`
- [ ] Reserved-word attributes are automatically aliased in `ExpressionAttributeNames` (e.g., `#Status` for `Status`)
- [ ] Value placeholders are replaced with `:v0`, `:v1`, etc. in `ExpressionAttributeValues`
- [ ] Handles compound expressions (AND/OR/NOT) with correct parenthesisation
- [ ] Handles function calls (`begins_with(#path, :v0)`)
- [ ] Handles update expressions (SET, ADD, REMOVE, DELETE actions)
- [ ] Unit tests: each node type translates correctly; round-trip from builder string to AST to DynamoDB string preserves semantics
- [ ] Typecheck and lint pass

### US-007: Expression AST — Evaluator

**Description:** As a library developer, I need an AST evaluator so that the in-memory backend (Phase 3) can evaluate conditions, filters, and projections directly without parsing DynamoDB expression strings.

**Acceptance Criteria:**
- [ ] `expr.Eval(node Node, item map[string]AttributeValue) (bool, error)` evaluates condition/filter expressions
- [ ] Comparison operators work for strings (lexicographic), numbers (numeric), and binary (byte comparison)
- [ ] Logical operators (AND, OR, NOT) short-circuit correctly
- [ ] `attribute_exists` returns true if path exists (including nested paths), false otherwise
- [ ] `attribute_not_exists` is the inverse of `attribute_exists`
- [ ] `begins_with(path, prefix)` works on string values
- [ ] `contains(path, value)` works on strings (substring) and lists (element membership)
- [ ] `size(path)` returns the size of strings (length), lists (length), maps (key count), binary (byte length)
- [ ] Nested path traversal: `Address.City` navigates into map attributes
- [ ] `expr.EvalUpdate(node Node, item map[string]AttributeValue) (map[string]AttributeValue, error)` applies update expressions
- [ ] Unit tests for every operator, function, and type combination
- [ ] Typecheck and lint pass

### US-008: Error Types and Sentinel Values

**Description:** As a developer using DynaGo, I want structured error types so that I can programmatically handle DynamoDB-specific error conditions without string matching.

**Acceptance Criteria:**
- [ ] `errors.go` defines sentinel errors: `ErrNotFound`, `ErrConditionFailed`, `ErrValidation`, `ErrTransactionCancelled`
- [ ] `IsNotFound(err) bool` — returns true when a Get/Query returns no item
- [ ] `IsCondCheckFailed(err) bool` — returns true for condition expression failures (conditional put/update/delete)
- [ ] `IsValidation(err) bool` — returns true for client-side validation errors (missing key, bad type)
- [ ] `IsTxCancelled(err) bool` — returns true for transaction cancellation errors
- [ ] All error types implement `error` and support `errors.Is` / `errors.As` unwrapping
- [ ] AWS SDK errors are wrapped into library error types by the AWS adapter
- [ ] Unit tests for error wrapping, `Is`, and `As` behaviour
- [ ] Typecheck and lint pass

### US-009: Table.Put and Table.Delete

**Description:** As a developer using DynaGo, I want to put and delete items via methods on `*Table` so that the type is inferred from the argument and I get editor autocompletion via `table.`.

**Acceptance Criteria:**
- [ ] `table.Put(ctx, item, opts ...PutOption) error` marshals the item and calls `Backend.PutItem`
- [ ] `table.Delete(ctx, key KeyValue, opts ...DeleteOption) error` calls `Backend.DeleteItem`
- [ ] `PutOption` supports: `IfNotExists(attr)`, `Condition(expr, vals...)` (builds AST, translates for AWS)
- [ ] `DeleteOption` supports: `Condition(expr, vals...)`
- [ ] Condition expressions are built via the AST builder (US-005) and translated via US-006
- [ ] `Put` returns `ErrConditionFailed` when condition fails
- [ ] Unit tests using a mock/stub backend that captures the request and validates encoding, key construction, and expression translation
- [ ] Typecheck and lint pass

### US-010: Get[T] Free Function

**Description:** As a developer using DynaGo, I want a generic `Get[T]` function so that I get a typed result without casting or out-parameters.

**Acceptance Criteria:**
- [ ] `func Get[T any](ctx context.Context, t *Table, key KeyValue, opts ...GetOption) (T, error)` implemented
- [ ] Calls `Backend.GetItem`, unmarshals response into `T`
- [ ] Returns `ErrNotFound` when item does not exist (response has nil/empty Item)
- [ ] `GetOption` supports: `ConsistentRead()`, `Project(attrs ...string)` (builds projection AST)
- [ ] Projection expressions are translated to DynamoDB format for the request
- [ ] Unit tests with stub backend for success, not-found, and projection cases
- [ ] Typecheck and lint pass

### US-011: KeyCondition and Query[T] Free Function

**Description:** As a developer using DynaGo, I want a generic `Query[T]` with `KeyCondition` as a required parameter so that the compiler enforces that I always provide a partition key.

**Acceptance Criteria:**
- [ ] `KeyCondition` struct with `Partition(attr, val)` constructor
- [ ] Sort key modifiers on `KeyCondition`: `SortEquals`, `SortBeginsWith`, `SortBetween`, `SortGreaterThan`, `SortLessThan`, `SortGreaterOrEqual`, `SortLessOrEqual`
- [ ] `func Query[T any](ctx context.Context, t *Table, key KeyCondition, opts ...QueryOption) ([]T, error)` implemented
- [ ] `QueryOption` supports: `Filter(expr, vals...)`, `Limit(n)`, `ScanForward(bool)`, `Index(name)`, `Project(attrs...)`, `ConsistentRead()`
- [ ] Key condition is translated to DynamoDB `KeyConditionExpression` via AST
- [ ] Filter is translated to `FilterExpression` via AST
- [ ] Results are unmarshalled into `[]T`
- [ ] Pagination: when DynamoDB returns `LastEvaluatedKey`, the function automatically fetches subsequent pages until all results are collected (up to `Limit` if set)
- [ ] Unit tests with stub backend covering: partition-only query, partition+sort, filters, limit, index, pagination across multiple pages
- [ ] Typecheck and lint pass

### US-012: Scan[T] Free Function

**Description:** As a developer using DynaGo, I want a generic `Scan[T]` function so that I can scan a table with optional filters and get typed results.

**Acceptance Criteria:**
- [ ] `func Scan[T any](ctx context.Context, t *Table, opts ...ScanOption) ([]T, error)` implemented
- [ ] `ScanOption` supports: `Filter(expr, vals...)`, `Limit(n)`, `Index(name)`, `Project(attrs...)`, `ConsistentRead()`
- [ ] Automatic pagination (same as Query)
- [ ] Results are unmarshalled into `[]T`
- [ ] Unit tests with stub backend covering: full scan, filtered scan, limit, pagination
- [ ] Typecheck and lint pass

### US-013: Table.Update Method and UpdateReturning[T] Free Function

**Description:** As a developer using DynaGo, I want `table.Update()` for fire-and-forget updates (returns only `error`) and `dynago.UpdateReturning[T]()` when I need the updated item back as a typed result — eliminating the zero-value trap of returning an unused `T`.

**Acceptance Criteria:**
- [ ] `table.Update(ctx, key KeyValue, opts ...UpdateOption) error` — method on `*Table`, no type parameter needed, returns only `error`
- [ ] `func UpdateReturning[T any](ctx context.Context, t *Table, key KeyValue, opts ...UpdateOption) (T, error)` — free function, returns typed item
- [ ] `UpdateReturning[T]` requires `ReturnNew()` or `ReturnOld()` option; returns an error if neither is provided (not a zero-value `T`)
- [ ] `UpdateOption` supports: `Set(attr, val)`, `Add(attr, val)`, `Remove(attr)`, `Delete(attr, val)` (for set removal)
- [ ] `UpdateOption` supports: `IfCondition(expr, vals...)` for conditional updates
- [ ] `UpdateOption` supports: `ReturnNew()`, `ReturnOld()` to control which item state is returned (only meaningful with `UpdateReturning[T]`)
- [ ] Update expressions are built via the AST (US-005) and translated to DynamoDB format (US-006)
- [ ] Both functions return `ErrConditionFailed` when condition fails
- [ ] Unit tests with stub backend covering: `table.Update` with SET/ADD/REMOVE/DELETE, `UpdateReturning[T]` with ReturnNew/ReturnOld, conditional update, missing return option error
- [ ] Typecheck and lint pass

### US-014: Typed Iterators with Pagination

**Description:** As a developer using DynaGo, I want typed iterators for Query and Scan so that I can process large result sets without loading everything into memory at once, using idiomatic Go 1.23 `range over func`.

**Acceptance Criteria:**
- [ ] `func QueryIter[T any](ctx context.Context, t *Table, key KeyCondition, opts ...QueryOption) iter.Seq2[T, error]` returns a Go 1.23+ iterator
- [ ] `func ScanIter[T any](ctx context.Context, t *Table, opts ...ScanOption) iter.Seq2[T, error]` returns a Go 1.23+ iterator
- [ ] Iterators automatically paginate: when a page is exhausted, the next page is fetched using `LastEvaluatedKey`
- [ ] Error convention: on a failing iteration, the iterator yields `(zeroT, err)` as the final element and stops — the error is in the loop body, not a separate `Err()` method
- [ ] Iteration stops when: all pages are consumed, an error occurs, or the caller breaks out of the range loop
- [ ] Breaking out of the range loop does not leak goroutines or leave pending requests
- [ ] `Query[T]` and `Scan[T]` (US-011, US-012) are the collect-all convenience functions; iterators are for streaming / large result sets / early termination
- [ ] Unit tests with stub backend: iterate over multi-page results, break mid-page, error mid-iteration, verify error is yielded as final element
- [ ] Typecheck and lint pass

### US-015: AWS Adapter

**Description:** As a developer using DynaGo, I need an AWS adapter that translates library-owned types to/from AWS SDK v2 types so that I can use DynaGo against real DynamoDB.

**Acceptance Criteria:**
- [ ] `AWSBackend` struct implements `Backend` interface
- [ ] `NewAWSBackend(client *dynamodb.Client) *AWSBackend` constructor (accepts pre-configured client)
- [ ] Translation functions: `toAWSKey`, `toAWSItem`, `fromAWSItem`, `toAWSAttributeValue`, `fromAWSAttributeValue` handle all `AttributeValue` variants (S, N, B, BOOL, NULL, L, M, SS, NS, BS)
- [ ] Expression AST nodes are translated to DynamoDB expression strings and wired into request inputs (`KeyConditionExpression`, `FilterExpression`, `ProjectionExpression`, `UpdateExpression`, `ConditionExpression` with corresponding `ExpressionAttributeNames` and `ExpressionAttributeValues`)
- [ ] AWS SDK errors are wrapped into library error types (US-008): `ConditionalCheckFailedException` → `ErrConditionFailed`, `ResourceNotFoundException` → appropriate error, etc.
- [ ] All 10 `Backend` methods are implemented (even if Batch/Transaction implementations are thin pass-throughs for now)
- [ ] `go.mod` includes AWS SDK v2 dependencies (`github.com/aws/aws-sdk-go-v2/service/dynamodb`, `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue`)
- [ ] Unit tests for attribute value translation (both directions) covering all DynamoDB types
- [ ] Typecheck and lint pass

### US-016: Tier 1 Benchmarks

**Description:** As a library developer, I need Tier 1 benchmarks so that I have performance baselines from day one and can detect regressions as the library evolves.

**Acceptance Criteria:**
- [ ] Encoding benchmarks: `BenchmarkEncode_Flat`, `BenchmarkEncode_Nested`, `BenchmarkEncode_Large` (30+ fields), `BenchmarkEncode_WithSets`
- [ ] Decoding benchmarks: `BenchmarkDecode_Flat`, `BenchmarkDecode_Nested`, `BenchmarkDecode_Large`
- [ ] Codec cache benchmarks: `BenchmarkEncode_ColdCache`, `BenchmarkEncode_WarmCache`
- [ ] Expression benchmarks: `BenchmarkExprBuild_SimpleFilter`, `BenchmarkExprBuild_ComplexCondition`, `BenchmarkExprTranslate_ToDynamo`
- [ ] Round-trip benchmarks (noop backend): `BenchmarkGetRoundTrip`, `BenchmarkPutRoundTrip`, `BenchmarkQueryRoundTrip`
- [ ] Key construction benchmark: `BenchmarkKeyConstruction`
- [ ] All benchmarks use `b.ReportAllocs()`
- [ ] All benchmarks pass with `go test -bench=. -benchmem ./...`
- [ ] Typecheck and lint pass

### US-017: Minimal README

**Description:** As a potential user visiting the repository, I want a README that explains what DynaGo is and shows a basic usage example so that I can quickly understand the project's purpose.

**Acceptance Criteria:**
- [ ] Project title and one-paragraph description (generics-first, single-table-native, in-memory testing)
- [ ] Install instructions (`go get`)
- [ ] One code example showing `Put` and `Get[T]` with a simple struct
- [ ] Go version requirement noted (1.23+)
- [ ] "Status: Work in Progress" badge or note
- [ ] License mention
- [ ] File saved as `README.md` in project root

## Functional Requirements

- FR-1: The `Backend` interface uses only library-owned types — no AWS SDK types cross the interface boundary
- FR-2: The `dynamo` struct tag is the sole tag name; the full tag grammar (`name`, `hash`, `range`, `gsi:<name>`, `omitempty`, `set`, `unixtime`, `-`) is parsed and cached on first access via `sync.Map`
- FR-3: `Table.Put`, `Table.Delete`, and `Table.Update` are methods on `*Table`; `Get[T]`, `Query[T]`, `Scan[T]`, `UpdateReturning[T]` are package-level generic free functions — methods for writes where type params aren't needed, free functions for typed reads/returns
- FR-4: `KeyCondition` is a required (non-variadic) parameter to `Query[T]` and `QueryIter[T]`
- FR-5: All expressions are represented internally as AST nodes; the DynamoDB string form is a serialisation produced by `expr.ToDynamo`; the evaluator operates on the AST directly
- FR-6: `?` is the value placeholder syntax; `#name` is the reserved-word attribute name placeholder
- FR-7: Iterators use Go 1.23 `iter.Seq2[T, error]` for idiomatic `range over func` support
- FR-8: The AWS adapter wraps AWS SDK errors into library sentinel errors using `errors.Is` / `errors.As` compatible wrapping
- FR-9: `Get[T]` returns `ErrNotFound` for missing items; it does not return a zero-value `T` with a nil error
- FR-10: The encoding pipeline supports custom marshalers (`MarshalDynamo`/`UnmarshalDynamo` interfaces)
- FR-11: `Key()` returns a `KeyValue` type; accepts 2 or 4 variadic `any` args; panics on programmer errors (odd count, non-string attr names) — same convention as `regexp.MustCompile`
- FR-12: `table.Update()` returns `error` only; `UpdateReturning[T]()` returns `(T, error)` and requires `ReturnNew()` or `ReturnOld()` — no zero-value `T` trap
- FR-13: Iterator error convention: `iter.Seq2[T, error]` yields `(zeroT, err)` as the final element on failure; no separate `Err()` method

## Non-Goals

- No single-table design features (Entity interface, Registry, polymorphic queries, collection queries) — that's Phase 2
- No in-memory backend (`memdb` package) — that's Phase 3; the evaluator (US-007) is built now because it shapes the AST, but `memdb` itself is deferred
- No transaction builders or batch operations beyond the Backend interface stubs — that's Phase 4
- No OpenTelemetry integration — that's Phase 4
- No structured logging — that's Phase 4
- No CI pipeline (GitHub Actions workflows) — that's Phase 5; benchmarks run locally for now
- No comprehensive documentation or migration guide — that's Phase 5
- No `SplitKey` / `ParseKey` utilities — that's Phase 2

## Technical Considerations

- Go 1.23+ required for `iter.Seq2` and `range over func`
- AWS SDK v2 dependencies: `github.com/aws/aws-sdk-go-v2/service/dynamodb`, `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue`, `github.com/aws/aws-sdk-go-v2/config`
- The `internal/expr` package is internal — it is not part of the public API and can change freely between versions
- `sync.Map` is used for codec caching because struct types are typically few and long-lived (write-rarely, read-often pattern)
- The expression evaluator (US-007) is built in Phase 1 even though `memdb` is Phase 3, because the evaluator's requirements shape the AST node types — deferring it risks rework

## Success Metrics

- All CRUD operations (`Put`, `Delete`, `Get`, `Query`, `Scan`, `Update`, `UpdateReturning`) work end-to-end with type safety
- Encoding/decoding round-trip preserves data for all supported types
- Expression AST builds correctly from placeholder syntax and translates to valid DynamoDB expressions
- Expression evaluator produces correct boolean results for all condition/filter types
- AWS adapter compiles and translates all attribute value types correctly
- All Tier 1 benchmarks run and report ns/op, bytes/op, allocs/op
- `go test ./... -race` passes with zero failures
- `go vet ./...` and linting pass

## Resolved Design Decisions

| Question | Decision | Rationale |
|---|---|---|
| `Key` representation | `Key(pairs ...any) KeyValue` variadic helper; panics on misuse (odd args, non-string attr names) | Readable call sites; DynamoDB keys are always 1-2 attrs; programmer errors should panic (same as `regexp.MustCompile`) |
| `Update` return type | `table.Update()` returns `error`; `dynago.UpdateReturning[T]()` returns `(T, error)` | Eliminates zero-value trap; follows methods-for-writes / free-functions-for-typed-reads principle |
| Iterator error convention | `iter.Seq2[T, error]` — yields `(zeroT, err)` as final element on failure; no separate `Err()` method | Go 1.23 commitment; error-in-loop prevents forgotten `Err()` checks; `Query[T]`/`Scan[T]` serve as collect-all convenience functions |
