// Package http provides HTTP adapters and request routing.
package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/cloud"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// CloudProviderFactory produces cloud.Provider instances for a given cloud provider type.
type CloudProviderFactory interface {
	GetProvider(providerType cloud.ProviderType) (cloud.Provider, error)
}

// CloudHandler provides REST API endpoints for cloud account management.
type CloudHandler struct {
	repo    cloud.AccountRepository
	factory CloudProviderFactory
	logger  *zap.Logger
}

// NewCloudHandler creates a new CloudHandler instance.
func NewCloudHandler(repo cloud.AccountRepository, factory CloudProviderFactory, logger *zap.Logger) *CloudHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CloudHandler{
		repo:    repo,
		factory: factory,
		logger:  logger,
	}
}

// RegisterRoutes registers all cloud account management endpoints on the chi router.
func (h *CloudHandler) RegisterRoutes(r chi.Router) {
	r.Post("/accounts", h.CreateAccount)
	r.Get("/accounts", h.ListAccounts)
	r.Get("/accounts/{id}", h.GetAccount)
	r.Delete("/accounts/{id}", h.DeleteAccount)
	r.Post("/accounts/{id}/validate", h.ValidateAccount)
}

type createCloudAccountRequest struct {
	Name        string             `json:"name"`
	Provider    cloud.ProviderType `json:"provider"`
	Credentials string             `json:"credentials"`
	Region      string             `json:"region"`
}

func (r *createCloudAccountRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if strings.TrimSpace(string(r.Provider)) == "" {
		ve.Add("provider", "provider is required")
	} else {
		switch r.Provider {
		case cloud.ProviderAWS, cloud.ProviderGCP, cloud.ProviderAzure:
		default:
			ve.Add("provider", "unsupported provider: "+string(r.Provider))
		}
	}
	if strings.TrimSpace(r.Credentials) == "" {
		ve.Add("credentials", "credentials is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// CreateAccount handles POST /accounts - creates a cloud account and encrypts credentials.
func (h *CloudHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createCloudAccountRequest](w, r)
	if !ok {
		return
	}

	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	account := cloud.NewCloudAccount(req.Name, req.Provider, req.Credentials, req.Region, tenantID)
	if err := h.repo.Create(r.Context(), account); err != nil {
		h.logger.Error("failed to create cloud account", zap.Error(err), zap.String("name", req.Name))
		writeError(w, http.StatusInternalServerError, "failed to create cloud account", err)
		return
	}

	account.EncryptedCreds = ""
	writeJSON(w, http.StatusCreated, account)
}

// ListAccounts handles GET /accounts - lists all cloud accounts for the tenant without credentials.
func (h *CloudHandler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	tenantID := tenancy.TenantIDFromContext(r.Context())
	accounts, err := h.repo.List(r.Context(), tenantID)
	if err != nil {
		h.logger.Error("failed to list cloud accounts", zap.Error(err), zap.String("tenant_id", tenantID))
		writeError(w, http.StatusInternalServerError, "failed to list cloud accounts", err)
		return
	}
	if accounts == nil {
		accounts = make([]cloud.CloudAccount, 0)
	}
	for i := range accounts {
		accounts[i].EncryptedCreds = ""
	}
	writeJSON(w, http.StatusOK, accounts)
}

// GetAccount handles GET /accounts/{id} - returns a single cloud account without credentials.
func (h *CloudHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "account id is required", nil)
		return
	}

	account, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get cloud account", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to get cloud account", err)
		return
	}
	if account == nil {
		writeError(w, http.StatusNotFound, "cloud account not found", nil)
		return
	}

	account.EncryptedCreds = ""
	writeJSON(w, http.StatusOK, account)
}

// DeleteAccount handles DELETE /accounts/{id} - deletes a cloud account.
func (h *CloudHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "account id is required", nil)
		return
	}

	account, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to find cloud account for deletion", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to delete cloud account", err)
		return
	}
	if account == nil {
		writeError(w, http.StatusNotFound, "cloud account not found", nil)
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "cloud account not found", err)
			return
		}
		h.logger.Error("failed to delete cloud account", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to delete cloud account", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ValidateAccount handles POST /accounts/{id}/validate - validates cloud credentials with the provider SDK.
func (h *CloudHandler) ValidateAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "account id is required", nil)
		return
	}

	account, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get cloud account for validation", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to get cloud account", err)
		return
	}
	if account == nil {
		writeError(w, http.StatusNotFound, "cloud account not found", nil)
		return
	}

	if h.factory == nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "not_configured",
			"message": "Cloud provider SDK not yet configured",
		})
		return
	}

	provider, err := h.factory.GetProvider(account.Provider)
	if err != nil || provider == nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "not_configured",
			"message": "Cloud provider SDK not yet configured",
		})
		return
	}

	if err := provider.ValidateCredentials(r.Context(), account); err != nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "invalid",
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "active",
		"message": "Cloud provider credentials validated successfully",
	})
}
