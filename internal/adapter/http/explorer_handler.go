package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/explorer"
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
	namespace := r.URL.Query().Get("namespace")
	query := r.URL.Query().Get("q")

	items, total, err := h.repo.Search(r.Context(), kind, cluster, namespace, query, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search resources", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

// SyncResource handles POST /api/v1/explorer/sync
func (h *ExplorerHandler) SyncResource(w http.ResponseWriter, r *http.Request) {
	var res explorer.Resource
	if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if err := h.repo.SyncResource(r.Context(), &res); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sync resource", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "synced"})
}
