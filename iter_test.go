package dynago

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// iterStub implements Backend for testing Query and Scan iterators.
type iterStub struct {
	queryPages []iterQueryPage
	scanPages  []iterScanPage
	queryCall  int
	scanCall   int
}

type iterQueryPage struct {
	items []map[string]AttributeValue
	last  map[string]AttributeValue
	err   error
}

type iterScanPage struct {
	items []map[string]AttributeValue
	last  map[string]AttributeValue
	err   error
}

func (s *iterStub) Query(_ context.Context, _ *QueryRequest) (*QueryResponse, error) {
	if s.queryCall >= len(s.queryPages) {
		return &QueryResponse{}, nil
	}
	page := s.queryPages[s.queryCall]
	s.queryCall++
	if page.err != nil {
		return nil, page.err
	}
	return &QueryResponse{
		Items:            page.items,
		Count:            int32(len(page.items)),
		LastEvaluatedKey: page.last,
	}, nil
}

func (s *iterStub) Scan(_ context.Context, _ *ScanRequest) (*ScanResponse, error) {
	if s.scanCall >= len(s.scanPages) {
		return &ScanResponse{}, nil
	}
	page := s.scanPages[s.scanCall]
	s.scanCall++
	if page.err != nil {
		return nil, page.err
	}
	return &ScanResponse{
		Items:            page.items,
		Count:            int32(len(page.items)),
		LastEvaluatedKey: page.last,
	}, nil
}

func (s *iterStub) GetItem(context.Context, *GetItemRequest) (*GetItemResponse, error) {
	panic("not implemented")
}
func (s *iterStub) PutItem(context.Context, *PutItemRequest) (*PutItemResponse, error) {
	panic("not implemented")
}
func (s *iterStub) DeleteItem(context.Context, *DeleteItemRequest) (*DeleteItemResponse, error) {
	panic("not implemented")
}
func (s *iterStub) UpdateItem(context.Context, *UpdateItemRequest) (*UpdateItemResponse, error) {
	panic("not implemented")
}
func (s *iterStub) BatchGetItem(context.Context, *BatchGetItemRequest) (*BatchGetItemResponse, error) {
	panic("not implemented")
}
func (s *iterStub) BatchWriteItem(context.Context, *BatchWriteItemRequest) (*BatchWriteItemResponse, error) {
	panic("not implemented")
}
func (s *iterStub) TransactGetItems(context.Context, *TransactGetItemsRequest) (*TransactGetItemsResponse, error) {
	panic("not implemented")
}
func (s *iterStub) TransactWriteItems(context.Context, *TransactWriteItemsRequest) (*TransactWriteItemsResponse, error) {
	panic("not implemented")
}

type iterTestItem struct {
	PK   string `dynamo:"PK"`
	Name string `dynamo:"Name"`
}

func makeItem(pk, name string) map[string]AttributeValue {
	return map[string]AttributeValue{
		"PK":   {Type: TypeS, S: pk},
		"Name": {Type: TypeS, S: name},
	}
}

func TestQueryIter_MultiPage(t *testing.T) {
	stub := &iterStub{
		queryPages: []iterQueryPage{
			{
				items: []map[string]AttributeValue{
					makeItem("1", "Alice"),
					makeItem("2", "Bob"),
				},
				last: map[string]AttributeValue{"PK": {Type: TypeS, S: "2"}},
			},
			{
				items: []map[string]AttributeValue{
					makeItem("3", "Charlie"),
				},
				last: nil, // final page
			},
		},
	}
	db := New(stub)
	tbl := db.Table("TestTable")

	var results []iterTestItem
	for item, err := range QueryIter[iterTestItem](context.Background(), tbl, Partition("PK", "test")) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		results = append(results, item)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 items, got %d", len(results))
	}
	if results[0].Name != "Alice" {
		t.Errorf("expected Alice, got %s", results[0].Name)
	}
	if results[2].Name != "Charlie" {
		t.Errorf("expected Charlie, got %s", results[2].Name)
	}
}

func TestQueryIter_BreakMidPage(t *testing.T) {
	stub := &iterStub{
		queryPages: []iterQueryPage{
			{
				items: []map[string]AttributeValue{
					makeItem("1", "Alice"),
					makeItem("2", "Bob"),
					makeItem("3", "Charlie"),
				},
				last: map[string]AttributeValue{"PK": {Type: TypeS, S: "3"}},
			},
			{
				items: []map[string]AttributeValue{
					makeItem("4", "Dave"),
				},
				last: nil,
			},
		},
	}
	db := New(stub)
	tbl := db.Table("TestTable")

	var count int
	for _, err := range QueryIter[iterTestItem](context.Background(), tbl, Partition("PK", "test")) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		count++
		if count == 2 {
			break
		}
	}

	if count != 2 {
		t.Errorf("expected to process 2 items, got %d", count)
	}
	// Only one page should have been fetched since we broke mid-first-page.
	if stub.queryCall != 1 {
		t.Errorf("expected 1 query call, got %d", stub.queryCall)
	}
}

func TestQueryIter_ErrorMidIteration(t *testing.T) {
	backendErr := fmt.Errorf("backend failure")
	stub := &iterStub{
		queryPages: []iterQueryPage{
			{
				items: []map[string]AttributeValue{
					makeItem("1", "Alice"),
				},
				last: map[string]AttributeValue{"PK": {Type: TypeS, S: "1"}},
			},
			{
				err: backendErr,
			},
		},
	}
	db := New(stub)
	tbl := db.Table("TestTable")

	var gotItems int
	var gotErr error
	for _, err := range QueryIter[iterTestItem](context.Background(), tbl, Partition("PK", "test")) {
		if err != nil {
			gotErr = err
			break
		}
		gotItems++
	}

	if gotItems != 1 {
		t.Errorf("expected 1 item before error, got %d", gotItems)
	}
	if gotErr == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(gotErr, backendErr) {
		t.Errorf("expected backend failure, got %v", gotErr)
	}
}

func TestQueryIter_Empty(t *testing.T) {
	stub := &iterStub{
		queryPages: []iterQueryPage{
			{
				items: nil,
				last:  nil,
			},
		},
	}
	db := New(stub)
	tbl := db.Table("TestTable")

	var count int
	for _, err := range QueryIter[iterTestItem](context.Background(), tbl, Partition("PK", "test")) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 items, got %d", count)
	}
}

func TestScanIter_MultiPage(t *testing.T) {
	stub := &iterStub{
		scanPages: []iterScanPage{
			{
				items: []map[string]AttributeValue{
					makeItem("1", "Alice"),
				},
				last: map[string]AttributeValue{"PK": {Type: TypeS, S: "1"}},
			},
			{
				items: []map[string]AttributeValue{
					makeItem("2", "Bob"),
				},
				last: nil,
			},
		},
	}
	db := New(stub)
	tbl := db.Table("TestTable")

	var results []iterTestItem
	for item, err := range ScanIter[iterTestItem](context.Background(), tbl) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		results = append(results, item)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 items, got %d", len(results))
	}
	if results[0].Name != "Alice" {
		t.Errorf("expected Alice, got %s", results[0].Name)
	}
	if results[1].Name != "Bob" {
		t.Errorf("expected Bob, got %s", results[1].Name)
	}
}

func TestScanIter_BreakMidPage(t *testing.T) {
	stub := &iterStub{
		scanPages: []iterScanPage{
			{
				items: []map[string]AttributeValue{
					makeItem("1", "Alice"),
					makeItem("2", "Bob"),
				},
				last: map[string]AttributeValue{"PK": {Type: TypeS, S: "2"}},
			},
		},
	}
	db := New(stub)
	tbl := db.Table("TestTable")

	var count int
	for _, err := range ScanIter[iterTestItem](context.Background(), tbl) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		count++
		break // break after first item
	}

	if count != 1 {
		t.Errorf("expected 1 item, got %d", count)
	}
}

func TestScanIter_Error(t *testing.T) {
	backendErr := fmt.Errorf("scan failure")
	stub := &iterStub{
		scanPages: []iterScanPage{
			{err: backendErr},
		},
	}
	db := New(stub)
	tbl := db.Table("TestTable")

	var gotErr error
	for _, err := range ScanIter[iterTestItem](context.Background(), tbl) {
		if err != nil {
			gotErr = err
			break
		}
	}

	if gotErr == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(gotErr, backendErr) {
		t.Errorf("expected scan failure, got %v", gotErr)
	}
}

func TestScanIter_ErrorYieldedAsFinal(t *testing.T) {
	backendErr := fmt.Errorf("page2 fail")
	stub := &iterStub{
		scanPages: []iterScanPage{
			{
				items: []map[string]AttributeValue{
					makeItem("1", "Alice"),
				},
				last: map[string]AttributeValue{"PK": {Type: TypeS, S: "1"}},
			},
			{err: backendErr},
		},
	}
	db := New(stub)
	tbl := db.Table("TestTable")

	var items int
	var lastErr error
	for _, err := range ScanIter[iterTestItem](context.Background(), tbl) {
		if err != nil {
			lastErr = err
			break
		}
		items++
	}

	if items != 1 {
		t.Errorf("expected 1 item before error, got %d", items)
	}
	if lastErr == nil || !errors.Is(lastErr, backendErr) {
		t.Errorf("expected page2 fail error, got %v", lastErr)
	}
}
