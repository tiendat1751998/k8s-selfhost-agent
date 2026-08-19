// Package http provides HTTP adapters and request routing.
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/plugin"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// PluginHandler provides REST API endpoints for runtime frontend plugins.
type PluginHandler struct {
	repo   plugin.Repository
	logger *zap.Logger
}

// NewPluginHandler creates a new PluginHandler instance.
func NewPluginHandler(repo plugin.Repository, logger *zap.Logger) *PluginHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &PluginHandler{
		repo:   repo,
		logger: logger,
	}
}

// RegisterRoutes registers all plugin endpoints on the chi router.
func (h *PluginHandler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.RegisterPlugin)
	r.Get("/", h.ListPlugins)
	r.Get("/stats", h.GetStats)
	r.Get("/{id}", h.GetPlugin)
	r.Put("/{id}", h.UpdatePlugin)
	r.Delete("/{id}", h.DeletePlugin)
	r.Post("/{id}/toggle", h.TogglePlugin)
}

type registerPluginRequest struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	Enabled     *bool             `json:"enabled"`
	EntryPoint  string            `json:"entry_point"`
	Icon        string            `json:"icon"`
	Category    string            `json:"category"`
	Permissions []string          `json:"permissions"`
	Config      map[string]string `json:"config"`
}

func (r *registerPluginRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if r.Category != "" && !plugin.ValidCategories[r.Category] {
		ve.Add("category", "invalid category: "+r.Category)
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

type updatePluginRequest struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	Enabled     *bool             `json:"enabled"`
	EntryPoint  string            `json:"entry_point"`
	Icon        string            `json:"icon"`
	Category    string            `json:"category"`
	Permissions []string          `json:"permissions"`
	Config      map[string]string `json:"config"`
}

func (r *updatePluginRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if r.Category != "" && !plugin.ValidCategories[r.Category] {
		ve.Add("category", "invalid category: "+r.Category)
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

type togglePluginRequest struct {
	Enabled *bool `json:"enabled"`
}

// RegisterPlugin handles POST /plugins - registers a new plugin.
func (h *PluginHandler) RegisterPlugin(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[registerPluginRequest](w, r)
	if !ok {
		return
	}

	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	version := req.Version
	if version == "" {
		version = "1.0.0"
	}

	category := req.Category
	if category == "" {
		category = plugin.CategoryDevtools
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	perms := req.Permissions
	if perms == nil {
		perms = []string{}
	}

	config := req.Config
	if config == nil {
		config = map[string]string{}
	}

	p := &plugin.Plugin{
		Name:        req.Name,
		Version:     version,
		Description: req.Description,
		Author:      req.Author,
		Enabled:     enabled,
		EntryPoint:  req.EntryPoint,
		Icon:        req.Icon,
		Category:    category,
		Permissions: perms,
		Config:      config,
		TenantID:    tenantID,
	}

	if err := h.repo.Create(r.Context(), p); err != nil {
		if errors.Is(err, plugin.ErrDuplicateName) {
			writeError(w, http.StatusConflict, "plugin with this name already exists", err)
			return
		}
		h.logger.Error("failed to create plugin", zap.Error(err), zap.String("name", req.Name))
		writeError(w, http.StatusInternalServerError, "failed to create plugin", err)
		return
	}

	writeJSON(w, http.StatusCreated, p)
}

// ListPlugins handles GET /plugins - lists plugins with ?enabled=true filter.
func (h *PluginHandler) ListPlugins(w http.ResponseWriter, r *http.Request) {
	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	enabledParam := r.URL.Query().Get("enabled")
	enabledOnly := enabledParam == "true" || enabledParam == "1"

	plugins, err := h.repo.List(r.Context(), tenantID, enabledOnly)
	if err != nil {
		h.logger.Error("failed to list plugins", zap.Error(err), zap.String("tenant_id", tenantID))
		writeError(w, http.StatusInternalServerError, "failed to list plugins", err)
		return
	}

	if plugins == nil {
		plugins = make([]plugin.Plugin, 0)
	}

	writeJSON(w, http.StatusOK, plugins)
}

// GetPlugin handles GET /plugins/{id} - retrieves a single plugin by ID.
func (h *PluginHandler) GetPlugin(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "plugin id is required", err)
		return
	}

	p, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get plugin", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to get plugin", err)
		return
	}
	if p == nil {
		writeError(w, http.StatusNotFound, "plugin not found", nil)
		return
	}

	writeJSON(w, http.StatusOK, p)
}

// UpdatePlugin handles PUT /plugins/{id} - updates an existing plugin.
func (h *PluginHandler) UpdatePlugin(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "plugin id is required", err)
		return
	}

	req, ok := decodeJSON[updatePluginRequest](w, r)
	if !ok {
		return
	}

	existing, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get plugin for update", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to get plugin", err)
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "plugin not found", nil)
		return
	}

	existing.Name = req.Name
	existing.Description = req.Description
	if req.Version != "" {
		existing.Version = req.Version
	}
	if req.Author != "" {
		existing.Author = req.Author
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	existing.EntryPoint = req.EntryPoint
	existing.Icon = req.Icon
	if req.Category != "" {
		existing.Category = req.Category
	}
	if req.Permissions != nil {
		existing.Permissions = req.Permissions
	}
	if req.Config != nil {
		existing.Config = req.Config
	}

	if err := h.repo.Update(r.Context(), existing); err != nil {
		if errors.Is(err, plugin.ErrNotFound) {
			writeError(w, http.StatusNotFound, "plugin not found", err)
			return
		}
		if errors.Is(err, plugin.ErrDuplicateName) {
			writeError(w, http.StatusConflict, "plugin with this name already exists", err)
			return
		}
		h.logger.Error("failed to update plugin", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to update plugin", err)
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

// DeletePlugin handles DELETE /plugins/{id} - deletes a plugin.
func (h *PluginHandler) DeletePlugin(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "plugin id is required", err)
		return
	}

	existing, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get plugin for deletion", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to delete plugin", err)
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "plugin not found", nil)
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		if errors.Is(err, plugin.ErrNotFound) {
			writeError(w, http.StatusNotFound, "plugin not found", err)
			return
		}
		h.logger.Error("failed to delete plugin", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to delete plugin", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TogglePlugin handles POST /plugins/{id}/toggle - enables or disables a plugin.
func (h *PluginHandler) TogglePlugin(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "plugin id is required", err)
		return
	}

	existing, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get plugin for toggle", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to get plugin", err)
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "plugin not found", nil)
		return
	}

	newEnabled := !existing.Enabled
	// If body provides specific enabled state, use it
	if r.ContentLength > 0 {
		var req togglePluginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Enabled != nil {
			newEnabled = *req.Enabled
		}
	}

	if err := h.repo.Toggle(r.Context(), id, newEnabled); err != nil {
		if errors.Is(err, plugin.ErrNotFound) {
			writeError(w, http.StatusNotFound, "plugin not found", err)
			return
		}
		h.logger.Error("failed to toggle plugin", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to toggle plugin", err)
		return
	}

	existing.Enabled = newEnabled
	writeJSON(w, http.StatusOK, existing)
}

// GetStats handles GET /plugins/stats - aggregates plugin metrics.
func (h *PluginHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	stats, err := h.repo.GetStats(r.Context(), tenantID)
	if err != nil {
		h.logger.Error("failed to get plugin stats", zap.Error(err), zap.String("tenant_id", tenantID))
		writeError(w, http.StatusInternalServerError, "failed to get plugin stats", err)
		return
	}

	if stats == nil {
		stats = &plugin.PluginStats{
			ByCategory: make(map[string]int),
		}
	}
	if stats.ByCategory == nil {
		stats.ByCategory = make(map[string]int)
	}

	writeJSON(w, http.StatusOK, stats)
}
