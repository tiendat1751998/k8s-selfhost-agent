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
	"go.uber.org/zap"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domain "github.com/datdt/k8sselfhost/internal/domain/netguard"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/netguard"
)

type mockHTTPProbeRunner struct {
	maxPassingSize int
}

func (m *mockHTTPProbeRunner) Probe(_ context.Context, _ string, sizeBytes int) (bool, time.Duration, error) {
	if sizeBytes <= m.maxPassingSize {
		return true, 15 * time.Millisecond, nil
	}
	return false, 2 * time.Second, nil
}

func setupNetGuardTestRouter(maxPassing int) (*chi.Mux, *usecase.TunnelManager) {
	runner := &mockHTTPProbeRunner{maxPassingSize: maxPassing}
	mockSysctl := map[string]int{
		"net.ipv4.conf.all.rp_filter":  1,
		"net.ipv4.conf.wg0.rp_filter":  1,
		"net.ipv4.conf.eth0.rp_filter": 2,
	}

	mgr := usecase.NewTunnelManager(
		usecase.WithProbeRunner(runner),
		usecase.WithSysctlMap(mockSysctl),
		usecase.WithSweepRange(1400, 1500, 4),
		usecase.WithLogger(zap.NewNop()),
	)

	handler := adapthttp.NewNetGuardHandler(mgr, zap.NewNop())

	r := chi.NewRouter()
	r.Route("/api/v1/network", handler.RegisterRoutes)

	return r, mgr
}

func TestNetGuardHandler_GetTunnelHealth(t *testing.T) {
	r, mgr := setupNetGuardTestRouter(1500)

	mgr.RegisterTunnelStatus(domain.HybridTunnelStatus{
		InterfaceName: "wg0",
		TunnelType:    "wireguard",
		TransportMode: domain.TransportP2PDirect,
		ConfiguredMTU: 1420,
		DetectedPMTU:  1420,
		MSSClampingOn: true,
		RPFilterMode:  2,
		RTTLatencyMs:  12.0,
		PacketLossPct: 0.0,
		LastTestedAt:  time.Now().UTC(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/network/tunnels/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Status  string                      `json:"status"`
		Tunnels []domain.HybridTunnelStatus `json:"tunnels"`
		Count   int                         `json:"count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}

	if resp.Count != 1 || resp.Tunnels[0].InterfaceName != "wg0" {
		t.Errorf("expected 1 tunnel wg0, got count=%d, tunnels=%+v", resp.Count, resp.Tunnels)
	}
}

func TestNetGuardHandler_ProbePMTU_Success(t *testing.T) {
	r, _ := setupNetGuardTestRouter(1420) // 1420 passes, 1500 drops

	body := []byte(`{"target_ip":"10.100.0.1","interface":"wg0"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/network/tunnels/pmtu-probe", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		TargetIP           string `json:"target_ip"`
		DetectedPMTU       int    `json:"detected_pmtu"`
		Iterations         int    `json:"iterations"`
		BlackHoleDetected  bool   `json:"blackhole_detected"`
		RecommendedMSSIPv4 int    `json:"recommended_mss_ipv4"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.DetectedPMTU != 1420 {
		t.Errorf("expected PMTU 1420, got %d", resp.DetectedPMTU)
	}
	if !resp.BlackHoleDetected {
		t.Errorf("expected blackhole_detected=true")
	}
	if resp.RecommendedMSSIPv4 != 1380 {
		t.Errorf("expected recommended MSS 1380, got %d", resp.RecommendedMSSIPv4)
	}
}

func TestNetGuardHandler_ProbePMTU_BlackHole502(t *testing.T) {
	r, _ := setupNetGuardTestRouter(1200) // Even 1400 fails -> blackhole

	body := []byte(`{"target_ip":"10.100.0.1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/network/tunnels/pmtu-probe", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 Bad Gateway for MTU black hole, got %d: %s", rec.Code, rec.Body.String())
	}

	var problem adapthttp.ProblemDetails
	if err := json.NewDecoder(rec.Body).Decode(&problem); err != nil {
		t.Fatalf("failed to decode RFC 7807 problem details: %v", err)
	}

	if problem.Code != domain.CodeTunnelMTUBlackHole {
		t.Errorf("expected code %s, got %s", domain.CodeTunnelMTUBlackHole, problem.Code)
	}
}

func TestNetGuardHandler_ApplyMSSClamping(t *testing.T) {
	r, _ := setupNetGuardTestRouter(1500)

	body := []byte(`{"pmtu":1420,"ipv6":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/network/tunnels/wg0/mss-clamp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Status    string   `json:"status"`
		Interface string   `json:"interface"`
		PMTU      int      `json:"pmtu"`
		TargetMSS int      `json:"target_mss"`
		Rules     []string `json:"rules"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "applied" || resp.Interface != "wg0" || resp.TargetMSS != 1380 {
		t.Errorf("unexpected response: %+v", resp)
	}
	if len(resp.Rules) < 2 {
		t.Errorf("expected rules in response, got %+v", resp.Rules)
	}
}

func TestNetGuardHandler_AuditRPFilter(t *testing.T) {
	r, _ := setupNetGuardTestRouter(1500)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/network/routing/rp-filter", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp usecase.RPFilterAuditResult
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.StrictMultiHomedInterfaces) == 0 {
		t.Errorf("expected strict multi-homed interfaces detected")
	}
}
