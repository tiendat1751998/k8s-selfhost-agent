package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/explorer"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// ExplorerHandler provides HTTP handlers for the resource explorer API.
type ExplorerHandler struct {
	repo explorer.Repository
}

// NewExplorerHandler creates a new explorer HTTP handler.
func NewExplorerHandler(repo explorer.Repository) *ExplorerHandler {
	return &ExplorerHandler{repo: repo}
}

// RegisterRoutes registers explorer routes.
func (h *ExplorerHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.SearchResources)
	r.Post("/sync", h.SyncResource)
}

// SearchResources handles GET /api/v1/explorer
func (h *ExplorerHandler) SearchResources(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)

	kind := r.URL.Query().Get("kind")
	cluster := r.URL.Query().Get("cluster")
	namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))
	query := r.URL.Query().Get("q")

	userRole := tenancy.UserRoleFromContext(r.Context())
	if userRole != "platform_admin" {
		if namespace == "" || strings.EqualFold(namespace, "all") {
			writeError(w, http.StatusForbidden, "forbidden: cross-tenant search across all namespaces requires platform_admin", nil)
			return
		}
		if err := checkNamespaceAccess(r, namespace); err != nil {
			writeError(w, http.StatusForbidden, err.Error(), nil)
			return
		}
	}

	items, total, err := h.repo.Search(r.Context(), kind, cluster, namespace, query, limit, offset)
	if err != nil {
		if isK8sDegradedError(err) {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"k8s_degraded":    true,
				"degraded_reason": "Kubernetes API server (:6443) is unreachable on registered control-plane nodes. Inspect live host daemons in Deployments or node telemetry in Infrastructure Hosts.",
				"data":            make([]explorer.Resource, 0),
				"resources":       make([]explorer.Resource, 0),
				"total":           0,
			})
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to search resources", err)
		return
	}
	if items == nil {
		items = make([]explorer.Resource, 0)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"k8s_degraded": false,
		"data":         items,
		"resources":    items,
		"total":        total,
	})
}

func isK8sDegradedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, ":6443") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connectex") ||
		strings.Contains(msg, "not initialized") ||
		strings.Contains(msg, "unreachable") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "dial tcp") ||
		strings.Contains(msg, "no route to host") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "k8s_degraded") ||
		strings.Contains(msg, "degraded")
}

type syncResourceRequest struct {
	explorer.Resource
}

func (r *syncResourceRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Kind) == "" {
		ve.Add("kind", "kind is required")
	}
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if strings.TrimSpace(r.Cluster) == "" {
		ve.Add("cluster", "cluster is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// SyncResource handles POST /api/v1/explorer/sync
func (h *ExplorerHandler) SyncResource(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[syncResourceRequest](w, r)
	if !ok {
		return
	}

	userRole := tenancy.UserRoleFromContext(r.Context())
	if userRole != "platform_admin" {
		if strings.TrimSpace(req.Namespace) == "" {
			writeError(w, http.StatusForbidden, "forbidden: syncing cluster-scoped or unnamespaced resources requires platform_admin", nil)
			return
		}
		if err := checkNamespaceAccess(r, req.Namespace); err != nil {
			writeError(w, http.StatusForbidden, err.Error(), nil)
			return
		}
	}
	if err := h.repo.SyncResource(r.Context(), &req.Resource); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sync resource", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "synced"})
}
