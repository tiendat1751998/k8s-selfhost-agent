package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// ListNamespaces handles GET /api/v1/k8s/{cluster}/namespaces
func (h *K8sResourceHandler) ListNamespaces(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	namespaces, err := h.repo.ListNamespaces(r.Context(), cluster)
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailableWithErr(w, err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list namespaces", err)
		return
	}
	if namespaces == nil {
		namespaces = []corev1.Namespace{}
	}

	userRole := tenancy.UserRoleFromContext(r.Context())
	tenantID := resolveTenant(r.Context(), r)
	if userRole != "platform_admin" {
		filtered := make([]corev1.Namespace, 0, len(namespaces))
		for _, ns := range namespaces {
			if isSystemNamespace(ns.Name) {
				continue
			}
			if tenantID != "" && !isTenantNamespaceAllowed(tenantID, ns.Name) {
				continue
			}
			filtered = append(filtered, ns)
		}
		namespaces = filtered
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  namespaces,
		"total": len(namespaces),
	})
}

// CreateNamespace handles POST /api/v1/k8s/{cluster}/namespaces
func (h *K8sResourceHandler) CreateNamespace(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "namespace name cannot be empty", nil)
		return
	}

	if err := checkNamespaceAccess(r, req.Name); err != nil {
		writeError(w, http.StatusForbidden, err.Error(), nil)
		return
	}

	ns, err := h.repo.CreateNamespace(r.Context(), cluster, req.Name)
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailableWithErr(w, err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create namespace", err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"data":    ns,
		"message": "namespace created successfully",
	})
}

// DeleteNamespace handles DELETE /api/v1/k8s/{cluster}/namespaces/{name}
func (h *K8sResourceHandler) DeleteNamespace(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	name := chi.URLParam(r, "name")
	if strings.TrimSpace(name) == "" {
		writeError(w, http.StatusBadRequest, "namespace name cannot be empty", nil)
		return
	}

	if err := checkNamespaceAccess(r, name); err != nil {
		writeError(w, http.StatusForbidden, err.Error(), nil)
		return
	}

	if err := h.repo.DeleteNamespace(r.Context(), cluster, name); err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailableWithErr(w, err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete namespace", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
		"name":   name,
	})
}
