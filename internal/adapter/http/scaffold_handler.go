// Package http provides HTTP adapters and request routing.
package http

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/scaffold"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
	usecaseScaffold "github.com/datdt/k8sselfhost/internal/usecase/scaffold"
)

// ScaffoldHandler provides REST API endpoints for scaffolding templates.
type ScaffoldHandler struct {
	service usecaseScaffold.Service
	logger  *zap.Logger
}

// NewScaffoldHandler creates a new ScaffoldHandler instance.
func NewScaffoldHandler(service usecaseScaffold.Service, logger *zap.Logger) *ScaffoldHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ScaffoldHandler{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes registers all scaffolder endpoints on the chi router.
func (h *ScaffoldHandler) RegisterRoutes(r chi.Router) {
	r.Get("/templates", h.ListTemplates)
	r.Get("/templates/{id}", h.GetTemplate)
	r.Post("/templates", h.CreateTemplate)
	r.Put("/templates/{id}", h.UpdateTemplate)
	r.Delete("/templates/{id}", h.DeleteTemplate)
	r.Post("/render", h.Render)
}

type createTemplateRequest struct {
	Name          string                     `json:"name"`
	Description   string                     `json:"description"`
	Category      string                     `json:"category"`
	Framework     string                     `json:"framework"`
	ManifestYAML  string                     `json:"manifest_yaml"`
	HelmValues    string                     `json:"helm_values"`
	DockerCompose string                     `json:"docker_compose"`
	Variables     []scaffold.TemplateVariable `json:"variables"`
	Tags          []string                   `json:"tags"`
}

func (r *createTemplateRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "template name is required")
	}
	if r.Category != "" {
		switch r.Category {
		case scaffold.CategoryWeb, scaffold.CategoryAPI, scaffold.CategoryWorker, scaffold.CategoryDatabase, scaffold.CategoryFullstack:
		default:
			ve.Add("category", "invalid category: "+r.Category)
		}
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

type updateTemplateRequest struct {
	Name          string                     `json:"name"`
	Description   string                     `json:"description"`
	Category      string                     `json:"category"`
	Framework     string                     `json:"framework"`
	ManifestYAML  string                     `json:"manifest_yaml"`
	HelmValues    string                     `json:"helm_values"`
	DockerCompose string                     `json:"docker_compose"`
	Variables     []scaffold.TemplateVariable `json:"variables"`
	Tags          []string                   `json:"tags"`
}

func (r *updateTemplateRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "template name is required")
	}
	if r.Category != "" {
		switch r.Category {
		case scaffold.CategoryWeb, scaffold.CategoryAPI, scaffold.CategoryWorker, scaffold.CategoryDatabase, scaffold.CategoryFullstack:
		default:
			ve.Add("category", "invalid category: "+r.Category)
		}
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

type renderRequestPayload struct {
	TemplateID      string             `json:"template_id"`
	CustomTemplate  *scaffold.Template `json:"custom_template,omitempty"`
	Variables       map[string]string  `json:"variables"`
	RegisterCatalog bool               `json:"register_catalog,omitempty"`
	OwnerTeam       string             `json:"owner_team,omitempty"`
	OwnerEmail      string             `json:"owner_email,omitempty"`
}

func (r *renderRequestPayload) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.TemplateID) == "" && r.CustomTemplate == nil {
		ve.Add("template_id", "either template_id or custom_template must be provided")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// ListTemplates handles GET /templates - retrieves templates matching filter.
func (h *ScaffoldHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	filter := scaffold.ListFilter{
		Category:  r.URL.Query().Get("category"),
		Framework: r.URL.Query().Get("framework"),
		Search:    r.URL.Query().Get("search"),
	}

	templates, err := h.service.ListTemplates(r.Context(), tenantID, filter)
	if err != nil {
		h.logger.Error("failed to list scaffold templates", zap.Error(err), zap.String("tenant_id", tenantID))
		writeError(w, http.StatusInternalServerError, "failed to list templates", err)
		return
	}

	if templates == nil {
		templates = make([]scaffold.Template, 0)
	}

	writeJSON(w, http.StatusOK, templates)
}

// GetTemplate handles GET /templates/{id} - retrieves a template by ID.
func (h *ScaffoldHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "template id is required", err)
		return
	}

	tmpl, err := h.service.GetTemplate(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get scaffold template", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to get template", err)
		return
	}
	if tmpl == nil {
		writeError(w, http.StatusNotFound, "template not found", nil)
		return
	}

	writeJSON(w, http.StatusOK, tmpl)
}

// CreateTemplate handles POST /templates - creates a new custom scaffold template.
func (h *ScaffoldHandler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createTemplateRequest](w, r)
	if !ok {
		return
	}

	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	category := req.Category
	if category == "" {
		category = scaffold.CategoryWeb
	}
	framework := req.Framework
	if framework == "" {
		framework = scaffold.FrameworkGoChi
	}

	variables := req.Variables
	if variables == nil {
		variables = []scaffold.TemplateVariable{}
	}
	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	tmpl := &scaffold.Template{
		Name:          req.Name,
		Description:   req.Description,
		Category:      category,
		Framework:     framework,
		ManifestYAML:  req.ManifestYAML,
		HelmValues:    req.HelmValues,
		DockerCompose: req.DockerCompose,
		Variables:     variables,
		Tags:          tags,
		BuiltIn:       false,
		TenantID:      tenantID,
	}

	if err := h.service.CreateTemplate(r.Context(), tmpl); err != nil {
		h.logger.Error("failed to create scaffold template", zap.Error(err), zap.String("name", req.Name))
		writeError(w, http.StatusInternalServerError, "failed to create template", err)
		return
	}

	writeJSON(w, http.StatusCreated, tmpl)
}

// UpdateTemplate handles PUT /templates/{id} - updates a custom scaffold template.
func (h *ScaffoldHandler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "template id is required", err)
		return
	}

	req, ok := decodeJSON[updateTemplateRequest](w, r)
	if !ok {
		return
	}

	existing, err := h.service.GetTemplate(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get scaffold template for update", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to get template", err)
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "template not found", nil)
		return
	}

	if existing.BuiltIn {
		writeError(w, http.StatusBadRequest, "cannot modify built-in system template", nil)
		return
	}

	existing.Name = req.Name
	existing.Description = req.Description
	if req.Category != "" {
		existing.Category = req.Category
	}
	if req.Framework != "" {
		existing.Framework = req.Framework
	}
	if req.ManifestYAML != "" {
		existing.ManifestYAML = req.ManifestYAML
	}
	if req.HelmValues != "" {
		existing.HelmValues = req.HelmValues
	}
	if req.DockerCompose != "" {
		existing.DockerCompose = req.DockerCompose
	}
	if req.Variables != nil {
		existing.Variables = req.Variables
	}
	if req.Tags != nil {
		existing.Tags = req.Tags
	}

	if err := h.service.UpdateTemplate(r.Context(), existing); err != nil {
		h.logger.Error("failed to update scaffold template", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to update template", err)
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

// DeleteTemplate handles DELETE /templates/{id} - deletes a custom scaffold template.
func (h *ScaffoldHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "template id is required", err)
		return
	}

	existing, err := h.service.GetTemplate(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get scaffold template for delete", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to get template", err)
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "template not found", nil)
		return
	}

	if existing.BuiltIn {
		writeError(w, http.StatusBadRequest, "cannot delete built-in system template", nil)
		return
	}

	if err := h.service.DeleteTemplate(r.Context(), id); err != nil {
		h.logger.Error("failed to delete scaffold template", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "failed to delete template", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Render handles POST /render - generates K8s YAML, Helm, and Compose manifests.
func (h *ScaffoldHandler) Render(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[renderRequestPayload](w, r)
	if !ok {
		return
	}

	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	res, err := h.service.Render(r.Context(), usecaseScaffold.RenderRequest{
		TemplateID:      req.TemplateID,
		CustomTemplate:  req.CustomTemplate,
		Variables:       req.Variables,
		RegisterCatalog: req.RegisterCatalog,
		OwnerTeam:       req.OwnerTeam,
		OwnerEmail:      req.OwnerEmail,
		TenantID:        tenantID,
	})
	if err != nil {
		h.logger.Warn("scaffold render failed", zap.Error(err))
		writeError(w, http.StatusBadRequest, fmt.Sprintf("render failed: %v", err), err)
		return
	}

	writeJSON(w, http.StatusOK, res)
}
