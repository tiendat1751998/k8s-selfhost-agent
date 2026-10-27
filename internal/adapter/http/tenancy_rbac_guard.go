package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/tenancy"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// RegisterTenantScopedRoutes registers tenancy routes enforcing tenant-admin isolation.
func (h *TenancyHandler) RegisterTenantScopedRoutes(r chi.Router) {
	adminOnly := middleware.RBACMiddleware("platform_admin")
	tenantAdminOrPlatform := middleware.RBACMiddleware("platform_admin", "tenant_admin")
	operatorOrAbove := middleware.RBACMiddleware("platform_admin", "tenant_admin", "operator")
	viewerOrAbove := middleware.RBACMiddleware("platform_admin", "tenant_admin", "operator", "viewer")

	// Global platform_admin only operations
	r.With(adminOnly).Post("/organizations", h.CreateOrganization)
	r.With(adminOnly).Delete("/organizations/{id}", h.DeleteOrganization)
	r.With(adminOnly).Put("/rbac", h.UpdateRBAC)

	// Tenant-scoped operations with URL tenant parameter guard
	r.With(tenantAdminOrPlatform).Put("/organizations/{id}/quota", h.tenantScopedHandler(h.UpdateQuota))
	r.With(tenantAdminOrPlatform).Get("/organizations/{id}/quota", h.tenantScopedHandler(h.GetQuota))
	r.With(tenantAdminOrPlatform).Get("/organizations", h.GetOrganizationsScoped)

	// Projects and members
	r.With(operatorOrAbove).Get("/projects", h.GetProjects)
	r.With(operatorOrAbove).Get("/members", h.GetMembers)
	r.With(operatorOrAbove).Get("/rbac", h.GetRBAC)
	r.With(viewerOrAbove).Get("/summary", h.GetSummary)

	r.With(tenantAdminOrPlatform).Post("/projects", h.CreateProject)
	r.With(tenantAdminOrPlatform).Post("/members", h.CreateMember)
	r.With(tenantAdminOrPlatform).Put("/members/{id}", h.UpdateMemberRole)
	r.With(tenantAdminOrPlatform).Put("/members/{id}/role", h.UpdateMemberRole)
	r.With(tenantAdminOrPlatform).Delete("/members/{id}", h.DeleteMember)
}

// tenantScopedHandler ensures non-platform_admin users only access their own tenant_id.
func (h *TenancyHandler) tenantScopedHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		role := middleware.UserRoleFromContext(r.Context())
		if role == "platform_admin" {
			next(w, r)
			return
		}

		targetOrgID := chi.URLParam(r, "id")
		userTenant := middleware.TenantIDFromContext(r.Context())
		if targetOrgID != "" && userTenant != "" && !strings.EqualFold(targetOrgID, userTenant) {
			writeError(w, http.StatusForbidden, "access denied: cross-tenant access prohibited", errors.NewConflict("tenancy", "cross-tenant access prohibited"))
			return
		}

		next(w, r)
	}
}

// GetOrganizationsScoped returns organizations filtered by tenant when the caller is tenant_admin or operator.
func (h *TenancyHandler) GetOrganizationsScoped(w http.ResponseWriter, r *http.Request) {
	role := middleware.UserRoleFromContext(r.Context())
	if role == "platform_admin" {
		h.GetOrganizations(w, r)
		return
	}

	userTenant := middleware.TenantIDFromContext(r.Context())
	if h.repo != nil {
		orgs, err := h.repo.GetOrganizations(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get organizations", err)
			return
		}
		var filtered []tenancy.Organization
		for _, org := range orgs {
			if strings.EqualFold(org.ID, userTenant) {
				filtered = append(filtered, org)
			}
		}
		writeJSON(w, http.StatusOK, filtered)
		return
	}

	if h.fallback != nil {
		h.fallback.mu.RLock()
		defer h.fallback.mu.RUnlock()
		var filtered []tenancy.Organization
		for _, org := range h.fallback.orgs {
			if strings.EqualFold(org.ID, userTenant) {
				filtered = append(filtered, org)
			}
		}
		writeJSON(w, http.StatusOK, filtered)
		return
	}

	writeJSON(w, http.StatusOK, []tenancy.Organization{})
}
