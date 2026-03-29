package dynago

import (
	"context"
	"errors"
	"testing"
)

func TestQueryCollection_MixedTypes(t *testing.T) {
	sb := newQueryStub(func(_ context.Context, _ *QueryRequest) (*QueryResponse, error) {
		return &QueryResponse{
			Items: []map[string]AttributeValue{
				{
					"_type": {Type: TypeS, S: "USER"},
					"PK":    {Type: TypeS, S: "CUST#1"},
					"SK":    {Type: TypeS, S: "PROFILE"},
					"Name":  {Type: TypeS, S: "Alice"},
				},
				{
					"_type":  {Type: TypeS, S: "ORDER"},
					"PK":     {Type: TypeS, S: "CUST#1"},
					"SK":     {Type: TypeS, S: "ORDER#001"},
					"Amount": {Type: TypeN, N: "99"},
				},
			},
			Count: 2,
		}, nil
	})

	r := NewRegistry("_type")
	r.Register(userEntity{})
	r.Register(orderEntity{})

	db := New(sb)
	tbl := db.Table("T", WithRegistry(r))

	coll, err := QueryCollection(context.Background(), tbl, Partition("PK", "CUST#1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	users := ItemsOf[userEntity](coll)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].Name != "Alice" {
		t.Fatalf("expected Alice, got %q", users[0].Name)
	}

	orders := ItemsOf[orderEntity](coll)
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}
	if orders[0].Amount != 99 {
		t.Fatalf("expected 99, got %d", orders[0].Amount)
	}
}

func TestQueryCollection_SkipsUnknownDiscriminator(t *testing.T) {
	sb := newQueryStub(func(_ context.Context, _ *QueryRequest) (*QueryResponse, error) {
		return &QueryResponse{
			Items: []map[string]AttributeValue{
				{
					"_type": {Type: TypeS, S: "USER"},
					"PK":    {Type: TypeS, S: "X"},
					"SK":    {Type: TypeS, S: "Y"},
					"Name":  {Type: TypeS, S: "Bob"},
				},
				{
					"_type": {Type: TypeS, S: "UNKNOWN"},
					"PK":    {Type: TypeS, S: "X"},
					"SK":    {Type: TypeS, S: "Z"},
				},
			},
			Count: 2,
		}, nil
	})

	r := NewRegistry("_type")
	r.Register(userEntity{})

	db := New(sb)
	tbl := db.Table("T", WithRegistry(r))

	coll, err := QueryCollection(context.Background(), tbl, Partition("PK", "X"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	users := ItemsOf[userEntity](coll)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
}

func TestQueryCollection_NoRegistry(t *testing.T) {
	sb := newQueryStub(func(_ context.Context, _ *QueryRequest) (*QueryResponse, error) {
		return &QueryResponse{}, nil
	})

	db := New(sb)
	tbl := db.Table("T") // no registry

	_, err := QueryCollection(context.Background(), tbl, Partition("PK", "X"))
	if err == nil {
		t.Fatal("expected error without registry")
	}
}

func TestQueryCollection_BackendError(t *testing.T) {
	sb := newQueryStub(func(_ context.Context, _ *QueryRequest) (*QueryResponse, error) {
		return nil, errors.New("timeout")
	})

	r := NewRegistry("_type")
	r.Register(userEntity{})

	db := New(sb)
	tbl := db.Table("T", WithRegistry(r))

	_, err := QueryCollection(context.Background(), tbl, Partition("PK", "X"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestQueryCollection_Pagination(t *testing.T) {
	callCount := 0
	sb := newQueryStub(func(_ context.Context, req *QueryRequest) (*QueryResponse, error) {
		callCount++
		if callCount == 1 {
			return &QueryResponse{
				Items: []map[string]AttributeValue{
					{
						"_type": {Type: TypeS, S: "USER"},
						"PK":    {Type: TypeS, S: "C#1"},
						"SK":    {Type: TypeS, S: "PROFILE"},
						"Name":  {Type: TypeS, S: "Alice"},
					},
				},
				Count: 1,
				LastEvaluatedKey: map[string]AttributeValue{
					"PK": {Type: TypeS, S: "C#1"},
					"SK": {Type: TypeS, S: "PROFILE"},
				},
			}, nil
		}
		return &QueryResponse{
			Items: []map[string]AttributeValue{
				{
					"_type":  {Type: TypeS, S: "ORDER"},
					"PK":     {Type: TypeS, S: "C#1"},
					"SK":     {Type: TypeS, S: "ORDER#1"},
					"Amount": {Type: TypeN, N: "50"},
				},
			},
			Count: 1,
		}, nil
	})

	r := NewRegistry("_type")
	r.Register(userEntity{})
	r.Register(orderEntity{})

	db := New(sb)
	tbl := db.Table("T", WithRegistry(r))

	coll, err := QueryCollection(context.Background(), tbl, Partition("PK", "C#1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 backend calls, got %d", callCount)
	}

	users := ItemsOf[userEntity](coll)
	orders := ItemsOf[orderEntity](coll)
	if len(users) != 1 || len(orders) != 1 {
		t.Fatalf("expected 1 user and 1 order, got %d users %d orders", len(users), len(orders))
	}
}

func TestCollectionIter_MixedTypes(t *testing.T) {
	sb := newQueryStub(func(_ context.Context, _ *QueryRequest) (*QueryResponse, error) {
		return &QueryResponse{
			Items: []map[string]AttributeValue{
				{
					"_type": {Type: TypeS, S: "USER"},
					"PK":    {Type: TypeS, S: "C#1"},
					"SK":    {Type: TypeS, S: "PROFILE"},
					"Name":  {Type: TypeS, S: "Alice"},
				},
				{
					"_type":  {Type: TypeS, S: "ORDER"},
					"PK":     {Type: TypeS, S: "C#1"},
					"SK":     {Type: TypeS, S: "ORDER#1"},
					"Amount": {Type: TypeN, N: "77"},
				},
			},
			Count: 2,
		}, nil
	})

	r := NewRegistry("_type")
	r.Register(userEntity{})
	r.Register(orderEntity{})

	db := New(sb)
	tbl := db.Table("T", WithRegistry(r))

	var users []userEntity
	var orders []orderEntity
	for v, err := range CollectionIter(context.Background(), tbl, Partition("PK", "C#1")) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		switch item := v.(type) {
		case userEntity:
			users = append(users, item)
		case orderEntity:
			orders = append(orders, item)
		}
	}

	if len(users) != 1 || users[0].Name != "Alice" {
		t.Fatalf("unexpected users: %+v", users)
	}
	if len(orders) != 1 || orders[0].Amount != 77 {
		t.Fatalf("unexpected orders: %+v", orders)
	}
}

func TestCollectionIter_Pagination(t *testing.T) {
	callCount := 0
	sb := newQueryStub(func(_ context.Context, _ *QueryRequest) (*QueryResponse, error) {
		callCount++
		if callCount == 1 {
			return &QueryResponse{
				Items: []map[string]AttributeValue{
					{
						"_type": {Type: TypeS, S: "USER"},
						"PK":    {Type: TypeS, S: "C#1"},
						"SK":    {Type: TypeS, S: "PROFILE"},
						"Name":  {Type: TypeS, S: "Alice"},
					},
				},
				Count: 1,
				LastEvaluatedKey: map[string]AttributeValue{
					"PK": {Type: TypeS, S: "C#1"},
					"SK": {Type: TypeS, S: "PROFILE"},
				},
			}, nil
		}
		return &QueryResponse{
			Items: []map[string]AttributeValue{
				{
					"_type":  {Type: TypeS, S: "ORDER"},
					"PK":     {Type: TypeS, S: "C#1"},
					"SK":     {Type: TypeS, S: "ORDER#1"},
					"Amount": {Type: TypeN, N: "50"},
				},
			},
			Count: 1,
		}, nil
	})

	r := NewRegistry("_type")
	r.Register(userEntity{})
	r.Register(orderEntity{})

	db := New(sb)
	tbl := db.Table("T", WithRegistry(r))

	count := 0
	for _, err := range CollectionIter(context.Background(), tbl, Partition("PK", "C#1")) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		count++
	}
	if count != 2 {
		t.Fatalf("expected 2 items, got %d", count)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 backend calls, got %d", callCount)
	}
}

func TestCollectionIter_SkipsUnknown(t *testing.T) {
	sb := newQueryStub(func(_ context.Context, _ *QueryRequest) (*QueryResponse, error) {
		return &QueryResponse{
			Items: []map[string]AttributeValue{
				{
					"_type": {Type: TypeS, S: "USER"},
					"PK":    {Type: TypeS, S: "X"},
					"SK":    {Type: TypeS, S: "Y"},
					"Name":  {Type: TypeS, S: "Bob"},
				},
				{
					"_type": {Type: TypeS, S: "WIDGET"},
					"PK":    {Type: TypeS, S: "X"},
					"SK":    {Type: TypeS, S: "Z"},
				},
			},
			Count: 2,
		}, nil
	})

	r := NewRegistry("_type")
	r.Register(userEntity{})

	db := New(sb)
	tbl := db.Table("T", WithRegistry(r))

	count := 0
	for _, err := range CollectionIter(context.Background(), tbl, Partition("PK", "X")) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		count++
	}
	if count != 1 {
		t.Fatalf("expected 1 item (unknown skipped), got %d", count)
	}
}

func TestCollectionIter_NoRegistry(t *testing.T) {
	sb := newQueryStub(func(_ context.Context, _ *QueryRequest) (*QueryResponse, error) {
		return &QueryResponse{}, nil
	})

	db := New(sb)
	tbl := db.Table("T")

	for _, err := range CollectionIter(context.Background(), tbl, Partition("PK", "X")) {
		if err == nil {
			t.Fatal("expected error without registry")
		}
		return // got expected error
	}
}

func TestCollectionIter_BreakEarly(t *testing.T) {
	sb := newQueryStub(func(_ context.Context, _ *QueryRequest) (*QueryResponse, error) {
		return &QueryResponse{
			Items: []map[string]AttributeValue{
				{
					"_type": {Type: TypeS, S: "USER"},
					"PK":    {Type: TypeS, S: "X"},
					"SK":    {Type: TypeS, S: "A"},
					"Name":  {Type: TypeS, S: "A"},
				},
				{
					"_type": {Type: TypeS, S: "USER"},
					"PK":    {Type: TypeS, S: "X"},
					"SK":    {Type: TypeS, S: "B"},
					"Name":  {Type: TypeS, S: "B"},
				},
			},
			Count: 2,
		}, nil
	})

	r := NewRegistry("_type")
	r.Register(userEntity{})

	db := New(sb)
	tbl := db.Table("T", WithRegistry(r))

	count := 0
	for _, err := range CollectionIter(context.Background(), tbl, Partition("PK", "X")) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		count++
		break // break after first item
	}
	if count != 1 {
		t.Fatalf("expected 1 item after break, got %d", count)
	}
}

func TestItemsOf_EmptyCollection(t *testing.T) {
	coll := &Collection{}
	users := ItemsOf[userEntity](coll)
	if len(users) != 0 {
		t.Fatalf("expected 0 users, got %d", len(users))
	}
}
