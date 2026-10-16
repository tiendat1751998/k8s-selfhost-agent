package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

// ErrEndpointNotFound indicates the requested agent endpoint returned HTTP 404.
var ErrEndpointNotFound = errors.New("agent endpoint not found (HTTP 404)")

// IsEndpointNotFound checks if the given error was caused by a 404 Not Found from an agent.
func IsEndpointNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrEndpointNotFound) || strings.Contains(err.Error(), "HTTP 404")
}

// NodeK8sHealthResponse matches k8s-agent's GET /k8s/health endpoint output.
type NodeK8sHealthResponse struct {
	NodeName          string   `json:"node_name"`
	KubeletHealthy    bool     `json:"kubelet_healthy"`
	KubeletLatencyMs  float64  `json:"kubelet_latency_ms"`
	PLEGHealthy       bool     `json:"pleg_healthy"`
	ContainerdHealthy bool     `json:"containerd_healthy"`
	DockerHealthy     bool     `json:"docker_healthy"`
	CRISocketPath     string   `json:"cri_socket_path,omitempty"`
	DiskPressure      bool     `json:"disk_pressure"`
	InodePressure     bool     `json:"inode_pressure"`
	MemoryPressure    bool     `json:"memory_pressure"`
	PIDPressure       bool     `json:"pid_pressure"`
	DStateProcesses   int      `json:"d_state_processes"`
	ZombieCount       int      `json:"zombie_count"`
	ConntrackUsagePct float64  `json:"conntrack_usage_pct"`
	DetectedAnomalies []string `json:"detected_anomalies"`
	CheckedAt         string   `json:"checked_at"`
}

// RemediationRequest defines payload for POST /remediate.
type RemediationRequest struct {
	Action string `json:"action"` // "restart_service" | "disk_pressure_cleanup" | "cni_ipam_cleanup" | "reap_zombies"
	Target string `json:"target,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// RemediationActionResult encapsulates execution results from k8s-agent /remediate.
type RemediationActionResult struct {
	Success     bool   `json:"success"`
	Action      string `json:"action"`
	Target      string `json:"target,omitempty"`
	BytesFreed  int64  `json:"bytes_freed,omitempty"`
	InodesFreed int64  `json:"inodes_freed,omitempty"`
	DurationMs  int64  `json:"duration_ms"`
	Details     string `json:"details"`
	Timestamp   string `json:"timestamp"`
}

// RemediationClientInterface defines client operations for node health & remediation.
type RemediationClientInterface interface {
	GetNodeK8sHealth(ctx context.Context, hostEndpoint, authToken string) (*NodeK8sHealthResponse, error)
	ExecuteRemediation(ctx context.Context, hostEndpoint, authToken string, req RemediationRequest) (*RemediationActionResult, error)
	ResolveHostForNode(ctx context.Context, hostRepo docker.ComputeHostRepository, nodeNameOrIP string) (*docker.ComputeHost, string, error)
	SynthesizeHealthFromMetrics(ctx context.Context, hostEndpoint, authToken, nodeName string) (*NodeK8sHealthResponse, error)
}

// AgentRemediationClient coordinates HTTP RPC with node k8s-agent (:9100).
type AgentRemediationClient struct {
	httpClient *http.Client
	timeout    time.Duration
	authToken  string
}

// AgentRemediationClientOption configures AgentRemediationClient.
type AgentRemediationClientOption func(*AgentRemediationClient)

// WithRemediationHTTPClient overrides the default HTTP client.
func WithRemediationHTTPClient(client *http.Client) AgentRemediationClientOption {
	return func(c *AgentRemediationClient) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// WithRemediationTimeout configures request timeout.
func WithRemediationTimeout(d time.Duration) AgentRemediationClientOption {
	return func(c *AgentRemediationClient) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// WithRemediationAuthToken configures default bearer token.
func WithRemediationAuthToken(token string) AgentRemediationClientOption {
	return func(c *AgentRemediationClient) {
		c.authToken = token
	}
}

// NewAgentRemediationClient instantiates a new AgentRemediationClient.
func NewAgentRemediationClient(opts ...AgentRemediationClientOption) *AgentRemediationClient {
	c := &AgentRemediationClient{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		timeout: 15 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *AgentRemediationClient) resolveToken(authToken string) string {
	if trimmed := strings.TrimSpace(authToken); trimmed != "" {
		return trimmed
	}
	if c != nil && c.authToken != "" {
		return c.authToken
	}
	return os.Getenv("AGENT_AUTH_TOKEN")
}

// GetNodeK8sHealth queries GET /k8s/health on the specified node agent.
func (c *AgentRemediationClient) GetNodeK8sHealth(ctx context.Context, hostEndpoint, authToken string) (*NodeK8sHealthResponse, error) {
	baseURL := NormalizeHostEndpoint(hostEndpoint)
	if baseURL == "" {
		return nil, fmt.Errorf("invalid host endpoint: %q", hostEndpoint)
	}

	reqCtx := ctx
	if c.timeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, baseURL+"/k8s/health", nil)
	if err != nil {
		return nil, fmt.Errorf("creating /k8s/health request: %w", err)
	}

	if token := c.resolveToken(authToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing /k8s/health request to %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading /k8s/health response body: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("agent returned HTTP 404: %w", ErrEndpointNotFound)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent /k8s/health returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var healthResp NodeK8sHealthResponse
	if err := json.Unmarshal(bodyBytes, &healthResp); err != nil {
		return nil, fmt.Errorf("decoding /k8s/health json: %w", err)
	}

	return &healthResp, nil
}

// ExecuteRemediation posts a remediation task to POST /remediate on the node agent.
func (c *AgentRemediationClient) ExecuteRemediation(ctx context.Context, hostEndpoint, authToken string, req RemediationRequest) (*RemediationActionResult, error) {
	baseURL := NormalizeHostEndpoint(hostEndpoint)
	if baseURL == "" {
		return nil, fmt.Errorf("invalid host endpoint: %q", hostEndpoint)
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling remediation request: %w", err)
	}

	reqCtx := ctx
	if c.timeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, baseURL+"/remediate", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("creating /remediate request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if token := c.resolveToken(authToken); token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("executing /remediate request to %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading /remediate response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent /remediate returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var actionResult RemediationActionResult
	if err := json.Unmarshal(bodyBytes, &actionResult); err != nil {
		return nil, fmt.Errorf("decoding /remediate json: %w", err)
	}

	return &actionResult, nil
}

// ResolveHostForNode locates the registered ComputeHost by ID, name, or IP endpoint.
// If hostRepo has no match but nodeNameOrIP is a valid IP, synthesizes a compute host endpoint.
func (c *AgentRemediationClient) ResolveHostForNode(ctx context.Context, hostRepo docker.ComputeHostRepository, nodeNameOrIP string) (*docker.ComputeHost, string, error) {
	nodeNameOrIP = strings.TrimSpace(nodeNameOrIP)
	if nodeNameOrIP == "" {
		return nil, "", fmt.Errorf("nodeNameOrIP is required")
	}

	if hostRepo != nil {
		hosts, err := hostRepo.ListAll(ctx)
		if err == nil {
			for _, h := range hosts {
				if h.ID == nodeNameOrIP || strings.EqualFold(h.Name, nodeNameOrIP) || strings.Contains(h.Endpoint, nodeNameOrIP) {
					token := ""
					if h.Labels != nil {
						if t, ok := h.Labels["auth_token"]; ok && t != "" {
							token = t
						} else if t, ok := h.Labels["token"]; ok && t != "" {
							token = t
						}
					}
					token = c.resolveToken(token)
					matched := h
					return &matched, token, nil
				}
			}
		}
	}

	// Check if nodeNameOrIP is a valid IPv4 or IPv6 address (or host:port with valid IP)
	ipCandidate := nodeNameOrIP
	if host, _, err := net.SplitHostPort(nodeNameOrIP); err == nil {
		ipCandidate = host
	}
	if parsedIP := net.ParseIP(ipCandidate); parsedIP != nil {
		syntheticHost := &docker.ComputeHost{
			ID:       nodeNameOrIP,
			Name:     nodeNameOrIP,
			Endpoint: "http://" + net.JoinHostPort(parsedIP.String(), "9100"),
			HostType: "agent",
			Status:   "active",
		}
		return syntheticHost, c.resolveToken(""), nil
	}

	return nil, "", fmt.Errorf("no compute host found for node %q", nodeNameOrIP)
}

// SynthesizeHealthFromMetrics probes /metrics on legacy agents to synthesize NodeK8sHealthResponse.
func (c *AgentRemediationClient) SynthesizeHealthFromMetrics(ctx context.Context, hostEndpoint, authToken, nodeName string) (*NodeK8sHealthResponse, error) {
	baseURL := NormalizeHostEndpoint(hostEndpoint)
	if baseURL == "" {
		return nil, fmt.Errorf("invalid host endpoint: %q", hostEndpoint)
	}

	reqCtx := ctx
	if c.timeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, baseURL+"/metrics", nil)
	if err != nil {
		return nil, fmt.Errorf("creating /metrics fallback request: %w", err)
	}

	if token := c.resolveToken(authToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying /metrics fallback on %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metrics fallback returned HTTP %d", resp.StatusCode)
	}

	var metricsData struct {
		Hostname    string  `json:"hostname"`
		DiskPercent float64 `json:"disk_percent"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&metricsData)

	resolvedName := nodeName
	if metricsData.Hostname != "" {
		resolvedName = metricsData.Hostname
	}

	return &NodeK8sHealthResponse{
		NodeName:          resolvedName,
		KubeletHealthy:    true,
		ContainerdHealthy: true,
		PLEGHealthy:       true,
		DiskPressure:      metricsData.DiskPercent >= 88.0,
		DetectedAnomalies: []string{},
		CheckedAt:         time.Now().UTC().Format(time.RFC3339),
	}, nil
}
