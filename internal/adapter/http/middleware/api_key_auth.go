package middleware

import (
	"context"
)

// APIKeyInfo holds caller credentials and scope resolved from an authenticated API key.
type APIKeyInfo struct {
	KeyID    string
	Name     string
	Role     string
	TenantID string
}

// APIKeyValidator defines the interface for validating hashed API keys.
type APIKeyValidator interface {
	ValidateAPIKey(ctx context.Context, tokenHash string) (*APIKeyInfo, error)
}

var defaultAPIKeyValidator APIKeyValidator

// SetAPIKeyValidator configures the global APIKeyValidator for JWTAuthMiddleware.
func SetAPIKeyValidator(v APIKeyValidator) {
	defaultAPIKeyValidator = v
}

// GetAPIKeyValidator returns the configured global APIKeyValidator.
func GetAPIKeyValidator() APIKeyValidator {
	return defaultAPIKeyValidator
}
