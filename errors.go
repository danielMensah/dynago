package dynago

import "errors"

var (
	// ErrNotFound is returned when a Get or Query returns no item.
	ErrNotFound = errors.New("dynago: item not found")

	// ErrConditionFailed is returned when a condition expression check fails.
	ErrConditionFailed = errors.New("dynago: condition check failed")

	// ErrValidation is returned for client-side validation errors such as
	// missing keys or unsupported types.
	ErrValidation = errors.New("dynago: validation error")

	// ErrTransactionCancelled is returned when a DynamoDB transaction is
	// cancelled.
	ErrTransactionCancelled = errors.New("dynago: transaction cancelled")
)

// Error is a dynago error that wraps a sentinel and an optional cause.
type Error struct {
	Sentinel error
	Cause    error
	Message  string
}

func (e *Error) Error() string {
	if e.Message != "" {
		if e.Cause != nil {
			return e.Message + ": " + e.Cause.Error()
		}
		return e.Message
	}
	if e.Cause != nil {
		return e.Sentinel.Error() + ": " + e.Cause.Error()
	}
	return e.Sentinel.Error()
}

func (e *Error) Is(target error) bool {
	return errors.Is(e.Sentinel, target)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// IsNotFound reports whether err matches ErrNotFound.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsCondCheckFailed reports whether err matches ErrConditionFailed.
func IsCondCheckFailed(err error) bool { return errors.Is(err, ErrConditionFailed) }

// IsValidation reports whether err matches ErrValidation.
func IsValidation(err error) bool { return errors.Is(err, ErrValidation) }

// IsTxCancelled reports whether err matches ErrTransactionCancelled.
func IsTxCancelled(err error) bool { return errors.Is(err, ErrTransactionCancelled) }

// wrapError creates a new Error that wraps the given cause with a sentinel.
func wrapError(sentinel error, cause error) error {
	return &Error{Sentinel: sentinel, Cause: cause}
}

// newError creates a new Error with a sentinel and a custom message.
func newError(sentinel error, msg string) error {
	return &Error{Sentinel: sentinel, Message: msg}
}
