package http

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/audit"
	"github.com/datdt/k8sselfhost/internal/domain/observability"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type mockObsRepoForBounds struct {
	observability.Repository
	defs map[string]*observability.SLODefinition
}

func (m *mockObsRepoForBounds) CreateSLODefinition(ctx context.Context, def *observability.SLODefinition) error {
	m.defs[def.ID] = def
	return nil
}

func (m *mockObsRepoForBounds) GetSLODefinition(ctx context.Context, id string) (*observability.SLODefinition, error) {
	if def, ok := m.defs[id]; ok {
		return def, nil
	}
	return nil, nil
}

func (m *mockObsRepoForBounds) UpdateSLODefinition(ctx context.Context, def *observability.SLODefinition) error {
	m.defs[def.ID] = def
	return nil
}

func TestObservability_TargetAndDaysBounds(t *testing.T) {
	repo := &mockObsRepoForBounds{defs: make(map[string]*observability.SLODefinition)}
	handler := NewObservabilityHandler(repo)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	t.Run("CreateSLODefinition rejects target > 100", func(t *testing.T) {
		body := `{"service":"api","target":105.0,"window":"30d"}`
		req := httptest.NewRequest(http.MethodPost, "/slo/definitions", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for target > 100, got %d", rec.Code)
		}
	})

	t.Run("UpdateSLODefinition rejects target > 100", func(t *testing.T) {
		repo.defs["def-1"] = &observability.SLODefinition{ID: "def-1", Service: "api", Target: 99.9}
		body := `{"service":"api","target":150.0}`
		req := httptest.NewRequest(http.MethodPut, "/slo/definitions/def-1", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for target > 100 on update, got %d", rec.Code)
		}
	})

	t.Run("parseWindowDuration bounds days to 365", func(t *testing.T) {
		d := parseWindowDuration("1000d")
		maxD := 365 * 24 * time.Hour
		if d > maxD {
			t.Fatalf("expected duration <= 365 days (%v), got %v", maxD, d)
		}
	})
}

func TestGitOps_XGitHubDelivery_ReplayDeduplication(t *testing.T) {
	secret := []byte("testsecret")
	handler := NewGitOpsHandler(nil, WithWebhookSecret(secret))

	r := chi.NewRouter()
	r.Post("/webhook", handler.HandleWebhook)

	deliveryID := "replay-uuid-001"
	payload := []byte(`{"ref":"refs/heads/main"}`)
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	// First delivery: valid
	req1 := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req1.Header.Set("X-Hub-Signature-256", sig)
	req1.Header.Set("X-GitHub-Delivery", deliveryID)
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first delivery to return 200 OK, got %d: %s", rec1.Code, rec1.Body.String())
	}

	var resp1 map[string]string
	_ = json.NewDecoder(rec1.Body).Decode(&resp1)
	if resp1["status"] != "received" {
		t.Fatalf("expected first delivery status 'received', got %q", resp1["status"])
	}

	// Replay delivery with same delivery ID: should be recognized as duplicate
	req2 := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req2.Header.Set("X-Hub-Signature-256", sig)
	req2.Header.Set("X-GitHub-Delivery", deliveryID)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected duplicate delivery to return 200 OK, got %d", rec2.Code)
	}

	var resp2 map[string]string
	_ = json.NewDecoder(rec2.Body).Decode(&resp2)
	if resp2["status"] != "duplicate" {
		t.Fatalf("expected duplicate delivery status 'duplicate', got %q", resp2["status"])
	}
}

func TestMiddleware_IsOriginAllowed_Cached(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app1.example.com, https://app2.example.com")

	if !middleware.IsOriginAllowed("https://app1.example.com") {
		t.Fatal("expected app1 to be allowed")
	}
	if !middleware.IsOriginAllowed("https://app2.example.com") {
		t.Fatal("expected app2 to be allowed")
	}
	if middleware.IsOriginAllowed("https://attacker.com") {
		t.Fatal("expected attacker to be disallowed")
	}

	// Test cache invalidation on env change
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://newapp.example.com")
	if middleware.IsOriginAllowed("https://app1.example.com") {
		t.Fatal("expected app1 to be disallowed after env change")
	}
	if !middleware.IsOriginAllowed("https://newapp.example.com") {
		t.Fatal("expected newapp to be allowed after env change")
	}
}

type mockAuditRepoForRBAC struct {
	audit.Repository
}

func (m *mockAuditRepoForRBAC) ListFindings(ctx context.Context, status string) ([]audit.AuditFinding, error) {
	return []audit.AuditFinding{}, nil
}

func (m *mockAuditRepoForRBAC) RecordRun(ctx context.Context, run *audit.AuditRun) error {
	return nil
}

func TestRBAC_PlatformAdminOnlyEndpoints(t *testing.T) {
	auditHandler := NewAuditHandler(&mockAuditRepoForRBAC{})
	platform := &PlatformHandlers{
		Audit: auditHandler,
	}
	r := chi.NewRouter()
	r.Route("/api/v1", func(sub chi.Router) {
		RegisterPlatformRoutes(sub, platform)
	})

	// Operator role on GET /api/v1/audit/findings is allowed per Criterion 6
	reqFindings := httptest.NewRequest(http.MethodGet, "/api/v1/audit/findings", nil)
	reqFindings = reqFindings.WithContext(tenancy.WithUserRole(reqFindings.Context(), "operator"))
	recFindings := httptest.NewRecorder()
	r.ServeHTTP(recFindings, reqFindings)

	if recFindings.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for operator on /api/v1/audit/findings, got %d", recFindings.Code)
	}

	// Operator role on mutating POST /api/v1/audit/run must be 403 Forbidden
	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/run", nil)
	req = req.WithContext(tenancy.WithUserRole(req.Context(), "operator"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for operator on POST /api/v1/audit/run, got %d", rec.Code)
	}

	// Platform admin should not be forbidden on POST /api/v1/audit/run
	reqAdmin := httptest.NewRequest(http.MethodPost, "/api/v1/audit/run", nil)
	reqAdmin = reqAdmin.WithContext(tenancy.WithUserRole(reqAdmin.Context(), "platform_admin"))
	recAdmin := httptest.NewRecorder()
	r.ServeHTTP(recAdmin, reqAdmin)

	if recAdmin.Code == http.StatusForbidden {
		t.Fatalf("expected platform_admin not to be forbidden on POST /api/v1/audit/run, got 403")
	}
}
