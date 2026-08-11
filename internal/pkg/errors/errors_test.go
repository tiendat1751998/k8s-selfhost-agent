package errors

import (
	"errors"
	"testing"
)

func TestNewNotFound(t *testing.T) {
	err := NewNotFound("incident", "abc-123")
	if err.Code != "NOT_FOUND" {
		t.Errorf("expected code NOT_FOUND, got %s", err.Code)
	}
	if !errors.Is(err, ErrNotFound) {
		t.Error("expected error to wrap ErrNotFound")
	}
}

func TestNewValidation(t *testing.T) {
	err := NewValidation("namespace", "must not be empty")
	if err.Code != "VALIDATION_ERROR" {
		t.Errorf("expected code VALIDATION_ERROR, got %s", err.Code)
	}
	if !errors.Is(err, ErrValidation) {
		t.Error("expected error to wrap ErrValidation")
	}
}

func TestNewInternal(t *testing.T) {
	cause := errors.New("connection refused")
	err := NewInternal("database unavailable", cause)
	if err.Code != "INTERNAL_ERROR" {
		t.Errorf("expected code INTERNAL_ERROR, got %s", err.Code)
	}
	if !errors.Is(err, ErrInternal) {
		t.Error("expected error to wrap ErrInternal")
	}
}

func TestNewConflict(t *testing.T) {
	err := NewConflict("incident", "already resolved")
	if err.Code != "CONFLICT" {
		t.Errorf("expected code CONFLICT, got %s", err.Code)
	}
	if !errors.Is(err, ErrConflict) {
		t.Error("expected error to wrap ErrConflict")
	}
}

func TestNewTimeout(t *testing.T) {
	err := NewTimeout("rca_analysis")
	if err.Code != "TIMEOUT" {
		t.Errorf("expected code TIMEOUT, got %s", err.Code)
	}
	if !errors.Is(err, ErrTimeout) {
		t.Error("expected error to wrap ErrTimeout")
	}
}

func TestDomainError_Error(t *testing.T) {
	err := NewNotFound("pod", "nginx-123")
	expected := "[NOT_FOUND] pod with id 'nginx-123' not found: not found"
	if err.Error() != expected {
		t.Errorf("expected error string '%s', got '%s'", expected, err.Error())
	}
}

func TestDomainError_ErrorWithoutCause(t *testing.T) {
	err := &DomainError{Code: "TEST", Message: "test error"}
	expected := "[TEST] test error"
	if err.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, err.Error())
	}
}

func TestWrap_NilError(t *testing.T) {
	if Wrap(nil, "context") != nil {
		t.Error("expected nil result when wrapping nil error")
	}
}

func TestWrap_WithError(t *testing.T) {
	original := errors.New("original error")
	wrapped := Wrap(original, "failed to connect")
	if wrapped == nil {
		t.Fatal("expected non-nil wrapped error")
	}
	if !errors.Is(wrapped, original) {
		t.Error("expected wrapped error to contain original")
	}
}

func TestIs(t *testing.T) {
	err := NewNotFound("incident", "123")
	if !Is(err, ErrNotFound) {
		t.Error("expected Is to return true")
	}
	if Is(err, ErrValidation) {
		t.Error("expected Is to return false for different sentinel")
	}
}

func TestAs(t *testing.T) {
	err := NewNotFound("incident", "123")
	var domainErr *DomainError
	if !As(err, &domainErr) {
		t.Error("expected As to return true for DomainError")
	}
	if domainErr.Code != "NOT_FOUND" {
		t.Errorf("expected code NOT_FOUND after As, got %s", domainErr.Code)
	}
}
