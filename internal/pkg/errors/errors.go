// Package errors provides domain-specific error types with context wrapping.
package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for common domain error categories.
var (
	ErrNotFound     = errors.New("not found")
	ErrValidation   = errors.New("validation error")
	ErrInternal     = errors.New("internal error")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrConflict     = errors.New("conflict")
	ErrTimeout      = errors.New("timeout")
)

// DomainError represents a domain-level error with context, an error code, and an underlying cause.
type DomainError struct {
	Code    string
	Message string
	Err     error
}

// Error returns the formatted error string.
func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *DomainError) Unwrap() error {
	return e.Err
}

// NewNotFound creates a not-found domain error.
func NewNotFound(resource string, id string) *DomainError {
	return &DomainError{
		Code:    "NOT_FOUND",
		Message: fmt.Sprintf("%s with id '%s' not found", resource, id),
		Err:     ErrNotFound,
	}
}

// NewValidation creates a validation domain error.
func NewValidation(field string, reason string) *DomainError {
	return &DomainError{
		Code:    "VALIDATION_ERROR",
		Message: fmt.Sprintf("validation failed for field '%s': %s", field, reason),
		Err:     ErrValidation,
	}
}

// NewInternal creates an internal domain error wrapping the underlying cause.
func NewInternal(message string, cause error) *DomainError {
	return &DomainError{
		Code:    "INTERNAL_ERROR",
		Message: message,
		Err:     fmt.Errorf("%w: %w", ErrInternal, cause),
	}
}

// NewConflict creates a conflict domain error.
func NewConflict(resource string, reason string) *DomainError {
	return &DomainError{
		Code:    "CONFLICT",
		Message: fmt.Sprintf("conflict on %s: %s", resource, reason),
		Err:     ErrConflict,
	}
}

// NewTimeout creates a timeout domain error.
func NewTimeout(operation string) *DomainError {
	return &DomainError{
		Code:    "TIMEOUT",
		Message: fmt.Sprintf("operation '%s' timed out", operation),
		Err:     ErrTimeout,
	}
}

// Wrap wraps an error with additional context message.
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// Is reports whether any error in err's chain matches target.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's chain that matches target, and if so, sets
// target to that error value and returns true.
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}
