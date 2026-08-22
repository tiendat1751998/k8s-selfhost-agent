package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// PasswordVerifier defines the interface for verifying a user's password.
type PasswordVerifier interface {
	VerifyPassword(ctx context.Context, userID, password string) error
}

// TokenViewRequest defines the request body for viewing swarm join tokens.
type TokenViewRequest struct {
	Password string `json:"password"`
}

// Validate validates the token view request.
func (r *TokenViewRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Password) == "" {
		ve.Add("password", "password is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// SwarmTokensResponse defines the protected response containing swarm join tokens.
type SwarmTokensResponse struct {
	WorkerToken        string `json:"worker_token"`
	ManagerToken       string `json:"manager_token"`
	ManagerAddr        string `json:"manager_addr"`
	WorkerTokenMasked  string `json:"worker_token_masked"`
	ManagerTokenMasked string `json:"manager_token_masked"`
	ExpiresInSeconds   int    `json:"expires_in_seconds"`
}

// TokenViewLimiter limits token views per user within a rolling time window.
type TokenViewLimiter struct {
	mu     sync.Mutex
	views  map[string][]time.Time
	max    int
	window time.Duration
}

const (
	defaultMaxTokenViews   = 3
	defaultTokenViewWindow = 15 * time.Minute
)

// NewTokenViewLimiter creates a new thread-safe token view rate limiter.
func NewTokenViewLimiter(max int, window time.Duration) *TokenViewLimiter {
	return &TokenViewLimiter{
		views:  make(map[string][]time.Time),
		max:    max,
		window: window,
	}
}

// Allow checks if a user is permitted to view tokens, recording the timestamp if permitted.
func (l *TokenViewLimiter) Allow(userID string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	var valid []time.Time
	for _, t := range l.views[userID] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= l.max {
		l.views[userID] = valid
		oldest := valid[0]
		retryAfter := oldest.Add(l.window).Sub(now)
		if retryAfter < 0 {
			retryAfter = 0
		}
		return false, retryAfter
	}

	valid = append(valid, now)
	l.views[userID] = valid
	return true, 0
}

func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "SWMTKN-1-***..." + token
	}
	last8 := token[len(token)-8:]
	return "SWMTKN-1-***..." + last8
}

// AgentMetricRemover defines the interface for removing agent metrics upon compute host deletion.
type AgentMetricRemover interface {
	RemoveAgentMetric(hostID string)
}

// DockerHandler provides HTTP handlers for Docker, Swarm, and multi-host compute registry.
type DockerHandler struct {
	repo             docker.Repository
	hostRepo         docker.ComputeHostRepository
	verifier         PasswordVerifier
	logger           *zap.Logger
	tokenLimiter     *TokenViewLimiter
	metricsCollector AgentMetricRemover
}

// NewDockerHandler creates a new Docker HTTP handler with optional dependencies.
func NewDockerHandler(repo docker.Repository, args ...interface{}) *DockerHandler {
	h := &DockerHandler{
		repo:         repo,
		logger:       logger.Get(),
		tokenLimiter: NewTokenViewLimiter(defaultMaxTokenViews, defaultTokenViewWindow),
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

	// Compute Hosts
	r.Get("/hosts", h.ListHosts)
	r.Post("/hosts", h.CreateHost)
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

// GetSwarmInfo handles GET /api/v1/docker/swarm
func (h *DockerHandler) GetSwarmInfo(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "docker service unavailable", nil)
		return
	}
	info, err := h.repo.GetSwarmInfo(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get swarm info", err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// GetSwarmTokens handles POST /api/v1/docker/swarm/tokens (platform_admin only with re-authentication)
func (h *DockerHandler) GetSwarmTokens(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "docker service unavailable", nil)
		return
	}

	// 1. Role verification - platform_admin only
	role := tenancy.UserRoleFromContext(r.Context())
	if role != "platform_admin" {
		writeError(w, http.StatusForbidden, "platform_admin role required to view swarm join tokens", nil)
		return
	}

	userID := tenancy.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "unknown"
	}

	// 2. Decode & validate request body (password confirmation required)
	req, ok := decodeJSON[TokenViewRequest](w, r)
	if !ok {
		return
	}

	// 3. Rate limiting - Max 3 token views per 15 minutes per user
	if h.tokenLimiter != nil {
		allowed, retryAfter := h.tokenLimiter.Allow(userID)
		if !allowed {
			minutes := int(retryAfter.Minutes()) + 1
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
			writeError(w, http.StatusTooManyRequests, fmt.Sprintf("token viewing rate limited. Try again in %d minutes", minutes), nil)
			return
		}
	}

	// 4. Re-authentication verification
	if h.verifier != nil {
		if err := h.verifier.VerifyPassword(r.Context(), userID, req.Password); err != nil {
			writeError(w, http.StatusForbidden, "invalid password", nil)
			return
		}
	}

	// 5. Security audit logging
	logInstance := h.logger
	if logInstance == nil {
		logInstance = logger.Get()
	}
	logInstance.Warn("SECURITY: Swarm join tokens accessed",
		zap.String("user_id", userID),
		zap.String("remote_addr", r.RemoteAddr),
		zap.String("user_agent", r.UserAgent()),
	)

	// 6. Fetch join tokens
	tokens, err := h.repo.GetSwarmJoinTokens(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get swarm join tokens", err)
		return
	}

	// 7. Return masked and full tokens response
	resp := SwarmTokensResponse{
		WorkerToken:        tokens.WorkerToken,
		ManagerToken:       tokens.ManagerToken,
		ManagerAddr:        tokens.ManagerAddr,
		WorkerTokenMasked:  maskToken(tokens.WorkerToken),
		ManagerTokenMasked: maskToken(tokens.ManagerToken),
		ExpiresInSeconds:   60,
	}

	writeJSON(w, http.StatusOK, resp)
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

// GetLogs handles GET /api/v1/docker/logs
func (h *DockerHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "docker service unavailable", nil)
		return
	}
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

// TestHost handles POST /api/v1/docker/hosts/{id}/test
func (h *DockerHandler) TestHost(w http.ResponseWriter, r *http.Request) {
	h.TestHostConnectivity(w, r)
}

// TestHostConnectivity tests connectivity to a compute host based on its host type (agent, docker, k8s).
func (h *DockerHandler) TestHostConnectivity(w http.ResponseWriter, r *http.Request) {
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

	now := time.Now().UTC()

	switch host.HostType {
	case "agent":
		latencyMs, agentInfo, testErr := testAgentHostConnection(r.Context(), host)
		if testErr != nil {
			_ = h.hostRepo.UpdateStatus(r.Context(), host.ID, "error", now)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"status":     "error",
				"message":    testErr.Error(),
				"latency_ms": latencyMs,
			})
			return
		}

		_ = h.hostRepo.UpdateStatus(r.Context(), host.ID, "connected", now)
		resp := map[string]interface{}{
			"status":     "connected",
			"message":    "successfully connected to agent host",
			"latency_ms": latencyMs,
		}
		if len(agentInfo) > 0 {
			resp["agent_info"] = agentInfo
		}
		writeJSON(w, http.StatusOK, resp)

	case "docker":
		latencyMs, testErr := testDockerHostConnection(r.Context(), host)
		if testErr != nil {
			_ = h.hostRepo.UpdateStatus(r.Context(), host.ID, "error", now)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"status":     "error",
				"message":    testErr.Error(),
				"latency_ms": latencyMs,
			})
			return
		}

		_ = h.hostRepo.UpdateStatus(r.Context(), host.ID, "connected", now)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "connected",
			"message":    "successfully connected to docker host",
			"latency_ms": latencyMs,
		})

	case "k8s":
		latencyMs, testErr := testK8sHostConnection(r.Context(), host)
		if testErr != nil {
			_ = h.hostRepo.UpdateStatus(r.Context(), host.ID, "error", now)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"status":     "error",
				"message":    testErr.Error(),
				"latency_ms": latencyMs,
			})
			return
		}

		_ = h.hostRepo.UpdateStatus(r.Context(), host.ID, "connected", now)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "connected",
			"message":    "successfully connected to kubernetes api",
			"latency_ms": latencyMs,
		})

	case "prometheus":
		latencyMs, testErr := testPrometheusHostConnection(r.Context(), host)
		if testErr != nil {
			_ = h.hostRepo.UpdateStatus(r.Context(), host.ID, "error", now)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"status":     "error",
				"message":    testErr.Error(),
				"latency_ms": latencyMs,
			})
			return
		}

		_ = h.hostRepo.UpdateStatus(r.Context(), host.ID, "connected", now)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "connected",
			"message":    "successfully connected to prometheus endpoint",
			"latency_ms": latencyMs,
		})

	case "git":
		latencyMs, testErr := testGitHostConnection(r.Context(), host)
		if testErr != nil {
			_ = h.hostRepo.UpdateStatus(r.Context(), host.ID, "error", now)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"status":     "error",
				"message":    testErr.Error(),
				"latency_ms": latencyMs,
			})
			return
		}

		_ = h.hostRepo.UpdateStatus(r.Context(), host.ID, "connected", now)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "connected",
			"message":    "successfully connected to git repository endpoint",
			"latency_ms": latencyMs,
		})

	case "database":
		latencyMs, testErr := testDatabaseHostConnection(r.Context(), host)
		if testErr != nil {
			_ = h.hostRepo.UpdateStatus(r.Context(), host.ID, "error", now)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"status":     "error",
				"message":    testErr.Error(),
				"latency_ms": latencyMs,
			})
			return
		}

		_ = h.hostRepo.UpdateStatus(r.Context(), host.ID, "connected", now)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "connected",
			"message":    "successfully connected to database endpoint",
			"latency_ms": latencyMs,
		})

	case "custom":
		fallthrough
	default:
		latencyMs, testErr := testCustomHostConnection(r.Context(), host)
		if testErr != nil {
			_ = h.hostRepo.UpdateStatus(r.Context(), host.ID, "error", now)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"status":     "error",
				"message":    testErr.Error(),
				"latency_ms": latencyMs,
			})
			return
		}

		_ = h.hostRepo.UpdateStatus(r.Context(), host.ID, "connected", now)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "connected",
			"message":    "successfully connected to endpoint",
			"latency_ms": latencyMs,
		})
	}
}

func testAgentHostConnection(ctx context.Context, host *docker.ComputeHost) (int64, map[string]interface{}, error) {
	endpoint := strings.TrimSpace(host.Endpoint)
	endpoint = strings.TrimRight(endpoint, "/")
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		if strings.HasPrefix(endpoint, "tcp://") {
			endpoint = "http://" + strings.TrimPrefix(endpoint, "tcp://")
		} else {
			endpoint = "http://" + endpoint
		}
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	start := time.Now()

	// 1. GET /health
	healthReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/health", nil)
	if err != nil {
		return 0, nil, fmt.Errorf("creating health request: %w", err)
	}
	healthResp, err := client.Do(healthReq)
	if err != nil {
		return 0, nil, fmt.Errorf("connecting to agent health endpoint: %w", err)
	}
	defer healthResp.Body.Close()

	if healthResp.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("agent health check returned status %d", healthResp.StatusCode)
	}

	var healthData struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(healthResp.Body).Decode(&healthData); err != nil {
		return 0, nil, fmt.Errorf("decoding agent health response: %w", err)
	}
	if !strings.EqualFold(healthData.Status, "ok") && !strings.EqualFold(healthData.Status, "healthy") {
		return 0, nil, fmt.Errorf("agent health status: %s", healthData.Status)
	}

	latencyMs := time.Since(start).Milliseconds()
	if latencyMs <= 0 {
		latencyMs = 1
	}

	// 2. GET /metrics for agent system info
	agentInfo := make(map[string]interface{})
	metricsReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/metrics", nil)
	if err == nil {
		metricsResp, err := client.Do(metricsReq)
		if err == nil {
			defer metricsResp.Body.Close()
			if metricsResp.StatusCode == http.StatusOK {
				var metricsData struct {
					Hostname      string `json:"hostname"`
					OS            string `json:"os"`
					Arch          string `json:"arch"`
					OSDistro      string `json:"os_distro"`
					KernelVersion string `json:"kernel_version"`
					UptimeSeconds int64  `json:"uptime_seconds"`
				}
				if err := json.NewDecoder(metricsResp.Body).Decode(&metricsData); err == nil {
					if metricsData.Hostname != "" {
						agentInfo["hostname"] = metricsData.Hostname
					}
					if metricsData.OS != "" {
						agentInfo["os"] = metricsData.OS
					}
					if metricsData.Arch != "" {
						agentInfo["arch"] = metricsData.Arch
					}
					if metricsData.OSDistro != "" {
						agentInfo["os_distro"] = metricsData.OSDistro
					}
					if metricsData.KernelVersion != "" {
						agentInfo["kernel_version"] = metricsData.KernelVersion
					}
					agentInfo["uptime"] = metricsData.UptimeSeconds
					agentInfo["uptime_seconds"] = metricsData.UptimeSeconds
				}
			}
		}
	}

	return latencyMs, agentInfo, nil
}

func testDockerHostConnection(ctx context.Context, host *docker.ComputeHost) (int64, error) {
	start := time.Now()
	err := testComputeHostConnection(ctx, host)
	latencyMs := time.Since(start).Milliseconds()
	if latencyMs <= 0 {
		latencyMs = 1
	}
	return latencyMs, err
}

func testComputeHostConnection(ctx context.Context, host *docker.ComputeHost) error {
	opts := []client.Opt{client.WithHost(host.Endpoint)}
	if host.APIVersion != "" {
		opts = append(opts, client.WithVersion(host.APIVersion))
	} else {
		opts = append(opts, client.WithAPIVersionNegotiation())
	}

	if host.TLSEnabled && host.TLSCA != "" && host.TLSCert != "" && host.TLSKey != "" {
		tlsConfig, err := configureDockerTLS(host.TLSCA, host.TLSCert, host.TLSKey)
		if err != nil {
			return fmt.Errorf("tls configuration error: %w", err)
		}
		opts = append(opts, client.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
			Timeout: 5 * time.Second,
		}))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return fmt.Errorf("creating docker client: %w", err)
	}
	defer cli.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err = cli.Ping(pingCtx)
	if err != nil {
		return fmt.Errorf("pinging docker daemon: %w", err)
	}
	return nil
}

func configureDockerTLS(caPEM, certPEM, keyPEM string) (*tls.Config, error) {
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM([]byte(caPEM)) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}
	cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return nil, fmt.Errorf("failed to parse client certificate/key: %w", err)
	}
	return &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func testK8sHostConnection(ctx context.Context, host *docker.ComputeHost) (int64, error) {
	endpoint := strings.TrimSpace(host.Endpoint)
	endpoint = strings.TrimRight(endpoint, "/")
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	if host.TLSEnabled && host.TLSCA != "" {
		tlsConfig, err := configureDockerTLS(host.TLSCA, host.TLSCert, host.TLSKey)
		if err == nil {
			transport.TLSClientConfig = tlsConfig
		}
	}

	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: transport,
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/livez", nil)
	if err != nil {
		return 0, fmt.Errorf("creating k8s request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		// Fallback probe to root
		reqRoot, rErr := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if rErr == nil {
			respRoot, err2 := client.Do(reqRoot)
			if err2 == nil {
				defer respRoot.Body.Close()
				latencyMs := time.Since(start).Milliseconds()
				if latencyMs <= 0 {
					latencyMs = 1
				}
				return latencyMs, nil
			}
		}
		return 0, fmt.Errorf("connecting to kubernetes endpoint: %w", err)
	}
	defer resp.Body.Close()

	latencyMs := time.Since(start).Milliseconds()
	if latencyMs <= 0 {
		latencyMs = 1
	}

	if resp.StatusCode >= 500 {
		return latencyMs, fmt.Errorf("kubernetes api returned status %d", resp.StatusCode)
	}

	return latencyMs, nil
}

func testPrometheusHostConnection(ctx context.Context, host *docker.ComputeHost) (int64, error) {
	endpoint := strings.TrimSpace(host.Endpoint)
	endpoint = strings.TrimRight(endpoint, "/")
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "http://" + endpoint
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/-/healthy", nil)
	if err != nil {
		return 0, fmt.Errorf("creating prometheus request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		reqRoot, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if reqRoot != nil {
			respRoot, rErr := client.Do(reqRoot)
			if rErr == nil {
				defer respRoot.Body.Close()
				latencyMs := time.Since(start).Milliseconds()
				if latencyMs <= 0 {
					latencyMs = 1
				}
				return latencyMs, nil
			}
		}
		return 0, fmt.Errorf("connecting to prometheus endpoint: %w", err)
	}
	defer resp.Body.Close()

	latencyMs := time.Since(start).Milliseconds()
	if latencyMs <= 0 {
		latencyMs = 1
	}

	if resp.StatusCode >= 500 {
		return latencyMs, fmt.Errorf("prometheus returned status %d", resp.StatusCode)
	}

	return latencyMs, nil
}

func testGitHostConnection(ctx context.Context, host *docker.ComputeHost) (int64, error) {
	endpoint := strings.TrimSpace(host.Endpoint)
	endpoint = strings.TrimRight(endpoint, "/")
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("creating git request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("connecting to git endpoint: %w", err)
	}
	defer resp.Body.Close()

	latencyMs := time.Since(start).Milliseconds()
	if latencyMs <= 0 {
		latencyMs = 1
	}

	if resp.StatusCode >= 500 {
		return latencyMs, fmt.Errorf("git server returned status %d", resp.StatusCode)
	}

	return latencyMs, nil
}

func testDatabaseHostConnection(ctx context.Context, host *docker.ComputeHost) (int64, error) {
	endpoint := strings.TrimSpace(host.Endpoint)

	var hostPort string
	if u, err := url.Parse(endpoint); err == nil && u.Host != "" {
		hostPort = u.Host
	} else if strings.Contains(endpoint, "@tcp(") {
		idx1 := strings.Index(endpoint, "@tcp(")
		idx2 := strings.Index(endpoint[idx1:], ")")
		if idx1 != -1 && idx2 != -1 {
			hostPort = endpoint[idx1+5 : idx1+idx2]
		}
	} else {
		clean := endpoint
		if idx := strings.Index(clean, "://"); idx != -1 {
			clean = clean[idx+3:]
		}
		if idx := strings.Index(clean, "/"); idx != -1 {
			clean = clean[:idx]
		}
		if idx := strings.Index(clean, "@"); idx != -1 {
			clean = clean[idx+1:]
		}
		hostPort = clean
	}

	if !strings.Contains(hostPort, ":") {
		hostPort = hostPort + ":5432"
	}

	start := time.Now()
	var d net.Dialer
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := d.DialContext(dialCtx, "tcp", hostPort)
	if err != nil {
		return 0, fmt.Errorf("tcp connection to database at %s failed: %w", hostPort, err)
	}
	defer conn.Close()

	latencyMs := time.Since(start).Milliseconds()
	if latencyMs <= 0 {
		latencyMs = 1
	}

	return latencyMs, nil
}

func testCustomHostConnection(ctx context.Context, host *docker.ComputeHost) (int64, error) {
	endpoint := strings.TrimSpace(host.Endpoint)
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		client := &http.Client{
			Timeout: 5 * time.Second,
		}
		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return 0, fmt.Errorf("creating custom request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return 0, fmt.Errorf("connecting to custom endpoint: %w", err)
		}
		defer resp.Body.Close()

		latencyMs := time.Since(start).Milliseconds()
		if latencyMs <= 0 {
			latencyMs = 1
		}
		if resp.StatusCode >= 500 {
			return latencyMs, fmt.Errorf("custom endpoint returned status %d", resp.StatusCode)
		}
		return latencyMs, nil
	}

	hostPort := endpoint
	if strings.HasPrefix(hostPort, "tcp://") {
		hostPort = strings.TrimPrefix(hostPort, "tcp://")
	}
	start := time.Now()
	var d net.Dialer
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := d.DialContext(dialCtx, "tcp", hostPort)
	if err != nil {
		return 0, fmt.Errorf("tcp connection to %s failed: %w", hostPort, err)
	}
	defer conn.Close()

	latencyMs := time.Since(start).Milliseconds()
	if latencyMs <= 0 {
		latencyMs = 1
	}
	return latencyMs, nil
}

