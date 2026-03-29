package dynago

import (
	"context"
	"errors"
	"testing"
)

func TestDelete_Basic(t *testing.T) {
	sb := &putDeleteStub{}
	db := New(sb)
	tbl := db.Table("MyTable")

	key := Key("PK", "user#1", "SK", "profile")
	err := tbl.Delete(context.Background(), key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sb.delReq == nil {
		t.Fatal("expected DeleteItem to be called")
	}
	if sb.delReq.TableName != "MyTable" {
		t.Fatalf("expected table name 'MyTable', got %q", sb.delReq.TableName)
	}
	if sb.delReq.Key == nil {
		t.Fatal("expected key to be set")
	}
	if sb.delReq.Key["PK"].S != "user#1" {
		t.Fatalf("expected PK 'user#1', got %q", sb.delReq.Key["PK"].S)
	}
	if sb.delReq.Key["SK"].S != "profile" {
		t.Fatalf("expected SK 'profile', got %q", sb.delReq.Key["SK"].S)
	}
	if sb.delReq.ConditionExpression != "" {
		t.Fatalf("expected no condition expression, got %q", sb.delReq.ConditionExpression)
	}
}

func TestDelete_Condition(t *testing.T) {
	sb := &putDeleteStub{}
	db := New(sb)
	tbl := db.Table("MyTable")

	key := Key("PK", "user#1")
	err := tbl.Delete(context.Background(), key, DeleteCondition("Active = ?", true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sb.delReq.ConditionExpression == "" {
		t.Fatal("expected condition expression to be set")
	}
	if sb.delReq.ExpressionAttributeValues == nil {
		t.Fatal("expected expression attribute values to be set")
	}
	v, ok := sb.delReq.ExpressionAttributeValues[":v0"]
	if !ok {
		t.Fatal("expected :v0 in expression attribute values")
	}
	if v.Type != TypeBOOL || v.BOOL != true {
		t.Fatalf("expected BOOL=true, got type=%d BOOL=%v", v.Type, v.BOOL)
	}
}

func TestDelete_ConditionFailed(t *testing.T) {
	sb := &putDeleteStub{
		delErr: &Error{Sentinel: ErrConditionFailed, Message: "condition not satisfied"},
	}
	db := New(sb)
	tbl := db.Table("MyTable")

	key := Key("PK", "user#1")
	err := tbl.Delete(context.Background(), key, DeleteCondition("Active = ?", true))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrConditionFailed) {
		t.Fatalf("expected ErrConditionFailed, got %v", err)
	}
}

func TestDelete_BackendError(t *testing.T) {
	sb := &putDeleteStub{
		delErr: errors.New("connection timeout"),
	}
	db := New(sb)
	tbl := db.Table("MyTable")

	key := Key("PK", "user#1")
	err := tbl.Delete(context.Background(), key)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "connection timeout" {
		t.Fatalf("expected 'connection timeout', got %q", err.Error())
	}
}
