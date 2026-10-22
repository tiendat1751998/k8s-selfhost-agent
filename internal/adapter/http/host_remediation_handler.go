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

// allowedRemediationActions whitelist for safety.
var allowedRemediationActions = map[string]bool{
	"restart_service":       true,
	"restart_container":     true,
	"disk_pressure_cleanup": true,
	"cni_ipam_cleanup":      true,
	"reap_zombies":          true,
}

// HostRemediationHandler provides HTTP endpoints for querying k8s node health and triggering remediation.
type HostRemediationHandler struct {
	hostRepo   docker.ComputeHostRepository
	client     agent.RemediationClientInterface
	agentToken string
	authMw     interface {
		RequirePermission(string) func(http.Handler) http.Handler
	}
	logger *zap.Logger
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

// SetAgentToken overrides the agent auth token used when contacting node agents.
func (h *HostRemediationHandler) SetAgentToken(token string) {
	h.agentToken = token
}

// SetAuthMiddleware sets optional permission enforcement middleware.
func (h *HostRemediationHandler) SetAuthMiddleware(authMw interface{ RequirePermission(string) func(http.Handler) http.Handler }) {
	h.authMw = authMw
}

// RegisterRoutes registers host remediation routes onto a Chi router.
func (h *HostRemediationHandler) RegisterRoutes(r chi.Router) {
	if h.authMw != nil {
		r.With(h.authMw.RequirePermission("cluster:read")).Get("/{id}/k8s-health", h.GetHostK8sHealth)
		r.With(h.authMw.RequirePermission("cluster:write")).Post("/{id}/remediate", h.RemediateHost)
		r.With(h.authMw.RequirePermission("cluster:read")).Get("/{id}/remediate/history", h.GetHostRemediationHistory)
		r.With(h.authMw.RequirePermission("cluster:read")).Get("/k8s/nodes/{node}/remediate/history", h.GetNodeRemediationHistory)
		r.With(h.authMw.RequirePermission("cluster:read")).Get("/nodes/{node}/remediate/history", h.GetNodeRemediationHistory)
	} else {
		r.Get("/{id}/k8s-health", h.GetHostK8sHealth)
		r.Post("/{id}/remediate", h.RemediateHost)
		r.Get("/{id}/remediate/history", h.GetHostRemediationHistory)
		r.Get("/k8s/nodes/{node}/remediate/history", h.GetNodeRemediationHistory)
		r.Get("/nodes/{node}/remediate/history", h.GetNodeRemediationHistory)
	}
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

	authToken := token
	if h.agentToken != "" {
		authToken = h.agentToken
	}

	health, err := h.client.GetNodeK8sHealth(r.Context(), host.Endpoint, authToken)
	if err != nil {
		if agent.IsEndpointNotFound(err) {
			// Older agent without /k8s/health endpoint: synthesize health from /metrics
			synth, synthErr := h.client.SynthesizeHealthFromMetrics(r.Context(), host.Endpoint, authToken, host.Name)
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
	if !allowedRemediationActions[action] {
		writeError(w, http.StatusBadRequest, "invalid action: must be restart_service, restart_container, disk_pressure_cleanup, cni_ipam_cleanup, or reap_zombies", nil)
		return
	}
	if (action == "restart_service" || action == "restart_container") && strings.TrimSpace(req.Target) == "" {
		writeError(w, http.StatusBadRequest, action+" requires a non-empty target", nil)
		return
	}

	host, token, err := h.client.ResolveHostForNode(r.Context(), h.hostRepo, id)
	if err != nil || host == nil {
		writeError(w, http.StatusNotFound, "compute host not found: "+id, err)
		return
	}

	authToken := token
	if h.agentToken != "" {
		authToken = h.agentToken
	}

	res, err := h.client.ExecuteRemediation(r.Context(), host.Endpoint, authToken, req)
	if err != nil {
		h.logger.Error("failed to execute remediation on host",
			zap.String("host", host.Name),
			zap.String("action", req.Action),
			zap.Error(err),
		)
		errStr := err.Error()
		if strings.Contains(errStr, "429") || strings.Contains(strings.ToLower(errStr), "cooldown") {
			writeError(w, http.StatusTooManyRequests, "remediation rate limit exceeded, target in cooldown", err)
			return
		}
		if strings.Contains(errStr, "400") || strings.Contains(strings.ToLower(errStr), "allowlist") {
			writeError(w, http.StatusBadRequest, "invalid remediation request", err)
			return
		}
		writeError(w, http.StatusBadGateway, "failed to execute remediation on host agent", err)
		return
	}
	if res != nil && (res.Status == "cooldown" || strings.Contains(strings.ToLower(res.Details), "cooldown")) {
		writeError(w, http.StatusTooManyRequests, "remediation rate limit exceeded, target in cooldown", nil)
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
	if !allowedRemediationActions[action] {
		writeError(w, http.StatusBadRequest, "invalid action: must be restart_service, restart_container, disk_pressure_cleanup, cni_ipam_cleanup, or reap_zombies", nil)
		return
	}
	if (action == "restart_service" || action == "restart_container") && strings.TrimSpace(req.Target) == "" {
		writeError(w, http.StatusBadRequest, action+" requires a non-empty target", nil)
		return
	}

	host, token, err := h.client.ResolveHostForNode(r.Context(), h.hostRepo, nodeName)
	if err != nil || host == nil {
		writeError(w, http.StatusNotFound, "compute host not found for node: "+nodeName, err)
		return
	}

	authToken := token
	if h.agentToken != "" {
		authToken = h.agentToken
	}

	res, err := h.client.ExecuteRemediation(r.Context(), host.Endpoint, authToken, req)
	if err != nil {
		h.logger.Error("failed to execute remediation on node",
			zap.String("node", nodeName),
			zap.String("action", req.Action),
			zap.Error(err),
		)
		errStr := err.Error()
		if strings.Contains(errStr, "429") || strings.Contains(strings.ToLower(errStr), "cooldown") {
			writeError(w, http.StatusTooManyRequests, "remediation rate limit exceeded, target in cooldown", err)
			return
		}
		if strings.Contains(errStr, "400") || strings.Contains(strings.ToLower(errStr), "allowlist") {
			writeError(w, http.StatusBadRequest, "invalid remediation request", err)
			return
		}
		writeError(w, http.StatusBadGateway, "failed to execute remediation on node agent", err)
		return
	}
	if res != nil && (res.Status == "cooldown" || strings.Contains(strings.ToLower(res.Details), "cooldown")) {
		writeError(w, http.StatusTooManyRequests, "remediation rate limit exceeded, target in cooldown", nil)
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// GetHostRemediationHistory handles GET /api/v1/hosts/{id}/remediate/history.
func (h *HostRemediationHandler) GetHostRemediationHistory(w http.ResponseWriter, r *http.Request) {
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

	authToken := token
	if h.agentToken != "" {
		authToken = h.agentToken
	}

	history, err := h.client.GetRemediationHistory(r.Context(), host.Endpoint, authToken)
	if err != nil {
		h.logger.Warn("failed to retrieve remediation history from host agent",
			zap.String("host", host.Name),
			zap.String("endpoint", host.Endpoint),
			zap.Error(err),
		)
		writeError(w, http.StatusBadGateway, "failed to query remediation history from host agent", err)
		return
	}

	if history == nil {
		history = []agent.RemediationResultDTO{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"data":   history,
	})
}

// GetNodeRemediationHistory handles GET /api/v1/k8s/nodes/{node}/remediate/history.
func (h *HostRemediationHandler) GetNodeRemediationHistory(w http.ResponseWriter, r *http.Request) {
	nodeName := chi.URLParam(r, "node")
	if nodeName == "" {
		nodeName = chi.URLParam(r, "name")
	}
	if strings.TrimSpace(nodeName) == "" {
		writeError(w, http.StatusBadRequest, "missing node parameter", nil)
		return
	}

	host, token, err := h.client.ResolveHostForNode(r.Context(), h.hostRepo, nodeName)
	if err != nil || host == nil {
		writeError(w, http.StatusNotFound, "compute host not found for node: "+nodeName, err)
		return
	}

	authToken := token
	if h.agentToken != "" {
		authToken = h.agentToken
	}

	history, err := h.client.GetRemediationHistory(r.Context(), host.Endpoint, authToken)
	if err != nil {
		h.logger.Warn("failed to retrieve remediation history from node agent",
			zap.String("node", nodeName),
			zap.String("endpoint", host.Endpoint),
			zap.Error(err),
		)
		writeError(w, http.StatusBadGateway, "failed to query remediation history from node agent", err)
		return
	}

	if history == nil {
		history = []agent.RemediationResultDTO{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"data":   history,
	})
}
