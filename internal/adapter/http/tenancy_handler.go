package http

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/tenancy"
)

type extendedTenancyRepo interface {
	DeleteOrganization(ctx context.Context, id string) error
	CreateMember(ctx context.Context, member tenancy.Member) error
	DeleteMember(ctx context.Context, id string) error
	UpdateRBAC(ctx context.Context, rbac map[string]map[string]bool) error
	UpdateQuota(ctx context.Context, orgID string, quota map[string]interface{}) error
}

type TenancyHandler struct {
	repo           tenancy.Repository
	mu             sync.RWMutex
	deletedOrgs    map[string]bool
	extraMembers   map[string]tenancy.Member
	deletedMembers map[string]bool
	rbacOverrides  map[string]map[string]bool
	quotas         map[string]map[string]interface{}
}

func NewTenancyHandler(repo tenancy.Repository) *TenancyHandler {
	return &TenancyHandler{
		repo:           repo,
		deletedOrgs:    make(map[string]bool),
		extraMembers:   make(map[string]tenancy.Member),
		deletedMembers: make(map[string]bool),
		rbacOverrides:  make(map[string]map[string]bool),
		quotas:         make(map[string]map[string]interface{}),
	}
}

func (h *TenancyHandler) RegisterRoutes(r chi.Router) {
	r.Get("/organizations", h.GetOrganizations)
	r.Get("/projects", h.GetProjects)
	r.Get("/members", h.GetMembers)
	r.Get("/rbac", h.GetRBAC)
	r.Get("/summary", h.GetSummary)
	r.Post("/organizations", h.CreateOrganization)
	r.Delete("/organizations/{id}", h.DeleteOrganization)
	r.Put("/organizations/{id}/quota", h.UpdateQuota)
	r.Post("/projects", h.CreateProject)
	r.Post("/members", h.CreateMember)
	r.Delete("/members/{id}", h.DeleteMember)
	r.Put("/rbac", h.UpdateRBAC)
}

func (h *TenancyHandler) GetOrganizations(w http.ResponseWriter, r *http.Request) {
	orgs, err := h.repo.GetOrganizations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get organizations", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	filtered := make([]tenancy.Organization, 0, len(orgs))
	for _, o := range orgs {
		if !h.deletedOrgs[o.ID] {
			filtered = append(filtered, o)
		}
	}
	writeJSON(w, http.StatusOK, filtered)
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
	h.mu.RLock()
	defer h.mu.RUnlock()
	res := make([]tenancy.Member, 0)
	for _, m := range members {
		if !h.deletedMembers[m.ID] {
			res = append(res, m)
		}
	}
	for id, m := range h.extraMembers {
		if !h.deletedMembers[id] {
			res = append(res, m)
		}
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *TenancyHandler) GetRBAC(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	if len(h.rbacOverrides) > 0 {
		out := make(map[string]map[string]bool)
		for k, v := range h.rbacOverrides {
			row := make(map[string]bool)
			for p, val := range v {
				row[p] = val
			}
			out[k] = row
		}
		h.mu.RUnlock()
		writeJSON(w, http.StatusOK, out)
		return
	}
	h.mu.RUnlock()

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

func (h *TenancyHandler) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "organization id is required", nil)
		return
	}
	if ext, ok := h.repo.(extendedTenancyRepo); ok {
		if err := ext.DeleteOrganization(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete organization", err)
			return
		}
	}
	h.mu.Lock()
	h.deletedOrgs[id] = true
	h.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type createMemberRequest struct {
	tenancy.Member
}

func (r *createMemberRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.ID) == "" {
		ve.Add("id", "id is required")
	}
	if strings.TrimSpace(r.User) == "" {
		ve.Add("user", "user is required")
	}
	if strings.TrimSpace(r.Role) == "" {
		ve.Add("role", "role is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (h *TenancyHandler) CreateMember(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createMemberRequest](w, r)
	if !ok {
		return
	}
	if ext, ok := h.repo.(extendedTenancyRepo); ok {
		if err := ext.CreateMember(r.Context(), req.Member); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create member", err)
			return
		}
	}
	h.mu.Lock()
	h.extraMembers[req.ID] = req.Member
	delete(h.deletedMembers, req.ID)
	h.mu.Unlock()
	writeJSON(w, http.StatusCreated, req.Member)
}

func (h *TenancyHandler) DeleteMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "member id is required", nil)
		return
	}
	if ext, ok := h.repo.(extendedTenancyRepo); ok {
		if err := ext.DeleteMember(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete member", err)
			return
		}
	}
	h.mu.Lock()
	h.deletedMembers[id] = true
	delete(h.extraMembers, id)
	h.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *TenancyHandler) UpdateRBAC(w http.ResponseWriter, r *http.Request) {
	matrix, ok := decodeJSON[map[string]map[string]bool](w, r)
	if !ok || matrix == nil {
		return
	}
	if ext, ok := h.repo.(extendedTenancyRepo); ok {
		if err := ext.UpdateRBAC(r.Context(), *matrix); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update rbac matrix", err)
			return
		}
	}
	h.mu.Lock()
	h.rbacOverrides = *matrix
	h.mu.Unlock()
	writeJSON(w, http.StatusOK, *matrix)
}

func (h *TenancyHandler) UpdateQuota(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "id")
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "organization id is required", nil)
		return
	}
	quota, ok := decodeJSON[map[string]interface{}](w, r)
	if !ok || quota == nil {
		return
	}
	if ext, ok := h.repo.(extendedTenancyRepo); ok {
		if err := ext.UpdateQuota(r.Context(), orgID, *quota); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update quota", err)
			return
		}
	}
	h.mu.Lock()
	h.quotas[orgID] = *quota
	h.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "quota_updated"})
}
