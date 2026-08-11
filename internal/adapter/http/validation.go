package http

import (
	"fmt"
	"strings"
)

// Validator is implemented by request structs that perform self-validation.
type Validator interface {
	Validate() error
}

// ValidationError contains field-level validation failure details.
type ValidationError struct {
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func NewValidationError(msg string) *ValidationError {
	return &ValidationError{
		Message: msg,
		Fields:  make(map[string]string),
	}
}

func (ve *ValidationError) Add(field, detail string) *ValidationError {
	ve.Fields[field] = detail
	return ve
}

func (ve *ValidationError) HasErrors() bool {
	return len(ve.Fields) > 0
}

func (ve *ValidationError) Error() string {
	if len(ve.Fields) == 0 {
		return ve.Message
	}
	var pairs []string
	for k, v := range ve.Fields {
		pairs = append(pairs, fmt.Sprintf("%s: %s", k, v))
	}
	return fmt.Sprintf("%s (%s)", ve.Message, strings.Join(pairs, ", "))
}
