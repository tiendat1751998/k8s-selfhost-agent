package http

import (
	"net/http"
	"strings"

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
	r.Get("/templates", h.ListTemplates)
	r.Post("/", h.CreateDeployment)
	r.Post("/scale", h.ScaleDeployment)
	r.Post("/resources", h.UpdateResources)
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

type scaleDeploymentRequest struct {
	Type      string `json:"type"`
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Replicas  int    `json:"replicas"`
}

func (r *scaleDeploymentRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "deployment name is required")
	}
	if strings.TrimSpace(r.Namespace) == "" && r.Type != "docker" {
		ve.Add("namespace", "namespace is required")
	}
	if r.Replicas < 0 {
		ve.Add("replicas", "replicas must be greater than or equal to 0")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// ScaleDeployment handles POST /api/v1/deployments/scale
func (h *DeploymentHandler) ScaleDeployment(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[scaleDeploymentRequest](w, r)
	if !ok {
		return
	}

	err := h.usecase.ScaleDeployment(r.Context(), req.Type, req.Cluster, req.Namespace, req.Name, req.Replicas)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to scale deployment", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "scaled"})
}

type updateResourcesRequest struct {
	Type              string `json:"type"`
	Cluster           string `json:"cluster"`
	Namespace         string `json:"namespace"`
	Name              string `json:"name"`
	Replicas          int    `json:"replicas"`
	MemoryLimit       string `json:"memory_limit"`
	MemoryReservation string `json:"memory_reservation"`
	CPULimit          string `json:"cpu_limit"`
	CPUReservation    string `json:"cpu_reservation"`
}

func (r *updateResourcesRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "deployment name is required")
	}
	if strings.TrimSpace(r.Namespace) == "" && r.Type != "docker" && r.Type != "swarm" {
		ve.Add("namespace", "namespace is required")
	}
	if r.Replicas < -1 || r.Replicas > 100 {
		ve.Add("replicas", "replicas must be between 0 and 100")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// UpdateResources handles POST /api/v1/deployments/resources
func (h *DeploymentHandler) UpdateResources(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[updateResourcesRequest](w, r)
	if !ok {
		return
	}

	err := h.usecase.UpdateResources(r.Context(), deploymentUsecase.UpdateResourcesRequest{
		Type:              req.Type,
		Cluster:           req.Cluster,
		Namespace:         req.Namespace,
		Name:              req.Name,
		Replicas:          req.Replicas,
		MemoryLimit:       req.MemoryLimit,
		MemoryReservation: req.MemoryReservation,
		CPULimit:          req.CPULimit,
		CPUReservation:    req.CPUReservation,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update deployment resources", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}


type restartDeploymentRequest struct {
	Type      string `json:"type"`
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (r *restartDeploymentRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "deployment name is required")
	}
	if strings.TrimSpace(r.Namespace) == "" && r.Type != "docker" {
		ve.Add("namespace", "namespace is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// RestartDeployment handles POST /api/v1/deployments/restart
func (h *DeploymentHandler) RestartDeployment(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[restartDeploymentRequest](w, r)
	if !ok {
		return
	}

	err := h.usecase.RestartDeployment(r.Context(), req.Type, req.Cluster, req.Namespace, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to restart deployment", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restarted"})
}

type deleteDeploymentRequest struct {
	Type      string `json:"type"`
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (r *deleteDeploymentRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "deployment name is required")
	}
	if strings.TrimSpace(r.Namespace) == "" && r.Type != "docker" {
		ve.Add("namespace", "namespace is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// DeleteDeployment handles POST /api/v1/deployments/delete
func (h *DeploymentHandler) DeleteDeployment(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[deleteDeploymentRequest](w, r)
	if !ok {
		return
	}

	err := h.usecase.DeleteDeployment(r.Context(), req.Type, req.Cluster, req.Namespace, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete deployment", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ListTemplates handles GET /api/v1/deployments/templates
func (h *DeploymentHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	templates := []map[string]interface{}{
		{"name": "Nginx Web Server", "category": "web", "version": "v1.25.1", "desc": "High performance HTTP and reverse proxy server.", "ports": 80, "cpu": "100m", "mem": "128Mi"},
		{"name": "Redis Cache Store", "category": "data", "version": "v7.2.0", "desc": "In-memory key-value data structure store.", "ports": 6379, "cpu": "200m", "mem": "256Mi"},
		{"name": "Postgres Database", "category": "data", "version": "v16.1", "desc": "Powerful, open source object-relational database system.", "ports": 5432, "cpu": "500m", "mem": "512Mi"},
		{"name": "Apache Kafka Broker", "category": "data", "version": "v3.6.0", "desc": "Distributed event streaming platform.", "ports": 9092, "cpu": "1000m", "mem": "2048Mi"},
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": templates})
}

type createDeploymentRequest struct {
	deployment.Application
}

func (r *createDeploymentRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if strings.TrimSpace(r.Namespace) == "" && r.Type != "docker" {
		ve.Add("namespace", "namespace is required")
	}
	if strings.TrimSpace(r.Image) == "" {
		ve.Add("image", "image is required")
	}
	if r.Replicas < 0 {
		ve.Add("replicas", "replicas must be greater than or equal to 0")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// CreateDeployment handles POST /api/v1/deployments
func (h *DeploymentHandler) CreateDeployment(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createDeploymentRequest](w, r)
	if !ok {
		return
	}

	err := h.usecase.CreateDeployment(r.Context(), req.Application)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create deployment", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "created", "name": req.Name})
}
