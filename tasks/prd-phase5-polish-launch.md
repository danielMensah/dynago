# PRD: Phase 5 — Polish, AWS Adapter, and Launch

## Introduction

Phase 5 brings DynaGo from a feature-complete library (after Phases 2-4) to a production-ready, published Go module. This phase implements the AWS adapter that makes the library usable against real DynamoDB, adds comprehensive documentation and guides, sets up the remaining CI/CD pipeline tiers (Tier 1 core and GitHub Pages benchmarks), and culminates in the v0.1.0 release on GitHub and pkg.go.dev.

## Goals

- Implement the AWS adapter (`AWSBackend`) so DynaGo works against real DynamoDB
- Ensure all public APIs have GoDoc comments and are reviewed for consistency
- Provide documentation that enables adoption: README, migration guide, single-table design guide
- Complete the CI/CD pipeline (Tier 1 core, GitHub Pages benchmarks)
- Ship v0.1.0 as a tagged, published Go module

## User Stories

### US-500: AWS adapter — core CRUD operations
**Description:** As a developer, I need an AWS adapter that translates DynaGo's library-owned request/response types to AWS SDK v2 calls so I can use DynaGo against real DynamoDB tables.

**Acceptance Criteria:**
- [ ] `awsbackend` package in `awsbackend/` directory within the main `dynago` module (single module — no separate `go.mod`)
- [ ] AWS SDK dependencies (`github.com/aws/aws-sdk-go-v2/service/dynamodb`) added to the root `go.mod`
- [ ] `NewAWSBackend(client *dynamodb.Client) *AWSBackend` constructor
- [ ] `AWSBackend` implements all 10 `dynago.Backend` interface methods
- [ ] Translates `dynago.AttributeValue` to/from AWS SDK `types.AttributeValue` correctly for all types: S, N, B, BOOL, NULL, L, M, SS, NS, BS
- [ ] Translates expression AST to DynamoDB expression strings using `internal/expr.ToDynamo` and `ToDynamoUpdates` (accessible since `awsbackend` is in the same module)
- [ ] Translates projection expressions using `internal/expr.ToDynamoProjection`
- [ ] Passes through `ConsistentRead`, `ScanIndexForward`, `Limit`, `ExclusiveStartKey`, `IndexName`, `ReturnValues` fields
- [ ] Returns `LastEvaluatedKey` in query/scan responses for pagination
- [ ] Maps AWS SDK errors to DynaGo error types (`IsCondCheckFailed`, `IsTxCancelled`, etc.)
- [ ] Unit tests with a mock AWS client or integration test pattern
- [ ] Typecheck passes
- [ ] Tests pass

### US-501: AWS adapter — batch and transaction operations
**Description:** As a developer, I need the AWS adapter to support batch and transaction operations so I can use all DynaGo features against real DynamoDB.

**Acceptance Criteria:**
- [ ] `BatchGetItem` translates keys and projection, handles `UnprocessedKeys` in response
- [ ] `BatchWriteItem` translates put and delete requests, handles `UnprocessedItems` in response
- [ ] `TransactWriteItems` translates all operation types (Put, Update, Delete, ConditionCheck) with their conditions
- [ ] `TransactGetItems` translates get operations with projections
- [ ] Transaction cancellation errors are mapped to `TxCancelledError` with per-operation `TxCancelReason`
- [ ] `ConsumedCapacity` is translated when returned by AWS
- [ ] Unit tests for batch and transaction translation
- [ ] Typecheck passes
- [ ] Tests pass

### US-502: AWS adapter — helper constructor from aws.Config
**Description:** As a developer, I need a convenience constructor that creates an AWSBackend from `aws.Config` so I can set up DynaGo with the standard AWS configuration pattern.

**Acceptance Criteria:**
- [ ] `NewFromConfig(cfg aws.Config, opts ...func(*dynamodb.Options)) *AWSBackend` constructor
- [ ] Creates a `dynamodb.Client` internally from the config
- [ ] Options are passed through to `dynamodb.NewFromConfig`
- [ ] Example in GoDoc showing typical setup: `cfg, _ := config.LoadDefaultConfig(ctx); backend := awsbackend.NewFromConfig(cfg)`
- [ ] Typecheck passes
- [ ] Tests pass

### US-503: API review and GoDoc completeness
**Description:** As a developer evaluating DynaGo, I need all public types, functions, and methods to have clear GoDoc comments so I can understand the API from pkg.go.dev without reading source code.

**Acceptance Criteria:**
- [ ] Every exported type has a GoDoc comment explaining its purpose
- [ ] Every exported function/method has a GoDoc comment with parameter semantics and return behavior
- [ ] Every option type (PutOption, GetOption, QueryOption, ScanOption, etc.) documents what it does
- [ ] Error types and sentinel errors document when they occur
- [ ] Package-level doc comment on `dynago` package explains the library's purpose and shows a quick-start example
- [ ] Package-level doc comment on `awsbackend` package shows setup
- [ ] Package-level doc comment on `memdb` (if it exists) or equivalent shows testing usage
- [ ] Package-level doc comment on `dynagotest` shows assertion usage
- [ ] Package-level doc comment on `dynagotel` shows OTel setup
- [ ] Run `go doc ./...` and verify no exported symbols are undocumented
- [ ] Consistent naming conventions across the API (verify no inconsistencies in parameter naming, option patterns, etc.)
- [ ] Typecheck passes

### US-504: README with quick-start and examples
**Description:** As a developer discovering DynaGo, I need a README that shows what the library does, how to install it, and gives copy-paste examples for common operations.

**Acceptance Criteria:**
- [ ] README.md in project root with sections: Overview, Installation, Quick Start, Features, Examples, Documentation links
- [ ] Quick Start shows: create backend, create DB/Table, Put, Get, Query with 10 lines or fewer
- [ ] Features section lists: type-safe generics API, single-table design, in-memory testing, OTel observability
- [ ] Examples section covers: basic CRUD, query with filter, single-table design with registry, testing with memdb, OpenTelemetry setup
- [ ] Installation shows `go get` command for the core module
- [ ] Separate install instructions for `awsbackend` and `dynagotel` sub-modules
- [ ] Badge for CI status, Go Reference (pkg.go.dev), Go Report Card
- [ ] Typecheck passes (any Go code in README should be valid)

### US-505: Migration guide from guregu/dynamo
**Description:** As a developer currently using guregu/dynamo, I need a migration guide that maps guregu patterns to DynaGo equivalents so I can evaluate switching.

**Acceptance Criteria:**
- [ ] `docs/migration-from-guregu.md` file
- [ ] Side-by-side comparison table for common operations: Put, Get, Query, Scan, Delete, Update, BatchGet, BatchWrite
- [ ] Struct tag migration section: `dynamo` tag is compatible but documents any option differences
- [ ] Testing migration: from `dynamodbiface` mocking to `dynago.Backend` + memdb
- [ ] Error handling differences documented
- [ ] "What's new" section highlighting features guregu doesn't have: type-safe generics, single-table design, in-memory backend, OTel
- [ ] Code examples are syntactically valid Go

### US-506: Single-table design guide
**Description:** As a developer new to single-table design, I need a guide that shows how to model entities, register types, and query heterogeneous collections using DynaGo.

**Acceptance Criteria:**
- [ ] `docs/single-table-design.md` file
- [ ] Explains the concept: multiple entity types in one table, composite keys, discriminator attribute
- [ ] Concrete example: e-commerce model with User, Order, Product entities
- [ ] Shows: defining entities with `DynagoEntity()`, creating a registry, binding to table
- [ ] Shows: key construction with plain functions, `SplitKey` for parsing
- [ ] Shows: `QueryCollection` for heterogeneous queries, `ItemsOf[T]` for typed extraction
- [ ] Shows: `CollectionIter` with type switch for streaming processing
- [ ] Shows: GSI overloading pattern with `Index()` option
- [ ] Shows: testing the whole pattern with memdb
- [ ] Code examples are syntactically valid Go

### US-507: Tier 1 core CI workflow
**Description:** As a library maintainer, I need a CI workflow that runs tests, linting, and benchmarks on every push so regressions are caught immediately.

**Acceptance Criteria:**
- [ ] `.github/workflows/ci.yml` workflow file
- [ ] Triggers on push and pull_request
- [ ] Runs `go test ./... -race -count=1` (excluding conformance/integration tags)
- [ ] Runs `go vet ./...`
- [ ] Runs `staticcheck ./...` (or golangci-lint)
- [ ] Runs `go test ./... -bench=. -benchmem -run=^$ -count=3 | tee bench.txt`
- [ ] Uploads `bench.txt` as artifact
- [ ] Uses Go 1.23
- [ ] Also tests `awsbackend/`, `dynagotest/` packages (in-module) and `dynagotel/`, `benchmarks/` sub-modules
- [ ] Matrix strategy for Go 1.23 and latest stable Go version
- [ ] Typecheck passes

### US-509: GitHub Pages benchmark publishing
**Description:** As a library maintainer, I need benchmark results published to GitHub Pages with historical tracking so I can detect performance regressions over time.

**Acceptance Criteria:**
- [ ] `benchmark-action/github-action-benchmark` integrated into the Tier 1 CI workflow
- [ ] Benchmark results from `bench.txt` are pushed to a `gh-pages` branch
- [ ] Historical chart is accessible at the repo's GitHub Pages URL
- [ ] Alert threshold set to 120% — PRs that regress beyond 20% get a comment
- [ ] `auto-push: true` for benchmark data on main branch pushes
- [ ] `comment-on-alert: true` for PR notifications
- [ ] Typecheck passes

### US-510: Release preparation
**Description:** As a library maintainer, I need the repository to be release-ready with proper module paths, license, and clean dependencies.

**Acceptance Criteria:**
- [ ] `go.mod` has the correct module path (`github.com/danielMensah/dynago` or as configured)
- [ ] Sub-modules with separate `go.mod` (`dynagotel/`, `benchmarks/`) have correct `require` directives
- [ ] In-module packages (`awsbackend/`, `dynagotest/`) have no separate `go.mod` — they're part of the root module
- [ ] `go.sum` is clean and committed
- [ ] `LICENSE` file exists (MIT or Apache 2.0, as chosen)
- [ ] No `TODO` or `FIXME` comments in shipped code (audit and resolve or convert to GitHub issues)
- [ ] No test-only files in the wrong packages
- [ ] `go vet ./...` passes on all modules
- [ ] `.gitignore` covers standard Go artifacts
- [ ] Typecheck passes

### US-511: Tag v0.1.0 and publish
**Description:** As a library maintainer, I need to tag v0.1.0 and create a GitHub release so the module is discoverable on pkg.go.dev.

**Acceptance Criteria:**
- [ ] Git tag `v0.1.0` on main branch
- [ ] Sub-module tags: `dynagotel/v0.1.0` (only separate module needs its own tag)
- [ ] GitHub release created with changelog summarizing all features
- [ ] Changelog organized by category: Core API, Single-Table Design, Testing Backend, Transactions & Batches, Observability, AWS Adapter
- [ ] Module is fetchable via `go get github.com/<org>/dynago@v0.1.0`
- [ ] pkg.go.dev page renders correctly with GoDoc

## Functional Requirements

- FR-1: `AWSBackend` must implement all 10 `dynago.Backend` interface methods with correct type translation
- FR-2: AWS SDK errors must be mapped to DynaGo error types so `IsCondCheckFailed`, `IsTxCancelled` work against real DynamoDB
- FR-3: Expression AST translation must produce valid DynamoDB expression strings for all supported expression types
- FR-4: All exported symbols must have GoDoc comments
- FR-5: CI must run tests on every push and block merges on failure
- FR-6: Benchmark history must be publicly accessible via GitHub Pages
- FR-7: The module must be publishable to pkg.go.dev with all sub-modules resolvable

## Non-Goals

- Full DynamoDB API coverage (Streams, DAX, table management beyond CreateTable)
- DynamoDB Local support in the AWS adapter (that's the conformance suite's concern)
- Blog post or external marketing (out of scope for code PRD)
- Backwards compatibility guarantees (this is v0.1.0, not v1.0)
- PartiQL support
- DynamoDB Accelerator (DAX) integration

## Technical Considerations

- `awsbackend` is a package within the single `dynago` module — AWS SDK v2 (`service/dynamodb`) becomes a direct dependency in `go.mod`. This is acceptable because virtually all production consumers need it, and AWS SDK v2 is modular (only the dynamodb service package is pulled in)
- `dynagotel` remains a separate module (`dynagotel/go.mod`) because OTel is genuinely optional and adds significant transitive deps
- `internal/expr` is directly importable by `awsbackend` since they share a module boundary — no need to re-export expression APIs
- GitHub Pages benchmark publishing requires write access to the `gh-pages` branch — ensure the CI token has appropriate permissions
- License: MIT — add `LICENSE` file at repo root

## Success Metrics

- `go get` installs the module without errors
- pkg.go.dev renders all packages with documentation
- CI pipeline catches a deliberately introduced test failure
- Benchmark history shows at least 3 data points after initial setup
- Migration guide covers all operations listed in PRD v2 Section 9

## Resolved Design Decisions

| Question | Decision | Rationale |
|---|---|---|
| Module path | `github.com/danielMensah/dynago` | GitHub user is danielMensah |
| License | MIT | Standard for Go libraries, straightforward |
| AWS adapter package name | `awsbackend` | Avoids collision with `github.com/aws/aws-sdk-go-v2/aws` — no aliased imports needed in consuming code |
| Module structure | **Single module** — `awsbackend` is a package within the main `dynago` module, not a separate module | Virtually every production consumer needs the AWS backend anyway. Single module keeps `internal/expr` accessible to `awsbackend`, avoids coordinated multi-module versioning, and keeps CI simple. AWS SDK v2 is already modular (only `service/dynamodb` is pulled in). |
| `internal/expr` visibility | Stays internal — `awsbackend` accesses it directly since it's in the same module | No need to re-export expression translation to the public API |
| `dynagotel` module | Separate module (`dynagotel/go.mod`) — OTel dependency is genuinely optional and heavy | Unlike AWS SDK, most users won't want OTel as a transitive dep |
| `dynagotest` module | Package within the main module — it depends on `internal/` and has no heavy external deps | Same reasoning as `awsbackend` |

## Open Questions

- None remaining — all architectural decisions resolved.