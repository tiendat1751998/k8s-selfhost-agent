package http

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/datdt/k8sselfhost/internal/domain/tenancy"
)

type extendedTenancyRepo interface {
	DeleteOrganization(ctx context.Context, id string) error
	CreateMember(ctx context.Context, member tenancy.Member) error
	UpdateMemberRole(ctx context.Context, id string, role string) error
	DeleteMember(ctx context.Context, id string) error
	UpdateRBAC(ctx context.Context, rbac map[string]map[string]bool) error
	UpdateQuota(ctx context.Context, orgID string, quota map[string]interface{}) error
	GetQuota(ctx context.Context, orgID string) (map[string]interface{}, error)
}

// TenancyHandler exposes REST APIs for multi-tenant organizations, projects, members, and RBAC.
type TenancyHandler struct {
	repo     tenancy.Repository
	fallback *tenancyRAMFallback // Lightweight fallback ONLY when repo == nil for unit tests
}

type tenancyRAMFallback struct {
	mu         sync.RWMutex
	orgs       []tenancy.Organization
	projects   []tenancy.Project
	members    []tenancy.Member
	rbacMatrix map[string]map[string]bool
	quotas     map[string]map[string]interface{}
}

// NewTenancyHandler initializes TenancyHandler with repository persistence.
func NewTenancyHandler(repo tenancy.Repository) *TenancyHandler {
	h := &TenancyHandler{repo: repo}
	if repo == nil {
		h.fallback = &tenancyRAMFallback{
			orgs: []tenancy.Organization{
				{ID: "default-tenant", Name: "Enterprise Operations Hub", Tier: "Enterprise"},
			},
			projects: []tenancy.Project{},
			members:  []tenancy.Member{},
			rbacMatrix: map[string]map[string]bool{
				"Platform Admin": {"pods:read": true, "pods:write": true},
			},
			quotas: make(map[string]map[string]interface{}),
		}
	}
	return h
}

// RegisterRoutes registers all tenancy and RBAC endpoints.
func (h *TenancyHandler) RegisterRoutes(r chi.Router) {
	r.Get("/organizations", h.GetOrganizations)
	r.Get("/projects", h.GetProjects)
	r.Get("/members", h.GetMembers)
	r.Get("/rbac", h.GetRBAC)
	r.Get("/summary", h.GetSummary)
	r.Post("/organizations", h.CreateOrganization)
	r.Delete("/organizations/{id}", h.DeleteOrganization)
	r.Put("/organizations/{id}/quota", h.UpdateQuota)
	r.Get("/organizations/{id}/quota", h.GetQuota)
	r.Post("/projects", h.CreateProject)
	r.Post("/members", h.CreateMember)
	r.Put("/members/{id}", h.UpdateMemberRole)
	r.Put("/members/{id}/role", h.UpdateMemberRole)
	r.Delete("/members/{id}", h.DeleteMember)
	r.Put("/rbac", h.UpdateRBAC)
}

func (h *TenancyHandler) GetOrganizations(w http.ResponseWriter, r *http.Request) {
	if h.repo != nil {
		if orgs, err := h.repo.GetOrganizations(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get organizations", err)
		} else {
			writeJSON(w, http.StatusOK, orgs)
		}
		return
	}
	h.fallback.mu.RLock()
	defer h.fallback.mu.RUnlock()
	res := make([]tenancy.Organization, len(h.fallback.orgs))
	copy(res, h.fallback.orgs)
	writeJSON(w, http.StatusOK, res)
}

func (h *TenancyHandler) GetProjects(w http.ResponseWriter, r *http.Request) {
	if h.repo != nil {
		if projects, err := h.repo.GetProjects(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get projects", err)
		} else {
			writeJSON(w, http.StatusOK, projects)
		}
		return
	}
	h.fallback.mu.RLock()
	defer h.fallback.mu.RUnlock()
	res := make([]tenancy.Project, len(h.fallback.projects))
	copy(res, h.fallback.projects)
	writeJSON(w, http.StatusOK, res)
}

func (h *TenancyHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	if h.repo != nil {
		if members, err := h.repo.GetMembers(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get members", err)
		} else {
			writeJSON(w, http.StatusOK, members)
		}
		return
	}
	h.fallback.mu.RLock()
	defer h.fallback.mu.RUnlock()
	res := make([]tenancy.Member, len(h.fallback.members))
	copy(res, h.fallback.members)
	writeJSON(w, http.StatusOK, res)
}

func (h *TenancyHandler) GetRBAC(w http.ResponseWriter, r *http.Request) {
	if h.repo != nil {
		if rbac, err := h.repo.GetRBAC(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get rbac matrix", err)
		} else {
			writeJSON(w, http.StatusOK, rbac)
		}
		return
	}
	h.fallback.mu.RLock()
	defer h.fallback.mu.RUnlock()
	writeJSON(w, http.StatusOK, h.fallback.rbacMatrix)
}

func (h *TenancyHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	if h.repo != nil {
		orgs, err1 := h.repo.GetOrganizations(r.Context())
		projects, err2 := h.repo.GetProjects(r.Context())
		members, err3 := h.repo.GetMembers(r.Context())
		rbac, err4 := h.repo.GetRBAC(r.Context())
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			writeError(w, http.StatusInternalServerError, "failed to get tenancy summary", nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"organizations": orgs, "projects": projects, "members": members, "rbacMatrix": rbac,
		})
		return
	}
	h.fallback.mu.RLock()
	defer h.fallback.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"organizations": h.fallback.orgs, "projects": h.fallback.projects, "members": h.fallback.members, "rbacMatrix": h.fallback.rbacMatrix,
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

	if h.repo != nil {
		if err := h.repo.CreateOrganization(r.Context(), req.Organization); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create organization", err)
			return
		}
		writeJSON(w, http.StatusCreated, req.Organization)
		return
	}

	h.fallback.mu.Lock()
	h.fallback.orgs = append(h.fallback.orgs, req.Organization)
	h.fallback.mu.Unlock()
	writeJSON(w, http.StatusCreated, req.Organization)
}

func (h *TenancyHandler) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "organization id is required", nil)
		return
	}

	if h.repo != nil {
		ext, ok := h.repo.(extendedTenancyRepo)
		if !ok {
			writeError(w, http.StatusNotImplemented, "delete organization not supported", nil)
			return
		}
		if err := ext.DeleteOrganization(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete organization", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		return
	}

	h.fallback.mu.Lock()
	var remaining []tenancy.Organization
	for _, o := range h.fallback.orgs {
		if o.ID != id {
			remaining = append(remaining, o)
		}
	}
	h.fallback.orgs = remaining
	h.fallback.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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

	if h.repo != nil {
		if err := h.repo.CreateProject(r.Context(), req.Project); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create project", err)
			return
		}
		writeJSON(w, http.StatusCreated, req.Project)
		return
	}

	h.fallback.mu.Lock()
	h.fallback.projects = append(h.fallback.projects, req.Project)
	h.fallback.mu.Unlock()
	writeJSON(w, http.StatusCreated, req.Project)
}

type createMemberRequest struct {
	tenancy.Member
}

func (r *createMemberRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.ID) == "" || uuid.Validate(r.ID) != nil {
		r.ID = uuid.NewString()
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
	if req.Member.ID == "" || uuid.Validate(req.Member.ID) != nil {
		req.Member.ID = uuid.NewString()
	}

	if h.repo != nil {
		ext, ok := h.repo.(extendedTenancyRepo)
		if !ok {
			writeError(w, http.StatusNotImplemented, "create member not supported", nil)
			return
		}
		if err := ext.CreateMember(r.Context(), req.Member); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create member", err)
			return
		}
		writeJSON(w, http.StatusCreated, req.Member)
		return
	}

	h.fallback.mu.Lock()
	h.fallback.members = append(h.fallback.members, req.Member)
	h.fallback.mu.Unlock()
	writeJSON(w, http.StatusCreated, req.Member)
}

type updateMemberRoleRequest struct {
	Role string `json:"role"`
}

func (h *TenancyHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "member id is required", nil)
		return
	}
	req, ok := decodeJSON[updateMemberRoleRequest](w, r)
	if !ok || strings.TrimSpace(req.Role) == "" {
		writeError(w, http.StatusBadRequest, "valid role is required", nil)
		return
	}

	if h.repo != nil {
		ext, ok := h.repo.(extendedTenancyRepo)
		if !ok {
			writeError(w, http.StatusNotImplemented, "update member role not supported", nil)
			return
		}
		if err := ext.UpdateMemberRole(r.Context(), id, req.Role); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update member role", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "id": id, "role": req.Role})
		return
	}

	h.fallback.mu.Lock()
	for i := range h.fallback.members {
		if h.fallback.members[i].ID == id {
			h.fallback.members[i].Role = req.Role
			break
		}
	}
	h.fallback.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "id": id, "role": req.Role})
}

func (h *TenancyHandler) DeleteMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "member id is required", nil)
		return
	}

	if h.repo != nil {
		ext, ok := h.repo.(extendedTenancyRepo)
		if !ok {
			writeError(w, http.StatusNotImplemented, "delete member not supported", nil)
			return
		}
		if err := ext.DeleteMember(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete member", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		return
	}

	h.fallback.mu.Lock()
	var remaining []tenancy.Member
	for _, m := range h.fallback.members {
		if m.ID != id {
			remaining = append(remaining, m)
		}
	}
	h.fallback.members = remaining
	h.fallback.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *TenancyHandler) UpdateRBAC(w http.ResponseWriter, r *http.Request) {
	matrix, ok := decodeJSON[map[string]map[string]bool](w, r)
	if !ok || matrix == nil {
		return
	}

	if h.repo != nil {
		ext, ok := h.repo.(extendedTenancyRepo)
		if !ok {
			writeError(w, http.StatusNotImplemented, "update rbac not supported", nil)
			return
		}
		if err := ext.UpdateRBAC(r.Context(), *matrix); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update rbac matrix", err)
			return
		}
		writeJSON(w, http.StatusOK, *matrix)
		return
	}

	h.fallback.mu.Lock()
	h.fallback.rbacMatrix = *matrix
	h.fallback.mu.Unlock()
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

	if h.repo != nil {
		ext, ok := h.repo.(extendedTenancyRepo)
		if !ok {
			writeError(w, http.StatusNotImplemented, "update quota not supported", nil)
			return
		}
		if err := ext.UpdateQuota(r.Context(), orgID, *quota); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update quota", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "quota_updated"})
		return
	}

	h.fallback.mu.Lock()
	h.fallback.quotas[orgID] = *quota
	h.fallback.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "quota_updated"})
}

func (h *TenancyHandler) GetQuota(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "id")
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "organization id is required", nil)
		return
	}

	if h.repo != nil {
		ext, ok := h.repo.(extendedTenancyRepo)
		if !ok {
			writeError(w, http.StatusNotImplemented, "get quota not supported", nil)
			return
		}
		q, err := ext.GetQuota(r.Context(), orgID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get quota", err)
			return
		}
		writeJSON(w, http.StatusOK, q)
		return
	}

	h.fallback.mu.RLock()
	q := h.fallback.quotas[orgID]
	h.fallback.mu.RUnlock()
	if q == nil {
		q = make(map[string]interface{})
	}
	writeJSON(w, http.StatusOK, q)
}
