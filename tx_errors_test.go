package dynago

import (
	"errors"
	"fmt"
	"testing"
)

func TestTxCancelledError_Error(t *testing.T) {
	tests := []struct {
		name    string
		reasons []TxCancelReason
		want    string
	}{
		{
			name:    "no reasons",
			reasons: nil,
			want:    "dynago: transaction cancelled",
		},
		{
			name: "single reason with code and message",
			reasons: []TxCancelReason{
				{Code: "ConditionalCheckFailed", Message: "condition not met"},
			},
			want: "dynago: transaction cancelled: [ConditionalCheckFailed: condition not met]",
		},
		{
			name: "code only",
			reasons: []TxCancelReason{
				{Code: "ValidationError"},
			},
			want: "dynago: transaction cancelled: [ValidationError]",
		},
		{
			name: "mixed reasons",
			reasons: []TxCancelReason{
				{Code: "ConditionalCheckFailed", Message: "version mismatch"},
				{},
				{Code: "ValidationError"},
			},
			want: "dynago: transaction cancelled: [ConditionalCheckFailed: version mismatch, None, ValidationError]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &TxCancelledError{Reasons: tt.reasons}
			if got := e.Error(); got != tt.want {
				t.Fatalf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTxCancelledError_Is_TransactionCancelled(t *testing.T) {
	e := &TxCancelledError{}
	if !errors.Is(e, ErrTransactionCancelled) {
		t.Fatal("expected Is(ErrTransactionCancelled) = true")
	}
}

func TestTxCancelledError_Is_ConditionFailed(t *testing.T) {
	e := &TxCancelledError{
		Reasons: []TxCancelReason{
			{Code: "ConditionalCheckFailed", Message: "version mismatch"},
		},
	}
	if !errors.Is(e, ErrConditionFailed) {
		t.Fatal("expected Is(ErrConditionFailed) = true when ConditionalCheckFailed present")
	}
}

func TestTxCancelledError_Is_NotConditionFailed(t *testing.T) {
	e := &TxCancelledError{
		Reasons: []TxCancelReason{
			{Code: "ValidationError"},
		},
	}
	if errors.Is(e, ErrConditionFailed) {
		t.Fatal("expected Is(ErrConditionFailed) = false when no ConditionalCheckFailed")
	}
}

func TestTxCancelledError_Is_NotOtherErrors(t *testing.T) {
	e := &TxCancelledError{}
	if errors.Is(e, ErrNotFound) {
		t.Fatal("expected Is(ErrNotFound) = false")
	}
	if errors.Is(e, ErrValidation) {
		t.Fatal("expected Is(ErrValidation) = false")
	}
}

func TestIsTxCancelled_WithTxCancelledError(t *testing.T) {
	e := &TxCancelledError{
		Reasons: []TxCancelReason{{Code: "ConditionalCheckFailed"}},
	}
	if !IsTxCancelled(e) {
		t.Fatal("expected IsTxCancelled = true")
	}
}

func TestIsTxCancelled_Wrapped(t *testing.T) {
	e := &TxCancelledError{
		Reasons: []TxCancelReason{{Code: "ConditionalCheckFailed"}},
	}
	wrapped := fmt.Errorf("outer: %w", e)
	if !IsTxCancelled(wrapped) {
		t.Fatal("expected IsTxCancelled = true for wrapped error")
	}
}

func TestTxCancelReasons_WithTxCancelledError(t *testing.T) {
	reasons := []TxCancelReason{
		{Code: "ConditionalCheckFailed", Message: "version mismatch"},
		{Code: "None"},
	}
	e := &TxCancelledError{Reasons: reasons}

	got := TxCancelReasons(e)
	if len(got) != 2 {
		t.Fatalf("expected 2 reasons, got %d", len(got))
	}
	if got[0].Code != "ConditionalCheckFailed" {
		t.Fatalf("expected first reason code 'ConditionalCheckFailed', got %q", got[0].Code)
	}
}

func TestTxCancelReasons_WithWrappedError(t *testing.T) {
	e := &TxCancelledError{Reasons: []TxCancelReason{{Code: "Test"}}}
	wrapped := fmt.Errorf("outer: %w", e)

	got := TxCancelReasons(wrapped)
	if len(got) != 1 {
		t.Fatalf("expected 1 reason, got %d", len(got))
	}
}

func TestTxCancelReasons_NonTxError(t *testing.T) {
	got := TxCancelReasons(errors.New("some error"))
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestTxCancelReasons_NilError(t *testing.T) {
	got := TxCancelReasons(nil)
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}
