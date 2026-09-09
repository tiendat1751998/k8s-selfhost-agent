package http

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/agent"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// DockerHandler provides HTTP handlers for Docker, Swarm, and multi-host compute registry.
type DockerHandler struct {
	repo             docker.Repository
	hostRepo         docker.ComputeHostRepository
	verifier         PasswordVerifier
	logger           *zap.Logger
	tokenLimiter     *TokenViewLimiter
	metricsCollector AgentMetricRemover
	logClient        agent.LogClientInterface
}

// NewDockerHandler creates a new Docker HTTP handler with optional dependencies.
func NewDockerHandler(repo docker.Repository, args ...interface{}) *DockerHandler {
	h := &DockerHandler{
		repo:         repo,
		logger:       logger.Get(),
		tokenLimiter: NewTokenViewLimiter(defaultMaxTokenViews, defaultTokenViewWindow),
		logClient:    agent.NewAgentLogClient(),
	}
	for _, arg := range args {
		switch v := arg.(type) {
		case docker.ComputeHostRepository:
			h.hostRepo = v
		case PasswordVerifier:
			h.verifier = v
		case *zap.Logger:
			if v != nil {
				h.logger = v
			}
		case *TokenViewLimiter:
			if v != nil {
				h.tokenLimiter = v
			}
		case AgentMetricRemover:
			h.metricsCollector = v
		case agent.LogClientInterface:
			if v != nil {
				h.logClient = v
			}
		case *agent.AgentLogClient:
			if v != nil {
				h.logClient = v
			}
		}
	}
	return h
}

// RegisterRoutes registers Docker, Swarm, and Compute Host routes.
func (h *DockerHandler) RegisterRoutes(r chi.Router) {
	// Containers
	r.Get("/containers", h.ListContainers)
	r.Post("/containers/{id}/toggle", h.ToggleContainer)

	// Services
	r.Get("/services", h.ListServices)
	r.Post("/services/{id}/scale", h.ScaleService)

	// Nodes
	r.Get("/nodes", h.ListNodes)
	r.Get("/nodes/{id}", h.GetNodeDetails)
	r.Post("/nodes/{id}/drain", h.DrainNode)
	r.Post("/nodes/{id}/activate", h.ActivateNode)
	r.Delete("/nodes/{id}", h.RemoveNode)

	// Swarm
	r.Get("/swarm", h.GetSwarmInfo)
	r.With(middleware.RBACMiddleware("platform_admin")).Post("/swarm/tokens", h.GetSwarmTokens)

	// Logs
	r.Get("/logs", h.GetLogs)
	r.Post("/logs/cluster-search", h.SearchClusterLogs)

	// Compute Hosts
	r.Get("/hosts", h.ListHosts)
	r.Post("/hosts", h.CreateHost)
	r.Get("/hosts/{id}", h.GetHost)
	r.Put("/hosts/{id}", h.UpdateHost)
	r.Delete("/hosts/{id}", h.DeleteHost)
	r.Post("/hosts/{id}/test", h.TestHost)
}

// ListContainers handles GET /api/v1/docker/containers
func (h *DockerHandler) ListContainers(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "docker service unavailable", nil)
		return
	}
	items, err := h.repo.ListContainers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list containers", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

// ListNodes handles GET /api/v1/docker/nodes
func (h *DockerHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "docker service unavailable", nil)
		return
	}
	items, err := h.repo.ListNodes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list nodes", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

// GetNodeDetails handles GET /api/v1/docker/nodes/{id}
func (h *DockerHandler) GetNodeDetails(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "docker service unavailable", nil)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing node id", nil)
		return
	}

	node, err := h.repo.GetNodeDetails(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get node details", err)
		return
	}
	if node == nil {
		writeError(w, http.StatusNotFound, "node not found", nil)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

// DrainNode handles POST /api/v1/docker/nodes/{id}/drain
func (h *DockerHandler) DrainNode(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "docker service unavailable", nil)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing node id", nil)
		return
	}
	err := h.repo.DrainNode(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to drain node", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "drained"})
}

// ActivateNode handles POST /api/v1/docker/nodes/{id}/activate
func (h *DockerHandler) ActivateNode(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "docker service unavailable", nil)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing node id", nil)
		return
	}
	err := h.repo.ActivateNode(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to activate node", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "activated"})
}

// RemoveNode handles DELETE /api/v1/docker/nodes/{id}
func (h *DockerHandler) RemoveNode(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "docker service unavailable", nil)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing node id", nil)
		return
	}
	force := r.URL.Query().Get("force") == "true"
	err := h.repo.RemoveNode(r.Context(), id, force)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove node", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// ListServices handles GET /api/v1/docker/services
func (h *DockerHandler) ListServices(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "docker service unavailable", nil)
		return
	}
	items, err := h.repo.ListServices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list services", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

type scaleServiceRequest struct {
	Replicas int `json:"replicas"`
}

func (r *scaleServiceRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if r.Replicas < 0 {
		ve.Add("replicas", "replicas must be greater than or equal to 0")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// ScaleService handles POST /api/v1/docker/services/{id}/scale
func (h *DockerHandler) ScaleService(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "docker service unavailable", nil)
		return
	}
	id := chi.URLParam(r, "id")
	req, ok := decodeJSON[scaleServiceRequest](w, r)
	if !ok {
		return
	}

	err := h.repo.ScaleService(r.Context(), id, req.Replicas)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to scale service", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "scaled"})
}

type toggleContainerRequest struct {
	Action string `json:"action"` // "start" or "stop"
}

func (r *toggleContainerRequest) Validate() error {
	ve := NewValidationError("validation failed")
	switch r.Action {
	case "start", "stop":
	default:
		ve.Add("action", "action must be start or stop")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// ToggleContainer handles POST /api/v1/docker/containers/{id}/toggle
func (h *DockerHandler) ToggleContainer(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "docker service unavailable", nil)
		return
	}
	id := chi.URLParam(r, "id")
	req, ok := decodeJSON[toggleContainerRequest](w, r)
	if !ok {
		return
	}

	err := h.repo.ToggleContainer(r.Context(), id, req.Action)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to toggle container state", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "toggled"})
}

// GetLogs handles GET /api/v1/docker/logs?id=<id>&type=<type>&tail=<tail>&since=<since>&until=<until>&q=<q>&level=<level>&node_id=<node_id>&node_name=<node_name>&host=<host>&limit=<limit>
func (h *DockerHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	targetType := r.URL.Query().Get("type")
	tail := r.URL.Query().Get("tail")
	since := r.URL.Query().Get("since")
	until := r.URL.Query().Get("until")
	q := r.URL.Query().Get("q")
	level := r.URL.Query().Get("level")
	limitStr := r.URL.Query().Get("limit")

	limit := 0
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	nodeID := r.URL.Query().Get("node_id")
	nodeName := r.URL.Query().Get("node_name")
	hostParam := r.URL.Query().Get("host")

	if id == "" {
		writeError(w, http.StatusBadRequest, "missing required query parameter: id", nil)
		return
	}

	// Check if a compute host is targeted and fetch directly from its k8s-agent endpoint
	if h.hostRepo != nil && h.logClient != nil && (nodeID != "" || nodeName != "" || hostParam != "") {
		var targetHost *docker.ComputeHost
		targetKey := nodeID
		if targetKey == "" {
			targetKey = nodeName
		}
		if targetKey == "" {
			targetKey = hostParam
		}

		if nodeID != "" {
			targetHost, _ = h.hostRepo.GetByID(r.Context(), nodeID)
		}
		if targetHost == nil {
			tenantID := tenancy.TenantIDFromContext(r.Context())
			hosts, _ := h.hostRepo.List(r.Context(), tenantID)
			if len(hosts) == 0 {
				hosts, _ = h.hostRepo.ListAll(r.Context())
			}
			for i := range hosts {
				if hosts[i].ID == targetKey || hosts[i].Name == targetKey || hosts[i].Endpoint == targetKey {
					targetHost = &hosts[i]
					break
				}
			}
		}

		if targetHost != nil && targetHost.Endpoint != "" {
			token := ""
			if targetHost.Labels != nil {
				if t, ok := targetHost.Labels["auth_token"]; ok && t != "" {
					token = t
				} else if t, ok := targetHost.Labels["token"]; ok && t != "" {
					token = t
				}
			}

			agentLogs, err := h.logClient.GetNodeLogs(r.Context(), targetHost.Endpoint, token, id, tail, since, until, q, level)
			if err == nil {
				filteredLogs := FilterLogStream(agentLogs, q, level, limit)
				writeJSON(w, http.StatusOK, map[string]string{"logs": filteredLogs})
				return
			}

			if h.logger != nil {
				h.logger.Warn("Failed to fetch logs from agent host, falling back to direct Docker API",
					zap.String("host", targetHost.Name),
					zap.String("endpoint", targetHost.Endpoint),
					zap.Error(err),
				)
			}
		}
	}

	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "docker service unavailable", nil)
		return
	}

	var logs string
	var err error
	if tail != "" || since != "" {
		logs, err = h.repo.GetLogsWithOptions(r.Context(), id, targetType, tail, since)
	} else {
		logs, err = h.repo.GetLogs(r.Context(), id, targetType)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get logs", err)
		return
	}

	filteredLogs := FilterLogStream(logs, q, level, limit)
	writeJSON(w, http.StatusOK, map[string]string{"logs": filteredLogs})
}

// SearchClusterLogs handles POST /api/v1/docker/logs/cluster-search
func (h *DockerHandler) SearchClusterLogs(w http.ResponseWriter, r *http.Request) {
	if h.hostRepo == nil {
		writeError(w, http.StatusServiceUnavailable, "compute host service unavailable", nil)
		return
	}
	if h.logClient == nil {
		writeError(w, http.StatusServiceUnavailable, "log client service unavailable", nil)
		return
	}

	req, ok := decodeJSON[agent.SearchLogsRequest](w, r)
	if !ok {
		return
	}

	tenantID := tenancy.TenantIDFromContext(r.Context())
	hosts, err := h.hostRepo.List(r.Context(), tenantID)
	if err != nil || len(hosts) == 0 {
		allHosts, errAll := h.hostRepo.ListAll(r.Context())
		if errAll == nil && len(allHosts) > 0 {
			hosts = allHosts
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list compute hosts", err)
			return
		}
	}

	results, err := h.logClient.SearchClusterLogs(r.Context(), hosts, *req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search cluster logs", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  results,
		"total": len(results),
	})
}
