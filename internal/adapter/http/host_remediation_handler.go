package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/agent"
)

// HostRemediationHandler provides HTTP endpoints for querying k8s node health and triggering remediation.
type HostRemediationHandler struct {
	hostRepo docker.ComputeHostRepository
	client   agent.RemediationClientInterface
	logger   *zap.Logger
}

// NewHostRemediationHandler creates a new HostRemediationHandler instance.
func NewHostRemediationHandler(hostRepo docker.ComputeHostRepository, client agent.RemediationClientInterface, logger *zap.Logger) *HostRemediationHandler {
	if client == nil {
		client = agent.NewAgentRemediationClient()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &HostRemediationHandler{
		hostRepo: hostRepo,
		client:   client,
		logger:   logger,
	}
}

// RegisterRoutes registers host remediation routes onto a Chi router.
func (h *HostRemediationHandler) RegisterRoutes(r chi.Router) {
	r.Get("/{id}/k8s-health", h.GetHostK8sHealth)
	r.Post("/{id}/remediate", h.RemediateHost)
}

// GetHostK8sHealth handles GET /api/v1/hosts/{id}/k8s-health.
func (h *HostRemediationHandler) GetHostK8sHealth(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = chi.URLParam(r, "name")
	}
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "missing host id parameter", nil)
		return
	}

	host, token, err := h.client.ResolveHostForNode(r.Context(), h.hostRepo, id)
	if err != nil || host == nil {
		writeError(w, http.StatusNotFound, "compute host not found: "+id, err)
		return
	}

	health, err := h.client.GetNodeK8sHealth(r.Context(), host.Endpoint, token)
	if err != nil {
		if agent.IsEndpointNotFound(err) {
			// Older agent without /k8s/health endpoint: synthesize health from /metrics
			synth, synthErr := h.client.SynthesizeHealthFromMetrics(r.Context(), host.Endpoint, token, host.Name)
			if synthErr == nil && synth != nil {
				writeJSON(w, http.StatusOK, synth)
				return
			}
		}
		h.logger.Warn("failed to retrieve k8s health from node agent",
			zap.String("host", host.Name),
			zap.String("endpoint", host.Endpoint),
			zap.Error(err),
		)
		writeError(w, http.StatusBadGateway, "failed to query k8s health from host agent", err)
		return
	}

	writeJSON(w, http.StatusOK, health)
}

// RemediateHost handles POST /api/v1/hosts/{id}/remediate.
func (h *HostRemediationHandler) RemediateHost(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = chi.URLParam(r, "name")
	}
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "missing host id parameter", nil)
		return
	}

	var req agent.RemediationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json request body", err)
		return
	}

	action := strings.TrimSpace(req.Action)
	switch action {
	case "restart_service", "disk_pressure_cleanup", "cni_ipam_cleanup", "reap_zombies":
		// Allowed action
	default:
		writeError(w, http.StatusBadRequest, "invalid action: must be restart_service, disk_pressure_cleanup, cni_ipam_cleanup, or reap_zombies", nil)
		return
	}

	host, token, err := h.client.ResolveHostForNode(r.Context(), h.hostRepo, id)
	if err != nil || host == nil {
		writeError(w, http.StatusNotFound, "compute host not found: "+id, err)
		return
	}

	res, err := h.client.ExecuteRemediation(r.Context(), host.Endpoint, token, req)
	if err != nil {
		h.logger.Error("failed to execute remediation on host",
			zap.String("host", host.Name),
			zap.String("action", req.Action),
			zap.Error(err),
		)
		writeError(w, http.StatusBadGateway, "failed to execute remediation on host agent", err)
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// RemediateNode handles POST /api/v1/k8s/nodes/{node}/remediate (and /k8s/{cluster}/nodes/{name}/remediate).
func (h *HostRemediationHandler) RemediateNode(w http.ResponseWriter, r *http.Request) {
	nodeName := chi.URLParam(r, "node")
	if nodeName == "" {
		nodeName = chi.URLParam(r, "name")
	}
	if strings.TrimSpace(nodeName) == "" {
		writeError(w, http.StatusBadRequest, "missing node parameter", nil)
		return
	}

	var req agent.RemediationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json request body", err)
		return
	}

	action := strings.TrimSpace(req.Action)
	switch action {
	case "restart_service", "disk_pressure_cleanup", "cni_ipam_cleanup", "reap_zombies":
		// Allowed action
	default:
		writeError(w, http.StatusBadRequest, "invalid action: must be restart_service, disk_pressure_cleanup, cni_ipam_cleanup, or reap_zombies", nil)
		return
	}

	host, token, err := h.client.ResolveHostForNode(r.Context(), h.hostRepo, nodeName)
	if err != nil || host == nil {
		writeError(w, http.StatusNotFound, "compute host not found for node: "+nodeName, err)
		return
	}

	res, err := h.client.ExecuteRemediation(r.Context(), host.Endpoint, token, req)
	if err != nil {
		h.logger.Error("failed to execute remediation on node",
			zap.String("node", nodeName),
			zap.String("action", req.Action),
			zap.Error(err),
		)
		writeError(w, http.StatusBadGateway, "failed to execute remediation on node agent", err)
		return
	}

	writeJSON(w, http.StatusOK, res)
}
