package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// AgentMetricRemover defines the interface for removing agent metrics upon compute host deletion.
type AgentMetricRemover interface {
	RemoveAgentMetric(hostID string)
}

// RegisterHostRoutes registers routes for standalone /api/v1/hosts endpoint.
func (h *DockerHandler) RegisterHostRoutes(r chi.Router) {
	r.Get("/", h.ListHosts)
	r.Post("/", h.CreateHost)
	r.Get("/{id}", h.GetHost)
	r.Put("/{id}", h.UpdateHost)
	r.Delete("/{id}", h.DeleteHost)
	r.Post("/{id}/test", h.TestHost)
}

// Compute Host Request DTOs
type createComputeHostRequest struct {
	Name       string            `json:"name"`
	HostType   string            `json:"host_type"`
	Endpoint   string            `json:"endpoint"`
	TLSEnabled bool              `json:"tls_enabled"`
	TLSCA      string            `json:"tls_ca"`
	TLSCert    string            `json:"tls_cert"`
	TLSKey     string            `json:"tls_key"`
	APIVersion string            `json:"api_version"`
	Labels     map[string]string `json:"labels"`
}

var validHostTypes = map[string]bool{
	"agent":      true, // K8s-Agent (CPU/RAM/Disk)
	"docker":     true, // Docker Engine
	"k8s":        true, // Kubernetes API
	"prometheus": true, // Prometheus endpoint
	"git":        true, // Git repository (GitHub/GitLab)
	"database":   true, // Database connection
	"custom":     true, // Custom HTTP endpoint
}

func (r *createComputeHostRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if strings.TrimSpace(r.Endpoint) == "" {
		ve.Add("endpoint", "endpoint is required")
	}
	if r.HostType != "" && !validHostTypes[r.HostType] {
		ve.Add("host_type", "host_type must be agent, docker, k8s, prometheus, git, database, or custom")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

type updateComputeHostRequest struct {
	Name       string            `json:"name"`
	HostType   string            `json:"host_type"`
	Endpoint   string            `json:"endpoint"`
	TLSEnabled *bool             `json:"tls_enabled"`
	TLSCA      string            `json:"tls_ca"`
	TLSCert    string            `json:"tls_cert"`
	TLSKey     string            `json:"tls_key"`
	APIVersion string            `json:"api_version"`
	Labels     map[string]string `json:"labels"`
}

func (r *updateComputeHostRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if r.HostType != "" && !validHostTypes[r.HostType] {
		ve.Add("host_type", "host_type must be agent, docker, k8s, prometheus, git, database, or custom")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// ListHosts handles GET /api/v1/docker/hosts
func (h *DockerHandler) ListHosts(w http.ResponseWriter, r *http.Request) {
	if h.hostRepo == nil {
		writeError(w, http.StatusServiceUnavailable, "compute host service unavailable", nil)
		return
	}
	tenantID := tenancy.TenantIDFromContext(r.Context())
	hosts, err := h.hostRepo.List(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list compute hosts", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": hosts})
}

// CreateHost handles POST /api/v1/docker/hosts
func (h *DockerHandler) CreateHost(w http.ResponseWriter, r *http.Request) {
	if h.hostRepo == nil {
		writeError(w, http.StatusServiceUnavailable, "compute host service unavailable", nil)
		return
	}
	req, ok := decodeJSON[createComputeHostRequest](w, r)
	if !ok {
		return
	}

	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	hostType := req.HostType
	if hostType == "" {
		hostType = "agent"
	}

	labels := req.Labels
	if labels == nil {
		labels = make(map[string]string)
	}

	host := &docker.ComputeHost{
		Name:       req.Name,
		HostType:   hostType,
		Endpoint:   req.Endpoint,
		TLSEnabled: req.TLSEnabled,
		TLSCA:      req.TLSCA,
		TLSCert:    req.TLSCert,
		TLSKey:     req.TLSKey,
		APIVersion: req.APIVersion,
		Status:     "pending",
		Labels:     labels,
		TenantID:   tenantID,
	}

	err := h.hostRepo.Create(r.Context(), host)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to register compute host", err)
		return
	}

	// Never leak private key in API response
	host.TLSKey = ""
	writeJSON(w, http.StatusCreated, host)
}

// GetHost handles GET /api/v1/docker/hosts/{id} or /api/v1/hosts/{id}
func (h *DockerHandler) GetHost(w http.ResponseWriter, r *http.Request) {
	if h.hostRepo == nil {
		writeError(w, http.StatusServiceUnavailable, "compute host service unavailable", nil)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing compute host id", nil)
		return
	}

	host, err := h.hostRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get compute host", err)
		return
	}
	if host == nil {
		writeError(w, http.StatusNotFound, "compute host not found", nil)
		return
	}

	host.TLSKey = ""
	writeJSON(w, http.StatusOK, host)
}

// UpdateHost handles PUT /api/v1/docker/hosts/{id}
func (h *DockerHandler) UpdateHost(w http.ResponseWriter, r *http.Request) {
	if h.hostRepo == nil {
		writeError(w, http.StatusServiceUnavailable, "compute host service unavailable", nil)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing compute host id", nil)
		return
	}

	req, ok := decodeJSON[updateComputeHostRequest](w, r)
	if !ok {
		return
	}

	existing, err := h.hostRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get compute host", err)
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "compute host not found", nil)
		return
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.HostType != "" {
		existing.HostType = req.HostType
	}
	if req.Endpoint != "" {
		existing.Endpoint = req.Endpoint
	}
	if req.TLSEnabled != nil {
		existing.TLSEnabled = *req.TLSEnabled
	}
	if req.TLSCA != "" {
		existing.TLSCA = req.TLSCA
	}
	if req.TLSCert != "" {
		existing.TLSCert = req.TLSCert
	}
	if req.TLSKey != "" {
		existing.TLSKey = req.TLSKey
	}
	if req.APIVersion != "" {
		existing.APIVersion = req.APIVersion
	}
	if req.Labels != nil {
		existing.Labels = req.Labels
	}

	err = h.hostRepo.Update(r.Context(), existing)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update compute host", err)
		return
	}

	existing.TLSKey = ""
	writeJSON(w, http.StatusOK, existing)
}

// DeleteHost handles DELETE /api/v1/docker/hosts/{id}
func (h *DockerHandler) DeleteHost(w http.ResponseWriter, r *http.Request) {
	if h.hostRepo == nil {
		writeError(w, http.StatusServiceUnavailable, "compute host service unavailable", nil)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing compute host id", nil)
		return
	}

	err := h.hostRepo.Delete(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete compute host", err)
		return
	}

	if h.metricsCollector != nil {
		h.metricsCollector.RemoveAgentMetric(id)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
