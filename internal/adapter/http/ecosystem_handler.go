package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/ecosystem"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
	usecaseEcosystem "github.com/datdt/k8sselfhost/internal/usecase/ecosystem"
)

// EcosystemHandler provides REST API endpoints for ecosystem tool detection, summary stats, and manual tool management.
type EcosystemHandler struct {
	usecase usecaseEcosystem.Usecase
	logger  *zap.Logger
}

// NewEcosystemHandler creates a new EcosystemHandler instance.
func NewEcosystemHandler(usecase usecaseEcosystem.Usecase, logger *zap.Logger) *EcosystemHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &EcosystemHandler{
		usecase: usecase,
		logger:  logger,
	}
}

// RegisterRoutes registers all ecosystem detection endpoints on the chi router.
func (h *EcosystemHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.GetTools)
	r.Get("/summary", h.GetSummary)
	r.Post("/scan", h.TriggerScan)
	r.Post("/tools", h.CreateTool)
	r.Delete("/tools/{id}", h.DeleteTool)
}

// GetTools handles GET / - returns all detected tools for the current tenant.
func (h *EcosystemHandler) GetTools(w http.ResponseWriter, r *http.Request) {
	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	category := strings.TrimSpace(r.URL.Query().Get("category"))
	tools, err := h.usecase.GetAll(r.Context(), tenantID)
	if err != nil {
		h.logger.Error("failed to get ecosystem tools", zap.Error(err), zap.String("tenant_id", tenantID))
		writeError(w, http.StatusInternalServerError, "failed to get ecosystem tools", err)
		return
	}

	if category != "" {
		filtered := make([]ecosystem.DetectedTool, 0)
		for _, t := range tools {
			if strings.EqualFold(t.Category, category) {
				filtered = append(filtered, t)
			}
		}
		tools = filtered
	}

	if tools == nil {
		tools = make([]ecosystem.DetectedTool, 0)
	}

	writeJSON(w, http.StatusOK, tools)
}

// GetSummary handles GET /summary - returns aggregated ecosystem metrics.
func (h *EcosystemHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	summary, err := h.usecase.GetSummary(r.Context(), tenantID)
	if err != nil {
		h.logger.Error("failed to get ecosystem summary", zap.Error(err), zap.String("tenant_id", tenantID))
		writeError(w, http.StatusInternalServerError, "failed to get ecosystem summary", err)
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// TriggerScan handles POST /scan - forces a new scan and returns detected tools.
func (h *EcosystemHandler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	tools, err := h.usecase.Scan(r.Context(), tenantID)
	if err != nil {
		h.logger.Error("failed to trigger ecosystem scan", zap.Error(err), zap.String("tenant_id", tenantID))
		writeError(w, http.StatusInternalServerError, "failed to trigger ecosystem scan", err)
		return
	}

	if tools == nil {
		tools = make([]ecosystem.DetectedTool, 0)
	}

	writeJSON(w, http.StatusOK, tools)
}

type createToolRequest struct {
	Name     string            `json:"name"`
	Category string            `json:"category"`
	Endpoint string            `json:"endpoint"`
	Version  string            `json:"version"`
	Status   string            `json:"status"`
	Health   string            `json:"health"`
	Metadata map[string]string `json:"metadata"`
}

func (r *createToolRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if strings.TrimSpace(r.Category) == "" {
		ve.Add("category", "category is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// CreateTool handles POST /tools - manually registers a tool.
func (h *EcosystemHandler) CreateTool(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createToolRequest](w, r)
	if !ok {
		return
	}

	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	tool := &ecosystem.DetectedTool{
		Name:     strings.TrimSpace(req.Name),
		Category: strings.ToLower(strings.TrimSpace(req.Category)),
		Endpoint: strings.TrimSpace(req.Endpoint),
		Version:  strings.TrimSpace(req.Version),
		Status:   req.Status,
		Health:   req.Health,
		Source:   ecosystem.SourceManual,
		TenantID: tenantID,
		Metadata: req.Metadata,
	}

	if tool.Status == "" {
		tool.Status = ecosystem.StatusDetected
	}
	if tool.Health == "" {
		tool.Health = ecosystem.HealthHealthy
	}

	if err := h.usecase.CreateTool(r.Context(), tool); err != nil {
		h.logger.Error("failed to create manual tool", zap.Error(err), zap.String("tenant_id", tenantID))
		writeError(w, http.StatusInternalServerError, "failed to create tool", err)
		return
	}

	writeJSON(w, http.StatusCreated, tool)
}

// DeleteTool handles DELETE /tools/{id} - deletes a registered tool.
func (h *EcosystemHandler) DeleteTool(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "tool id is required", nil)
		return
	}

	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	if err := h.usecase.DeleteTool(r.Context(), tenantID, id); err != nil {
		h.logger.Error("failed to delete tool", zap.Error(err), zap.String("tenant_id", tenantID), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to delete tool", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
