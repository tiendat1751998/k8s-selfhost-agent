package http

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
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
}

// GetSettings handles GET / - returns settings for a tenant, optionally filtered by ?category=
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
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

	writeJSON(w, http.StatusOK, items)
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

	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	userID := tenancy.UserIDFromContext(r.Context())

	items := make([]settings.Setting, len(req.Settings))
	for i, s := range req.Settings {
		items[i] = settings.Setting{
			Category:  s.Category,
			Key:       s.Key,
			Value:     s.Value,
			TenantID:  tenantID,
			UpdatedBy: userID,
		}
	}

	if err := h.repo.BulkUpsert(r.Context(), items); err != nil {
		h.logger.Error("failed to update settings", zap.Error(err), zap.String("tenant_id", tenantID))
		writeError(w, http.StatusInternalServerError, "failed to update settings", err)
		return
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
