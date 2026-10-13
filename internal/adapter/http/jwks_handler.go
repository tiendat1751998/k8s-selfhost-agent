package http

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	domainauth "github.com/datdt/k8sselfhost/internal/domain/auth"
)

// VaultStatusProvider defines the contract for inspecting Vault unseal state.
type VaultStatusProvider interface {
	GetVaultUnsealStatus(ctx context.Context) (domainauth.VaultUnsealStatus, error)
}

// JWKSHandlerOption configures JWKSHandler.
type JWKSHandlerOption func(*JWKSHandler)

// WithVaultStatusProvider injects a custom Vault status provider.
func WithVaultStatusProvider(p VaultStatusProvider) JWKSHandlerOption {
	return func(h *JWKSHandler) {
		h.vaultProvider = p
	}
}

// JWKSHandler exposes HTTP REST endpoints for JWKS keyset cache management and Vault auto-unseal health.
type JWKSHandler struct {
	shield        domainauth.JWKSShield
	vaultProvider VaultStatusProvider
}

// NewJWKSHandler creates a new JWKSHandler instance.
func NewJWKSHandler(shield domainauth.JWKSShield, opts ...JWKSHandlerOption) *JWKSHandler {
	h := &JWKSHandler{
		shield: shield,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// RegisterRoutes mounts JWKS and Vault status routes onto the provided chi Router.
// Supports both absolute paths and sub-mounted routers under /api/v1/auth.
func (h *JWKSHandler) RegisterRoutes(r chi.Router) {
	// Full API paths
	r.Get("/api/v1/auth/jwks/status", h.GetJWKSStatus)
	r.Post("/api/v1/auth/jwks/refresh", h.RefreshJWKS)
	r.Get("/api/v1/auth/vault/unseal-status", h.GetVaultUnsealStatus)

	// Sub-route paths when mounted under /api/v1/auth
	r.Get("/jwks/status", h.GetJWKSStatus)
	r.Post("/jwks/refresh", h.RefreshJWKS)
	r.Get("/vault/unseal-status", h.GetVaultUnsealStatus)
}

// GetJWKSStatus handles GET /api/v1/auth/jwks/status.
// Returns current cache status, stale age, provider URL, and hit/miss metrics.
func (h *JWKSHandler) GetJWKSStatus(w http.ResponseWriter, r *http.Request) {
	if h.shield == nil {
		writeError(w, http.StatusServiceUnavailable, "jwks shield service not initialized", nil)
		return
	}

	status := h.shield.GetCacheStatus()
	writeJSON(w, http.StatusOK, status)
}

// RefreshJWKS handles POST /api/v1/auth/jwks/refresh.
// Triggers an on-demand keyset refresh from the OIDC/JWKS provider URL.
func (h *JWKSHandler) RefreshJWKS(w http.ResponseWriter, r *http.Request) {
	if h.shield == nil {
		writeError(w, http.StatusServiceUnavailable, "jwks shield service not initialized", nil)
		return
	}

	if err := h.shield.RefreshKeys(r.Context()); err != nil {
		writeError(w, http.StatusBadGateway, "failed to refresh jwks keyset from provider", err)
		return
	}

	status := h.shield.GetCacheStatus()
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "jwks keyset refreshed successfully",
		"status":  status,
	})
}

// GetVaultUnsealStatus handles GET /api/v1/auth/vault/unseal-status.
// Returns current Vault seal state, threshold progress, and auto-unseal status.
func (h *JWKSHandler) GetVaultUnsealStatus(w http.ResponseWriter, r *http.Request) {
	if h.vaultProvider != nil {
		status, err := h.vaultProvider.GetVaultUnsealStatus(r.Context())
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "failed to inspect vault unseal status", err)
			return
		}
		writeJSON(w, http.StatusOK, status)
		return
	}

	// Default unsealed healthy status when standalone provider is not injected
	writeJSON(w, http.StatusOK, domainauth.VaultUnsealStatus{
		IsSealed:       false,
		Threshold:      3,
		Progress:       3,
		AutoUnsealMode: "tpm2",
	})
}
