package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	mw "github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
)

func TestDockerHandler_RegisterAgent_SuccessAndHeartbeat(t *testing.T) {
	repo := newMockComputeHostRepo()
	handler := NewDockerHandler(nil, repo)

	r := chi.NewRouter()
	r.Post("/agents/register", handler.RegisterAgent)

	payload := RegisterAgentRequest{
		Hostname:         "k8s-worker-01",
		IPAddress:        "10.0.1.50",
		Port:             9100,
		OS:               "linux",
		Arch:             "amd64",
		KernelVersion:    "6.5.0-35-generic",
		Role:             "compute",
		CPUCores:         16,
		MemoryTotalBytes: 68719476736,
		AuthToken:        "secret-agent-token",
	}

	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/agents/register", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("expected status 201 or 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "registered" {
		t.Errorf("expected status 'registered', got %v", resp["status"])
	}
	hostID, ok := resp["host_id"].(string)
	if !ok || hostID == "" {
		t.Fatalf("expected non-empty host_id in response")
	}

	// Verify host in repo
	hosts, _ := repo.ListAll(req.Context())
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host in repo, got %d", len(hosts))
	}
	h := hosts[0]
	if h.Name != "k8s-worker-01" {
		t.Errorf("expected host name 'k8s-worker-01', got %s", h.Name)
	}
	if h.HostType != "agent" {
		t.Errorf("expected host type 'agent', got %s", h.HostType)
	}
	if h.Endpoint != "http://10.0.1.50:9100" {
		t.Errorf("expected endpoint 'http://10.0.1.50:9100', got %s", h.Endpoint)
	}
	if h.Status != "connected" {
		t.Errorf("expected status 'connected', got %s", h.Status)
	}
	if h.Labels["os"] != "linux" || h.Labels["arch"] != "amd64" {
		t.Errorf("expected os/arch in labels, got %+v", h.Labels)
	}
	if h.Labels["auth_token"] != "secret-agent-token" {
		t.Errorf("expected auth_token in labels, got %+v", h.Labels)
	}

	// 2. Heartbeat (second call with same hostname)
	payload.CPUCores = 32
	bodyBytes2, _ := json.Marshal(payload)
	req2 := httptest.NewRequest(http.MethodPost, "/agents/register", bytes.NewReader(bodyBytes2))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()

	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected status 200 on heartbeat update, got %d: %s", rec2.Code, rec2.Body.String())
	}

	hostsAfter, _ := repo.ListAll(req.Context())
	if len(hostsAfter) != 1 {
		t.Fatalf("expected still 1 host (not duplicated), got %d", len(hostsAfter))
	}
	if hostsAfter[0].Labels["cpu_cores"] != "32" {
		t.Errorf("expected updated cpu_cores '32', got %s", hostsAfter[0].Labels["cpu_cores"])
	}
}

func TestDockerHandler_RegisterAgent_Validation(t *testing.T) {
	repo := newMockComputeHostRepo()
	handler := NewDockerHandler(nil, repo)

	req := httptest.NewRequest(http.MethodPost, "/agents/register", bytes.NewReader([]byte(`{"port":9100}`)))
	rec := httptest.NewRecorder()

	handler.RegisterAgent(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for missing hostname and ip, got %d", rec.Code)
	}
}

func TestPlatformRoutes_AgentsRegister_Mounted(t *testing.T) {
	repo := newMockComputeHostRepo()
	dockerHandler := NewDockerHandler(nil, repo)
	platform := &PlatformHandlers{
		Docker: dockerHandler,
	}

	healthHandler := health.NewHandler(5 * time.Second)
	router := NewRouterWithWS(healthHandler, nil, platform)

	server := httptest.NewServer(router)
	defer server.Close()

	token, err := mw.GenerateJWT("admin", "platform_admin", "default-tenant")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	payload := map[string]any{
		"hostname":   "remote-agent-01",
		"ip_address": "192.168.1.100",
		"port":       9100,
	}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/agents/register", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	client := server.Client()
	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("failed to call /api/v1/agents/register: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 or 201, got %d", resp.StatusCode)
	}
}
