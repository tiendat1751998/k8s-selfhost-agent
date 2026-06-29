package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

// DockerHandler provides HTTP handlers for the Docker and Swarm provider.
type DockerHandler struct {
	repo docker.Repository
}

// NewDockerHandler creates a new Docker HTTP handler.
func NewDockerHandler(repo docker.Repository) *DockerHandler {
	return &DockerHandler{repo: repo}
}

// RegisterRoutes registers Docker and Swarm routes.
func (h *DockerHandler) RegisterRoutes(r chi.Router) {
	r.Get("/containers", h.ListContainers)
	r.Get("/nodes", h.ListNodes)
	r.Get("/services", h.ListServices)
	r.Get("/logs", h.GetLogs)
	r.Post("/services/{id}/scale", h.ScaleService)
	r.Post("/nodes/{id}/drain", h.DrainNode)
	r.Post("/containers/{id}/toggle", h.ToggleContainer)
}

// ListContainers handles GET /api/v1/docker/containers
func (h *DockerHandler) ListContainers(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.ListContainers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list containers", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

// ListNodes handles GET /api/v1/docker/nodes
func (h *DockerHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.ListNodes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list nodes", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

// ListServices handles GET /api/v1/docker/services
func (h *DockerHandler) ListServices(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.ListServices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list services", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

// ScaleService handles POST /api/v1/docker/services/{id}/scale
func (h *DockerHandler) ScaleService(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Replicas int `json:"replicas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	err := h.repo.ScaleService(r.Context(), id, req.Replicas)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to scale service", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "scaled"})
}

// DrainNode handles POST /api/v1/docker/nodes/{id}/drain
func (h *DockerHandler) DrainNode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.repo.UpdateNodeAvailability(r.Context(), id, "drain")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to drain node", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "drained"})
}

// ToggleContainer handles POST /api/v1/docker/containers/{id}/toggle
func (h *DockerHandler) ToggleContainer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Action string `json:"action"` // "start" or "stop"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	err := h.repo.ToggleContainer(r.Context(), id, req.Action)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to toggle container state", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "toggled"})
}

// GetLogs handles GET /api/v1/docker/logs
func (h *DockerHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	targetType := r.URL.Query().Get("type")

	if id == "" {
		writeError(w, http.StatusBadRequest, "missing required query parameter: id", nil)
		return
	}

	logs, err := h.repo.GetLogs(r.Context(), id, targetType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get logs", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"logs": logs})
}
