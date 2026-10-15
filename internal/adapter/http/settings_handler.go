package http

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/settings"
	"github.com/datdt/k8sselfhost/internal/pkg/httputil"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// SettingsHandler provides REST API endpoints for platform settings CRUD and integration testing.
type SettingsHandler struct {
	repo       settings.Repository
	httpClient *http.Client
	logger     *zap.Logger
}

// NewSettingsHandler creates a new SettingsHandler instance.
func NewSettingsHandler(repo settings.Repository, logger *zap.Logger) *SettingsHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SettingsHandler{
		repo:       repo,
		httpClient: httputil.NewSafeHTTPClient(5 * time.Second),
		logger:     logger,
	}
}

// RegisterRoutes registers all platform settings endpoints on the chi router.
func (h *SettingsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.GetSettings)
	r.Put("/", h.UpdateSettings)
	r.Get("/defaults", h.GetDefaults)
	r.Post("/integrations/test", h.TestIntegration)
	r.Get("/api-keys", h.ListAPIKeys)
	r.Post("/api-keys", h.CreateAPIKey)
	r.Delete("/api-keys/{id}", h.DeleteAPIKey)
	r.Get("/keys", h.ListAPIKeys)
	r.Post("/keys", h.CreateAPIKey)
	r.Delete("/keys/{id}", h.DeleteAPIKey)
}

func isSensitiveKey(k string) bool {
	lower := strings.ToLower(k)
	for _, sub := range []string{"password", "secret", "token", "key", "webhook_url", "bot_token"} {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}

// GetSettings handles GET / - returns settings for a tenant, optionally filtered by ?category=
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeJSON(w, http.StatusOK, []settings.Setting{})
		return
	}
	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	category := strings.TrimSpace(r.URL.Query().Get("category"))
	var items []settings.Setting
	var err error

	if category != "" {
		items, err = h.repo.GetByCategory(r.Context(), tenantID, category)
	} else {
		items, err = h.repo.GetAll(r.Context(), tenantID)
	}

	if err != nil {
		h.logger.Error("failed to get settings", zap.Error(err), zap.String("tenant_id", tenantID), zap.String("category", category))
		writeError(w, http.StatusInternalServerError, "failed to get settings", err)
		return
	}

	if items == nil {
		items = make([]settings.Setting, 0)
	}

	maskedItems := make([]settings.Setting, len(items))
	for i, item := range items {
		masked := item
		if isSensitiveKey(item.Key) && masked.Value != "" {
			masked.Value = "***"
		}
		maskedItems[i] = masked
	}

	writeJSON(w, http.StatusOK, maskedItems)
}

type updateSettingItem struct {
	Category string `json:"category"`
	Key      string `json:"key"`
	Value    string `json:"value"`
}

type updateSettingsRequest struct {
	Settings []updateSettingItem `json:"settings"`
}

func (r *updateSettingsRequest) Validate() error {
	ve := NewValidationError("validation failed")
	for i, item := range r.Settings {
		if strings.TrimSpace(item.Category) == "" {
			ve.Add(fmt.Sprintf("settings[%d].category", i), "category is required")
		}
		if strings.TrimSpace(item.Key) == "" {
			ve.Add(fmt.Sprintf("settings[%d].key", i), "key is required")
		}
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// UpdateSettings handles PUT / - bulk updates platform settings for the tenant.
func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[updateSettingsRequest](w, r)
	if !ok {
		return
	}
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "settings repository unavailable", nil)
		return
	}

	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	userID := tenancy.UserIDFromContext(r.Context())

	items := make([]settings.Setting, 0, len(req.Settings))
	for _, s := range req.Settings {
		if s.Value == "***" {
			// Skip masked placeholders so real secrets are preserved
			continue
		}
		items = append(items, settings.Setting{
			Category:  s.Category,
			Key:       s.Key,
			Value:     s.Value,
			TenantID:  tenantID,
			UpdatedBy: userID,
		})
	}

	if len(items) > 0 {
		if err := h.repo.BulkUpsert(r.Context(), items); err != nil {
			h.logger.Error("failed to update settings", zap.Error(err), zap.String("tenant_id", tenantID))
			writeError(w, http.StatusInternalServerError, "failed to update settings", err)
			return
		}
	}

	writeJSON(w, http.StatusOK, items)
}

// GetDefaults handles GET /defaults - returns the default settings map organized by category.
func (h *SettingsHandler) GetDefaults(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, settings.Defaults)
}

type testIntegrationRequest struct {
	URL string `json:"url"`
}

func (r *testIntegrationRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.URL) == "" {
		ve.Add("url", "url is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

type testIntegrationResponse struct {
	Reachable  bool  `json:"reachable"`
	StatusCode int   `json:"status_code"`
	LatencyMS  int64 `json:"latency_ms"`
}

// TestIntegration handles POST /integrations/test - tests the reachability of a service URL.
func (h *SettingsHandler) TestIntegration(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[testIntegrationRequest](w, r)
	if !ok {
		return
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodHead, req.URL, nil)
	if err != nil {
		writeJSON(w, http.StatusOK, testIntegrationResponse{
			Reachable:  false,
			StatusCode: 0,
			LatencyMS:  time.Since(start).Milliseconds(),
		})
		return
	}

	client := h.httpClient
	if client == nil {
		client = httputil.NewSafeHTTPClient(5 * time.Second)
	}

	resp, err := client.Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		writeJSON(w, http.StatusOK, testIntegrationResponse{
			Reachable:  false,
			StatusCode: 0,
			LatencyMS:  latency,
		})
		return
	}
	defer resp.Body.Close()

	writeJSON(w, http.StatusOK, testIntegrationResponse{
		Reachable:  true,
		StatusCode: resp.StatusCode,
		LatencyMS:  latency,
	})
}

// ApiKeyItem represents an API token metadata record.
type ApiKeyItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
	LastUsed  string `json:"last_used"`
	Token     string `json:"token,omitempty"`
}

type createApiKeyRequest struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

func (r *createApiKeyRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if strings.TrimSpace(r.Role) == "" {
		ve.Add("role", "role is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// ListAPIKeys handles GET /api-keys - returns all active API keys for tenant.
func (h *SettingsHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeJSON(w, http.StatusOK, []ApiKeyItem{})
		return
	}
	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	items, err := h.repo.GetByCategory(r.Context(), tenantID, "api_keys")
	if err != nil {
		h.logger.Error("failed to get api keys", zap.Error(err), zap.String("tenant_id", tenantID))
		writeError(w, http.StatusInternalServerError, "failed to get api keys", err)
		return
	}
	res := make([]ApiKeyItem, 0)
	for _, it := range items {
		if it.Value == "" || it.Value == "deleted" {
			continue
		}
		var key ApiKeyItem
		if err := json.Unmarshal([]byte(it.Value), &key); err == nil && key.ID != "" {
			res = append(res, key)
		}
	}
	writeJSON(w, http.StatusOK, res)
}

// CreateAPIKey handles POST /api-keys - generates and stores a new API key.
func (h *SettingsHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createApiKeyRequest](w, r)
	if !ok {
		return
	}
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "settings repository unavailable", nil)
		return
	}
	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	userID := tenancy.UserIDFromContext(r.Context())

	rawBytes := make([]byte, 24)
	if _, err := cryptorand.Read(rawBytes); err != nil {
		h.logger.Error("failed to generate random bytes for api key", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to generate api key", err)
		return
	}
	token := fmt.Sprintf("k8s_live_%s", hex.EncodeToString(rawBytes))
	prefix := fmt.Sprintf("%s...", token[:16])

	id := uuid.NewString()
	keyItem := ApiKeyItem{
		ID:        id,
		Name:      req.Name,
		Prefix:    prefix,
		Role:      req.Role,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		LastUsed:  "never",
	}

	storedItem := keyItem
	storedItem.Token = ""
	valBytes, err := json.Marshal(storedItem)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode api key", err)
		return
	}
	if err := h.repo.Upsert(r.Context(), &settings.Setting{
		ID:        id,
		TenantID:  tenantID,
		Category:  "api_keys",
		Key:       id,
		Value:     string(valBytes),
		UpdatedBy: userID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store api key", err)
		return
	}

	keyItem.Token = token
	writeJSON(w, http.StatusCreated, keyItem)
}

// DeleteAPIKey handles DELETE /api-keys/{id} - revokes an API key.
func (h *SettingsHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "settings repository unavailable", nil)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "api key id is required", nil)
		return
	}
	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	userID := tenancy.UserIDFromContext(r.Context())
	if err := h.repo.Upsert(r.Context(), &settings.Setting{
		ID:        id,
		TenantID:  tenantID,
		Category:  "api_keys",
		Key:       id,
		Value:     "",
		UpdatedBy: userID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete api key", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
