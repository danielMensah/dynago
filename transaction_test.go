package dynago

import (
	"context"
	"errors"
	"testing"
)

// txStub implements Backend for testing transaction operations.
type txStub struct {
	writeReq  *TransactWriteItemsRequest
	writeResp *TransactWriteItemsResponse
	writeErr  error

	getReq  *TransactGetItemsRequest
	getResp *TransactGetItemsResponse
	getErr  error
}

func (s *txStub) TransactWriteItems(_ context.Context, req *TransactWriteItemsRequest) (*TransactWriteItemsResponse, error) {
	s.writeReq = req
	if s.writeErr != nil {
		return nil, s.writeErr
	}
	if s.writeResp != nil {
		return s.writeResp, nil
	}
	return &TransactWriteItemsResponse{}, nil
}

func (s *txStub) TransactGetItems(_ context.Context, req *TransactGetItemsRequest) (*TransactGetItemsResponse, error) {
	s.getReq = req
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.getResp != nil {
		return s.getResp, nil
	}
	return &TransactGetItemsResponse{}, nil
}

func (s *txStub) GetItem(context.Context, *GetItemRequest) (*GetItemResponse, error) {
	panic("not implemented")
}
func (s *txStub) PutItem(context.Context, *PutItemRequest) (*PutItemResponse, error) {
	panic("not implemented")
}
func (s *txStub) DeleteItem(context.Context, *DeleteItemRequest) (*DeleteItemResponse, error) {
	panic("not implemented")
}
func (s *txStub) UpdateItem(context.Context, *UpdateItemRequest) (*UpdateItemResponse, error) {
	panic("not implemented")
}
func (s *txStub) Query(context.Context, *QueryRequest) (*QueryResponse, error) {
	panic("not implemented")
}
func (s *txStub) Scan(context.Context, *ScanRequest) (*ScanResponse, error) {
	panic("not implemented")
}
func (s *txStub) BatchGetItem(context.Context, *BatchGetItemRequest) (*BatchGetItemResponse, error) {
	panic("not implemented")
}
func (s *txStub) BatchWriteItem(context.Context, *BatchWriteItemRequest) (*BatchWriteItemResponse, error) {
	panic("not implemented")
}

// ---------------------------------------------------------------------------
// WriteTx tests
// ---------------------------------------------------------------------------

func TestWriteTx_Put(t *testing.T) {
	sb := &txStub{}
	db := New(sb)
	tbl := db.Table("Users")

	item := testItem{PK: "user#1", SK: "profile", Name: "Alice"}
	err := WriteTx(context.Background(), db).
		Put(tbl, item).
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sb.writeReq == nil {
		t.Fatal("expected TransactWriteItems to be called")
	}
	if len(sb.writeReq.TransactItems) != 1 {
		t.Fatalf("expected 1 item, got %d", len(sb.writeReq.TransactItems))
	}
	p := sb.writeReq.TransactItems[0].Put
	if p == nil {
		t.Fatal("expected Put to be set")
	}
	if p.TableName != "Users" {
		t.Fatalf("expected table 'Users', got %q", p.TableName)
	}
	if p.Item["PK"].S != "user#1" {
		t.Fatalf("expected PK 'user#1', got %q", p.Item["PK"].S)
	}
}

func TestWriteTx_PutWithCondition(t *testing.T) {
	sb := &txStub{}
	db := New(sb)
	tbl := db.Table("Users")

	item := testItem{PK: "user#1", SK: "profile", Name: "Alice"}
	err := WriteTx(context.Background(), db).
		Put(tbl, item, IfNotExists("PK")).
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p := sb.writeReq.TransactItems[0].Put
	if p.ConditionExpression == "" {
		t.Fatal("expected condition expression")
	}
	if p.ExpressionAttributeNames["#PK"] != "PK" {
		t.Fatal("expected #PK name alias")
	}
}

func TestWriteTx_Update(t *testing.T) {
	sb := &txStub{}
	db := New(sb)
	tbl := db.Table("Users")

	err := WriteTx(context.Background(), db).
		Update(tbl, Key("PK", "user#1", "SK", "profile"), Set("Name", "Bob")).
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := sb.writeReq.TransactItems[0].Update
	if u == nil {
		t.Fatal("expected Update to be set")
	}
	if u.UpdateExpression == "" {
		t.Fatal("expected update expression")
	}
	if u.TableName != "Users" {
		t.Fatalf("expected table 'Users', got %q", u.TableName)
	}
}

func TestWriteTx_Delete(t *testing.T) {
	sb := &txStub{}
	db := New(sb)
	tbl := db.Table("Users")

	err := WriteTx(context.Background(), db).
		Delete(tbl, Key("PK", "user#1", "SK", "profile")).
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d := sb.writeReq.TransactItems[0].Delete
	if d == nil {
		t.Fatal("expected Delete to be set")
	}
	if d.TableName != "Users" {
		t.Fatalf("expected table 'Users', got %q", d.TableName)
	}
}

func TestWriteTx_DeleteWithCondition(t *testing.T) {
	sb := &txStub{}
	db := New(sb)
	tbl := db.Table("Users")

	err := WriteTx(context.Background(), db).
		Delete(tbl, Key("PK", "user#1", "SK", "profile"), DeleteCondition("Version = ?", 1)).
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d := sb.writeReq.TransactItems[0].Delete
	if d.ConditionExpression == "" {
		t.Fatal("expected condition expression")
	}
}

func TestWriteTx_Check(t *testing.T) {
	sb := &txStub{}
	db := New(sb)
	tbl := db.Table("Users")

	err := WriteTx(context.Background(), db).
		Check(tbl, Key("PK", "user#1", "SK", "profile"), "attribute_exists(PK)").
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c := sb.writeReq.TransactItems[0].ConditionCheck
	if c == nil {
		t.Fatal("expected ConditionCheck to be set")
	}
	if c.ConditionExpression == "" {
		t.Fatal("expected condition expression")
	}
}

func TestWriteTx_MultipleOperations(t *testing.T) {
	sb := &txStub{}
	db := New(sb)
	tbl := db.Table("Users")

	item := testItem{PK: "user#1", SK: "profile", Name: "Alice"}
	err := WriteTx(context.Background(), db).
		Put(tbl, item).
		Update(tbl, Key("PK", "user#2", "SK", "profile"), Set("Name", "Bob")).
		Delete(tbl, Key("PK", "user#3", "SK", "profile")).
		Check(tbl, Key("PK", "user#4", "SK", "profile"), "attribute_exists(PK)").
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sb.writeReq.TransactItems) != 4 {
		t.Fatalf("expected 4 items, got %d", len(sb.writeReq.TransactItems))
	}
}

func TestWriteTx_EmptyReturnsError(t *testing.T) {
	sb := &txStub{}
	db := New(sb)

	err := WriteTx(context.Background(), db).Run()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestWriteTx_ExceedsMaxOperations(t *testing.T) {
	sb := &txStub{}
	db := New(sb)
	tbl := db.Table("Users")

	b := WriteTx(context.Background(), db)
	for i := 0; i <= maxTransactItems; i++ {
		b.Delete(tbl, Key("PK", "user"))
	}

	err := b.Run()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestWriteTx_BackendError(t *testing.T) {
	sb := &txStub{writeErr: errors.New("connection timeout")}
	db := New(sb)
	tbl := db.Table("Users")

	err := WriteTx(context.Background(), db).
		Delete(tbl, Key("PK", "user#1")).
		Run()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "connection timeout" {
		t.Fatalf("expected 'connection timeout', got %q", err.Error())
	}
}

func TestWriteTx_MarshalError(t *testing.T) {
	sb := &txStub{}
	db := New(sb)
	tbl := db.Table("Users")

	err := WriteTx(context.Background(), db).
		Put(tbl, "not a struct").
		Run()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWriteTx_UpdateNoClauses(t *testing.T) {
	sb := &txStub{}
	db := New(sb)
	tbl := db.Table("Users")

	err := WriteTx(context.Background(), db).
		Update(tbl, Key("PK", "user#1")).
		Run()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ReadTx tests
// ---------------------------------------------------------------------------

func TestReadTx_Get(t *testing.T) {
	sb := &txStub{
		getResp: &TransactGetItemsResponse{
			Responses: []map[string]AttributeValue{
				{
					"PK":   {Type: TypeS, S: "user#1"},
					"SK":   {Type: TypeS, S: "profile"},
					"Name": {Type: TypeS, S: "Alice"},
				},
			},
		},
	}
	db := New(sb)
	tbl := db.Table("Users")

	result, err := ReadTx(context.Background(), db).
		Get(tbl, Key("PK", "user#1", "SK", "profile")).
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sb.getReq == nil {
		t.Fatal("expected TransactGetItems to be called")
	}
	if len(sb.getReq.TransactItems) != 1 {
		t.Fatalf("expected 1 item, got %d", len(sb.getReq.TransactItems))
	}

	item, ok := result.Item(0)
	if !ok {
		t.Fatal("expected item at index 0")
	}
	if item["Name"].S != "Alice" {
		t.Fatalf("expected Name 'Alice', got %q", item["Name"].S)
	}
}

func TestReadTx_GetWithProjection(t *testing.T) {
	sb := &txStub{
		getResp: &TransactGetItemsResponse{
			Responses: []map[string]AttributeValue{
				{"PK": {Type: TypeS, S: "user#1"}},
			},
		},
	}
	db := New(sb)
	tbl := db.Table("Users")

	_, err := ReadTx(context.Background(), db).
		Get(tbl, Key("PK", "user#1"), Project("PK", "Name")).
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	item := sb.getReq.TransactItems[0]
	if item.ProjectionExpression == "" {
		t.Fatal("expected projection expression")
	}
}

func TestReadTx_ItemOutOfRange(t *testing.T) {
	sb := &txStub{
		getResp: &TransactGetItemsResponse{
			Responses: []map[string]AttributeValue{},
		},
	}
	db := New(sb)
	tbl := db.Table("Users")

	result, err := ReadTx(context.Background(), db).
		Get(tbl, Key("PK", "user#1")).
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := result.Item(0)
	if ok {
		t.Fatal("expected false for out-of-range index")
	}

	_, ok = result.Item(-1)
	if ok {
		t.Fatal("expected false for negative index")
	}
}

func TestReadTx_ItemNilResponse(t *testing.T) {
	sb := &txStub{
		getResp: &TransactGetItemsResponse{
			Responses: []map[string]AttributeValue{nil},
		},
	}
	db := New(sb)
	tbl := db.Table("Users")

	result, err := ReadTx(context.Background(), db).
		Get(tbl, Key("PK", "user#1")).
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := result.Item(0)
	if ok {
		t.Fatal("expected false for nil item")
	}
}

func TestReadTx_GetAs(t *testing.T) {
	sb := &txStub{
		getResp: &TransactGetItemsResponse{
			Responses: []map[string]AttributeValue{
				{
					"PK":   {Type: TypeS, S: "user#1"},
					"SK":   {Type: TypeS, S: "profile"},
					"Name": {Type: TypeS, S: "Alice"},
				},
			},
		},
	}
	db := New(sb)
	tbl := db.Table("Users")

	result, err := ReadTx(context.Background(), db).
		Get(tbl, Key("PK", "user#1", "SK", "profile")).
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	item, err := GetAs[testItem](result, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.PK != "user#1" {
		t.Fatalf("expected PK 'user#1', got %q", item.PK)
	}
	if item.Name != "Alice" {
		t.Fatalf("expected Name 'Alice', got %q", item.Name)
	}
}

func TestReadTx_GetAsNotFound(t *testing.T) {
	sb := &txStub{
		getResp: &TransactGetItemsResponse{
			Responses: []map[string]AttributeValue{nil},
		},
	}
	db := New(sb)
	tbl := db.Table("Users")

	result, err := ReadTx(context.Background(), db).
		Get(tbl, Key("PK", "user#1")).
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = GetAs[testItem](result, 0)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestReadTx_EmptyReturnsError(t *testing.T) {
	sb := &txStub{}
	db := New(sb)

	_, err := ReadTx(context.Background(), db).Run()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestReadTx_ExceedsMaxOperations(t *testing.T) {
	sb := &txStub{}
	db := New(sb)
	tbl := db.Table("Users")

	b := ReadTx(context.Background(), db)
	for i := 0; i <= maxTransactItems; i++ {
		b.Get(tbl, Key("PK", "user"))
	}

	_, err := b.Run()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestReadTx_BackendError(t *testing.T) {
	sb := &txStub{getErr: errors.New("connection timeout")}
	db := New(sb)
	tbl := db.Table("Users")

	_, err := ReadTx(context.Background(), db).
		Get(tbl, Key("PK", "user#1")).
		Run()
	if err == nil {
		t.Fatal("expected error")
	}
}
