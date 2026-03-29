---
title: "Full Example: ETL Pipeline"
weight: 2
---

# Full Example: ETL Pipeline

This guide walks through a complete single-table design using an **ETL pipeline**
domain. It builds on the concepts from the
[Single-Table Design]({{< relref "/docs/guides/single-table-design" >}}) guide
and puts every major DynaGo feature into practice:

- Entity definitions and the registry
- Composite key construction
- CRUD operations with conditions
- Write and read transactions
- Batch operations with concurrency
- Read-modify-write with optimistic locking
- Collection queries and iterators
- GSI overloading
- Testing with `memdb`

Every code example uses `dynamo:"..."` struct tags and imports from
`github.com/danielmensah/dynago`.

---

## Data Model

Our ETL system has four entity types:

| Entity | Description |
|---|---|
| **Pipeline** | A named pipeline definition (e.g. "Daily Sales Import") |
| **PipelineRun** | A single execution of a pipeline, with status tracking |
| **Step** | An individual step within a run (Extract, Transform, Load) |
| **StepMetric** | Performance metrics recorded after a step completes |

### Key Schema

All four entities live in one table. The partition key (`PK`) and sort key (`SK`)
encode the entity type and relationships:

| Entity | PK | SK | GSI1PK | GSI1SK |
|---|---|---|---|---|
| Pipeline | `PIPELINE#<id>` | `#METADATA` | -- | -- |
| PipelineRun | `PIPELINE#<id>` | `RUN#<timestamp>#<runId>` | `STATUS#<status>` | `RUN#<timestamp>` |
| Step | `RUN#<runId>` | `STEP#<order>#<stepId>` | -- | -- |
| StepMetric | `RUN#<runId>` | `METRIC#<stepId>` | -- | -- |

Key design decisions:

- **Pipeline and PipelineRun share a partition key** (`PIPELINE#<id>`), so you
  can query a pipeline and all its runs in one request.
- **PipelineRun sort keys start with a timestamp**, so descending queries
  naturally return the newest runs first.
- **Step and StepMetric share a partition key** (`RUN#<runId>`), making it
  possible to fetch an entire run's details in a single collection query.
- **Step sort keys include a zero-padded order** (`STEP#001#...`), ensuring
  lexicographic sort matches execution order.
- **GSI1** indexes runs by status, enabling queries like "find all failed runs".

### Access Patterns

| # | Access Pattern | DynaGo API |
|---|---|---|
| 1 | Create a pipeline | `table.Put` + `IfNotExists` |
| 2 | Get pipeline metadata | `Get[T]` |
| 3 | Start a run atomically | `WriteTx` |
| 4 | List runs (newest first) | `Query[T]` + `SortBeginsWith` + `ScanForward(false)` |
| 5 | Runs in a date range | `Query[T]` + `SortBetween` |
| 6 | Steps for a run | `Query[T]` + `SortBeginsWith` |
| 7 | Runs by status | `Query[T]` + `QueryIndex` |
| 8 | Update step progress | `table.Update` + `Set` / `Add` |
| 9 | Complete a run safely | `ReadModifyWrite[T]` + `OptimisticLock` |
| 10 | Bulk insert metrics | `table.BatchPut` + `MaxConcurrency` |
| 11 | Full run with all items | `QueryCollection` + `ItemsOf[T]` |
| 12 | Stream run items | `CollectionIter` |
| 13 | Iterate typed results | `QueryIter[T]` |
| 14 | Fetch multiple runs | `ReadTx` + `GetAs[T]` |
| 15 | Delete with condition | `table.Delete` + `DeleteCondition` |
| 16 | Clean up old runs | `table.BatchDelete` |
| 17 | Parse composite keys | `SplitKey` |

---

## Entity Definitions

Each struct implements the `Entity` interface so the registry can identify its
type via a discriminator attribute (`_type`).

```go
package etl

import (
	"fmt"
	"time"

	"github.com/danielmensah/dynago"
)

// ---------------------------------------------------------------------------
// Pipeline
// ---------------------------------------------------------------------------

// Pipeline represents a named ETL pipeline definition.
type Pipeline struct {
	PK          string    `dynamo:"PK,hash"`
	SK          string    `dynamo:"SK,range"`
	Name        string    `dynamo:"Name"`
	Description string    `dynamo:"Description,omitempty"`
	CreatedAt   time.Time `dynamo:"CreatedAt,unixtime"`
}

func (p Pipeline) DynagoEntity() dynago.EntityInfo {
	return dynago.EntityInfo{Discriminator: "PIPELINE"}
}

// ---------------------------------------------------------------------------
// PipelineRun
// ---------------------------------------------------------------------------

// PipelineRun represents a single execution of a pipeline.
// The Version field is used for optimistic locking when updating run status.
type PipelineRun struct {
	PK        string    `dynamo:"PK,hash"`
	SK        string    `dynamo:"SK,range"`
	RunID     string    `dynamo:"RunID"`
	Status    string    `dynamo:"Status"`
	GSI1PK    string    `dynamo:"GSI1PK,gsi:GSI1"`
	GSI1SK    string    `dynamo:"GSI1SK,gsi:GSI1"`
	StartedAt time.Time `dynamo:"StartedAt,unixtime"`
	EndedAt   time.Time `dynamo:"EndedAt,omitempty,unixtime"`
	Version   int64     `dynamo:"Version"`
}

func (r PipelineRun) DynagoEntity() dynago.EntityInfo {
	return dynago.EntityInfo{Discriminator: "PIPELINE_RUN"}
}

// ---------------------------------------------------------------------------
// Step
// ---------------------------------------------------------------------------

// Step represents an individual step within a pipeline run.
type Step struct {
	PK     string `dynamo:"PK,hash"`
	SK     string `dynamo:"SK,range"`
	StepID string `dynamo:"StepID"`
	Name   string `dynamo:"Name"`
	Order  int    `dynamo:"Order"`
	Status string `dynamo:"Status"`
	Error  string `dynamo:"Error,omitempty"`
}

func (s Step) DynagoEntity() dynago.EntityInfo {
	return dynago.EntityInfo{Discriminator: "STEP"}
}

// ---------------------------------------------------------------------------
// StepMetric
// ---------------------------------------------------------------------------

// StepMetric holds performance metrics for a completed step.
type StepMetric struct {
	PK            string `dynamo:"PK,hash"`
	SK            string `dynamo:"SK,range"`
	StepID        string `dynamo:"StepID"`
	RowsProcessed int64  `dynamo:"RowsProcessed"`
	RowsFailed    int64  `dynamo:"RowsFailed"`
	DurationMs    int64  `dynamo:"DurationMs"`
}

func (m StepMetric) DynagoEntity() dynago.EntityInfo {
	return dynago.EntityInfo{Discriminator: "STEP_METRIC"}
}
```

A few things to note:

- **`dynamo:"PK,hash"`** marks the partition key. **`dynamo:"SK,range"`** marks
  the sort key. DynaGo uses these tags to build key schemas automatically.
- **`dynamo:"GSI1PK,gsi:GSI1"`** declares a GSI key attribute. The part after
  `gsi:` is the index name.
- **`dynamo:",omitempty"`** skips the attribute when the value is the zero value
  for its type. This keeps items lean.
- **`dynamo:"StartedAt,unixtime"`** stores `time.Time` as a Unix epoch number
  (`N` type) instead of the default RFC 3339 string. This is more compact and
  enables numeric comparisons.
- **`Version int64`** on `PipelineRun` serves as the optimistic lock counter --
  more on this in the [Read-Modify-Write](#9-complete-a-run-with-optimistic-locking) section.
- Each **`DynagoEntity()`** method returns a unique discriminator string. The
  registry uses this to route items to the correct struct during unmarshaling.

---

## Registry and Table Setup

The registry maps discriminator strings to Go types. When you `Put` an entity
the registry automatically sets the `_type` attribute, and when you use
`QueryCollection` or `CollectionIter` it uses `_type` to pick the right struct.

```go
// Create a registry with "_type" as the discriminator attribute name.
reg := dynago.NewRegistry("_type")
reg.Register(Pipeline{})
reg.Register(PipelineRun{})
reg.Register(Step{})
reg.Register(StepMetric{})

// Create a DB handle (backend comes from AWS adapter or memdb for tests).
db := dynago.New(backend)

// Bind the registry to the table.
table := db.Table("ETLTable", dynago.WithRegistry(reg))
```

- **`NewRegistry("_type")`** means every item stored through this table will
  have a `_type` attribute whose value is the discriminator (e.g. `"PIPELINE"`,
  `"STEP"`).
- **`Register`** panics if you register two types with the same discriminator,
  catching mistakes at startup rather than at runtime.
- **`WithRegistry(reg)`** enables three things on the table: (1) auto-setting
  `_type` on `Put`, (2) polymorphic unmarshaling in `QueryCollection`, and
  (3) type routing in `CollectionIter`.

---

## Key Helper Functions

Centralising key construction in helper functions keeps business logic clean and
prevents typos in key prefixes.

```go
// pipelineKey returns the key for a Pipeline metadata item.
func pipelineKey(id string) dynago.KeyValue {
	return dynago.StringPairKey("PK", "PIPELINE#"+id, "SK", "#METADATA")
}

// runKey returns the key for a PipelineRun item.
func runKey(pipelineID, timestamp, runID string) dynago.KeyValue {
	return dynago.StringPairKey(
		"PK", "PIPELINE#"+pipelineID,
		"SK", "RUN#"+timestamp+"#"+runID,
	)
}

// stepKey returns the key for a Step item.
// The order is zero-padded to 3 digits so lexicographic sort matches numeric order.
func stepKey(runID string, order int, stepID string) dynago.KeyValue {
	return dynago.Key(
		"PK", "RUN#"+runID,
		"SK", fmt.Sprintf("STEP#%03d#%s", order, stepID),
	)
}

// metricKey returns the key for a StepMetric item.
func metricKey(runID, stepID string) dynago.KeyValue {
	return dynago.StringPairKey("PK", "RUN#"+runID, "SK", "METRIC#"+stepID)
}
```

- **`StringPairKey`** is the zero-allocation fast path when both key parts are
  strings. Use it whenever you can.
- **`Key`** is the general-purpose builder. It accepts 2 or 4 arguments and
  supports mixed types (string, int, float64, []byte, etc.).
- **`fmt.Sprintf("STEP#%03d#...", order, ...)`** zero-pads the step order so
  that `STEP#001` sorts before `STEP#010` lexicographically.

---

## Access Patterns

### 1. Create a Pipeline

Use `table.Put` with an `IfNotExists` condition to insert a pipeline only if it
does not already exist.

```go
err := table.Put(ctx, Pipeline{
	PK:          "PIPELINE#daily-sales",
	SK:          "#METADATA",
	Name:        "Daily Sales Import",
	Description: "Imports sales data from S3 into the warehouse",
	CreatedAt:   time.Now(),
}, dynago.IfNotExists("PK"))
```

- **`IfNotExists("PK")`** adds a condition expression
  `attribute_not_exists(PK)`. If an item with the same PK and SK already exists,
  DynamoDB rejects the write and DynaGo returns `dynago.ErrConditionFailed`.
- The registry automatically sets `_type: "PIPELINE"` on the stored item because
  `Pipeline` implements the `Entity` interface.

---

### 2. Get Pipeline Metadata

Use the generic `Get[T]` function with the pipeline's composite key.

```go
pipeline, err := dynago.Get[Pipeline](ctx, table, pipelineKey("daily-sales"))
if err != nil {
	if errors.Is(err, dynago.ErrNotFound) {
		// Pipeline does not exist -- handle accordingly.
	}
	return err
}
fmt.Println(pipeline.Name) // "Daily Sales Import"
```

- **`Get[Pipeline]`** fetches the item and unmarshals it directly into a
  `Pipeline` struct. No manual type assertion needed.
- **`dynago.ErrNotFound`** is returned when the key does not match any item.
  Always check with `errors.Is` because the error may be wrapped.

---

### 3. Start a Run Atomically

When starting a new pipeline run, you want to create the run record **and** all
its initial steps in a single atomic operation. If any part fails, nothing is
written. This is exactly what write transactions are for.

```go
runID := "run-001"
ts := "2025-06-15T10:00:00Z"
now := time.Now()

run := PipelineRun{
	PK:        "PIPELINE#daily-sales",
	SK:        "RUN#" + ts + "#" + runID,
	RunID:     runID,
	Status:    "RUNNING",
	GSI1PK:    "STATUS#RUNNING",
	GSI1SK:    "RUN#" + ts,
	StartedAt: now,
	Version:   1,
}

steps := []Step{
	{PK: "RUN#" + runID, SK: "STEP#001#extract", StepID: "extract",
		Name: "Extract from S3", Order: 1, Status: "PENDING"},
	{PK: "RUN#" + runID, SK: "STEP#002#transform", StepID: "transform",
		Name: "Transform records", Order: 2, Status: "PENDING"},
	{PK: "RUN#" + runID, SK: "STEP#003#load", StepID: "load",
		Name: "Load into warehouse", Order: 3, Status: "PENDING"},
}

err := dynago.WriteTx(ctx, db).
	Put(table, run, dynago.IfNotExists("PK")).
	Put(table, steps[0]).
	Put(table, steps[1]).
	Put(table, steps[2]).
	Run()
if err != nil {
	if dynago.IsTxCancelled(err) {
		// One of the conditions failed -- the run may already exist.
	}
	return err
}
```

- **`WriteTx(ctx, db)`** starts a transaction builder. Transactions take `db`
  (not `table`) because DynamoDB transactions can span multiple tables.
- **`.Put(table, run, dynago.IfNotExists("PK"))`** ensures the run does not
  already exist. The other three puts create the initial step items with no
  conditions.
- **`.Run()`** executes all four operations atomically. Either all succeed or
  none are written.
- **`dynago.IsTxCancelled(err)`** checks specifically for transaction
  cancellation (e.g. a condition check failed on one of the items).

---

### 4. List Runs for a Pipeline (Newest First)

Query all runs under a pipeline, sorted by most recent first.

```go
runs, err := dynago.Query[PipelineRun](ctx, table,
	dynago.Partition("PK", "PIPELINE#daily-sales").SortBeginsWith("SK", "RUN#"),
	dynago.ScanForward(false),
	dynago.QueryLimit(20),
)
```

- **`Partition("PK", "PIPELINE#daily-sales")`** sets the partition key condition.
  This is required for every query.
- **`.SortBeginsWith("SK", "RUN#")`** adds a `begins_with(SK, :prefix)` key
  condition. This filters out the Pipeline metadata item (whose SK is
  `#METADATA`) and returns only PipelineRun items.
- **`ScanForward(false)`** returns results in descending sort key order. Because
  the sort key starts with the timestamp, this means newest runs come first.
- **`QueryLimit(20)`** caps the total results at 20 items. DynaGo auto-paginates
  through DynamoDB pages until the limit is reached or there are no more results.

---

### 5. Get Runs in a Date Range

Fetch all runs for a pipeline within a specific month.

```go
runs, err := dynago.Query[PipelineRun](ctx, table,
	dynago.Partition("PK", "PIPELINE#daily-sales").
		SortBetween("SK", "RUN#2025-06-01", "RUN#2025-06-30\xff"),
)
```

- **`SortBetween`** generates a `BETWEEN :low AND :high` key condition on the
  sort key.
- **`\xff`** appended to the upper bound is a lexicographic trick: it ensures
  that all sort keys starting with `2025-06-30` (regardless of the time and
  run ID suffix) fall within the range.

---

### 6. Get All Steps for a Run

Steps live under the `RUN#<runId>` partition with sort keys prefixed by `STEP#`.

```go
steps, err := dynago.Query[Step](ctx, table,
	dynago.Partition("PK", "RUN#run-001").SortBeginsWith("SK", "STEP#"),
)
```

Because step sort keys include zero-padded order numbers (`STEP#001#extract`,
`STEP#002#transform`, `STEP#003#load`), the default ascending sort returns them
in execution order.

---

### 7. Query Runs by Status (GSI)

The GSI1 index partitions runs by status, enabling queries like "find all failed
runs".

```go
failedRuns, err := dynago.Query[PipelineRun](ctx, table,
	dynago.Partition("GSI1PK", "STATUS#FAILED"),
	dynago.QueryIndex("GSI1"),
	dynago.ScanForward(false),
)
```

- **`QueryIndex("GSI1")`** directs the query to the Global Secondary Index
  instead of the base table.
- The partition key on the GSI is `GSI1PK`, which stores values like
  `STATUS#RUNNING` or `STATUS#FAILED`.
- **`ScanForward(false)`** returns the most recent failures first (GSI1SK is
  `RUN#<timestamp>`).

---

### 8. Update Step Progress

Use `table.Update` to modify individual attributes without replacing the entire
item.

**Mark a step as running (with a condition):**

```go
err := table.Update(ctx,
	stepKey("run-001", 1, "extract"),
	dynago.Set("Status", "RUNNING"),
	dynago.IfCondition("Status = ?", "PENDING"),
)
```

- **`Set("Status", "RUNNING")`** generates a `SET #Status = :v0` update
  expression.
- **`IfCondition("Status = ?", "PENDING")`** adds a condition expression that
  only allows the update if the current status is `"PENDING"`. The `?`
  placeholder is automatically replaced with the provided value. If the
  condition fails, DynaGo returns `dynago.ErrConditionFailed`.

**Mark a step as completed and increment a counter:**

```go
err := table.Update(ctx,
	stepKey("run-001", 1, "extract"),
	dynago.Set("Status", "COMPLETED"),
	dynago.Add("RowsProcessed", 500),
)
```

- **`Set`** assigns a new value to an attribute.
- **`Add`** increments a numeric attribute by the given amount. If the attribute
  does not exist, DynamoDB initialises it to the value. This is useful for
  counters that may be updated from multiple sources.

---

### 9. Complete a Run with Optimistic Locking

When multiple processes might update a run's status concurrently (e.g. step
workers completing at the same time), use `ReadModifyWrite` with optimistic
locking to prevent lost updates.

```go
err := dynago.ReadModifyWrite[PipelineRun](ctx, table,
	runKey("daily-sales", "2025-06-15T10:00:00Z", "run-001"),
	func(run *PipelineRun) error {
		run.Status = "COMPLETED"
		run.EndedAt = time.Now()
		// Update the GSI key so status-based queries reflect the new state.
		run.GSI1PK = "STATUS#COMPLETED"
		return nil
	},
	dynago.OptimisticLock("Version"),
	dynago.MaxRetries(5),
)
```

Here is what happens under the hood:

1. **Read** -- DynaGo fetches the current item (e.g. `Version: 1`).
2. **Modify** -- Your callback receives a mutable `*PipelineRun` pointer. You
   change `Status`, `EndedAt`, and `GSI1PK`.
3. **Write** -- DynaGo writes the entire item back with a condition:
   `Version = 1` (the value it read). It also increments `Version` to `2`
   automatically.
4. **Retry** -- If another writer changed the item between step 1 and 3,
   the condition fails. DynaGo re-reads the item (now with the other writer's
   changes), calls your callback again on the fresh data, and retries the write.
   This repeats up to `MaxRetries(5)` times.

- **`OptimisticLock("Version")`** names the integer field used as the version
  counter.
- **`MaxRetries(5)`** sets the maximum number of retry attempts (default is 3).
- If your callback returns an error, the write is skipped and that error is
  returned to the caller.

---

### 10. Bulk Insert Step Metrics

After all steps complete, write their metrics in a single batch operation.

```go
metrics := []any{
	StepMetric{
		PK: "RUN#run-001", SK: "METRIC#extract",
		StepID: "extract", RowsProcessed: 15000, RowsFailed: 3, DurationMs: 4500,
	},
	StepMetric{
		PK: "RUN#run-001", SK: "METRIC#transform",
		StepID: "transform", RowsProcessed: 14997, RowsFailed: 12, DurationMs: 8200,
	},
	StepMetric{
		PK: "RUN#run-001", SK: "METRIC#load",
		StepID: "load", RowsProcessed: 14985, RowsFailed: 0, DurationMs: 3100,
	},
}

err := table.BatchPut(ctx, metrics,
	dynago.MaxConcurrency(3),
	dynago.OnProgress(func(completed, total int) {
		fmt.Printf("Wrote %d / %d metrics\n", completed, total)
	}),
)
```

- **`BatchPut`** automatically chunks items into groups of 25 (the DynamoDB
  limit per request) and retries any unprocessed items with exponential backoff.
- **`MaxConcurrency(3)`** sends up to 3 chunks in parallel. For our 3 items
  this makes no difference, but for hundreds of metrics it dramatically reduces
  wall-clock time.
- **`OnProgress`** fires after each chunk completes, reporting cumulative
  progress. Useful for logging or progress bars.
- The items slice must be `[]any` -- DynaGo marshals each item individually.

---

### 11. Get a Full Run with Steps and Metrics

A collection query fetches all entity types under a single partition key and
lets you filter them by type on the client side.

```go
coll, err := dynago.QueryCollection(ctx, table,
	dynago.Partition("PK", "RUN#run-001"),
)
if err != nil {
	return err
}

steps := dynago.ItemsOf[Step](coll)
metrics := dynago.ItemsOf[StepMetric](coll)

fmt.Printf("Run has %d steps and %d metrics\n", len(steps), len(metrics))

for _, s := range steps {
	fmt.Printf("  Step %d: %s [%s]\n", s.Order, s.Name, s.Status)
}
for _, m := range metrics {
	fmt.Printf("  Metric %s: %d rows in %dms\n", m.StepID, m.RowsProcessed, m.DurationMs)
}
```

- **`QueryCollection`** executes a single DynamoDB Query and unmarshals each
  item polymorphically via the registry's `_type` attribute. Items with
  unrecognised discriminators are silently skipped.
- **`ItemsOf[Step](coll)`** extracts only the `Step` items from the collection.
  This is a type-safe filter -- no casting needed.
- This is the power of single-table design: **one Query, one network round trip**,
  and you get all the data for a complete run.

---

### 12. Stream Run Items with an Iterator

For large runs where you do not want to load everything into memory at once, use
`CollectionIter`. It returns items lazily, page by page.

```go
for item, err := range dynago.CollectionIter(ctx, table,
	dynago.Partition("PK", "RUN#run-001"),
) {
	if err != nil {
		return err
	}
	switch v := item.(type) {
	case Step:
		fmt.Printf("Step %d: %s [%s]\n", v.Order, v.Name, v.Status)
	case StepMetric:
		fmt.Printf("Metric for %s: %d rows in %dms\n",
			v.StepID, v.RowsProcessed, v.DurationMs)
	}
}
```

- **`CollectionIter`** returns `iter.Seq2[any, error]` (Go 1.23+ range-over-func).
  Each iteration yields one item from the query result.
- Use a **type switch** to handle each entity type. The concrete types match
  what you registered in the registry.
- **Breaking out of the loop** stops pagination early -- no wasted reads or
  leaked goroutines.

---

### 13. Iterate Steps with QueryIter

When you know every item in the result is the same type, use `QueryIter[T]` for
a fully typed iterator.

```go
for step, err := range dynago.QueryIter[Step](ctx, table,
	dynago.Partition("PK", "RUN#run-001").SortBeginsWith("SK", "STEP#"),
) {
	if err != nil {
		return err
	}
	fmt.Printf("Step: %s - %s\n", step.Name, step.Status)
}
```

- **`QueryIter[Step]`** returns `iter.Seq2[Step, error]` -- no type assertion
  needed.
- Like `CollectionIter`, it auto-paginates and supports early `break`.

---

### 14. Fetch Multiple Specific Runs

Use a read transaction to fetch several runs by their exact keys in one atomic
request.

```go
result, err := dynago.ReadTx(ctx, db).
	Get(table, runKey("daily-sales", "2025-06-15T10:00:00Z", "run-001")).
	Get(table, runKey("daily-sales", "2025-06-14T10:00:00Z", "run-002")).
	Run()
if err != nil {
	return err
}

today, err := dynago.GetAs[PipelineRun](result, 0)
if err != nil {
	return err
}

yesterday, err := dynago.GetAs[PipelineRun](result, 1)
if err != nil {
	return err
}

fmt.Printf("Today: %s, Yesterday: %s\n", today.Status, yesterday.Status)
```

- **`ReadTx(ctx, db)`** starts a transactional read that can fetch up to 100
  items atomically. Like write transactions, it takes `db` not `table`.
- **`GetAs[PipelineRun](result, 0)`** unmarshals the item at index 0 (matching
  the order of `.Get` calls) into a `PipelineRun`. Returns
  `dynago.ErrNotFound` if the item at that index does not exist.

---

### 15. Delete a Run

Delete a run, but only if it has a specific status.

```go
err := table.Delete(ctx,
	runKey("daily-sales", "2025-06-15T10:00:00Z", "run-001"),
	dynago.DeleteCondition("Status = ?", "FAILED"),
)
```

- **`DeleteCondition`** ensures only failed runs can be deleted. If the status
  is not `"FAILED"`, the operation returns `dynago.ErrConditionFailed`.
- The `?` placeholder works the same way as in `IfCondition` and `QueryFilter`.

---

### 16. Clean Up Old Runs

Use a two-phase approach: query first, then batch-delete.

```go
// 1. Find all runs from 2024.
oldRuns, err := dynago.Query[PipelineRun](ctx, table,
	dynago.Partition("PK", "PIPELINE#daily-sales").
		SortBetween("SK", "RUN#2024-01-01", "RUN#2024-12-31\xff"),
)
if err != nil {
	return err
}

// 2. Build the key list from the query results.
keys := make([]dynago.KeyValue, len(oldRuns))
for i, run := range oldRuns {
	keys[i] = dynago.StringPairKey("PK", run.PK, "SK", run.SK)
}

// 3. Delete in batches.
err = table.BatchDelete(ctx, keys,
	dynago.MaxConcurrency(2),
	dynago.OnProgress(func(completed, total int) {
		fmt.Printf("Deleted %d / %d old runs\n", completed, total)
	}),
)
```

- **`BatchDelete`** chunks keys into groups of 25, retries unprocessed keys
  with exponential backoff, and supports the same `MaxConcurrency` and
  `OnProgress` options as `BatchPut`.
- In a real system you would also delete the associated steps and metrics under
  each run's `RUN#<runId>` partition.

---

### 17. Parse Composite Keys

Use `SplitKey` to extract components from a composite sort key.

```go
run := PipelineRun{SK: "RUN#2025-06-15T10:00:00Z#run-001"}

parts := dynago.SplitKey(run.SK, "#")
// parts[0] = "RUN"
// parts[1] = "2025-06-15T10:00:00Z"
// parts[2] = "run-001"
timestamp := parts[1]
runID := parts[2]

fmt.Printf("Run %s started at %s\n", runID, timestamp)
```

`SplitKey` is a thin convenience wrapper that makes the intent explicit in code
reviews. It works with any delimiter.

---

### 18. Atomic Run Completion (Mixed Transaction)

A write transaction can mix different operation types: puts, updates, deletes,
and condition checks.

```go
err := dynago.WriteTx(ctx, db).
	// Mark the final step as completed.
	Update(table, stepKey("run-001", 3, "load"),
		dynago.Set("Status", "COMPLETED"),
	).
	// Write the step's metric.
	Put(table, StepMetric{
		PK: "RUN#run-001", SK: "METRIC#load",
		StepID: "load", RowsProcessed: 14985, DurationMs: 3100,
	}).
	// Verify the run is still in RUNNING status before proceeding.
	Check(table,
		runKey("daily-sales", "2025-06-15T10:00:00Z", "run-001"),
		"Status = ?", "RUNNING",
	).
	Run()
```

- **`.Update`** modifies an existing item (marks the step as completed).
- **`.Put`** writes a new item (the step metric).
- **`.Check`** validates a condition without modifying the item. If the run
  has already been marked as completed or failed by another process, the entire
  transaction rolls back.
- All three operations succeed or fail together. This prevents the metric from
  being written if the step update fails, and prevents both if the run is no
  longer in the expected state.

---

## Filtering Query Results

DynamoDB filter expressions are applied **after** items are read but **before**
they are returned to the client. They do not reduce read capacity consumption,
but they simplify client-side logic.

```go
metrics, err := dynago.Query[StepMetric](ctx, table,
	dynago.Partition("PK", "RUN#run-001").SortBeginsWith("SK", "METRIC#"),
	dynago.QueryFilter("RowsFailed > ?", 0),
)
```

- **`QueryFilter("RowsFailed > ?", 0)`** only returns metrics where at least
  one row failed. The `?` placeholder is replaced with the value `0`.
- Filters support the same operators and functions as condition expressions:
  `=`, `<>`, `<`, `>`, `BETWEEN`, `attribute_exists()`, `begins_with()`, `AND`,
  `OR`, `NOT`, etc.

---

## Testing with memdb

DynaGo's `memdb` package provides an in-memory backend that implements the full
`Backend` interface. Combined with `dynagotest` helpers, you can write fast,
deterministic tests with no AWS credentials.

```go
package etl_test

import (
	"context"
	"testing"
	"time"

	"github.com/danielmensah/dynago"
	"github.com/danielmensah/dynago/dynagotest"
	"github.com/danielmensah/dynago/memdb"
)

func TestStartPipelineRun(t *testing.T) {
	// 1. Create an in-memory backend with a GSI.
	backend := memdb.New()
	rangeKey := memdb.KeyDef{Name: "SK", Type: memdb.StringKey}
	backend.CreateTable("ETLTable", memdb.TableSchema{
		HashKey:  memdb.KeyDef{Name: "PK", Type: memdb.StringKey},
		RangeKey: &rangeKey,
		GSIs: []memdb.GSISchema{
			{
				Name:     "GSI1",
				HashKey:  memdb.KeyDef{Name: "GSI1PK", Type: memdb.StringKey},
				RangeKey: &memdb.KeyDef{Name: "GSI1SK", Type: memdb.StringKey},
			},
		},
	})

	// 2. Set up the registry and bind it to the table.
	reg := dynago.NewRegistry("_type")
	reg.Register(Pipeline{})
	reg.Register(PipelineRun{})
	reg.Register(Step{})
	reg.Register(StepMetric{})

	db := dynago.New(backend)
	table := db.Table("ETLTable", dynago.WithRegistry(reg))
	ctx := context.Background()

	// 3. Seed a pipeline.
	err := dynagotest.Seed(ctx, table, []any{
		Pipeline{
			PK: "PIPELINE#daily-sales", SK: "#METADATA",
			Name: "Daily Sales Import", CreatedAt: time.Now(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 4. Start a run using a write transaction.
	run := PipelineRun{
		PK: "PIPELINE#daily-sales", SK: "RUN#2025-06-15T10:00:00Z#run-001",
		RunID: "run-001", Status: "RUNNING",
		GSI1PK: "STATUS#RUNNING", GSI1SK: "RUN#2025-06-15T10:00:00Z",
		StartedAt: time.Now(), Version: 1,
	}

	err = dynago.WriteTx(ctx, db).
		Put(table, run, dynago.IfNotExists("PK")).
		Put(table, Step{
			PK: "RUN#run-001", SK: "STEP#001#extract",
			StepID: "extract", Name: "Extract from S3", Order: 1, Status: "PENDING",
		}).
		Run()
	if err != nil {
		t.Fatal(err)
	}

	// 5. Assert the run was created with the correct attributes.
	dynagotest.AssertItemExists(t, table,
		dynago.StringPairKey("PK", "PIPELINE#daily-sales",
			"SK", "RUN#2025-06-15T10:00:00Z#run-001"),
		dynagotest.HasAttribute("Status", "RUNNING"),
		dynagotest.HasAttribute("RunID", "run-001"),
	)

	// 6. Assert the step was created.
	dynagotest.AssertItemExists(t, table,
		dynago.StringPairKey("PK", "RUN#run-001", "SK", "STEP#001#extract"),
		dynagotest.HasAttribute("Name", "Extract from S3"),
	)

	// 7. Query runs for the pipeline and assert we get exactly one.
	dynagotest.AssertCount(t, table,
		dynago.Partition("PK", "PIPELINE#daily-sales").SortBeginsWith("SK", "RUN#"),
		1,
	)
}
```

Key testing patterns:

- **`memdb.New()`** creates an in-memory backend that implements
  `dynago.Backend`. No AWS credentials, no network calls, sub-millisecond
  operations.
- **`CreateTable`** defines the key schema including GSIs, mirroring your
  production table design. The `RangeKey` field is a pointer because range keys
  are optional.
- **`dynagotest.Seed`** puts each item sequentially using `table.Put`. It is a
  convenience for test setup.
- **`dynagotest.AssertItemExists`** fetches the item by key and validates
  attributes in one call. The test fails immediately with a clear message if any
  check fails.
- **`dynagotest.HasAttribute("Status", "RUNNING")`** adds an attribute assertion
  to the existence check.
- **`dynagotest.AssertCount`** queries with a key condition and asserts the
  number of results matches the expected count.

---

## Summary

| Access Pattern | API |
|---|---|
| Create pipeline | `table.Put(ctx, item, IfNotExists("PK"))` |
| Get pipeline | `Get[Pipeline](ctx, table, key)` |
| Start run atomically | `WriteTx(ctx, db).Put(...).Put(...).Run()` |
| List runs (newest first) | `Query[PipelineRun]` + `SortBeginsWith` + `ScanForward(false)` |
| Runs in date range | `Query[PipelineRun]` + `SortBetween` |
| Steps for a run | `Query[Step]` + `SortBeginsWith` |
| Runs by status (GSI) | `Query[PipelineRun]` + `QueryIndex("GSI1")` |
| Update step progress | `table.Update` + `Set` / `Add` + `IfCondition` |
| Complete run safely | `ReadModifyWrite[PipelineRun]` + `OptimisticLock` |
| Bulk insert metrics | `table.BatchPut` + `MaxConcurrency` + `OnProgress` |
| Full run collection | `QueryCollection` + `ItemsOf[T]` |
| Stream run items | `CollectionIter` / `QueryIter[T]` |
| Fetch multiple runs | `ReadTx(ctx, db).Get(...).Run()` + `GetAs[T]` |
| Delete with condition | `table.Delete` + `DeleteCondition` |
| Bulk delete old runs | `table.BatchDelete` + `MaxConcurrency` |
| Parse composite keys | `SplitKey(key, "#")` |
| Mixed transaction | `WriteTx` + `.Update` + `.Put` + `.Check` |
| Test everything | `memdb.New()` + `dynagotest.Seed` + `dynagotest.AssertItemExists` |
