package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domain "github.com/datdt/k8sselfhost/internal/domain/servicemesh"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/servicemesh"
)

type mockMeshEngine struct {
	auditSDSFn      func(ctx context.Context, socketPath string) (*domain.SDSSocketStatus, error)
	recoverSDSFn    func(ctx context.Context, socketPath string) error
	enforceEgressFn func(ctx context.Context, namespace string, deps []string) error
	auditTunnelFn   func(ctx context.Context, targetIP string, payloadSize int) (*domain.TunnelMTUProbeResult, error)
}

func (m *mockMeshEngine) AuditSDSSocket(ctx context.Context, socketPath string) (*domain.SDSSocketStatus, error) {
	if m.auditSDSFn != nil {
		return m.auditSDSFn(ctx, socketPath)
	}
	return &domain.SDSSocketStatus{
		SocketPath:    socketPath,
		IsAlive:       true,
		CertRemaining: 10 * time.Hour,
		LastRotatedAt: time.Now().Add(-1 * time.Hour),
	}, nil
}

func (m *mockMeshEngine) RecoverSDSSocket(ctx context.Context, socketPath string) error {
	if m.recoverSDSFn != nil {
		return m.recoverSDSFn(ctx, socketPath)
	}
	return nil
}

func (m *mockMeshEngine) EnforceEgressScope(ctx context.Context, namespace string, deps []string) error {
	if m.enforceEgressFn != nil {
		return m.enforceEgressFn(ctx, namespace, deps)
	}
	return nil
}

func (m *mockMeshEngine) AuditTunnelMTUBlackHole(ctx context.Context, targetIP string, payloadSize int) (*domain.TunnelMTUProbeResult, error) {
	if m.auditTunnelFn != nil {
		return m.auditTunnelFn(ctx, targetIP, payloadSize)
	}
	return &domain.TunnelMTUProbeResult{
		TunnelInterface: "vxlan0",
		DetectedPMTU:    1450,
		ConfiguredMTU:   1500,
		BlackHoleFound:  true,
		ActionTaken:     "Clamped TCP MSS to 1410",
	}, nil
}

func setupTestServiceMeshRouter(engine domain.ServiceMeshResilienceEngine) *chi.Mux {
	r := chi.NewRouter()
	handler := adapthttp.NewServiceMeshHandler(engine)
	r.Route("/api/v1/mesh", handler.RegisterRoutes)
	return r
}

func TestServiceMeshHandler_GetSDSSocketStatus_Success(t *testing.T) {
	engine := &mockMeshEngine{}
	router := setupTestServiceMeshRouter(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mesh/sds/socket-status", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var status domain.SDSSocketStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !status.IsAlive {
		t.Errorf("expected socket to be alive")
	}
}

func TestServiceMeshHandler_GetSDSSocketStatus_Unresponsive(t *testing.T) {
	engine := &mockMeshEngine{
		auditSDSFn: func(ctx context.Context, socketPath string) (*domain.SDSSocketStatus, error) {
			return nil, domain.ErrSDSSocketUnresponsive
		},
	}
	router := setupTestServiceMeshRouter(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mesh/sds/socket-status", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}

	var prob adapthttp.ProblemDetails
	if err := json.Unmarshal(rr.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to parse problem details: %v", err)
	}
	if prob.Code != domain.CodeSDSSocketUnresponsive {
		t.Errorf("expected code %s, got %s", domain.CodeSDSSocketUnresponsive, prob.Code)
	}
}

func TestServiceMeshHandler_RecoverSDSSocket_Success(t *testing.T) {
	engine := &mockMeshEngine{}
	router := setupTestServiceMeshRouter(engine)

	body := []byte(`{"socket_path": "/var/run/secrets/workload-spiffe-uds/socket"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mesh/sds/recover", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestServiceMeshHandler_RecoverSDSSocket_InvalidJSON(t *testing.T) {
	engine := &mockMeshEngine{}
	router := setupTestServiceMeshRouter(engine)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mesh/sds/recover", bytes.NewReader([]byte("invalid json")))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for invalid json, got %d", rr.Code)
	}
	var prob adapthttp.ProblemDetails
	if err := json.Unmarshal(rr.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to parse problem details: %v", err)
	}
	if prob.Code != "VALIDATION_ERROR" {
		t.Errorf("expected code VALIDATION_ERROR, got %s", prob.Code)
	}
}

func TestServiceMeshHandler_EnforceEgressScope_Success(t *testing.T) {
	engine := &mockMeshEngine{}
	router := setupTestServiceMeshRouter(engine)

	payload := map[string]interface{}{
		"namespace":    "production",
		"dependencies": []string{"cart-service", "auth-service"},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mesh/egress/enforce-scope", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestServiceMeshHandler_EnforceEgressScope_ValidationError(t *testing.T) {
	engine := &mockMeshEngine{}
	router := setupTestServiceMeshRouter(engine)

	payload := map[string]interface{}{
		"namespace": "",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mesh/egress/enforce-scope", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for empty namespace, got %d", rr.Code)
	}
	var prob adapthttp.ProblemDetails
	_ = json.Unmarshal(rr.Body.Bytes(), &prob)
	if prob.Code != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %s", prob.Code)
	}
}

func TestServiceMeshHandler_EnforceEgressScope_MemoryExhausted_Returns409(t *testing.T) {
	engine := &mockMeshEngine{
		enforceEgressFn: func(ctx context.Context, namespace string, deps []string) error {
			return domain.ErrEnvoyEgressMemoryExhausted
		},
	}
	router := setupTestServiceMeshRouter(engine)

	payload := map[string]interface{}{
		"namespace":    "heavy-namespace",
		"dependencies": []string{"dep1"},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mesh/egress/enforce-scope", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected status 409 Conflict, got %d", rr.Code)
	}
	var prob adapthttp.ProblemDetails
	if err := json.Unmarshal(rr.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to parse problem details: %v", err)
	}
	if prob.Code != domain.CodeEnvoyEgressMemoryExhausted {
		t.Errorf("expected code %s, got %s", domain.CodeEnvoyEgressMemoryExhausted, prob.Code)
	}
}

func TestServiceMeshHandler_AuditTunnelMTU_Success(t *testing.T) {
	engine := &mockMeshEngine{}
	router := setupTestServiceMeshRouter(engine)

	payload := map[string]interface{}{
		"target_ip":         "10.244.3.4",
		"test_payload_size": 1500,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mesh/tunnel/mtu-audit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var res domain.TunnelMTUProbeResult
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !res.BlackHoleFound {
		t.Errorf("expected black hole to be detected")
	}
}

func TestServiceMeshHandler_AuditTunnelMTU_ValidationError(t *testing.T) {
	engine := &mockMeshEngine{}
	router := setupTestServiceMeshRouter(engine)

	payload := map[string]interface{}{
		"target_ip": "",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mesh/tunnel/mtu-audit", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for empty target IP, got %d", rr.Code)
	}
}

func TestServiceMeshHandler_AuditTunnelMTU_BlackHoleError_Returns502(t *testing.T) {
	engine := &mockMeshEngine{
		auditTunnelFn: func(ctx context.Context, targetIP string, payloadSize int) (*domain.TunnelMTUProbeResult, error) {
			return nil, domain.ErrTunnelPMTUBlackHole
		},
	}
	router := setupTestServiceMeshRouter(engine)

	payload := map[string]interface{}{
		"target_ip":         "10.244.3.4",
		"test_payload_size": 1500,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mesh/tunnel/mtu-audit", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502 Bad Gateway, got %d", rr.Code)
	}
	var prob adapthttp.ProblemDetails
	if err := json.Unmarshal(rr.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to parse problem details: %v", err)
	}
	if prob.Code != domain.CodeTunnelPMTUBlackHole {
		t.Errorf("expected code %s, got %s", domain.CodeTunnelPMTUBlackHole, prob.Code)
	}
}

func TestServiceMeshHandler_WithRealUsecaseManager(t *testing.T) {
	mgr := usecase.NewMeshManager()
	router := setupTestServiceMeshRouter(mgr)

	// Test real manager fallback flow via API
	payload := map[string]interface{}{
		"namespace":             "e2e-namespace",
		"dependencies":          []string{"dep-a", "dep-b"},
		"fallback_to_namespace": true,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mesh/egress/enforce-scope", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 from real usecase, got %d: %s", rr.Code, rr.Body.String())
	}
}
