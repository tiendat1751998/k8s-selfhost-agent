package cluster

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/fleet"
	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
	"go.uber.org/zap"
)

// DialContextFunc defines the signature for dialing network connections.
type DialContextFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// HealthCheckerOption configures optional HealthChecker dependencies.
type HealthCheckerOption func(*HealthChecker)

// WithComputeHostRepo sets the compute host repository for probing edge node agents.
func WithComputeHostRepo(repo docker.ComputeHostRepository) HealthCheckerOption {
	return func(h *HealthChecker) {
		h.computeHostRepo = repo
	}
}

// WithHTTPClient sets a custom HTTP client for node agent health probes.
func WithHTTPClient(client *http.Client) HealthCheckerOption {
	return func(h *HealthChecker) {
		h.httpClient = client
	}
}

// WithDialContext sets a custom network dialer for TCP connectivity checks.
func WithDialContext(dialFn DialContextFunc) HealthCheckerOption {
	return func(h *HealthChecker) {
		h.dialContext = dialFn
	}
}

// HealthChecker orchestrates real-time health checks across all fleet clusters.
type HealthChecker struct {
	fleetRepo       fleet.Repository
	discovery       DiscoveryPort
	computeHostRepo docker.ComputeHostRepository
	logger          *zap.Logger
	httpClient      *http.Client
	dialContext     DialContextFunc
}

// NewHealthChecker creates a HealthChecker with configured or safe default dependencies.
func NewHealthChecker(fleetRepo fleet.Repository, discovery DiscoveryPort, logger *zap.Logger, opts ...HealthCheckerOption) *HealthChecker {
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	h := &HealthChecker{
		fleetRepo:   fleetRepo,
		discovery:   discovery,
		logger:      logger,
		httpClient:  &http.Client{Timeout: 2 * time.Second},
		dialContext: dialer.DialContext,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Start initiates the health check loop, executing an immediate check on startup.
func (h *HealthChecker) Start(ctx context.Context, interval time.Duration) {
	// Immediate probe on start
	h.checkAll(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.logger.Info("health checker stopping")
			return
		case <-ticker.C:
			h.checkAll(ctx)
		}
	}
}

func (h *HealthChecker) checkAll(ctx context.Context) {
	clusters, err := h.fleetRepo.ListClusters(ctx)
	if err != nil {
		h.logger.Error("failed to list clusters for health check", zap.Error(err))
		return
	}

	var wg sync.WaitGroup
	for _, c := range clusters {
		wg.Add(1)
		go func(target fleet.Cluster) {
			defer wg.Done()
			h.checkCluster(ctx, target)
		}(c)
	}
	wg.Wait()
}

func (h *HealthChecker) checkCluster(ctx context.Context, c fleet.Cluster) {
	now := time.Now().UTC()

	// 1. Check K8s API connectivity
	k8sAPIReachable := false
	if c.ImportMethod == "kubeconfig" && c.EncryptedToken != "" {
		rawKubeconfig := c.EncryptedToken
		if decrypted, decErr := crypto.Decrypt(c.EncryptedToken); decErr == nil && decrypted != "" {
			rawKubeconfig = decrypted
		}
		if err := h.discovery.ValidateConnectivity(ctx, []byte(rawKubeconfig)); err == nil {
			k8sAPIReachable = true
		} else {
			h.logger.Warn("kubeconfig cluster health check failed", zap.String("id", c.ID), zap.Error(err))
		}
	} else {
		// Non-kubeconfig cluster (e.g. k8snode or direct node agent imports):
		// Probe :6443 on candidate addresses
		candidates := h.resolveAPIServerCandidates(c)
		for _, addr := range candidates {
			if h.probeTCP(ctx, addr) {
				k8sAPIReachable = true
				break
			}
		}
	}

	// 2. Probe node agents (:9100/health)
	liveNodes := 0
	matchedHosts := h.getHostsForCluster(ctx, c)
	if len(matchedHosts) > 0 {
		for _, host := range matchedHosts {
			if h.probeAgentHealth(ctx, host) {
				liveNodes++
			}
		}
	}

	// 3. Determine health_status, status, and nodes count
	var healthStatus, clusterStatus string
	nodesCount := c.Nodes

	if k8sAPIReachable {
		healthStatus = "healthy"
		clusterStatus = "connected"
		if liveNodes > 0 {
			nodesCount = liveNodes
		}
	} else if liveNodes > 0 {
		// API server down/unreachable, but node agents are responding:
		// Honestly report degraded!
		healthStatus = "degraded"
		clusterStatus = "degraded"
		nodesCount = liveNodes
	} else {
		// Neither API server nor node agents reachable
		healthStatus = "unhealthy"
		clusterStatus = "offline"
		nodesCount = 0
	}

	// 4. Persist health and status
	if err := h.fleetRepo.UpdateHealth(ctx, c.ID, healthStatus, now); err != nil {
		h.logger.Error("failed to update cluster health", zap.String("id", c.ID), zap.Error(err))
	}
	if err := h.fleetRepo.UpdateClusterStatus(ctx, c.ID, clusterStatus, c.Version, nodesCount); err != nil {
		h.logger.Error("failed to update cluster status", zap.String("id", c.ID), zap.Error(err))
	}
}

func (h *HealthChecker) getHostsForCluster(ctx context.Context, c fleet.Cluster) []docker.ComputeHost {
	if h.computeHostRepo == nil {
		return nil
	}
	hosts, err := h.computeHostRepo.ListAll(ctx)
	if err != nil || len(hosts) == 0 {
		return nil
	}

	var matched []docker.ComputeHost
	for _, host := range hosts {
		if c.TenantID != "" && host.TenantID != "" && c.TenantID != host.TenantID {
			continue
		}
		if host.Labels != nil {
			if clusterTag, ok := host.Labels["cluster"]; ok {
				if strings.EqualFold(clusterTag, c.Name) || strings.EqualFold(clusterTag, c.ID) {
					matched = append(matched, host)
				}
			}
		}
	}
	if len(matched) > 0 {
		return matched
	}
	// Fallback: If hosts do not have explicit "cluster" tags, associate them with non-kubeconfig / primary cluster
	if c.Name == "k8snode" || c.ID == "k8snode" || c.ImportMethod != "kubeconfig" {
		return hosts
	}
	return nil
}

func (h *HealthChecker) probeAgentHealth(ctx context.Context, host docker.ComputeHost) bool {
	baseURL := formatAgentBaseURL(host.Endpoint)
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return false
	}

	if token := resolveComputeHostToken(host); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := h.httpClient
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func (h *HealthChecker) probeTCP(ctx context.Context, addr string) bool {
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	dialFn := h.dialContext
	if dialFn == nil {
		dialer := &net.Dialer{Timeout: 2 * time.Second}
		dialFn = dialer.DialContext
	}

	conn, err := dialFn(probeCtx, "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (h *HealthChecker) resolveAPIServerCandidates(c fleet.Cluster) []string {
	var candidates []string
	seen := make(map[string]bool)

	addCandidate := func(addr string) {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			return
		}
		if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
			if u, err := url.Parse(addr); err == nil && u.Host != "" {
				host := u.Hostname()
				port := u.Port()
				if port == "" {
					port = "6443"
				}
				addr = net.JoinHostPort(host, port)
			}
		} else {
			host, port, err := net.SplitHostPort(addr)
			if err != nil || host == "" {
				addr = net.JoinHostPort(addr, "6443")
			} else if port == "" {
				addr = net.JoinHostPort(host, "6443")
			}
		}
		if !seen[addr] {
			seen[addr] = true
			candidates = append(candidates, addr)
		}
	}

	if c.APIServerURL != "" {
		addCandidate(c.APIServerURL)
	}

	if c.DiscoveredResources != nil {
		if rawURL, ok := c.DiscoveredResources["api_server_url"].(string); ok && rawURL != "" {
			addCandidate(rawURL)
		}
	}

	if h.computeHostRepo != nil {
		hosts, err := h.computeHostRepo.ListAll(context.Background())
		if err == nil {
			for _, host := range hosts {
				ip := extractHostIP(host.Endpoint)
				if ip != "" && ip != "localhost" && ip != "127.0.0.1" {
					addCandidate(net.JoinHostPort(ip, "6443"))
				}
			}
		}
	}

	return candidates
}

func extractHostIP(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		if u, err := url.Parse(endpoint); err == nil && u.Hostname() != "" {
			return u.Hostname()
		}
	}
	if strings.HasPrefix(endpoint, "tcp://") {
		endpoint = strings.TrimPrefix(endpoint, "tcp://")
	}
	host, _, err := net.SplitHostPort(endpoint)
	if err == nil && host != "" {
		return host
	}
	return endpoint
}

func formatAgentBaseURL(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimRight(endpoint, "/")
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		if strings.HasPrefix(endpoint, "tcp://") {
			endpoint = "http://" + strings.TrimPrefix(endpoint, "tcp://")
		} else {
			endpoint = "http://" + endpoint
		}
	}
	if u, err := url.Parse(endpoint); err == nil && u.Port() == "" && u.Hostname() != "" {
		u.Host = net.JoinHostPort(u.Hostname(), "9100")
		endpoint = u.String()
	}
	return endpoint
}

func resolveComputeHostToken(host docker.ComputeHost) string {
	if host.Labels != nil {
		if token, ok := host.Labels["auth_token"]; ok && strings.TrimSpace(token) != "" {
			return strings.TrimSpace(token)
		}
		if token, ok := host.Labels["token"]; ok && strings.TrimSpace(token) != "" {
			return strings.TrimSpace(token)
		}
	}
	return os.Getenv("AGENT_AUTH_TOKEN")
}
