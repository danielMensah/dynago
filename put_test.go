package dynago

import (
	"context"
	"errors"
	"testing"
)

// putDeleteStub implements Backend for testing Put and Delete operations.
type putDeleteStub struct {
	putReq  *PutItemRequest
	putResp *PutItemResponse
	putErr  error
	delReq  *DeleteItemRequest
	delResp *DeleteItemResponse
	delErr  error
}

func (s *putDeleteStub) PutItem(_ context.Context, req *PutItemRequest) (*PutItemResponse, error) {
	s.putReq = req
	if s.putErr != nil {
		return nil, s.putErr
	}
	if s.putResp != nil {
		return s.putResp, nil
	}
	return &PutItemResponse{}, nil
}

func (s *putDeleteStub) DeleteItem(_ context.Context, req *DeleteItemRequest) (*DeleteItemResponse, error) {
	s.delReq = req
	if s.delErr != nil {
		return nil, s.delErr
	}
	if s.delResp != nil {
		return s.delResp, nil
	}
	return &DeleteItemResponse{}, nil
}

func (s *putDeleteStub) GetItem(context.Context, *GetItemRequest) (*GetItemResponse, error) {
	panic("not implemented")
}
func (s *putDeleteStub) UpdateItem(context.Context, *UpdateItemRequest) (*UpdateItemResponse, error) {
	panic("not implemented")
}
func (s *putDeleteStub) Query(context.Context, *QueryRequest) (*QueryResponse, error) {
	panic("not implemented")
}
func (s *putDeleteStub) Scan(context.Context, *ScanRequest) (*ScanResponse, error) {
	panic("not implemented")
}
func (s *putDeleteStub) BatchGetItem(context.Context, *BatchGetItemRequest) (*BatchGetItemResponse, error) {
	panic("not implemented")
}
func (s *putDeleteStub) BatchWriteItem(context.Context, *BatchWriteItemRequest) (*BatchWriteItemResponse, error) {
	panic("not implemented")
}
func (s *putDeleteStub) TransactGetItems(context.Context, *TransactGetItemsRequest) (*TransactGetItemsResponse, error) {
	panic("not implemented")
}
func (s *putDeleteStub) TransactWriteItems(context.Context, *TransactWriteItemsRequest) (*TransactWriteItemsResponse, error) {
	panic("not implemented")
}

type testItem struct {
	PK   string `dynamo:"PK"`
	SK   string `dynamo:"SK"`
	Name string `dynamo:"Name"`
}

func TestPut_Basic(t *testing.T) {
	sb := &putDeleteStub{}
	db := New(sb)
	tbl := db.Table("MyTable")

	item := testItem{PK: "user#1", SK: "profile", Name: "Alice"}
	err := tbl.Put(context.Background(), item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sb.putReq == nil {
		t.Fatal("expected PutItem to be called")
	}
	if sb.putReq.TableName != "MyTable" {
		t.Fatalf("expected table name 'MyTable', got %q", sb.putReq.TableName)
	}
	if sb.putReq.Item == nil {
		t.Fatal("expected item to be set")
	}
	if sb.putReq.Item["PK"].S != "user#1" {
		t.Fatalf("expected PK 'user#1', got %q", sb.putReq.Item["PK"].S)
	}
	if sb.putReq.Item["Name"].S != "Alice" {
		t.Fatalf("expected Name 'Alice', got %q", sb.putReq.Item["Name"].S)
	}
	if sb.putReq.ConditionExpression != "" {
		t.Fatalf("expected no condition expression, got %q", sb.putReq.ConditionExpression)
	}
}

func TestPut_IfNotExists(t *testing.T) {
	sb := &putDeleteStub{}
	db := New(sb)
	tbl := db.Table("MyTable")

	item := testItem{PK: "user#1", SK: "profile", Name: "Alice"}
	err := tbl.Put(context.Background(), item, IfNotExists("PK"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sb.putReq.ConditionExpression == "" {
		t.Fatal("expected condition expression to be set")
	}
	if sb.putReq.ExpressionAttributeNames == nil {
		t.Fatal("expected expression attribute names to be set")
	}
	want := "attribute_not_exists(#PK)"
	if sb.putReq.ConditionExpression != want {
		t.Fatalf("expected condition %q, got %q", want, sb.putReq.ConditionExpression)
	}
	if sb.putReq.ExpressionAttributeNames["#PK"] != "PK" {
		t.Fatalf("expected #PK -> PK, got %v", sb.putReq.ExpressionAttributeNames)
	}
}

func TestPut_Condition(t *testing.T) {
	sb := &putDeleteStub{}
	db := New(sb)
	tbl := db.Table("MyTable")

	item := testItem{PK: "user#1", SK: "profile", Name: "Alice"}
	err := tbl.Put(context.Background(), item, PutCondition("Version = ?", 1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sb.putReq.ConditionExpression == "" {
		t.Fatal("expected condition expression to be set")
	}
	if sb.putReq.ExpressionAttributeValues == nil {
		t.Fatal("expected expression attribute values to be set")
	}
	v, ok := sb.putReq.ExpressionAttributeValues[":v0"]
	if !ok {
		t.Fatal("expected :v0 in expression attribute values")
	}
	if v.Type != TypeN || v.N != "1" {
		t.Fatalf("expected N=1, got type=%d N=%q", v.Type, v.N)
	}
}

func TestPut_ConditionFailed(t *testing.T) {
	sb := &putDeleteStub{
		putErr: &Error{Sentinel: ErrConditionFailed, Message: "condition not satisfied"},
	}
	db := New(sb)
	tbl := db.Table("MyTable")

	item := testItem{PK: "user#1", SK: "profile", Name: "Alice"}
	err := tbl.Put(context.Background(), item, IfNotExists("PK"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrConditionFailed) {
		t.Fatalf("expected ErrConditionFailed, got %v", err)
	}
}

func TestPut_MarshalError(t *testing.T) {
	sb := &putDeleteStub{}
	db := New(sb)
	tbl := db.Table("MyTable")

	err := tbl.Put(context.Background(), "not a struct")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestPut_BackendError(t *testing.T) {
	sb := &putDeleteStub{
		putErr: errors.New("connection timeout"),
	}
	db := New(sb)
	tbl := db.Table("MyTable")

	item := testItem{PK: "user#1", SK: "profile", Name: "Alice"}
	err := tbl.Put(context.Background(), item)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "connection timeout" {
		t.Fatalf("expected 'connection timeout', got %q", err.Error())
	}
}

func TestPut_IfNotExistsAndCondition(t *testing.T) {
	sb := &putDeleteStub{}
	db := New(sb)
	tbl := db.Table("MyTable")

	item := testItem{PK: "user#1", SK: "profile", Name: "Alice"}
	err := tbl.Put(context.Background(), item, IfNotExists("PK"), PutCondition("Version = ?", 1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sb.putReq.ConditionExpression == "" {
		t.Fatal("expected condition expression to be set")
	}
	if _, ok := sb.putReq.ExpressionAttributeNames["#PK"]; !ok {
		t.Fatal("expected #PK in expression attribute names")
	}
	if _, ok := sb.putReq.ExpressionAttributeValues[":v0"]; !ok {
		t.Fatal("expected :v0 in expression attribute values")
	}
}
