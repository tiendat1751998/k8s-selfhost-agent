package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/deployment"
	deploymentUsecase "github.com/datdt/k8sselfhost/internal/usecase/deployment"
)

// DeploymentHandler provides HTTP endpoints for managing deployments.
type DeploymentHandler struct {
	usecase *deploymentUsecase.Usecase
}

// NewDeploymentHandler creates a new DeploymentHandler.
func NewDeploymentHandler(usecase *deploymentUsecase.Usecase) *DeploymentHandler {
	return &DeploymentHandler{usecase: usecase}
}

// RegisterRoutes registers deployment routes.
func (h *DeploymentHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListDeployments)
	r.Post("/", h.CreateDeployment)
	r.Post("/scale", h.ScaleDeployment)
	r.Post("/restart", h.RestartDeployment)
	r.Post("/delete", h.DeleteDeployment)
}

// ListDeployments handles GET /api/v1/deployments
func (h *DeploymentHandler) ListDeployments(w http.ResponseWriter, r *http.Request) {
	apps, err := h.usecase.ListDeployments(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list deployments", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": apps})
}

// ScaleDeployment handles POST /api/v1/deployments/scale
func (h *DeploymentHandler) ScaleDeployment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type      string `json:"type"`
		Cluster   string `json:"cluster"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
		Replicas  int    `json:"replicas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	err := h.usecase.ScaleDeployment(r.Context(), req.Type, req.Cluster, req.Namespace, req.Name, req.Replicas)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to scale deployment", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "scaled"})
}

// RestartDeployment handles POST /api/v1/deployments/restart
func (h *DeploymentHandler) RestartDeployment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type      string `json:"type"`
		Cluster   string `json:"cluster"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	err := h.usecase.RestartDeployment(r.Context(), req.Type, req.Cluster, req.Namespace, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to restart deployment", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restarted"})
}

// DeleteDeployment handles POST /api/v1/deployments/delete
func (h *DeploymentHandler) DeleteDeployment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type      string `json:"type"`
		Cluster   string `json:"cluster"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	err := h.usecase.DeleteDeployment(r.Context(), req.Type, req.Cluster, req.Namespace, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete deployment", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// CreateDeployment handles POST /api/v1/deployments
func (h *DeploymentHandler) CreateDeployment(w http.ResponseWriter, r *http.Request) {
	var app deployment.Application
	if err := json.NewDecoder(r.Body).Decode(&app); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	err := h.usecase.CreateDeployment(r.Context(), app)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create deployment", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "created", "name": app.Name})
}

