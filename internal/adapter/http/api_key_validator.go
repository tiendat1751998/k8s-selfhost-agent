package http

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/settings"
)

type apiKeyFinder interface {
	FindAPIKeyByHash(ctx context.Context, tokenHash string) (*settings.Setting, error)
}

// SettingsAPIKeyValidator validates API keys stored in settings repository.
type SettingsAPIKeyValidator struct {
	repo settings.Repository
}

// NewSettingsAPIKeyValidator constructs a new SettingsAPIKeyValidator.
func NewSettingsAPIKeyValidator(repo settings.Repository) *SettingsAPIKeyValidator {
	return &SettingsAPIKeyValidator{repo: repo}
}

// ValidateAPIKey checks if the tokenHash corresponds to an active non-revoked API key.
func (v *SettingsAPIKeyValidator) ValidateAPIKey(ctx context.Context, tokenHash string) (*middleware.APIKeyInfo, error) {
	if v == nil || v.repo == nil || strings.TrimSpace(tokenHash) == "" {
		return nil, nil
	}

	// Try optimized SQL finder if repository supports it
	if finder, ok := v.repo.(apiKeyFinder); ok {
		setting, err := finder.FindAPIKeyByHash(ctx, tokenHash)
		if err == nil && setting != nil && setting.Value != "" && setting.Value != "deleted" {
			var stored ApiKeyStored
			if err := json.Unmarshal([]byte(setting.Value), &stored); err == nil && stored.TokenHash == tokenHash {
				tenantID := stored.TenantID
				if tenantID == "" {
					tenantID = setting.TenantID
				}
				if tenantID == "" {
					tenantID = "default-tenant"
				}
				return &middleware.APIKeyInfo{
					KeyID:    stored.ID,
					Name:     stored.Name,
					Role:     stored.Role,
					TenantID: tenantID,
				}, nil
			}
		}
	}

	// Fallback scanning (e.g. mock repositories in tests)
	items, err := v.repo.GetAll(ctx, "")
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		if it.Category != "api_keys" || it.Value == "" || it.Value == "deleted" {
			continue
		}
		var stored ApiKeyStored
		if err := json.Unmarshal([]byte(it.Value), &stored); err == nil && stored.TokenHash == tokenHash {
			tenantID := stored.TenantID
			if tenantID == "" {
				tenantID = it.TenantID
			}
			if tenantID == "" {
				tenantID = "default-tenant"
			}
			return &middleware.APIKeyInfo{
				KeyID:    stored.ID,
				Name:     stored.Name,
				Role:     stored.Role,
				TenantID: tenantID,
			}, nil
		}
	}

	return nil, nil
}
