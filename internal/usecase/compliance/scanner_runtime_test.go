package compliance_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	usecaseCompliance "github.com/datdt/k8sselfhost/internal/usecase/compliance"
)

func TestDetectRuntimeFindings_AllDetections(t *testing.T) {
	// Set sslmode=disable to trigger RUNTIME-TLS-5432
	_ = os.Setenv("K8S_POSTGRES_SSLMODE", "disable")

	// Create a test server simulating agent with zombies and down container runtime
	agentMux := http.NewServeMux()
	agentMux.HandleFunc("/k8s/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"node_name":          "k8smaster60",
			"zombie_count":       3,
			"containerd_healthy": false,
			"docker_healthy":     false,
		})
	})
	agentMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"hostname": "k8smaster60",
			"memory":   map[string]interface{}{"usage_percent": 30.0},
			"disks":    []map[string]interface{}{{"mount_point": "/", "usage_percent": 40.0}},
		})
	})
	agentServer := httptest.NewServer(agentMux)
	defer agentServer.Close()

	_, openPort, err := net.SplitHostPort(strings.TrimPrefix(agentServer.URL, "http://"))
	if err != nil {
		t.Fatalf("failed to split host port: %v", err)
	}

	scanner := usecaseCompliance.NewScanner(
		usecaseCompliance.WithHTTPClient(agentServer.Client()),
		usecaseCompliance.WithKubeletProbePort(openPort), // Simulate open kubelet port
		usecaseCompliance.WithAPIServerProbePort("59999"), // Unreachable port to simulate closed 6443
	)

	hosts := []usecaseCompliance.ComputeHostRow{
		{
			ID:       "host-master",
			Name:     "k8smaster60",
			Endpoint: agentServer.URL,
			Address:  "127.0.0.1",
			Status:   "connected",
		},
	}

	findings := usecaseCompliance.DetectRuntimeFindings(context.Background(), scanner, hosts)
	if len(findings) == 0 {
		t.Fatal("expected runtime findings, got 0")
	}

	foundMap := make(map[string]usecaseCompliance.AuditFindingRecord)
	for _, f := range findings {
		for _, prefix := range []string{
			"RUNTIME-TLS-5432",
			"RUNTIME-K8S-10250",
			"RUNTIME-CP-6443",
			"RUNTIME-CR-DOWN",
			"RUNTIME-PROC-ZOMBIE",
		} {
			if strings.Contains(f.Description, prefix) {
				foundMap[prefix] = f
			}
		}
	}

	if _, ok := foundMap["RUNTIME-TLS-5432"]; !ok {
		t.Errorf("expected finding RUNTIME-TLS-5432 for sslmode=disable")
	}
	if _, ok := foundMap["RUNTIME-K8S-10250"]; !ok {
		t.Errorf("expected finding RUNTIME-K8S-10250 for open kubelet probe")
	}
	if _, ok := foundMap["RUNTIME-CP-6443"]; !ok {
		t.Errorf("expected finding RUNTIME-CP-6443 for unreachable control plane")
	}
	if _, ok := foundMap["RUNTIME-CR-DOWN"]; !ok {
		t.Errorf("expected finding RUNTIME-CR-DOWN for down container runtime")
	}
	if _, ok := foundMap["RUNTIME-PROC-ZOMBIE"]; !ok {
		t.Errorf("expected finding RUNTIME-PROC-ZOMBIE for zombie processes")
	}
}
