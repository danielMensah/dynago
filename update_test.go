package dynago

import (
	"context"
	"errors"
	"testing"
)

// updateStub implements Backend for testing Update operations.
type updateStub struct {
	req  *UpdateItemRequest
	resp *UpdateItemResponse
	err  error
}

func (s *updateStub) UpdateItem(_ context.Context, req *UpdateItemRequest) (*UpdateItemResponse, error) {
	s.req = req
	if s.err != nil {
		return nil, s.err
	}
	if s.resp != nil {
		return s.resp, nil
	}
	return &UpdateItemResponse{}, nil
}

func (s *updateStub) GetItem(context.Context, *GetItemRequest) (*GetItemResponse, error) {
	panic("not implemented")
}
func (s *updateStub) PutItem(context.Context, *PutItemRequest) (*PutItemResponse, error) {
	panic("not implemented")
}
func (s *updateStub) DeleteItem(context.Context, *DeleteItemRequest) (*DeleteItemResponse, error) {
	panic("not implemented")
}
func (s *updateStub) Query(context.Context, *QueryRequest) (*QueryResponse, error) {
	panic("not implemented")
}
func (s *updateStub) Scan(context.Context, *ScanRequest) (*ScanResponse, error) {
	panic("not implemented")
}
func (s *updateStub) BatchGetItem(context.Context, *BatchGetItemRequest) (*BatchGetItemResponse, error) {
	panic("not implemented")
}
func (s *updateStub) BatchWriteItem(context.Context, *BatchWriteItemRequest) (*BatchWriteItemResponse, error) {
	panic("not implemented")
}
func (s *updateStub) TransactGetItems(context.Context, *TransactGetItemsRequest) (*TransactGetItemsResponse, error) {
	panic("not implemented")
}
func (s *updateStub) TransactWriteItems(context.Context, *TransactWriteItemsRequest) (*TransactWriteItemsResponse, error) {
	panic("not implemented")
}

type updateTestItem struct {
	PK   string `dynamo:"PK"`
	Name string `dynamo:"Name"`
	Age  int    `dynamo:"Age"`
}

func TestUpdate_SET(t *testing.T) {
	stub := &updateStub{}
	db := New(stub)
	tbl := db.Table("TestTable")

	err := tbl.Update(context.Background(), Key("PK", "user#1"),
		Set("Name", "Alice"),
		Set("Age", 30),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.req == nil {
		t.Fatal("expected request to be captured")
	}
	if stub.req.TableName != "TestTable" {
		t.Errorf("expected table TestTable, got %s", stub.req.TableName)
	}
	if stub.req.ReturnValues != "NONE" {
		t.Errorf("expected ReturnValues NONE, got %s", stub.req.ReturnValues)
	}
	if stub.req.UpdateExpression == "" {
		t.Error("expected non-empty UpdateExpression")
	}
}

func TestUpdate_ADD(t *testing.T) {
	stub := &updateStub{}
	db := New(stub)
	tbl := db.Table("TestTable")

	err := tbl.Update(context.Background(), Key("PK", "user#1"),
		Add("LoginCount", 1),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.req.UpdateExpression == "" {
		t.Error("expected non-empty UpdateExpression")
	}
}

func TestUpdate_REMOVE(t *testing.T) {
	stub := &updateStub{}
	db := New(stub)
	tbl := db.Table("TestTable")

	err := tbl.Update(context.Background(), Key("PK", "user#1"),
		Remove("OldField"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.req.UpdateExpression == "" {
		t.Error("expected non-empty UpdateExpression")
	}
}

func TestUpdate_DELETE(t *testing.T) {
	stub := &updateStub{}
	db := New(stub)
	tbl := db.Table("TestTable")

	err := tbl.Update(context.Background(), Key("PK", "user#1"),
		Delete("Tags", []string{"old"}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.req.UpdateExpression == "" {
		t.Error("expected non-empty UpdateExpression")
	}
}

func TestUpdate_MixedActions(t *testing.T) {
	stub := &updateStub{}
	db := New(stub)
	tbl := db.Table("TestTable")

	err := tbl.Update(context.Background(), Key("PK", "user#1"),
		Set("Name", "Bob"),
		Add("Count", 1),
		Remove("Deprecated"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.req.UpdateExpression == "" {
		t.Error("expected non-empty UpdateExpression")
	}
}

func TestUpdate_ConditionFailed(t *testing.T) {
	stub := &updateStub{
		err: ErrConditionFailed,
	}
	db := New(stub)
	tbl := db.Table("TestTable")

	err := tbl.Update(context.Background(), Key("PK", "user#1"),
		Set("Name", "Alice"),
		IfCondition("Version = ?", 5),
	)
	if !errors.Is(err, ErrConditionFailed) {
		t.Errorf("expected ErrConditionFailed, got %v", err)
	}
}

func TestUpdate_NoActions(t *testing.T) {
	stub := &updateStub{}
	db := New(stub)
	tbl := db.Table("TestTable")

	err := tbl.Update(context.Background(), Key("PK", "user#1"))
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation for no actions, got %v", err)
	}
}

func TestUpdate_ConditionExpression(t *testing.T) {
	stub := &updateStub{}
	db := New(stub)
	tbl := db.Table("TestTable")

	err := tbl.Update(context.Background(), Key("PK", "user#1"),
		Set("Name", "Alice"),
		IfCondition("Version = ?", 5),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.req.ConditionExpression == "" {
		t.Error("expected non-empty ConditionExpression")
	}
}

func TestUpdateReturning_ReturnNew(t *testing.T) {
	stub := &updateStub{
		resp: &UpdateItemResponse{
			Attributes: map[string]AttributeValue{
				"PK":   {Type: TypeS, S: "user#1"},
				"Name": {Type: TypeS, S: "Alice"},
				"Age":  {Type: TypeN, N: "31"},
			},
		},
	}
	db := New(stub)
	tbl := db.Table("TestTable")

	result, err := UpdateReturning[updateTestItem](context.Background(), tbl, Key("PK", "user#1"),
		Set("Name", "Alice"),
		Set("Age", 31),
		ReturnNew(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "Alice" {
		t.Errorf("expected Name Alice, got %s", result.Name)
	}
	if result.Age != 31 {
		t.Errorf("expected Age 31, got %d", result.Age)
	}
	if stub.req.ReturnValues != "ALL_NEW" {
		t.Errorf("expected ReturnValues ALL_NEW, got %s", stub.req.ReturnValues)
	}
}

func TestUpdateReturning_ReturnOld(t *testing.T) {
	stub := &updateStub{
		resp: &UpdateItemResponse{
			Attributes: map[string]AttributeValue{
				"PK":   {Type: TypeS, S: "user#1"},
				"Name": {Type: TypeS, S: "OldName"},
				"Age":  {Type: TypeN, N: "30"},
			},
		},
	}
	db := New(stub)
	tbl := db.Table("TestTable")

	result, err := UpdateReturning[updateTestItem](context.Background(), tbl, Key("PK", "user#1"),
		Set("Name", "NewName"),
		ReturnOld(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "OldName" {
		t.Errorf("expected Name OldName, got %s", result.Name)
	}
	if stub.req.ReturnValues != "ALL_OLD" {
		t.Errorf("expected ReturnValues ALL_OLD, got %s", stub.req.ReturnValues)
	}
}

func TestUpdateReturning_MissingReturnOption(t *testing.T) {
	stub := &updateStub{}
	db := New(stub)
	tbl := db.Table("TestTable")

	_, err := UpdateReturning[updateTestItem](context.Background(), tbl, Key("PK", "user#1"),
		Set("Name", "Alice"),
	)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation for missing return option, got %v", err)
	}
}

func TestUpdateReturning_NoActions(t *testing.T) {
	stub := &updateStub{}
	db := New(stub)
	tbl := db.Table("TestTable")

	_, err := UpdateReturning[updateTestItem](context.Background(), tbl, Key("PK", "user#1"),
		ReturnNew(),
	)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation for no actions, got %v", err)
	}
}

func TestUpdateReturning_ConditionFailed(t *testing.T) {
	stub := &updateStub{
		err: ErrConditionFailed,
	}
	db := New(stub)
	tbl := db.Table("TestTable")

	_, err := UpdateReturning[updateTestItem](context.Background(), tbl, Key("PK", "user#1"),
		Set("Name", "Alice"),
		IfCondition("Version = ?", 5),
		ReturnNew(),
	)
	if !errors.Is(err, ErrConditionFailed) {
		t.Errorf("expected ErrConditionFailed, got %v", err)
	}
}
