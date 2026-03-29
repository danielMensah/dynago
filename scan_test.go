package dynago

import (
	"context"
	"errors"
	"testing"
)

// scanStubBackend is a Backend that captures Scan requests and returns
// configured responses.
type scanStubBackend struct {
	stubBackend
	scanFunc  func(ctx context.Context, req *ScanRequest) (*ScanResponse, error)
	scanCalls []*ScanRequest
}

func (s *scanStubBackend) Scan(ctx context.Context, req *ScanRequest) (*ScanResponse, error) {
	s.scanCalls = append(s.scanCalls, req)
	return s.scanFunc(ctx, req)
}

func newScanStub(fn func(ctx context.Context, req *ScanRequest) (*ScanResponse, error)) *scanStubBackend {
	return &scanStubBackend{
		stubBackend: stubBackend{
			getItemFunc: func(context.Context, *GetItemRequest) (*GetItemResponse, error) {
				panic("not implemented")
			},
		},
		scanFunc: fn,
	}
}

func TestScan_Basic(t *testing.T) {
	sb := newScanStub(func(_ context.Context, req *ScanRequest) (*ScanResponse, error) {
		if req.TableName != "Orders" {
			t.Fatalf("expected table Orders, got %s", req.TableName)
		}
		return &ScanResponse{
			Items: []map[string]AttributeValue{
				{
					"PK":     {Type: TypeS, S: "USER#1"},
					"SK":     {Type: TypeS, S: "ORDER#001"},
					"Amount": {Type: TypeN, N: "100"},
					"Status": {Type: TypeS, S: "shipped"},
				},
				{
					"PK":     {Type: TypeS, S: "USER#2"},
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

	items, err := Scan[orderItem](context.Background(), tbl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].PK != "USER#1" {
		t.Errorf("expected USER#1, got %s", items[0].PK)
	}
	if items[1].Amount != 250 {
		t.Errorf("expected 250, got %d", items[1].Amount)
	}
}

func TestScan_WithFilter(t *testing.T) {
	sb := newScanStub(func(_ context.Context, req *ScanRequest) (*ScanResponse, error) {
		if req.FilterExpression == "" {
			t.Fatal("expected FilterExpression to be set")
		}
		if len(req.ExpressionAttributeNames) == 0 {
			t.Error("expected ExpressionAttributeNames")
		}
		if len(req.ExpressionAttributeValues) == 0 {
			t.Error("expected ExpressionAttributeValues")
		}
		return &ScanResponse{
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

	items, err := Scan[orderItem](context.Background(), tbl,
		ScanFilter("Status = ?", "shipped"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestScan_Pagination(t *testing.T) {
	callCount := 0
	sb := newScanStub(func(_ context.Context, req *ScanRequest) (*ScanResponse, error) {
		callCount++
		if callCount == 1 {
			return &ScanResponse{
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
		if req.ExclusiveStartKey == nil {
			t.Fatal("expected ExclusiveStartKey on second call")
		}
		return &ScanResponse{
			Items: []map[string]AttributeValue{
				{
					"PK":     {Type: TypeS, S: "USER#2"},
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

	items, err := Scan[orderItem](context.Background(), tbl)
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

func TestScan_Limit(t *testing.T) {
	sb := newScanStub(func(_ context.Context, req *ScanRequest) (*ScanResponse, error) {
		if req.Limit != 1 {
			t.Fatalf("expected Limit=1, got %d", req.Limit)
		}
		return &ScanResponse{
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

	items, err := Scan[orderItem](context.Background(), tbl, ScanLimit(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestScan_Options(t *testing.T) {
	sb := newScanStub(func(_ context.Context, req *ScanRequest) (*ScanResponse, error) {
		if req.IndexName != "StatusIndex" {
			t.Errorf("expected index StatusIndex, got %s", req.IndexName)
		}
		if req.ConsistentRead != true {
			t.Error("expected ConsistentRead=true")
		}
		if req.ProjectionExpression == "" {
			t.Error("expected ProjectionExpression to be set")
		}
		return &ScanResponse{
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

	_, err := Scan[orderItem](context.Background(), tbl,
		ScanIndex("StatusIndex"),
		ScanConsistentRead(),
		ScanProject("PK", "Status"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScan_BackendError(t *testing.T) {
	sb := newScanStub(func(_ context.Context, _ *ScanRequest) (*ScanResponse, error) {
		return nil, errors.New("backend error")
	})

	db := New(sb)
	tbl := db.Table("Orders")

	_, err := Scan[orderItem](context.Background(), tbl)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "backend error" {
		t.Errorf("expected 'backend error', got %q", err.Error())
	}
}

func TestScan_GSI(t *testing.T) {
	sb := newScanStub(func(_ context.Context, req *ScanRequest) (*ScanResponse, error) {
		if req.IndexName != "GSI1" {
			t.Fatalf("expected IndexName GSI1, got %q", req.IndexName)
		}
		if req.TableName != "Orders" {
			t.Fatalf("expected table Orders, got %s", req.TableName)
		}
		return &ScanResponse{
			Items: []map[string]AttributeValue{
				{
					"PK":     {Type: TypeS, S: "USER#1"},
					"SK":     {Type: TypeS, S: "ORDER#001"},
					"Amount": {Type: TypeN, N: "100"},
					"Status": {Type: TypeS, S: "shipped"},
				},
				{
					"PK":     {Type: TypeS, S: "USER#2"},
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

	items, err := Scan[orderItem](context.Background(), tbl, ScanIndex("GSI1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	req := sb.scanCalls[0]
	if req.IndexName != "GSI1" {
		t.Errorf("expected IndexName GSI1 in request, got %q", req.IndexName)
	}
}

func TestScan_GSI_WithFilter(t *testing.T) {
	sb := newScanStub(func(_ context.Context, req *ScanRequest) (*ScanResponse, error) {
		if req.IndexName != "StatusIndex" {
			t.Fatalf("expected IndexName StatusIndex, got %q", req.IndexName)
		}
		if req.FilterExpression == "" {
			t.Fatal("expected FilterExpression to be set")
		}
		return &ScanResponse{
			Items: []map[string]AttributeValue{
				{
					"PK":     {Type: TypeS, S: "USER#1"},
					"SK":     {Type: TypeS, S: "ORDER#001"},
					"Amount": {Type: TypeN, N: "500"},
					"Status": {Type: TypeS, S: "shipped"},
				},
			},
			Count: 1,
		}, nil
	})

	db := New(sb)
	tbl := db.Table("Orders")

	items, err := Scan[orderItem](context.Background(), tbl,
		ScanIndex("StatusIndex"),
		ScanFilter("Amount > ?", 100),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestScan_NoIndex(t *testing.T) {
	sb := newScanStub(func(_ context.Context, req *ScanRequest) (*ScanResponse, error) {
		if req.IndexName != "" {
			t.Fatalf("expected empty IndexName, got %q", req.IndexName)
		}
		return &ScanResponse{}, nil
	})

	db := New(sb)
	tbl := db.Table("Orders")

	_, err := Scan[orderItem](context.Background(), tbl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScan_LimitStopsPagination(t *testing.T) {
	callCount := 0
	sb := newScanStub(func(_ context.Context, _ *ScanRequest) (*ScanResponse, error) {
		callCount++
		return &ScanResponse{
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
	})

	db := New(sb)
	tbl := db.Table("Orders")

	items, err := Scan[orderItem](context.Background(), tbl, ScanLimit(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call (limit should stop pagination), got %d", callCount)
	}
}
