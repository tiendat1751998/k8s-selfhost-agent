package agent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/agent"
)

func TestGetNodeK8sHealth_Success(t *testing.T) {
	expectedToken := "test-secret-token"
	expectedResp := agent.NodeK8sHealthResponse{
		NodeName:          "node-worker-01",
		KubeletHealthy:    true,
		KubeletLatencyMs:  12.5,
		PLEGHealthy:       true,
		ContainerdHealthy: true,
		DockerHealthy:     false,
		CRISocketPath:     "/run/containerd/containerd.sock",
		DiskPressure:      false,
		InodePressure:     false,
		MemoryPressure:    false,
		PIDPressure:       false,
		DStateProcesses:   0,
		ZombieCount:       0,
		ConntrackUsagePct: 15.2,
		DetectedAnomalies: []string{},
		CheckedAt:         "2026-10-16T12:00:00Z",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/k8s/health" {
			t.Errorf("expected path /k8s/health, got %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
			http.Error(w, "invalid method", http.StatusMethodNotAllowed)
			return
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer "+expectedToken {
			t.Errorf("expected Authorization 'Bearer %s', got '%s'", expectedToken, authHeader)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(expectedResp)
	}))
	defer server.Close()

	client := agent.NewAgentRemediationClient(agent.WithRemediationTimeout(3 * time.Second))
	resp, err := client.GetNodeK8sHealth(context.Background(), server.URL, expectedToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.NodeName != expectedResp.NodeName {
		t.Errorf("expected NodeName %s, got %s", expectedResp.NodeName, resp.NodeName)
	}
	if !resp.KubeletHealthy || !resp.ContainerdHealthy || !resp.PLEGHealthy {
		t.Errorf("expected healthy flags to be true")
	}
	if resp.KubeletLatencyMs != 12.5 {
		t.Errorf("expected latency 12.5, got %f", resp.KubeletLatencyMs)
	}
}

func TestExecuteRemediation_Success(t *testing.T) {
	expectedToken := "remedy-token-42"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/remediate" {
			t.Errorf("expected path /remediate, got %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			http.Error(w, "invalid method", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+expectedToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req agent.RemediationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.Action != "restart_service" || req.Target != "containerd" {
			t.Errorf("unexpected payload: %+v", req)
		}

		res := agent.RemediationActionResult{
			Success:    true,
			Action:     req.Action,
			Target:     req.Target,
			DurationMs: 450,
			Details:    "containerd restarted cleanly; active (running)",
			Timestamp:  "2026-10-16T12:01:00Z",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	}))
	defer server.Close()

	client := agent.NewAgentRemediationClient()
	res, err := client.ExecuteRemediation(context.Background(), server.URL, expectedToken, agent.RemediationRequest{
		Action: "restart_service",
		Target: "containerd",
		Reason: "Soft-death recovery",
	})
	if err != nil {
		t.Fatalf("unexpected error executing remediation: %v", err)
	}

	if !res.Success {
		t.Errorf("expected remediation success")
	}
	if res.Target != "containerd" {
		t.Errorf("expected target containerd, got %s", res.Target)
	}
}

func TestResolveHostForNode(t *testing.T) {
	_ = os.Setenv("AGENT_AUTH_TOKEN", "env-secret-token")
	defer os.Unsetenv("AGENT_AUTH_TOKEN")

	repo := &mockComputeHostRepo{
		hosts: []docker.ComputeHost{
			{
				ID:       "host-id-01",
				Name:     "k8s-worker-01",
				Endpoint: "http://10.0.0.101:9100",
				Labels:   map[string]string{"auth_token": "token-for-worker-01"},
			},
			{
				ID:       "host-id-02",
				Name:     "k8s-worker-02",
				Endpoint: "http://10.0.0.102:9100",
				Labels:   map[string]string{"token": "token-for-worker-02"},
			},
			{
				ID:       "host-id-03",
				Name:     "k8s-worker-03",
				Endpoint: "http://10.0.0.103:9100",
			},
		},
	}

	client := agent.NewAgentRemediationClient()

	// 1. Resolve by ID
	h1, token1, err := client.ResolveHostForNode(context.Background(), repo, "host-id-01")
	if err != nil {
		t.Fatalf("failed resolving by ID: %v", err)
	}
	if h1.Name != "k8s-worker-01" || token1 != "token-for-worker-01" {
		t.Errorf("expected k8s-worker-01, got %s / %s", h1.Name, token1)
	}

	// 2. Resolve by Name (case-insensitive)
	h2, token2, err := client.ResolveHostForNode(context.Background(), repo, "K8S-WORKER-02")
	if err != nil {
		t.Fatalf("failed resolving by Name: %v", err)
	}
	if h2.ID != "host-id-02" || token2 != "token-for-worker-02" {
		t.Errorf("expected host-id-02, got %s / %s", h2.ID, token2)
	}

	// 3. Resolve by Endpoint substring & fallback to ENV auth token
	h3, token3, err := client.ResolveHostForNode(context.Background(), repo, "10.0.0.103")
	if err != nil {
		t.Fatalf("failed resolving by IP endpoint: %v", err)
	}
	if h3.ID != "host-id-03" || token3 != "env-secret-token" {
		t.Errorf("expected host-id-03 and env token, got %s / %s", h3.ID, token3)
	}

	// 4. Resolve unlisted valid IP
	h4, token4, err := client.ResolveHostForNode(context.Background(), repo, "192.168.1.50")
	if err != nil {
		t.Fatalf("failed resolving unlisted IP: %v", err)
	}
	if h4.Endpoint != "http://192.168.1.50:9100" || token4 != "env-secret-token" {
		t.Errorf("expected http://192.168.1.50:9100, got %s / %s", h4.Endpoint, token4)
	}

	// 5. Unknown invalid node
	_, _, err = client.ResolveHostForNode(context.Background(), repo, "nonexistent-node-name")
	if err == nil {
		t.Errorf("expected error for nonexistent node name")
	}
}

func TestSynthesizeHealthFromMetrics_On404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/k8s/health":
			http.NotFound(w, r)
		case "/metrics":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"hostname": "legacy-worker",
				"os": "linux",
				"arch": "amd64",
				"uptime_seconds": 12345
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := agent.NewAgentRemediationClient()
	_, err := client.GetNodeK8sHealth(context.Background(), server.URL, "")
	if err == nil {
		t.Fatalf("expected 404 error from /k8s/health")
	}
	if !agent.IsEndpointNotFound(err) {
		t.Errorf("expected IsEndpointNotFound to be true, got error: %v", err)
	}

	synced, err := client.SynthesizeHealthFromMetrics(context.Background(), server.URL, "", "fallback-node")
	if err != nil {
		t.Fatalf("unexpected error synthesizing health: %v", err)
	}
	if synced.NodeName != "legacy-worker" {
		t.Errorf("expected NodeName legacy-worker, got %s", synced.NodeName)
	}
	if !synced.KubeletHealthy || !synced.ContainerdHealthy {
		t.Errorf("expected synthesized kubelet and containerd to be healthy")
	}
}

func TestGetRemediationHistory_Success(t *testing.T) {
	expectedToken := "history-token-99"
	expectedHistory := []agent.RemediationResultDTO{
		{
			Success:    true,
			Action:     "restart_container",
			Target:     "coredns-test",
			Status:     "success",
			DurationMs: 120,
			Details:    "Container coredns-test restarted cleanly",
			Timestamp:  "2026-10-16T12:00:00Z",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/remediate/history" {
			t.Errorf("expected path /remediate/history, got %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
			http.Error(w, "invalid method", http.StatusMethodNotAllowed)
			return
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer "+expectedToken {
			t.Errorf("expected Authorization 'Bearer %s', got '%s'", expectedToken, authHeader)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(expectedHistory)
	}))
	defer server.Close()

	client := agent.NewAgentRemediationClient()
	history, err := client.GetRemediationHistory(context.Background(), server.URL, expectedToken)
	if err != nil {
		t.Fatalf("unexpected error fetching remediation history: %v", err)
	}

	if len(history) != 1 {
		t.Fatalf("expected 1 history item, got %d", len(history))
	}
	if history[0].Action != "restart_container" || history[0].Target != "coredns-test" {
		t.Errorf("unexpected history item: %+v", history[0])
	}
	if !history[0].Success {
		t.Errorf("expected history item success to be true")
	}
}

func TestGetNodeK8sHealth_WithUnhealthyContainers(t *testing.T) {
	expectedResp := agent.NodeK8sHealthResponse{
		NodeName:       "worker-02",
		KubeletHealthy: true,
		UnhealthyContainers: []agent.UnhealthyContainerDTO{
			{
				ID:           "cnt-123",
				Name:         "kube-proxy",
				Image:        "registry.k8s.io/kube-proxy:v1.30.0",
				State:        "waiting",
				Status:       "CrashLoopBackOff",
				RestartCount: 14,
			},
		},
		CheckedAt: "2026-10-16T12:00:00Z",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(expectedResp)
	}))
	defer server.Close()

	client := agent.NewAgentRemediationClient()
	resp, err := client.GetNodeK8sHealth(context.Background(), server.URL, "dummy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.UnhealthyContainers) != 1 {
		t.Fatalf("expected 1 unhealthy container, got %d", len(resp.UnhealthyContainers))
	}
	c := resp.UnhealthyContainers[0]
	if c.ID != "cnt-123" || c.Name != "kube-proxy" || c.RestartCount != 14 {
		t.Errorf("unexpected container fields: %+v", c)
	}
}
