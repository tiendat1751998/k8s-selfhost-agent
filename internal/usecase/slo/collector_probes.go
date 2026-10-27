package slo

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/observability"
	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

// resolveHostAgentToken resolves Bearer auth token for agent requests.
// Precedence: host.Labels["auth_token"] > host.Labels["token"] > AGENT_AUTH_TOKEN env.
func resolveHostAgentToken(host docker.ComputeHost) string {
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

// formatAgentBaseURL normalizes host endpoint into a base HTTP URL (e.g. http://10.10.10.60:9100).
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

// extractHostIP extracts the hostname or IPv4 address from host endpoint.
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

// getK8sHosts returns registered hosts associated with Kubernetes cluster roles or addresses.
func (c *SLOCollector) getK8sHosts(hosts []docker.ComputeHost) []docker.ComputeHost {
	var k8sHosts []docker.ComputeHost
	for _, h := range hosts {
		role := strings.ToLower(h.Labels["host_role"])
		if h.HostType == "k8s" || role == "k8s" || role == "control-plane" || role == "master" {
			k8sHosts = append(k8sHosts, h)
			continue
		}
		ip := extractHostIP(h.Endpoint)
		if ip == "10.10.10.60" || ip == "10.10.10.80" {
			k8sHosts = append(k8sHosts, h)
		}
	}
	if len(k8sHosts) > 0 {
		return k8sHosts
	}
	if len(hosts) > 0 {
		return hosts
	}
	// Fallback to configured KUBECONFIG host or standard default node
	defaultIP := "10.10.10.60"
	return []docker.ComputeHost{{Name: "k8s-node", Endpoint: defaultIP}}
}

// collectFleetSLOs probes registered SLO definition targets and records point-in-time health samples.
func (c *SLOCollector) collectFleetSLOs(ctx context.Context) {
	if c.repo == nil {
		return
	}

	evalCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	defs, err := c.getSLODefinitions(evalCtx)
	if err != nil || len(defs) == 0 {
		return
	}

	var hosts []docker.ComputeHost
	if c.hostRepo != nil {
		if hList, hErr := c.hostRepo.ListAll(evalCtx); hErr == nil {
			hosts = hList
		} else {
			c.logger.Debug("Failed to list compute hosts for SLO collection", zap.Error(hErr))
		}
	}

	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}

	for _, d := range defs {
		switch d.Service {
		case "k8s-agent-fleet":
			c.probeK8sAgentFleet(evalCtx, client, hosts)
		case "postgres-primary":
			c.probePostgresPrimary(evalCtx)
		case "workload-runtime":
			c.probeWorkloadRuntime(evalCtx, client, hosts)
		case "kube-apiserver":
			c.probeTCPPort(evalCtx, hosts, 6443, "kube-apiserver")
		case "coredns":
			c.probeTCPPort(evalCtx, hosts, 53, "coredns")
		case "traefik-ingress":
			c.probeTraefikIngress(evalCtx, client, hosts)
		default:
			c.probeCustomSLO(evalCtx, client, d)
		}
	}
}

func (c *SLOCollector) probeK8sAgentFleet(ctx context.Context, client *http.Client, hosts []docker.ComputeHost) {
	desired := len(hosts)
	if desired == 0 {
		desired = 1
		c.recordHealth(ctx, "k8s-agent-fleet", desired, 0, false, nil)
		return
	}

	start := time.Now()
	running := 0
	for _, h := range hosts {
		baseURL := formatAgentBaseURL(h.Endpoint)
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, baseURL+"/health", nil)
		if err == nil {
			if token := resolveHostAgentToken(h); token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			resp, rErr := client.Do(req)
			if rErr == nil {
				if resp.StatusCode == http.StatusOK {
					running++
				}
				resp.Body.Close()
			}
		}
		cancel()
	}

	latencyMs := int(time.Since(start).Milliseconds())
	isHealthy := (len(hosts) > 0 && running == len(hosts))
	c.recordHealth(ctx, "k8s-agent-fleet", desired, running, isHealthy, &latencyMs)
}

func (c *SLOCollector) probePostgresPrimary(ctx context.Context) {
	desired := 1
	if c.dbPingFn == nil {
		c.recordHealth(ctx, "postgres-primary", desired, 0, false, nil)
		return
	}

	start := time.Now()
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	err := c.dbPingFn(probeCtx)
	latencyMs := int(time.Since(start).Milliseconds())
	isHealthy := (err == nil)
	running := 0
	if isHealthy {
		running = 1
	}
	c.recordHealth(ctx, "postgres-primary", desired, running, isHealthy, &latencyMs)
}

func (c *SLOCollector) probeWorkloadRuntime(ctx context.Context, client *http.Client, hosts []docker.ComputeHost) {
	desired := len(hosts)
	if desired == 0 {
		desired = 1
		c.recordHealth(ctx, "workload-runtime", desired, 0, false, nil)
		return
	}

	start := time.Now()
	running := 0
	for _, h := range hosts {
		baseURL := formatAgentBaseURL(h.Endpoint)
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, baseURL+"/k8s/health", nil)
		if err == nil {
			if token := resolveHostAgentToken(h); token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			resp, rErr := client.Do(req)
			if rErr == nil {
				if resp.StatusCode == http.StatusOK {
					var payload struct {
						ContainerdHealthy bool `json:"containerd_healthy"`
					}
					if dErr := json.NewDecoder(resp.Body).Decode(&payload); dErr == nil && payload.ContainerdHealthy {
						running++
					}
				}
				resp.Body.Close()
			}
		}
		cancel()
	}

	latencyMs := int(time.Since(start).Milliseconds())
	isHealthy := (len(hosts) > 0 && running == len(hosts))
	c.recordHealth(ctx, "workload-runtime", desired, running, isHealthy, &latencyMs)
}

func (c *SLOCollector) probeTCPPort(ctx context.Context, hosts []docker.ComputeHost, port int, serviceName string) {
	targetHosts := c.getK8sHosts(hosts)
	desired := len(targetHosts)
	if desired == 0 {
		desired = 1
	}

	start := time.Now()
	running := 0
	for _, h := range targetHosts {
		ip := extractHostIP(h.Endpoint)
		addr := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
		d := net.Dialer{Timeout: 2 * time.Second}
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		conn, err := d.DialContext(probeCtx, "tcp", addr)
		cancel()
		if err == nil {
			conn.Close()
			running++
		}
	}

	latencyMs := int(time.Since(start).Milliseconds())
	isHealthy := (running > 0 && running == desired)
	c.recordHealth(ctx, serviceName, desired, running, isHealthy, &latencyMs)
}

func (c *SLOCollector) probeTraefikIngress(ctx context.Context, client *http.Client, hosts []docker.ComputeHost) {
	targetHosts := c.getK8sHosts(hosts)
	desired := len(targetHosts)
	if desired == 0 {
		desired = 1
	}

	start := time.Now()
	running := 0
	for _, h := range targetHosts {
		ip := extractHostIP(h.Endpoint)
		addr := net.JoinHostPort(ip, "80")
		d := net.Dialer{Timeout: 2 * time.Second}
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		conn, err := d.DialContext(probeCtx, "tcp", addr)
		cancel()
		if err == nil {
			conn.Close()
			running++
		}
	}

	latencyMs := int(time.Since(start).Milliseconds())
	isHealthy := (running > 0 && running == desired)
	c.recordHealth(ctx, "traefik-ingress", desired, running, isHealthy, &latencyMs)
}

func (c *SLOCollector) probeCustomSLO(ctx context.Context, client *http.Client, d observability.SLODefinition) {
	q := strings.TrimSpace(d.Query)
	if strings.HasPrefix(q, "http://") || strings.HasPrefix(q, "https://") {
		start := time.Now()
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, q, nil)
		if err == nil {
			resp, rErr := client.Do(req)
			if rErr == nil {
				defer resp.Body.Close()
				isHealthy := resp.StatusCode >= 200 && resp.StatusCode < 400
				running := 0
				if isHealthy {
					running = 1
				}
				latencyMs := int(time.Since(start).Milliseconds())
				c.recordHealth(ctx, d.Service, 1, running, isHealthy, &latencyMs)
				return
			}
		}
		c.recordHealth(ctx, d.Service, 1, 0, false, nil)
		return
	}

	if strings.HasPrefix(q, "tcp://") {
		addr := strings.TrimPrefix(q, "tcp://")
		start := time.Now()
		dDial := net.Dialer{Timeout: 2 * time.Second}
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		conn, err := dDial.DialContext(probeCtx, "tcp", addr)
		if err == nil {
			conn.Close()
			latencyMs := int(time.Since(start).Milliseconds())
			c.recordHealth(ctx, d.Service, 1, 1, true, &latencyMs)
			return
		}
		c.recordHealth(ctx, d.Service, 1, 0, false, nil)
		return
	}

	// For non-network custom queries, do NOT record fake samples (stays unmeasured)
}

func (c *SLOCollector) recordHealth(ctx context.Context, service string, desired, running int, isHealthy bool, latencyMs *int) {
	if c.repo == nil {
		return
	}
	if err := c.repo.RecordHealthSample(ctx, c.tenantID, service, desired, running, isHealthy, latencyMs); err != nil {
		c.logger.Debug("Failed to record SLO health sample", zap.String("service", service), zap.Error(err))
	}
}
