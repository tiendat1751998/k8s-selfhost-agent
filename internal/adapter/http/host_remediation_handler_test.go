package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/agent"
)

type mockRemediationClient struct {
	healthResp      *agent.NodeK8sHealthResponse
	healthErr       error
	synthResp       *agent.NodeK8sHealthResponse
	synthErr        error
	actionResult    *agent.RemediationActionResult
	actionErr       error
	lastAction      agent.RemediationRequest
	resolvedHost    *docker.ComputeHost
	resolvedToken   string
	resolveErr      error
}

func (m *mockRemediationClient) GetNodeK8sHealth(ctx context.Context, hostEndpoint, authToken string) (*agent.NodeK8sHealthResponse, error) {
	if m.healthErr != nil {
		return nil, m.healthErr
	}
	return m.healthResp, nil
}

func (m *mockRemediationClient) ExecuteRemediation(ctx context.Context, hostEndpoint, authToken string, req agent.RemediationRequest) (*agent.RemediationActionResult, error) {
	m.lastAction = req
	if m.actionErr != nil {
		return nil, m.actionErr
	}
	return m.actionResult, nil
}

func (m *mockRemediationClient) ResolveHostForNode(ctx context.Context, hostRepo docker.ComputeHostRepository, nodeNameOrIP string) (*docker.ComputeHost, string, error) {
	if m.resolveErr != nil {
		return nil, "", m.resolveErr
	}
	if m.resolvedHost != nil {
		return m.resolvedHost, m.resolvedToken, nil
	}
	return &docker.ComputeHost{
		ID:       nodeNameOrIP,
		Name:     nodeNameOrIP,
		Endpoint: "http://10.10.10.10:9100",
	}, "test-token", nil
}

func (m *mockRemediationClient) SynthesizeHealthFromMetrics(ctx context.Context, hostEndpoint, authToken, nodeName string) (*agent.NodeK8sHealthResponse, error) {
	if m.synthErr != nil {
		return nil, m.synthErr
	}
	return m.synthResp, nil
}

func setupRemediationTestRouter(handler *adapthttp.HostRemediationHandler) chi.Router {
	r := chi.NewRouter()
	r.Route("/api/v1/hosts", func(sub chi.Router) {
		handler.RegisterRoutes(sub)
	})
	r.Post("/api/v1/k8s/nodes/{node}/remediate", handler.RemediateNode)
	return r
}

func TestHostRemediationHandler_GetHostK8sHealth_Success(t *testing.T) {
	client := &mockRemediationClient{
		healthResp: &agent.NodeK8sHealthResponse{
			NodeName:          "worker-01",
			KubeletHealthy:    true,
			ContainerdHealthy: true,
			PLEGHealthy:       true,
		},
	}
	handler := adapthttp.NewHostRemediationHandler(nil, client, nil)
	router := setupRemediationTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts/worker-01/k8s-health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	var resp agent.NodeK8sHealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding body failed: %v", err)
	}
	if !resp.KubeletHealthy || resp.NodeName != "worker-01" {
		t.Errorf("unexpected health response: %+v", resp)
	}
}

func TestHostRemediationHandler_GetHostK8sHealth_FallbackToMetricsOn404(t *testing.T) {
	client := &mockRemediationClient{
		healthErr: agent.ErrEndpointNotFound,
		synthResp: &agent.NodeK8sHealthResponse{
			NodeName:          "legacy-worker-01",
			KubeletHealthy:    true,
			ContainerdHealthy: true,
		},
	}
	handler := adapthttp.NewHostRemediationHandler(nil, client, nil)
	router := setupRemediationTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts/legacy-worker-01/k8s-health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK fallback, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	var resp agent.NodeK8sHealthResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.NodeName != "legacy-worker-01" {
		t.Errorf("expected synthesized name legacy-worker-01, got %s", resp.NodeName)
	}
}

func TestHostRemediationHandler_RemediateHost_Success(t *testing.T) {
	client := &mockRemediationClient{
		actionResult: &agent.RemediationActionResult{
			Success:    true,
			Action:     "restart_service",
			Target:     "containerd",
			DurationMs: 350,
			Details:    "containerd restarted cleanly",
		},
	}
	handler := adapthttp.NewHostRemediationHandler(nil, client, nil)
	router := setupRemediationTestRouter(handler)

	payload := `{"action":"restart_service","target":"containerd","reason":"test rescue"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/hosts/worker-01/remediate", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	var res agent.RemediationActionResult
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed decoding result: %v", err)
	}
	if !res.Success || res.Target != "containerd" {
		t.Errorf("unexpected action result: %+v", res)
	}
}

func TestHostRemediationHandler_RemediateHost_InvalidAction(t *testing.T) {
	client := &mockRemediationClient{}
	handler := adapthttp.NewHostRemediationHandler(nil, client, nil)
	router := setupRemediationTestRouter(handler)

	payload := `{"action":"rm -rf /","target":"dangerous","reason":"hack"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/hosts/worker-01/remediate", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for unwhitelisted action, got %d", rec.Code)
	}
}

func TestHostRemediationHandler_RemediateNode_Success(t *testing.T) {
	client := &mockRemediationClient{
		actionResult: &agent.RemediationActionResult{
			Success:    true,
			Action:     "disk_pressure_cleanup",
			BytesFreed: 104857600,
			DurationMs: 1200,
			Details:    "pruned 100MB of dangling images",
		},
	}
	handler := adapthttp.NewHostRemediationHandler(nil, client, nil)
	router := setupRemediationTestRouter(handler)

	payload := `{"action":"disk_pressure_cleanup","reason":"disk pressure event"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/nodes/node-alpha/remediate", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	var res agent.RemediationActionResult
	_ = json.NewDecoder(rec.Body).Decode(&res)
	if !res.Success || res.BytesFreed != 104857600 {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestHostRemediationHandler_RemediateHost_CooldownReturns429(t *testing.T) {
	// Case 1: Error containing 429
	client := &mockRemediationClient{
		actionErr: errors.New("agent /remediate returned HTTP 429: rate limit exceeded, target in cooldown"),
	}
	handler := adapthttp.NewHostRemediationHandler(nil, client, nil)
	router := setupRemediationTestRouter(handler)

	payload := `{"action":"restart_service","target":"kubelet","reason":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/hosts/worker-01/remediate", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests, got %d: %s", rec.Code, rec.Body.String())
	}

	// Case 2: Result with Status == "cooldown"
	client2 := &mockRemediationClient{
		actionResult: &agent.RemediationActionResult{
			Success: false,
			Action:  "restart_service",
			Target:  "kubelet",
			Status:  "cooldown",
			Details: "target in cooldown",
		},
	}
	handler2 := adapthttp.NewHostRemediationHandler(nil, client2, nil)
	router2 := setupRemediationTestRouter(handler2)

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/hosts/worker-01/remediate", bytes.NewBufferString(payload))
	req2.Header.Set("Content-Type", "application/json")
	router2.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for cooldown status, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestHostRemediationHandler_RemediateNode_BadRequestReturns400(t *testing.T) {
	client := &mockRemediationClient{
		actionErr: errors.New("agent /remediate returned HTTP 400: allowlist violation"),
	}
	handler := adapthttp.NewHostRemediationHandler(nil, client, nil)
	router := setupRemediationTestRouter(handler)

	payload := `{"action":"restart_service","target":"kubelet","reason":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/nodes/node-alpha/remediate", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d: %s", rec.Code, rec.Body.String())
	}
}
