// Package http provides HTTP adapters and request routing.
package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/catalog"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// CatalogHandler provides REST API endpoints for the service catalog.
type CatalogHandler struct {
	repo   catalog.Repository
	logger *zap.Logger
}

// NewCatalogHandler creates a new CatalogHandler instance.
func NewCatalogHandler(repo catalog.Repository, logger *zap.Logger) *CatalogHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CatalogHandler{
		repo:   repo,
		logger: logger,
	}
}

// RegisterRoutes registers all service catalog endpoints on the chi router.
func (h *CatalogHandler) RegisterRoutes(r chi.Router) {
	r.Post("/services", h.CreateService)
	r.Get("/services", h.ListServices)
	r.Get("/services/{id}", h.GetService)
	r.Put("/services/{id}", h.UpdateService)
	r.Delete("/services/{id}", h.DeleteService)
	r.Get("/stats", h.GetStats)
}

type createServiceRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        string            `json:"type"`
	Lifecycle   string            `json:"lifecycle"`
	OwnerTeam   string            `json:"owner_team"`
	OwnerEmail  string            `json:"owner_email"`
	RepoURL     string            `json:"repo_url"`
	DocsURL     string            `json:"docs_url"`
	Tags        []string          `json:"tags"`
	Annotations map[string]string `json:"annotations"`
}

func (r *createServiceRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if r.Type != "" {
		switch r.Type {
		case catalog.TypeService, catalog.TypeAPI, catalog.TypeLibrary, catalog.TypeDatabase, catalog.TypeFrontend, catalog.TypeWorker:
		default:
			ve.Add("type", "invalid service type: "+r.Type)
		}
	}
	if r.Lifecycle != "" {
		switch r.Lifecycle {
		case catalog.LifecycleDevelopment, catalog.LifecycleStaging, catalog.LifecycleProduction, catalog.LifecycleDeprecated:
		default:
			ve.Add("lifecycle", "invalid lifecycle: "+r.Lifecycle)
		}
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

type updateServiceRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        string            `json:"type"`
	Lifecycle   string            `json:"lifecycle"`
	OwnerTeam   string            `json:"owner_team"`
	OwnerEmail  string            `json:"owner_email"`
	RepoURL     string            `json:"repo_url"`
	DocsURL     string            `json:"docs_url"`
	Tags        []string          `json:"tags"`
	Annotations map[string]string `json:"annotations"`
}

func (r *updateServiceRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if r.Type != "" {
		switch r.Type {
		case catalog.TypeService, catalog.TypeAPI, catalog.TypeLibrary, catalog.TypeDatabase, catalog.TypeFrontend, catalog.TypeWorker:
		default:
			ve.Add("type", "invalid service type: "+r.Type)
		}
	}
	if r.Lifecycle != "" {
		switch r.Lifecycle {
		case catalog.LifecycleDevelopment, catalog.LifecycleStaging, catalog.LifecycleProduction, catalog.LifecycleDeprecated:
		default:
			ve.Add("lifecycle", "invalid lifecycle: "+r.Lifecycle)
		}
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// CreateService handles POST /services - registers a new service in the catalog.
func (h *CatalogHandler) CreateService(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createServiceRequest](w, r)
	if !ok {
		return
	}

	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	serviceType := req.Type
	if serviceType == "" {
		serviceType = catalog.TypeService
	}

	lifecycle := req.Lifecycle
	if lifecycle == "" {
		lifecycle = catalog.LifecycleDevelopment
	}

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	annotations := req.Annotations
	if annotations == nil {
		annotations = map[string]string{}
	}

	entry := &catalog.ServiceEntry{
		Name:        req.Name,
		Description: req.Description,
		Type:        serviceType,
		Lifecycle:   lifecycle,
		OwnerTeam:   req.OwnerTeam,
		OwnerEmail:  req.OwnerEmail,
		RepoURL:     req.RepoURL,
		DocsURL:     req.DocsURL,
		Tags:        tags,
		Annotations: annotations,
		TenantID:    tenantID,
	}

	if err := h.repo.Create(r.Context(), entry); err != nil {
		h.logger.Error("failed to create service catalog entry", zap.Error(err), zap.String("name", req.Name))
		writeError(w, http.StatusInternalServerError, "failed to create service", err)
		return
	}

	writeJSON(w, http.StatusCreated, entry)
}

// ListServices handles GET /services - lists service catalog entries with query filters.
func (h *CatalogHandler) ListServices(w http.ResponseWriter, r *http.Request) {
	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	filter := catalog.ListFilter{
		Type:      r.URL.Query().Get("type"),
		Lifecycle: r.URL.Query().Get("lifecycle"),
		OwnerTeam: r.URL.Query().Get("owner_team"),
		Search:    r.URL.Query().Get("search"),
	}

	services, err := h.repo.List(r.Context(), tenantID, filter)
	if err != nil {
		h.logger.Error("failed to list service catalog entries", zap.Error(err), zap.String("tenant_id", tenantID))
		writeError(w, http.StatusInternalServerError, "failed to list services", err)
		return
	}

	if services == nil {
		services = make([]catalog.ServiceEntry, 0)
	}

	writeJSON(w, http.StatusOK, services)
}

// GetService handles GET /services/{id} - retrieves a single service catalog entry by ID.
func (h *CatalogHandler) GetService(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "service id is required", nil)
		return
	}

	entry, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get service catalog entry", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to get service", err)
		return
	}
	if entry == nil {
		writeError(w, http.StatusNotFound, "service not found", nil)
		return
	}

	writeJSON(w, http.StatusOK, entry)
}

// UpdateService handles PUT /services/{id} - updates an existing service catalog entry.
func (h *CatalogHandler) UpdateService(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "service id is required", nil)
		return
	}

	req, ok := decodeJSON[updateServiceRequest](w, r)
	if !ok {
		return
	}

	existing, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get service catalog entry for update", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to get service", err)
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "service not found", nil)
		return
	}

	existing.Name = req.Name
	existing.Description = req.Description
	if req.Type != "" {
		existing.Type = req.Type
	}
	if req.Lifecycle != "" {
		existing.Lifecycle = req.Lifecycle
	}
	existing.OwnerTeam = req.OwnerTeam
	existing.OwnerEmail = req.OwnerEmail
	existing.RepoURL = req.RepoURL
	existing.DocsURL = req.DocsURL
	if req.Tags != nil {
		existing.Tags = req.Tags
	} else if existing.Tags == nil {
		existing.Tags = []string{}
	}
	if req.Annotations != nil {
		existing.Annotations = req.Annotations
	} else if existing.Annotations == nil {
		existing.Annotations = map[string]string{}
	}

	if err := h.repo.Update(r.Context(), existing); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "service not found", err)
			return
		}
		h.logger.Error("failed to update service catalog entry", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to update service", err)
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

// DeleteService handles DELETE /services/{id} - removes a service catalog entry.
func (h *CatalogHandler) DeleteService(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "service id is required", nil)
		return
	}

	existing, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get service catalog entry for deletion", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to delete service", err)
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "service not found", nil)
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "service not found", err)
			return
		}
		h.logger.Error("failed to delete service catalog entry", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to delete service", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetStats handles GET /stats - aggregates metrics for the catalog scoped to a tenant.
func (h *CatalogHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	stats, err := h.repo.Stats(r.Context(), tenantID)
	if err != nil {
		h.logger.Error("failed to get service catalog stats", zap.Error(err), zap.String("tenant_id", tenantID))
		writeError(w, http.StatusInternalServerError, "failed to get catalog stats", err)
		return
	}

	if stats == nil {
		stats = &catalog.CatalogStats{
			ByType:      make(map[string]int),
			ByLifecycle: make(map[string]int),
		}
	}
	if stats.ByType == nil {
		stats.ByType = make(map[string]int)
	}
	if stats.ByLifecycle == nil {
		stats.ByLifecycle = make(map[string]int)
	}

	writeJSON(w, http.StatusOK, stats)
}
