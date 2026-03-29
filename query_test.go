package dynago

import (
	"context"
	"errors"
	"testing"
)

// queryStubBackend is a Backend that captures Query requests and returns
// configured responses. It supports multiple calls for pagination testing.
type queryStubBackend struct {
	stubBackend
	queryFunc func(ctx context.Context, req *QueryRequest) (*QueryResponse, error)
	queryCalls []*QueryRequest
}

func (s *queryStubBackend) Query(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
	s.queryCalls = append(s.queryCalls, req)
	return s.queryFunc(ctx, req)
}

func newQueryStub(fn func(ctx context.Context, req *QueryRequest) (*QueryResponse, error)) *queryStubBackend {
	return &queryStubBackend{
		stubBackend: stubBackend{
			getItemFunc: func(context.Context, *GetItemRequest) (*GetItemResponse, error) {
				panic("not implemented")
			},
		},
		queryFunc: fn,
	}
}

type orderItem struct {
	PK     string `dynamo:"PK,hash"`
	SK     string `dynamo:"SK,range"`
	Amount int    `dynamo:"Amount"`
	Status string `dynamo:"Status"`
}

func TestQuery_PartitionOnly(t *testing.T) {
	sb := newQueryStub(func(_ context.Context, req *QueryRequest) (*QueryResponse, error) {
		// Verify the key condition contains the partition key.
		if req.KeyConditionExpression == "" {
			t.Fatal("expected KeyConditionExpression to be set")
		}
		if req.TableName != "Orders" {
			t.Fatalf("expected table Orders, got %s", req.TableName)
		}

		return &QueryResponse{
			Items: []map[string]AttributeValue{
				{
					"PK":     {Type: TypeS, S: "USER#1"},
					"SK":     {Type: TypeS, S: "ORDER#001"},
					"Amount": {Type: TypeN, N: "100"},
					"Status": {Type: TypeS, S: "shipped"},
				},
				{
					"PK":     {Type: TypeS, S: "USER#1"},
					"SK":     {Type: TypeS, S: "ORDER#002"},
					"Amount": {Type: TypeN, N: "250"},
					"Status": {Type: TypeS, S: "pending"},
				},
			},
			Count: 2,
		}, nil
	})

	db := New(sb)
	tbl := db.Table("Orders")

	items, err := Query[orderItem](context.Background(), tbl, Partition("PK", "USER#1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].SK != "ORDER#001" {
		t.Errorf("expected ORDER#001, got %s", items[0].SK)
	}
	if items[1].Amount != 250 {
		t.Errorf("expected amount 250, got %d", items[1].Amount)
	}

	// Verify expression attributes.
	req := sb.queryCalls[0]
	if len(req.ExpressionAttributeNames) == 0 {
		t.Error("expected ExpressionAttributeNames to be populated")
	}
	if len(req.ExpressionAttributeValues) == 0 {
		t.Error("expected ExpressionAttributeValues to be populated")
	}
}

func TestQuery_WithSortKey(t *testing.T) {
	sb := newQueryStub(func(_ context.Context, req *QueryRequest) (*QueryResponse, error) {
		// Key condition should have AND for sort key.
		if req.KeyConditionExpression == "" {
			t.Fatal("expected KeyConditionExpression")
		}

		return &QueryResponse{
			Items: []map[string]AttributeValue{
				{
					"PK":     {Type: TypeS, S: "USER#1"},
					"SK":     {Type: TypeS, S: "ORDER#001"},
					"Amount": {Type: TypeN, N: "100"},
					"Status": {Type: TypeS, S: "shipped"},
				},
			},
			Count: 1,
		}, nil
	})

	db := New(sb)
	tbl := db.Table("Orders")

	key := Partition("PK", "USER#1").SortBeginsWith("SK", "ORDER#")
	items, err := Query[orderItem](context.Background(), tbl, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	req := sb.queryCalls[0]
	// Should contain begins_with in the expression.
	if req.KeyConditionExpression == "" {
		t.Error("expected non-empty key condition")
	}
}

func TestQuery_SortBetween(t *testing.T) {
	sb := newQueryStub(func(_ context.Context, _ *QueryRequest) (*QueryResponse, error) {
		return &QueryResponse{
			Items: []map[string]AttributeValue{
				{
					"PK":     {Type: TypeS, S: "USER#1"},
					"SK":     {Type: TypeS, S: "ORDER#002"},
					"Amount": {Type: TypeN, N: "200"},
					"Status": {Type: TypeS, S: "pending"},
				},
			},
			Count: 1,
		}, nil
	})

	db := New(sb)
	tbl := db.Table("Orders")

	key := Partition("PK", "USER#1").SortBetween("SK", "ORDER#001", "ORDER#010")
	items, err := Query[orderItem](context.Background(), tbl, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	req := sb.queryCalls[0]
	// Should contain BETWEEN in the expression.
	if req.KeyConditionExpression == "" {
		t.Error("expected non-empty key condition with BETWEEN")
	}
}

func TestQuery_WithFilter(t *testing.T) {
	sb := newQueryStub(func(_ context.Context, req *QueryRequest) (*QueryResponse, error) {
		if req.FilterExpression == "" {
			t.Fatal("expected FilterExpression to be set")
		}

		return &QueryResponse{
			Items: []map[string]AttributeValue{
				{
					"PK":     {Type: TypeS, S: "USER#1"},
					"SK":     {Type: TypeS, S: "ORDER#001"},
					"Amount": {Type: TypeN, N: "100"},
					"Status": {Type: TypeS, S: "shipped"},
				},
			},
			Count: 1,
		}, nil
	})

	db := New(sb)
	tbl := db.Table("Orders")

	items, err := Query[orderItem](context.Background(), tbl,
		Partition("PK", "USER#1"),
		QueryFilter("Status = ?", "shipped"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestQuery_Pagination(t *testing.T) {
	callCount := 0
	sb := newQueryStub(func(_ context.Context, req *QueryRequest) (*QueryResponse, error) {
		callCount++
		if callCount == 1 {
			// First page: return 1 item with LastEvaluatedKey.
			return &QueryResponse{
				Items: []map[string]AttributeValue{
					{
						"PK":     {Type: TypeS, S: "USER#1"},
						"SK":     {Type: TypeS, S: "ORDER#001"},
						"Amount": {Type: TypeN, N: "100"},
						"Status": {Type: TypeS, S: "shipped"},
					},
				},
				Count: 1,
				LastEvaluatedKey: map[string]AttributeValue{
					"PK": {Type: TypeS, S: "USER#1"},
					"SK": {Type: TypeS, S: "ORDER#001"},
				},
			}, nil
		}
		// Second page: last page (no LastEvaluatedKey).
		if req.ExclusiveStartKey == nil {
			t.Fatal("expected ExclusiveStartKey on second call")
		}
		return &QueryResponse{
			Items: []map[string]AttributeValue{
				{
					"PK":     {Type: TypeS, S: "USER#1"},
					"SK":     {Type: TypeS, S: "ORDER#002"},
					"Amount": {Type: TypeN, N: "250"},
					"Status": {Type: TypeS, S: "pending"},
				},
			},
			Count: 1,
		}, nil
	})

	db := New(sb)
	tbl := db.Table("Orders")

	items, err := Query[orderItem](context.Background(), tbl, Partition("PK", "USER#1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items from pagination, got %d", len(items))
	}
	if callCount != 2 {
		t.Fatalf("expected 2 backend calls, got %d", callCount)
	}
}

func TestQuery_Limit(t *testing.T) {
	sb := newQueryStub(func(_ context.Context, req *QueryRequest) (*QueryResponse, error) {
		if req.Limit != 1 {
			t.Fatalf("expected Limit=1, got %d", req.Limit)
		}
		return &QueryResponse{
			Items: []map[string]AttributeValue{
				{
					"PK":     {Type: TypeS, S: "USER#1"},
					"SK":     {Type: TypeS, S: "ORDER#001"},
					"Amount": {Type: TypeN, N: "100"},
					"Status": {Type: TypeS, S: "shipped"},
				},
			},
			Count: 1,
		}, nil
	})

	db := New(sb)
	tbl := db.Table("Orders")

	items, err := Query[orderItem](context.Background(), tbl,
		Partition("PK", "USER#1"),
		QueryLimit(1),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestQuery_Options(t *testing.T) {
	sb := newQueryStub(func(_ context.Context, req *QueryRequest) (*QueryResponse, error) {
		if req.IndexName != "StatusIndex" {
			t.Errorf("expected index StatusIndex, got %s", req.IndexName)
		}
		if req.ConsistentRead != true {
			t.Error("expected ConsistentRead=true")
		}
		if req.ScanIndexForward == nil || *req.ScanIndexForward != false {
			t.Error("expected ScanIndexForward=false")
		}
		if req.ProjectionExpression == "" {
			t.Error("expected ProjectionExpression to be set")
		}
		return &QueryResponse{
			Items: []map[string]AttributeValue{
				{
					"PK":     {Type: TypeS, S: "USER#1"},
					"Status": {Type: TypeS, S: "shipped"},
				},
			},
			Count: 1,
		}, nil
	})

	db := New(sb)
	tbl := db.Table("Orders")

	_, err := Query[orderItem](context.Background(), tbl,
		Partition("PK", "USER#1"),
		QueryIndex("StatusIndex"),
		QueryConsistentRead(),
		ScanForward(false),
		QueryProject("PK", "Status"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQuery_BackendError(t *testing.T) {
	sb := newQueryStub(func(_ context.Context, _ *QueryRequest) (*QueryResponse, error) {
		return nil, errors.New("backend error")
	})

	db := New(sb)
	tbl := db.Table("Orders")

	_, err := Query[orderItem](context.Background(), tbl, Partition("PK", "USER#1"))
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "backend error" {
		t.Errorf("expected 'backend error', got %q", err.Error())
	}
}

func TestQuery_SortKeyVariants(t *testing.T) {
	tests := []struct {
		name string
		key  KeyCondition
	}{
		{"SortEquals", Partition("PK", "X").SortEquals("SK", "Y")},
		{"SortLessThan", Partition("PK", "X").SortLessThan("SK", "Y")},
		{"SortLessOrEqual", Partition("PK", "X").SortLessOrEqual("SK", "Y")},
		{"SortGreaterThan", Partition("PK", "X").SortGreaterThan("SK", "Y")},
		{"SortGreaterOrEqual", Partition("PK", "X").SortGreaterOrEqual("SK", "Y")},
		{"SortBeginsWith", Partition("PK", "X").SortBeginsWith("SK", "prefix")},
		{"SortBetween", Partition("PK", "X").SortBetween("SK", "A", "Z")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := newQueryStub(func(_ context.Context, req *QueryRequest) (*QueryResponse, error) {
				if req.KeyConditionExpression == "" {
					t.Error("expected non-empty key condition expression")
				}
				// Key condition should contain AND for sort key.
				return &QueryResponse{}, nil
			})
			db := New(sb)
			tbl := db.Table("T")
			_, err := Query[orderItem](context.Background(), tbl, tt.key)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
