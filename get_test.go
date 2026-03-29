package dynago

import (
	"context"
	"errors"
	"testing"
)

// stubBackend is a minimal Backend implementation for testing.
// Only GetItem is functional; all other methods panic.
type stubBackend struct {
	getItemFunc func(ctx context.Context, req *GetItemRequest) (*GetItemResponse, error)
	// capture the last GetItem request for assertions
	lastGetReq *GetItemRequest
}

func (s *stubBackend) GetItem(ctx context.Context, req *GetItemRequest) (*GetItemResponse, error) {
	s.lastGetReq = req
	return s.getItemFunc(ctx, req)
}

func (s *stubBackend) PutItem(context.Context, *PutItemRequest) (*PutItemResponse, error) {
	panic("not implemented")
}

func (s *stubBackend) DeleteItem(context.Context, *DeleteItemRequest) (*DeleteItemResponse, error) {
	panic("not implemented")
}

func (s *stubBackend) UpdateItem(context.Context, *UpdateItemRequest) (*UpdateItemResponse, error) {
	panic("not implemented")
}

func (s *stubBackend) Query(context.Context, *QueryRequest) (*QueryResponse, error) {
	panic("not implemented")
}

func (s *stubBackend) Scan(context.Context, *ScanRequest) (*ScanResponse, error) {
	panic("not implemented")
}

func (s *stubBackend) BatchGetItem(context.Context, *BatchGetItemRequest) (*BatchGetItemResponse, error) {
	panic("not implemented")
}

func (s *stubBackend) BatchWriteItem(context.Context, *BatchWriteItemRequest) (*BatchWriteItemResponse, error) {
	panic("not implemented")
}

func (s *stubBackend) TransactGetItems(context.Context, *TransactGetItemsRequest) (*TransactGetItemsResponse, error) {
	panic("not implemented")
}

func (s *stubBackend) TransactWriteItems(context.Context, *TransactWriteItemsRequest) (*TransactWriteItemsResponse, error) {
	panic("not implemented")
}

type testUser struct {
	PK    string `dynamo:"PK,hash"`
	SK    string `dynamo:"SK,range"`
	Name  string `dynamo:"Name"`
	Email string `dynamo:"Email"`
	Age   int    `dynamo:"Age"`
}

func TestGet_Success(t *testing.T) {
	sb := &stubBackend{
		getItemFunc: func(_ context.Context, _ *GetItemRequest) (*GetItemResponse, error) {
			return &GetItemResponse{
				Item: map[string]AttributeValue{
					"PK":    {Type: TypeS, S: "USER#123"},
					"SK":    {Type: TypeS, S: "PROFILE"},
					"Name":  {Type: TypeS, S: "Alice"},
					"Email": {Type: TypeS, S: "alice@example.com"},
					"Age":   {Type: TypeN, N: "30"},
				},
			}, nil
		},
	}

	db := New(sb)
	tbl := db.Table("users")

	user, err := Get[testUser](context.Background(), tbl, Key("PK", "USER#123", "SK", "PROFILE"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.PK != "USER#123" {
		t.Errorf("PK = %q, want %q", user.PK, "USER#123")
	}
	if user.SK != "PROFILE" {
		t.Errorf("SK = %q, want %q", user.SK, "PROFILE")
	}
	if user.Name != "Alice" {
		t.Errorf("Name = %q, want %q", user.Name, "Alice")
	}
	if user.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", user.Email, "alice@example.com")
	}
	if user.Age != 30 {
		t.Errorf("Age = %d, want %d", user.Age, 30)
	}

	// Verify the request was constructed correctly.
	if sb.lastGetReq.TableName != "users" {
		t.Errorf("TableName = %q, want %q", sb.lastGetReq.TableName, "users")
	}
	if sb.lastGetReq.ConsistentRead {
		t.Error("ConsistentRead should be false by default")
	}
}

func TestGet_NotFound_NilItem(t *testing.T) {
	sb := &stubBackend{
		getItemFunc: func(_ context.Context, _ *GetItemRequest) (*GetItemResponse, error) {
			return &GetItemResponse{Item: nil}, nil
		},
	}

	db := New(sb)
	tbl := db.Table("users")

	_, err := Get[testUser](context.Background(), tbl, Key("PK", "USER#999"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGet_NotFound_EmptyItem(t *testing.T) {
	sb := &stubBackend{
		getItemFunc: func(_ context.Context, _ *GetItemRequest) (*GetItemResponse, error) {
			return &GetItemResponse{Item: map[string]AttributeValue{}}, nil
		},
	}

	db := New(sb)
	tbl := db.Table("users")

	_, err := Get[testUser](context.Background(), tbl, Key("PK", "USER#999"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGet_BackendError(t *testing.T) {
	backendErr := errors.New("backend failure")
	sb := &stubBackend{
		getItemFunc: func(_ context.Context, _ *GetItemRequest) (*GetItemResponse, error) {
			return nil, backendErr
		},
	}

	db := New(sb)
	tbl := db.Table("users")

	_, err := Get[testUser](context.Background(), tbl, Key("PK", "USER#123"))
	if !errors.Is(err, backendErr) {
		t.Fatalf("expected backend error, got %v", err)
	}
}

func TestGet_ConsistentRead(t *testing.T) {
	sb := &stubBackend{
		getItemFunc: func(_ context.Context, _ *GetItemRequest) (*GetItemResponse, error) {
			return &GetItemResponse{
				Item: map[string]AttributeValue{
					"PK":   {Type: TypeS, S: "USER#123"},
					"Name": {Type: TypeS, S: "Alice"},
				},
			}, nil
		},
	}

	db := New(sb)
	tbl := db.Table("users")

	_, err := Get[testUser](context.Background(), tbl, Key("PK", "USER#123"), ConsistentRead())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sb.lastGetReq.ConsistentRead {
		t.Error("ConsistentRead should be true")
	}
}

func TestGet_Projection(t *testing.T) {
	sb := &stubBackend{
		getItemFunc: func(_ context.Context, _ *GetItemRequest) (*GetItemResponse, error) {
			return &GetItemResponse{
				Item: map[string]AttributeValue{
					"PK":   {Type: TypeS, S: "USER#123"},
					"Name": {Type: TypeS, S: "Alice"},
				},
			}, nil
		},
	}

	db := New(sb)
	tbl := db.Table("users")

	_, err := Get[testUser](context.Background(), tbl, Key("PK", "USER#123"), Project("PK", "Name"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := sb.lastGetReq
	if req.ProjectionExpression == "" {
		t.Fatal("ProjectionExpression should not be empty")
	}

	// Verify ExpressionAttributeNames contains the projected attributes.
	if len(req.ExpressionAttributeNames) == 0 {
		t.Fatal("ExpressionAttributeNames should not be empty")
	}

	// The projection expression should reference both attributes.
	// Names are aliased as #PK, #Name (or similar).
	foundPK := false
	foundName := false
	for alias, name := range req.ExpressionAttributeNames {
		if name == "PK" {
			foundPK = true
			_ = alias
		}
		if name == "Name" {
			foundName = true
			_ = alias
		}
	}
	if !foundPK {
		t.Error("ExpressionAttributeNames missing PK")
	}
	if !foundName {
		t.Error("ExpressionAttributeNames missing Name")
	}
}

func TestGet_ProjectionAndConsistentRead(t *testing.T) {
	sb := &stubBackend{
		getItemFunc: func(_ context.Context, _ *GetItemRequest) (*GetItemResponse, error) {
			return &GetItemResponse{
				Item: map[string]AttributeValue{
					"PK":   {Type: TypeS, S: "USER#123"},
					"Name": {Type: TypeS, S: "Alice"},
				},
			}, nil
		},
	}

	db := New(sb)
	tbl := db.Table("users")

	_, err := Get[testUser](context.Background(), tbl, Key("PK", "USER#123"),
		ConsistentRead(),
		Project("PK", "Name"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := sb.lastGetReq
	if !req.ConsistentRead {
		t.Error("ConsistentRead should be true")
	}
	if req.ProjectionExpression == "" {
		t.Error("ProjectionExpression should not be empty")
	}
}
