package dynago

import (
	"context"
	"errors"
	"testing"
)

type versionedItem struct {
	PK      string `dynamo:"PK"`
	SK      string `dynamo:"SK"`
	Name    string `dynamo:"Name"`
	Version int    `dynamo:"Version"`
}

// rmwStub implements Backend for testing ReadModifyWrite.
type rmwStub struct {
	getResp    *GetItemResponse
	getErr     error
	putReqs    []*PutItemRequest
	putErr     error
	putErrOnce int // fail on this attempt (0-based), then succeed
	putCalls   int
}

func (s *rmwStub) GetItem(_ context.Context, req *GetItemRequest) (*GetItemResponse, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.getResp != nil {
		return s.getResp, nil
	}
	return &GetItemResponse{}, nil
}

func (s *rmwStub) PutItem(_ context.Context, req *PutItemRequest) (*PutItemResponse, error) {
	s.putReqs = append(s.putReqs, req)
	call := s.putCalls
	s.putCalls++
	if s.putErrOnce >= 0 && call == s.putErrOnce {
		return nil, s.putErr
	}
	if s.putErrOnce < 0 && s.putErr != nil {
		return nil, s.putErr
	}
	return &PutItemResponse{}, nil
}

func (s *rmwStub) DeleteItem(context.Context, *DeleteItemRequest) (*DeleteItemResponse, error) {
	panic("not implemented")
}
func (s *rmwStub) UpdateItem(context.Context, *UpdateItemRequest) (*UpdateItemResponse, error) {
	panic("not implemented")
}
func (s *rmwStub) Query(context.Context, *QueryRequest) (*QueryResponse, error) {
	panic("not implemented")
}
func (s *rmwStub) Scan(context.Context, *ScanRequest) (*ScanResponse, error) {
	panic("not implemented")
}
func (s *rmwStub) BatchGetItem(context.Context, *BatchGetItemRequest) (*BatchGetItemResponse, error) {
	panic("not implemented")
}
func (s *rmwStub) BatchWriteItem(context.Context, *BatchWriteItemRequest) (*BatchWriteItemResponse, error) {
	panic("not implemented")
}
func (s *rmwStub) TransactGetItems(context.Context, *TransactGetItemsRequest) (*TransactGetItemsResponse, error) {
	panic("not implemented")
}
func (s *rmwStub) TransactWriteItems(context.Context, *TransactWriteItemsRequest) (*TransactWriteItemsResponse, error) {
	panic("not implemented")
}

func TestReadModifyWrite_Basic(t *testing.T) {
	sb := &rmwStub{
		getResp: &GetItemResponse{
			Item: map[string]AttributeValue{
				"PK":   {Type: TypeS, S: "user#1"},
				"SK":   {Type: TypeS, S: "profile"},
				"Name": {Type: TypeS, S: "Alice"},
			},
		},
		putErrOnce: -1, // never fail
	}
	db := New(sb)
	tbl := db.Table("Users")

	err := ReadModifyWrite[testItem](context.Background(), tbl, Key("PK", "user#1", "SK", "profile"), func(item *testItem) error {
		item.Name = "Bob"
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sb.putReqs) != 1 {
		t.Fatalf("expected 1 put call, got %d", len(sb.putReqs))
	}
	if sb.putReqs[0].Item["Name"].S != "Bob" {
		t.Fatalf("expected Name 'Bob', got %q", sb.putReqs[0].Item["Name"].S)
	}
	// No condition expression without optimistic locking
	if sb.putReqs[0].ConditionExpression != "" {
		t.Fatalf("expected no condition expression, got %q", sb.putReqs[0].ConditionExpression)
	}
}

func TestReadModifyWrite_WithOptimisticLock(t *testing.T) {
	sb := &rmwStub{
		getResp: &GetItemResponse{
			Item: map[string]AttributeValue{
				"PK":      {Type: TypeS, S: "user#1"},
				"SK":      {Type: TypeS, S: "profile"},
				"Name":    {Type: TypeS, S: "Alice"},
				"Version": {Type: TypeN, N: "5"},
			},
		},
		putErrOnce: -1,
	}
	db := New(sb)
	tbl := db.Table("Users")

	err := ReadModifyWrite[versionedItem](context.Background(), tbl, Key("PK", "user#1", "SK", "profile"), func(item *versionedItem) error {
		item.Name = "Bob"
		return nil
	}, OptimisticLock("Version"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sb.putReqs) != 1 {
		t.Fatalf("expected 1 put call, got %d", len(sb.putReqs))
	}
	req := sb.putReqs[0]
	if req.ConditionExpression == "" {
		t.Fatal("expected condition expression for optimistic lock")
	}
	// Version in the written item should be incremented to 6
	if req.Item["Version"].N != "6" {
		t.Fatalf("expected Version '6', got %q", req.Item["Version"].N)
	}
}

func TestReadModifyWrite_RetryOnConditionFailure(t *testing.T) {
	sb := &rmwStub{
		getResp: &GetItemResponse{
			Item: map[string]AttributeValue{
				"PK":      {Type: TypeS, S: "user#1"},
				"SK":      {Type: TypeS, S: "profile"},
				"Name":    {Type: TypeS, S: "Alice"},
				"Version": {Type: TypeN, N: "5"},
			},
		},
		putErr:     &Error{Sentinel: ErrConditionFailed, Message: "version mismatch"},
		putErrOnce: 0, // fail first attempt, succeed after
	}
	db := New(sb)
	tbl := db.Table("Users")

	err := ReadModifyWrite[versionedItem](context.Background(), tbl, Key("PK", "user#1", "SK", "profile"), func(item *versionedItem) error {
		item.Name = "Bob"
		return nil
	}, OptimisticLock("Version"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sb.putCalls != 2 {
		t.Fatalf("expected 2 put calls (1 failed + 1 success), got %d", sb.putCalls)
	}
}

func TestReadModifyWrite_ExhaustsRetries(t *testing.T) {
	sb := &rmwStub{
		getResp: &GetItemResponse{
			Item: map[string]AttributeValue{
				"PK":      {Type: TypeS, S: "user#1"},
				"SK":      {Type: TypeS, S: "profile"},
				"Name":    {Type: TypeS, S: "Alice"},
				"Version": {Type: TypeN, N: "5"},
			},
		},
		putErr:     &Error{Sentinel: ErrConditionFailed, Message: "version mismatch"},
		putErrOnce: -1, // always fail
	}
	db := New(sb)
	tbl := db.Table("Users")

	err := ReadModifyWrite[versionedItem](context.Background(), tbl, Key("PK", "user#1", "SK", "profile"), func(item *versionedItem) error {
		item.Name = "Bob"
		return nil
	}, OptimisticLock("Version"), MaxRetries(2))
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if !errors.Is(err, ErrConditionFailed) {
		t.Fatalf("expected ErrConditionFailed, got %v", err)
	}
	// 2 retries + 1 initial = 3 attempts
	if sb.putCalls != 3 {
		t.Fatalf("expected 3 put calls, got %d", sb.putCalls)
	}
}

func TestReadModifyWrite_FnReturnsError(t *testing.T) {
	sb := &rmwStub{
		getResp: &GetItemResponse{
			Item: map[string]AttributeValue{
				"PK":   {Type: TypeS, S: "user#1"},
				"SK":   {Type: TypeS, S: "profile"},
				"Name": {Type: TypeS, S: "Alice"},
			},
		},
		putErrOnce: -1,
	}
	db := New(sb)
	tbl := db.Table("Users")

	fnErr := errors.New("business logic error")
	err := ReadModifyWrite[testItem](context.Background(), tbl, Key("PK", "user#1", "SK", "profile"), func(item *testItem) error {
		return fnErr
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, fnErr) {
		t.Fatalf("expected fn error, got %v", err)
	}
	if len(sb.putReqs) != 0 {
		t.Fatalf("expected no put calls when fn fails, got %d", len(sb.putReqs))
	}
}

func TestReadModifyWrite_GetError(t *testing.T) {
	sb := &rmwStub{
		getErr:     errors.New("get failed"),
		putErrOnce: -1,
	}
	db := New(sb)
	tbl := db.Table("Users")

	err := ReadModifyWrite[testItem](context.Background(), tbl, Key("PK", "user#1"), func(item *testItem) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadModifyWrite_NonConditionPutError(t *testing.T) {
	sb := &rmwStub{
		getResp: &GetItemResponse{
			Item: map[string]AttributeValue{
				"PK":      {Type: TypeS, S: "user#1"},
				"SK":      {Type: TypeS, S: "profile"},
				"Version": {Type: TypeN, N: "1"},
			},
		},
		putErr:     errors.New("network error"),
		putErrOnce: -1,
	}
	db := New(sb)
	tbl := db.Table("Users")

	err := ReadModifyWrite[versionedItem](context.Background(), tbl, Key("PK", "user#1", "SK", "profile"), func(item *versionedItem) error {
		return nil
	}, OptimisticLock("Version"))
	if err == nil {
		t.Fatal("expected error")
	}
	// Should NOT retry on non-condition errors
	if sb.putCalls != 1 {
		t.Fatalf("expected 1 put call (no retry for non-condition error), got %d", sb.putCalls)
	}
}
