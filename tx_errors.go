package dynago

import (
	"errors"
	"fmt"
	"strings"
)

// TxCancelReason describes why a single operation within a transaction was
// cancelled.
type TxCancelReason struct {
	Code    string
	Message string
}

// TxCancelledError is returned when a DynamoDB transaction is cancelled. It
// contains per-operation failure reasons.
type TxCancelledError struct {
	Reasons []TxCancelReason
}

// Error returns a human-readable description of the cancelled transaction.
func (e *TxCancelledError) Error() string {
	if len(e.Reasons) == 0 {
		return "dynago: transaction cancelled"
	}
	parts := make([]string, len(e.Reasons))
	for i, r := range e.Reasons {
		if r.Code == "" && r.Message == "" {
			parts[i] = "None"
		} else if r.Message != "" {
			parts[i] = fmt.Sprintf("%s: %s", r.Code, r.Message)
		} else {
			parts[i] = r.Code
		}
	}
	return fmt.Sprintf("dynago: transaction cancelled: [%s]", strings.Join(parts, ", "))
}

// Is reports whether this error matches target. It matches
// ErrTransactionCancelled and ErrConditionFailed (when at least one reason
// has code "ConditionalCheckFailed").
func (e *TxCancelledError) Is(target error) bool {
	if target == ErrTransactionCancelled {
		return true
	}
	if target == ErrConditionFailed {
		for _, r := range e.Reasons {
			if r.Code == "ConditionalCheckFailed" {
				return true
			}
		}
	}
	return false
}

// TxCancelReasons extracts the per-operation cancellation reasons from err.
// It returns nil if err is not a TxCancelledError.
func TxCancelReasons(err error) []TxCancelReason {
	var txErr *TxCancelledError
	if errors.As(err, &txErr) {
		return txErr.Reasons
	}
	return nil
}
