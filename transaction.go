package dynago

import (
	"context"
	"fmt"
)

const maxTransactItems = 100

// ---------------------------------------------------------------------------
// WriteTxBuilder
// ---------------------------------------------------------------------------

// WriteTxBuilder accumulates operations for a DynamoDB TransactWriteItems call.
type WriteTxBuilder struct {
	ctx   context.Context
	db    *DB
	items []TransactWriteItem
	err   error
}

// WriteTx creates a new write transaction builder.
func WriteTx(ctx context.Context, db *DB) *WriteTxBuilder {
	return &WriteTxBuilder{ctx: ctx, db: db}
}

// Put adds a put operation to the transaction. The item is marshalled and
// optional conditions are applied.
func (b *WriteTxBuilder) Put(t *Table, item any, opts ...PutOption) *WriteTxBuilder {
	if b.err != nil {
		return b
	}

	av, err := Marshal(item)
	if err != nil {
		b.err = err
		return b
	}

	var cfg putConfig
	for _, o := range opts {
		o(&cfg)
	}

	tp := &TransactPut{
		TableName: t.Name(),
		Item:      av,
	}

	if cfg.condition != nil {
		tp.ConditionExpression = cfg.condition.expression
		tp.ExpressionAttributeNames = cfg.condition.names
		tp.ExpressionAttributeValues = cfg.condition.values
	}

	b.items = append(b.items, TransactWriteItem{Put: tp})
	return b
}

// Update adds an update operation to the transaction.
func (b *WriteTxBuilder) Update(t *Table, key KeyValue, opts ...UpdateOption) *WriteTxBuilder {
	if b.err != nil {
		return b
	}

	var cfg updateConfig
	for _, o := range opts {
		o(&cfg)
	}

	if len(cfg.clauses) == 0 {
		b.err = newError(ErrValidation, "dynago.WriteTx.Update: at least one update action is required")
		return b
	}

	req := buildUpdateRequest(t.Name(), key, &cfg)

	tu := &TransactUpdate{
		TableName:                 req.TableName,
		Key:                       req.Key,
		UpdateExpression:          req.UpdateExpression,
		ExpressionAttributeNames:  req.ExpressionAttributeNames,
		ExpressionAttributeValues: req.ExpressionAttributeValues,
		ConditionExpression:       req.ConditionExpression,
	}

	b.items = append(b.items, TransactWriteItem{Update: tu})
	return b
}

// Delete adds a delete operation to the transaction.
func (b *WriteTxBuilder) Delete(t *Table, key KeyValue, opts ...DeleteOption) *WriteTxBuilder {
	if b.err != nil {
		return b
	}

	var cfg deleteConfig
	for _, o := range opts {
		o(&cfg)
	}

	td := &TransactDelete{
		TableName: t.Name(),
		Key:       key.Map(),
	}

	if cfg.condition != nil {
		td.ConditionExpression = cfg.condition.expression
		td.ExpressionAttributeNames = cfg.condition.names
		td.ExpressionAttributeValues = cfg.condition.values
	}

	b.items = append(b.items, TransactWriteItem{Delete: td})
	return b
}

// Check adds a condition check (no data modification) to the transaction.
func (b *WriteTxBuilder) Check(t *Table, key KeyValue, condition string, vals ...any) *WriteTxBuilder {
	if b.err != nil {
		return b
	}

	cond, err := buildCondition(condition, vals...)
	if err != nil {
		b.err = fmt.Errorf("dynago.WriteTx.Check: %w", err)
		return b
	}

	tc := &TransactConditionCheck{
		TableName:                 t.Name(),
		Key:                       key.Map(),
		ConditionExpression:       cond.expression,
		ExpressionAttributeNames:  cond.names,
		ExpressionAttributeValues: cond.values,
	}

	b.items = append(b.items, TransactWriteItem{ConditionCheck: tc})
	return b
}

// Run executes the write transaction.
func (b *WriteTxBuilder) Run() error {
	if b.err != nil {
		return b.err
	}

	if len(b.items) == 0 {
		return newError(ErrValidation, "dynago.WriteTx: at least one operation is required")
	}

	if len(b.items) > maxTransactItems {
		return newError(ErrValidation, fmt.Sprintf("dynago.WriteTx: transaction exceeds maximum of %d operations", maxTransactItems))
	}

	req := &TransactWriteItemsRequest{
		TransactItems: b.items,
	}

	_, err := b.db.backend.TransactWriteItems(b.ctx, req)
	return err
}

// ---------------------------------------------------------------------------
// ReadTxBuilder
// ---------------------------------------------------------------------------

// ReadTxBuilder accumulates operations for a DynamoDB TransactGetItems call.
type ReadTxBuilder struct {
	ctx   context.Context
	db    *DB
	items []TransactGetItem
}

// ReadTxResult holds the response from a read transaction.
type ReadTxResult struct {
	responses []map[string]AttributeValue
}

// ReadTx creates a new read transaction builder.
func ReadTx(ctx context.Context, db *DB) *ReadTxBuilder {
	return &ReadTxBuilder{ctx: ctx, db: db}
}

// Get adds a get operation to the transaction.
func (b *ReadTxBuilder) Get(t *Table, key KeyValue, opts ...GetOption) *ReadTxBuilder {
	var cfg getConfig
	for _, o := range opts {
		o(&cfg)
	}

	item := TransactGetItem{
		TableName: t.Name(),
		Key:       key.Map(),
	}

	if len(cfg.projection) > 0 {
		projExpr, names := buildProjection(cfg.projection)
		item.ProjectionExpression = projExpr
		item.ExpressionAttributeNames = names
	}

	b.items = append(b.items, item)
	return b
}

// Run executes the read transaction.
func (b *ReadTxBuilder) Run() (*ReadTxResult, error) {
	if len(b.items) == 0 {
		return nil, newError(ErrValidation, "dynago.ReadTx: at least one operation is required")
	}

	if len(b.items) > maxTransactItems {
		return nil, newError(ErrValidation, fmt.Sprintf("dynago.ReadTx: transaction exceeds maximum of %d operations", maxTransactItems))
	}

	req := &TransactGetItemsRequest{
		TransactItems: b.items,
	}

	resp, err := b.db.backend.TransactGetItems(b.ctx, req)
	if err != nil {
		return nil, err
	}

	return &ReadTxResult{responses: resp.Responses}, nil
}

// Item returns the raw item at the given index. If the index is out of range
// or the item is nil/empty, ok is false.
func (r *ReadTxResult) Item(index int) (map[string]AttributeValue, bool) {
	if index < 0 || index >= len(r.responses) {
		return nil, false
	}
	item := r.responses[index]
	if len(item) == 0 {
		return nil, false
	}
	return item, true
}

// GetAs unmarshals the item at the given index into type T.
func GetAs[T any](result *ReadTxResult, index int) (T, error) {
	var zero T
	item, ok := result.Item(index)
	if !ok {
		return zero, ErrNotFound
	}

	var out T
	if err := Unmarshal(item, &out); err != nil {
		return zero, err
	}
	return out, nil
}
