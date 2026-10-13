package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/domain/k8sresilience"
)

// In-memory watchdog stub for handler tests.
type stubWatchdog struct {
	auditFunc    func(ctx context.Context) ([]k8sresilience.WebhookHealthStatus, error)
	bypassFunc   func(ctx context.Context, name string) error
	reapFunc     func(ctx context.Context, ns string) error
	plegFunc     func(ctx context.Context, node string) (*k8sresilience.PLEGStatus, error)
	restartFunc  func(ctx context.Context, node string) error
}

func (s *stubWatchdog) AuditWebhooks(ctx context.Context) ([]k8sresilience.WebhookHealthStatus, error) {
	if s.auditFunc != nil {
		return s.auditFunc(ctx)
	}
	return []k8sresilience.WebhookHealthStatus{}, nil
}

func (s *stubWatchdog) EmergencyBypassDeadlockedWebhook(ctx context.Context, webhookName string) error {
	if s.bypassFunc != nil {
		return s.bypassFunc(ctx, webhookName)
	}
	return nil
}

func (s *stubWatchdog) ReapOrphanedFinalizers(ctx context.Context, namespace string) error {
	if s.reapFunc != nil {
		return s.reapFunc(ctx, namespace)
	}
	return nil
}

func (s *stubWatchdog) CheckPLEGHealth(ctx context.Context, nodeName string) (*k8sresilience.PLEGStatus, error) {
	if s.plegFunc != nil {
		return s.plegFunc(ctx, nodeName)
	}
	st := k8sresilience.NewPLEGStatus(nodeName, 30*time.Second, true)
	return &st, nil
}

func (s *stubWatchdog) RestartStalledContainerd(ctx context.Context, nodeName string) error {
	if s.restartFunc != nil {
		return s.restartFunc(ctx, nodeName)
	}
	return nil
}

func setupResilienceTestRouter(watchdog k8sresilience.ControlPlaneWatchdog) *chi.Mux {
	r := chi.NewRouter()
	handler := adapthttp.NewK8sResilienceHandler(watchdog, nil)
	handler.WireToRouter(r)
	return r
}

func TestK8sResilienceHandler_AuditWebhooks(t *testing.T) {
	wd := &stubWatchdog{
		auditFunc: func(ctx context.Context) ([]k8sresilience.WebhookHealthStatus, error) {
			return []k8sresilience.WebhookHealthStatus{
				k8sresilience.NewWebhookHealthStatus(
					"calico-webhook",
					"validating",
					"Fail",
					"https://calico.svc:443/val",
					5,
					120*time.Millisecond,
					false,
				),
			}, nil
		},
	}

	router := setupResilienceTestRouter(wd)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/k8s/webhooks/audit", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp []k8sresilience.WebhookHealthStatus
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 1 || resp[0].Name != "calico-webhook" || !resp[0].IsDeadlocked {
		t.Fatalf("unexpected webhook audit response: %+v", resp)
	}
}

func TestK8sResilienceHandler_BypassWebhook(t *testing.T) {
	t.Run("successful bypass", func(t *testing.T) {
		wd := &stubWatchdog{
			bypassFunc: func(ctx context.Context, name string) error {
				if name != "calico-webhook" {
					t.Fatalf("expected name calico-webhook, got %s", name)
				}
				return nil
			},
		}

		router := setupResilienceTestRouter(wd)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/webhooks/calico-webhook/bypass", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("webhook not found returns 404 ProblemDetails", func(t *testing.T) {
		wd := &stubWatchdog{
			bypassFunc: func(ctx context.Context, name string) error {
				return k8sresilience.ErrWebhookNotFound
			},
		}

		router := setupResilienceTestRouter(wd)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/webhooks/non-existent/bypass", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rr.Code)
		}
		var prob adapthttp.ProblemDetails
		if err := json.NewDecoder(rr.Body).Decode(&prob); err != nil {
			t.Fatalf("failed to decode problem details: %v", err)
		}
		if prob.Status != http.StatusNotFound {
			t.Fatalf("expected problem status 404, got %d", prob.Status)
		}
	})

	t.Run("deadlock error returns 500 ProblemDetails", func(t *testing.T) {
		wd := &stubWatchdog{
			bypassFunc: func(ctx context.Context, name string) error {
				return k8sresilience.ErrK8sWebhookDeadlock
			},
		}

		router := setupResilienceTestRouter(wd)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/webhooks/deadlocked/bypass", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", rr.Code)
		}
		var prob adapthttp.ProblemDetails
		if err := json.NewDecoder(rr.Body).Decode(&prob); err != nil {
			t.Fatalf("failed to decode problem details: %v", err)
		}
		if prob.Status != http.StatusInternalServerError {
			t.Fatalf("expected problem status 500, got %d", prob.Status)
		}
	})
}

func TestK8sResilienceHandler_ReapFinalizers(t *testing.T) {
	t.Run("successful reap", func(t *testing.T) {
		wd := &stubWatchdog{
			reapFunc: func(ctx context.Context, ns string) error {
				if ns != "stuck-ns" {
					t.Fatalf("expected stuck-ns, got %s", ns)
				}
				return nil
			},
		}

		router := setupResilienceTestRouter(wd)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/namespaces/stuck-ns/reap-finalizers", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("namespace not stuck returns 409 ProblemDetails", func(t *testing.T) {
		wd := &stubWatchdog{
			reapFunc: func(ctx context.Context, ns string) error {
				return k8sresilience.ErrNamespaceNotStuck
			},
		}

		router := setupResilienceTestRouter(wd)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/namespaces/active-ns/reap-finalizers", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d", rr.Code)
		}
	})
}

func TestK8sResilienceHandler_PLEGHealth(t *testing.T) {
	t.Run("healthy node returns 200", func(t *testing.T) {
		wd := &stubWatchdog{
			plegFunc: func(ctx context.Context, node string) (*k8sresilience.PLEGStatus, error) {
				st := k8sresilience.NewPLEGStatus(node, 20*time.Second, true)
				return &st, nil
			},
		}

		router := setupResilienceTestRouter(wd)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/k8s/nodes/worker-01/pleg-health", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("pleg timeout returns 504 ProblemDetails", func(t *testing.T) {
		wd := &stubWatchdog{
			plegFunc: func(ctx context.Context, node string) (*k8sresilience.PLEGStatus, error) {
				return nil, k8sresilience.ErrK8sPLEGTimeout
			},
		}

		router := setupResilienceTestRouter(wd)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/k8s/nodes/worker-02/pleg-health", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusGatewayTimeout {
			t.Fatalf("expected status 504, got %d", rr.Code)
		}

		var prob adapthttp.ProblemDetails
		if err := json.NewDecoder(rr.Body).Decode(&prob); err != nil {
			t.Fatalf("failed to decode problem details: %v", err)
		}
		if prob.Status != http.StatusGatewayTimeout {
			t.Fatalf("expected problem status 504, got %d", prob.Status)
		}
	})
}
