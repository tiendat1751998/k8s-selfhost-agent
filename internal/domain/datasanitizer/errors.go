package datasanitizer

import (
	"errors"
	"fmt"
)

// Sentinel domain errors.
var (
	ErrValidation                = errors.New("validation failed")
	ErrInvalidPolicy             = errors.New("sanitization policy is invalid")
	ErrInvalidRule               = errors.New("masking rule is invalid")
	ErrProductionTargetViolation = errors.New("target cluster or database is production; cloning to production is strictly forbidden")
	ErrPIILeakDetected           = errors.New("post-sanitization audit failed: unmasked PII detected in output stream")
	ErrExecutionFailed           = errors.New("sanitization clone execution failed")
)

// RFC 7807 Error Code Constants.
const (
	CodeValidation                = "ERR_VALIDATION_FAILED"
	CodeSanitizationPolicyInvalid = "ERR_SANITIZATION_POLICY_INVALID"
	CodeMaskingRuleInvalid        = "ERR_MASKING_RULE_INVALID"
	CodeProductionTargetViolation = "ERR_PRODUCTION_TARGET_VIOLATION"
	CodeClonePIILeakDetected      = "ERR_CLONE_PII_LEAK_DETECTED"
	CodeCloneExecutionFailed      = "ERR_CLONE_EXECUTION_FAILED"
)

// RFC7807ProblemDetail defines the standard RFC 7807 error problem detail.
type RFC7807ProblemDetail struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance,omitempty"`
	Code     string `json:"code"`
}

// Error implements error interface.
func (p RFC7807ProblemDetail) Error() string {
	return fmt.Sprintf("[%s] %s: %s (status %d)", p.Code, p.Title, p.Detail, p.Status)
}

// NewPolicyInvalidError builds RFC 7807 compliant error for invalid policy.
func NewPolicyInvalidError(detail string) RFC7807ProblemDetail {
	return RFC7807ProblemDetail{
		Title:  "Sanitization Policy Invalid",
		Status: 400,
		Detail: detail,
		Code:   CodeSanitizationPolicyInvalid,
	}
}

// NewPIILeakError builds RFC 7807 compliant error for PII leak.
func NewPIILeakError(detail string) RFC7807ProblemDetail {
	return RFC7807ProblemDetail{
		Title:  "PII Leak Detected",
		Status: 500,
		Detail: detail,
		Code:   CodeClonePIILeakDetected,
	}
}

// NewProductionTargetError builds RFC 7807 compliant error for production target violations.
func NewProductionTargetError(detail string) RFC7807ProblemDetail {
	return RFC7807ProblemDetail{
		Title:  "Production Target Violation",
		Status: 403,
		Detail: detail,
		Code:   CodeProductionTargetViolation,
	}
}
