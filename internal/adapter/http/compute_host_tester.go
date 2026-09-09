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
	"time"

	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

func measureLatency(start time.Time) int64 {
	latencyMs := time.Since(start).Milliseconds()
	if latencyMs <= 0 {
		return 1
	}
	return latencyMs
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
	var (
		latencyMs int64
		testErr   error
		msg       string
		agentInfo map[string]interface{}
	)

	switch host.HostType {
	case "agent":
		latencyMs, agentInfo, testErr = testAgentHostConnection(r.Context(), host)
		msg = "successfully connected to agent host"
	case "docker":
		latencyMs, testErr = testDockerHostConnection(r.Context(), host)
		msg = "successfully connected to docker host"
	case "k8s":
		latencyMs, testErr = testK8sHostConnection(r.Context(), host)
		msg = "successfully connected to kubernetes api"
	case "prometheus":
		latencyMs, testErr = testPrometheusHostConnection(r.Context(), host)
		msg = "successfully connected to prometheus endpoint"
	case "git":
		latencyMs, testErr = testGitHostConnection(r.Context(), host)
		msg = "successfully connected to git repository endpoint"
	case "database":
		latencyMs, testErr = testDatabaseHostConnection(r.Context(), host)
		msg = "successfully connected to database endpoint"
	case "custom":
		fallthrough
	default:
		latencyMs, testErr = testCustomHostConnection(r.Context(), host)
		msg = "successfully connected to endpoint"
	}

	if testErr != nil {
		_ = h.hostRepo.UpdateStatus(r.Context(), host.ID, "error", now)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "error",
			"message":    testErr.Error(),
			"latency_ms": 0,
		})
		return
	}

	_ = h.hostRepo.UpdateStatus(r.Context(), host.ID, "connected", now)
	resp := map[string]interface{}{
		"status":     "connected",
		"message":    msg,
		"latency_ms": latencyMs,
	}
	if len(agentInfo) > 0 {
		resp["agent_info"] = agentInfo
	}
	writeJSON(w, http.StatusOK, resp)
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

	if u, err := url.Parse(endpoint); err == nil && u.Port() == "" && u.Hostname() != "" {
		u.Host = net.JoinHostPort(u.Hostname(), "9100")
		endpoint = u.String()
	}

	var transport *http.Transport
	if host.TLSEnabled && host.TLSCA != "" && host.TLSCert != "" && host.TLSKey != "" {
		tlsConfig, err := configureDockerTLS(host.TLSCA, host.TLSCert, host.TLSKey)
		if err != nil {
			return 0, nil, fmt.Errorf("tls configuration error: %w", err)
		}
		transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	if transport != nil {
		client.Transport = transport
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

	latencyMs := measureLatency(start)

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
					osDistro, kernelVer := metricsData.OSDistro, metricsData.KernelVersion
					if metricsData.OS != "" {
						titleOS := strings.ToUpper(metricsData.OS[:1]) + strings.ToLower(metricsData.OS[1:])
						if osDistro == "" { osDistro = titleOS }
						if kernelVer == "" { kernelVer = titleOS }
					}
					if metricsData.Hostname != "" { agentInfo["hostname"] = metricsData.Hostname }
					if metricsData.OS != "" { agentInfo["os"] = metricsData.OS }
					if metricsData.Arch != "" { agentInfo["arch"] = metricsData.Arch }
					if osDistro != "" { agentInfo["os_distro"] = osDistro }
					if kernelVer != "" { agentInfo["kernel_version"] = kernelVer }
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
	latencyMs := measureLatency(start)
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
				latencyMs := measureLatency(start)
				return latencyMs, nil
			}
		}
		return 0, fmt.Errorf("connecting to kubernetes endpoint: %w", err)
	}
	defer resp.Body.Close()

	latencyMs := measureLatency(start)

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
				latencyMs := measureLatency(start)
				return latencyMs, nil
			}
		}
		return 0, fmt.Errorf("connecting to prometheus endpoint: %w", err)
	}
	defer resp.Body.Close()

	latencyMs := measureLatency(start)

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

	latencyMs := measureLatency(start)

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

	latencyMs := measureLatency(start)

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

		latencyMs := measureLatency(start)
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

	latencyMs := measureLatency(start)
	return latencyMs, nil
}

