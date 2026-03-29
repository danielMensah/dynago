package dynago

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinelErrors_DirectMatch(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", ErrNotFound},
		{"ErrConditionFailed", ErrConditionFailed},
		{"ErrValidation", ErrValidation},
		{"ErrTransactionCancelled", ErrTransactionCancelled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, tt.err) {
				t.Errorf("errors.Is(%v, %v) = false, want true", tt.err, tt.err)
			}
		})
	}
}

func TestSentinelErrors_AreDistinct(t *testing.T) {
	sentinels := []error{ErrNotFound, ErrConditionFailed, ErrValidation, ErrTransactionCancelled}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j && errors.Is(a, b) {
				t.Errorf("errors.Is(%v, %v) = true, want false", a, b)
			}
		}
	}
}

func TestWrappedError_MatchesSentinel(t *testing.T) {
	cause := fmt.Errorf("underlying problem")
	wrapped := wrapError(ErrNotFound, cause)

	if !errors.Is(wrapped, ErrNotFound) {
		t.Error("wrapped error should match ErrNotFound")
	}
	if errors.Is(wrapped, ErrConditionFailed) {
		t.Error("wrapped error should not match ErrConditionFailed")
	}
}

func TestWrappedError_UnwrapsCause(t *testing.T) {
	cause := fmt.Errorf("root cause")
	wrapped := wrapError(ErrValidation, cause)

	var dynaErr *Error
	if !errors.As(wrapped, &dynaErr) {
		t.Fatal("errors.As should extract *Error")
	}
	if dynaErr.Cause != cause {
		t.Errorf("Cause = %v, want %v", dynaErr.Cause, cause)
	}
	if dynaErr.Sentinel != ErrValidation {
		t.Errorf("Sentinel = %v, want %v", dynaErr.Sentinel, ErrValidation)
	}
}

func TestErrorsAs_ExtractsErrorType(t *testing.T) {
	err := newError(ErrConditionFailed, "put condition failed")

	var dynaErr *Error
	if !errors.As(err, &dynaErr) {
		t.Fatal("errors.As should extract *Error from newError result")
	}
	if dynaErr.Message != "put condition failed" {
		t.Errorf("Message = %q, want %q", dynaErr.Message, "put condition failed")
	}
}

func TestErrorMessage_SentinelOnly(t *testing.T) {
	err := &Error{Sentinel: ErrNotFound}
	want := "dynago: item not found"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestErrorMessage_SentinelWithCause(t *testing.T) {
	err := wrapError(ErrNotFound, fmt.Errorf("timeout"))
	want := "dynago: item not found: timeout"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestErrorMessage_CustomMessage(t *testing.T) {
	err := newError(ErrValidation, "missing primary key")
	want := "missing primary key"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestErrorMessage_CustomMessageWithCause(t *testing.T) {
	err := &Error{
		Sentinel: ErrValidation,
		Message:  "field check failed",
		Cause:    fmt.Errorf("name is empty"),
	}
	want := "field check failed: name is empty"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestIsNotFound(t *testing.T) {
	if !IsNotFound(ErrNotFound) {
		t.Error("IsNotFound(ErrNotFound) = false, want true")
	}
	if !IsNotFound(wrapError(ErrNotFound, fmt.Errorf("cause"))) {
		t.Error("IsNotFound(wrapped) = false, want true")
	}
	if IsNotFound(ErrConditionFailed) {
		t.Error("IsNotFound(ErrConditionFailed) = true, want false")
	}
	if IsNotFound(nil) {
		t.Error("IsNotFound(nil) = true, want false")
	}
}

func TestIsCondCheckFailed(t *testing.T) {
	if !IsCondCheckFailed(ErrConditionFailed) {
		t.Error("IsCondCheckFailed(ErrConditionFailed) = false, want true")
	}
	if !IsCondCheckFailed(wrapError(ErrConditionFailed, fmt.Errorf("cause"))) {
		t.Error("IsCondCheckFailed(wrapped) = false, want true")
	}
	if IsCondCheckFailed(ErrNotFound) {
		t.Error("IsCondCheckFailed(ErrNotFound) = true, want false")
	}
}

func TestIsValidation(t *testing.T) {
	if !IsValidation(ErrValidation) {
		t.Error("IsValidation(ErrValidation) = false, want true")
	}
	if !IsValidation(wrapError(ErrValidation, fmt.Errorf("cause"))) {
		t.Error("IsValidation(wrapped) = false, want true")
	}
	if IsValidation(ErrNotFound) {
		t.Error("IsValidation(ErrNotFound) = true, want false")
	}
}

func TestIsTxCancelled(t *testing.T) {
	if !IsTxCancelled(ErrTransactionCancelled) {
		t.Error("IsTxCancelled(ErrTransactionCancelled) = false, want true")
	}
	if !IsTxCancelled(wrapError(ErrTransactionCancelled, fmt.Errorf("cause"))) {
		t.Error("IsTxCancelled(wrapped) = false, want true")
	}
	if IsTxCancelled(ErrNotFound) {
		t.Error("IsTxCancelled(ErrNotFound) = true, want false")
	}
}

func TestWrappedError_FmtErrorfWrapping(t *testing.T) {
	inner := wrapError(ErrNotFound, fmt.Errorf("db timeout"))
	outer := fmt.Errorf("operation failed: %w", inner)

	if !errors.Is(outer, ErrNotFound) {
		t.Error("fmt.Errorf wrapped error should still match sentinel via errors.Is")
	}
	if !IsNotFound(outer) {
		t.Error("IsNotFound should work through fmt.Errorf wrapping")
	}
}

func TestNewError_MatchesSentinel(t *testing.T) {
	err := newError(ErrTransactionCancelled, "2 of 3 actions failed")
	if !IsTxCancelled(err) {
		t.Error("newError should match its sentinel via IsTxCancelled")
	}
	if !errors.Is(err, ErrTransactionCancelled) {
		t.Error("newError should match its sentinel via errors.Is")
	}
}
