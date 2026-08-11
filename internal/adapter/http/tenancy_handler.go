package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/tenancy"
)

type TenancyHandler struct {
	repo tenancy.Repository
}

func NewTenancyHandler(repo tenancy.Repository) *TenancyHandler {
	return &TenancyHandler{repo: repo}
}

func (h *TenancyHandler) RegisterRoutes(r chi.Router) {
	r.Get("/organizations", h.GetOrganizations)
	r.Get("/projects", h.GetProjects)
	r.Get("/members", h.GetMembers)
	r.Get("/rbac", h.GetRBAC)
	r.Get("/summary", h.GetSummary)
	r.Post("/organizations", h.CreateOrganization)
	r.Post("/projects", h.CreateProject)
}

func (h *TenancyHandler) GetOrganizations(w http.ResponseWriter, r *http.Request) {
	orgs, err := h.repo.GetOrganizations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get organizations", err)
		return
	}
	writeJSON(w, http.StatusOK, orgs)
}

func (h *TenancyHandler) GetProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.repo.GetProjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get projects", err)
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func (h *TenancyHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.repo.GetMembers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get members", err)
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (h *TenancyHandler) GetRBAC(w http.ResponseWriter, r *http.Request) {
	rbac, err := h.repo.GetRBAC(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get rbac matrix", err)
		return
	}
	writeJSON(w, http.StatusOK, rbac)
}

func (h *TenancyHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	orgs, err := h.repo.GetOrganizations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get organizations", err)
		return
	}

	projects, err := h.repo.GetProjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get projects", err)
		return
	}

	members, err := h.repo.GetMembers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get members", err)
		return
	}

	rbac, err := h.repo.GetRBAC(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get rbac matrix", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"organizations": orgs,
		"projects":      projects,
		"members":       members,
		"rbacMatrix":    rbac,
	})
}

type createOrganizationRequest struct {
	tenancy.Organization
}

func (r *createOrganizationRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.ID) == "" {
		ve.Add("id", "id is required")
	}
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (h *TenancyHandler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createOrganizationRequest](w, r)
	if !ok {
		return
	}

	if err := h.repo.CreateOrganization(r.Context(), req.Organization); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create organization", err)
		return
	}

	writeJSON(w, http.StatusCreated, req.Organization)
}

type createProjectRequest struct {
	tenancy.Project
}

func (r *createProjectRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.ID) == "" {
		ve.Add("id", "id is required")
	}
	if strings.TrimSpace(r.OrgID) == "" {
		ve.Add("org_id", "org_id is required")
	}
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (h *TenancyHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createProjectRequest](w, r)
	if !ok {
		return
	}

	if err := h.repo.CreateProject(r.Context(), req.Project); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create project", err)
		return
	}

	writeJSON(w, http.StatusCreated, req.Project)
}
