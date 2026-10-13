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
	domain "github.com/datdt/k8sselfhost/internal/domain/transitnet"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/transitnet"
)

type mockTransitBGPAuditor struct {
	primary string
	ret     string
	rtt     time.Duration
}

func (m *mockTransitBGPAuditor) GetRoutePaths(_ context.Context, _ string) (usecase.RouteAuditData, error) {
	return usecase.RouteAuditData{
		PrimaryPath:          m.primary,
		ReturnPath:           m.ret,
		RTTLatency:           m.rtt,
		DirectConnectHealthy: true,
	}, nil
}

func (m *mockTransitBGPAuditor) ApplyBGPPrepending(_ context.Context, _ string, _ int) error {
	return nil
}

type mockTransitVPCProber struct {
	sourceMTU int
	targetMTU int
}

func (m *mockTransitVPCProber) ProbePath(_ context.Context, _ string, testBytes int) (usecase.VPCProbeData, error) {
	effective := m.sourceMTU
	if m.targetMTU < effective {
		effective = m.targetMTU
	}
	pass := testBytes <= effective
	return usecase.VPCProbeData{
		SourceVPCID:  "vpc-1",
		TargetVPCID:  "vpc-2",
		SourceMTU:    m.sourceMTU,
		TargetMTU:    m.targetMTU,
		ProbeSuccess: pass,
	}, nil
}

type mockTransitNLBProvider struct {
	crossZoneEnabled bool
	shares           map[string]float64
}

func (m *mockTransitNLBProvider) GetNLBStatus(_ context.Context, _ string) (bool, map[string]float64, error) {
	return m.crossZoneEnabled, m.shares, nil
}

func (m *mockTransitNLBProvider) SetCrossZoneBalancing(_ context.Context, _ string, enabled bool) error {
	m.crossZoneEnabled = enabled
	return nil
}

type mockTransitCoreDNSAdapter struct {
	corefile string
}

func (m *mockTransitCoreDNSAdapter) GetCorefile(_ context.Context) (string, error) {
	return m.corefile, nil
}

func (m *mockTransitCoreDNSAdapter) ApplyCorefile(_ context.Context, c string) error {
	m.corefile = c
	return nil
}

func setupTransitNetRouter(
	bgp *mockTransitBGPAuditor,
	vpc *mockTransitVPCProber,
	nlb *mockTransitNLBProvider,
	dns *mockTransitCoreDNSAdapter,
) *chi.Mux {
	mgr := usecase.NewRoutingManager(
		usecase.WithBGPAuditor(bgp),
		usecase.WithVPCProber(vpc),
		usecase.WithNLBProvider(nlb),
		usecase.WithCoreDNSAdapter(dns),
		usecase.WithLogger(zap.NewNop()),
	)

	handler := adapthttp.NewTransitNetHandler(mgr, zap.NewNop())
	r := chi.NewRouter()
	handler.MountAPIRoutes(r)
	return r
}

func TestTransitNetHandler_AuditRoutingSymmetry(t *testing.T) {
	t.Run("Symmetric path returns 200 OK", func(t *testing.T) {
		r := setupTransitNetRouter(
			&mockTransitBGPAuditor{primary: "dx-con-1", ret: "dx-con-1", rtt: 5 * time.Millisecond},
			nil, nil, nil,
		)

		body := []byte(`{"target_endpoint":"10.0.1.1"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/cloud/network/audit-symmetry", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Asymmetric path returns 502 Bad Gateway RFC 7807", func(t *testing.T) {
		r := setupTransitNetRouter(
			&mockTransitBGPAuditor{primary: "dx-con-1", ret: "vpn-ipsec-1", rtt: 50 * time.Millisecond},
			nil, nil, nil,
		)

		body := []byte(`{"target_endpoint":"10.0.1.1"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/cloud/network/audit-symmetry", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502 Bad Gateway, got %d: %s", rec.Code, rec.Body.String())
		}

		var prob adapthttp.ProblemDetails
		if err := json.NewDecoder(rec.Body).Decode(&prob); err != nil {
			t.Fatalf("failed to decode RFC 7807 problem details: %v", err)
		}
		if prob.Code != domain.CodeBGPAsymmetricPath {
			t.Errorf("expected error code %s, got %s", domain.CodeBGPAsymmetricPath, prob.Code)
		}
	})

	t.Run("Missing target endpoint returns 400 Bad Request", func(t *testing.T) {
		r := setupTransitNetRouter(&mockTransitBGPAuditor{}, nil, nil, nil)
		body := []byte(`{"target_endpoint":""}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/cloud/network/audit-symmetry", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})
}

func TestTransitNetHandler_ValidateVPCPathMTU(t *testing.T) {
	t.Run("Valid MTU returns 200 OK", func(t *testing.T) {
		r := setupTransitNetRouter(
			nil,
			&mockTransitVPCProber{sourceMTU: 1500, targetMTU: 1500},
			nil, nil,
		)

		body := []byte(`{"target_host":"172.16.0.5","test_bytes":1400}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/cloud/network/validate-path-mtu", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Black hole returns 502 Bad Gateway RFC 7807", func(t *testing.T) {
		r := setupTransitNetRouter(
			nil,
			&mockTransitVPCProber{sourceMTU: 9001, targetMTU: 1500},
			nil, nil,
		)

		body := []byte(`{"target_host":"172.16.0.5","test_bytes":9001}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/cloud/network/validate-path-mtu", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502 Bad Gateway, got %d: %s", rec.Code, rec.Body.String())
		}

		var prob adapthttp.ProblemDetails
		if err := json.NewDecoder(rec.Body).Decode(&prob); err != nil {
			t.Fatalf("failed to decode RFC 7807 problem details: %v", err)
		}
		if prob.Code != domain.CodeVPCPathMTUBlackHole {
			t.Errorf("expected error code %s, got %s", domain.CodeVPCPathMTUBlackHole, prob.Code)
		}
	})
}

func TestTransitNetHandler_AuditNLBCrossZoneBalance(t *testing.T) {
	t.Run("Balanced returns 200 OK", func(t *testing.T) {
		r := setupTransitNetRouter(
			nil, nil,
			&mockTransitNLBProvider{
				crossZoneEnabled: false,
				shares:           map[string]float64{"zone-a": 0.50, "zone-b": 0.50},
			},
			nil,
		)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/cloud/network/nlb/cross-zone-balance?lb_arn=arn:aws:nlb:123", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Imbalance returns 409 Conflict RFC 7807", func(t *testing.T) {
		r := setupTransitNetRouter(
			nil, nil,
			&mockTransitNLBProvider{
				crossZoneEnabled: false,
				shares:           map[string]float64{"zone-a": 0.70, "zone-b": 0.30},
			},
			nil,
		)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/cloud/network/nlb/cross-zone-balance?lb_arn=arn:aws:nlb:123", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409 Conflict, got %d: %s", rec.Code, rec.Body.String())
		}

		var prob adapthttp.ProblemDetails
		if err := json.NewDecoder(rec.Body).Decode(&prob); err != nil {
			t.Fatalf("failed to decode RFC 7807 problem details: %v", err)
		}
		if prob.Code != domain.CodeNLBAZImbalance {
			t.Errorf("expected error code %s, got %s", domain.CodeNLBAZImbalance, prob.Code)
		}
	})
}

func TestTransitNetHandler_SanitizeCoreDNSNegativeCache(t *testing.T) {
	r := setupTransitNetRouter(
		nil, nil, nil,
		&mockTransitCoreDNSAdapter{
			corefile: ".:53 {\n    cache 30\n}\n",
		},
	)

	body := []byte(`{"domain_zone":"internal.corp","max_negative_ttl_seconds":5}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cloud/network/coredns/sanitize-negative-cache", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
}
