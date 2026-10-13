package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"

	adapterHTTP "github.com/datdt/k8sselfhost/internal/adapter/http"
	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	usecaseGitops "github.com/datdt/k8sselfhost/internal/usecase/gitops"
)

type mockGitCommander struct {
	ancestors map[string]bool
}

func (m *mockGitCommander) IsAncestor(ctx context.Context, repoPath, ancestorSHA, descendantSHA string) (bool, error) {
	return m.ancestors[ancestorSHA+":"+descendantSHA], nil
}

type mockClusterProvider struct {
	resources map[string][]byte
}

func (m *mockClusterProvider) GetLiveResource(ctx context.Context, namespace, kind, name string) ([]byte, error) {
	return m.resources[namespace+"/"+kind+"/"+name], nil
}

func setupGitOpsSyncWaveRouter(engine domainGitops.GitOpsResilienceEngine) *chi.Mux {
	r := chi.NewRouter()
	handler := adapterHTTP.NewGitOpsSyncWaveHandler(engine)
	handler.RegisterRoutes(r)
	return r
}

// TestVerifyOrdering_SuccessAndObsolete tests monotonic ordering HTTP responses.
func TestVerifyOrdering_SuccessAndObsolete(t *testing.T) {
	mockGit := &mockGitCommander{
		ancestors: map[string]bool{
			"c-old:c-new": true,
			"c-new:c-old": false,
		},
	}
	engine := usecaseGitops.NewSyncWaveManager(usecaseGitops.WithGitCommander(mockGit))
	router := setupGitOpsSyncWaveRouter(engine)

	t.Run("Accepts newer commit with 200 OK", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"delivery_id":  "deliv-101",
			"repo_path":    "/repos/web",
			"incoming_sha": "c-new",
			"current_sha":  "c-old",
		}
		raw, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/gitops/webhooks/verify-ordering", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["status"] != "ok" || resp["is_newer"] != true {
			t.Fatalf("unexpected response: %v", resp)
		}
	})

	t.Run("Rejects obsolete commit with 409 RFC 7807", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"delivery_id":  "deliv-102",
			"repo_path":    "/repos/web",
			"incoming_sha": "c-old",
			"current_sha":  "c-new",
		}
		raw, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/gitops/webhooks/verify-ordering", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected HTTP 409, got %d: %s", rec.Code, rec.Body.String())
		}
		contentType := rec.Header().Get("Content-Type")
		if contentType != "application/problem+json" {
			t.Errorf("expected application/problem+json, got: %s", contentType)
		}

		var prob domainGitops.RFC7807ProblemDetail
		if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
			t.Fatalf("failed to decode RFC 7807 response: %v", err)
		}
		if prob.Code != domainGitops.CodeGitOpsObsoleteWebhook {
			t.Fatalf("expected code %s, got: %s", domainGitops.CodeGitOpsObsoleteWebhook, prob.Code)
		}
	})

	t.Run("Malformed JSON returns 400 Bad Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/gitops/webhooks/verify-ordering", bytes.NewReader([]byte("{invalid-json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected HTTP 400, got %d", rec.Code)
		}
	})
}

// TestAuditCycle_AcyclicAndCircular tests sync-wave DAG cycle audit endpoint.
func TestAuditCycle_AcyclicAndCircular(t *testing.T) {
	engine := usecaseGitops.NewSyncWaveManager()
	router := setupGitOpsSyncWaveRouter(engine)

	t.Run("Valid acyclic DAG returns 200 OK", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"manifests": []string{
				"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cfg\n  annotations:\n    argocd.argoproj.io/sync-wave: \"-1\"\n",
				"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: app\n  annotations:\n    argocd.argoproj.io/sync-wave: \"0\"\n    argocd.argoproj.io/sync-wave-depends-on: \"ConfigMap/cfg\"\n",
			},
		}
		raw, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/gitops/sync-waves/audit-cycle", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Circular sync-wave dependency returns 422 RFC 7807", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"manifests": []string{
				"apiVersion: v1\nkind: Service\nmetadata:\n  name: svc-a\n  annotations:\n    argocd.argoproj.io/sync-wave: \"1\"\n    argocd.argoproj.io/sync-wave-depends-on: \"Service/svc-b\"\n",
				"apiVersion: v1\nkind: Service\nmetadata:\n  name: svc-b\n  annotations:\n    argocd.argoproj.io/sync-wave: \"1\"\n    argocd.argoproj.io/sync-wave-depends-on: \"Service/svc-a\"\n",
			},
		}
		raw, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/gitops/sync-waves/audit-cycle", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected HTTP 422, got %d: %s", rec.Code, rec.Body.String())
		}
		contentType := rec.Header().Get("Content-Type")
		if contentType != "application/problem+json" {
			t.Errorf("expected application/problem+json, got: %s", contentType)
		}

		var prob domainGitops.RFC7807ProblemDetail
		if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
			t.Fatalf("failed to decode RFC 7807: %v", err)
		}
		if prob.Code != domainGitops.CodeGitOpsCircularSyncWave {
			t.Fatalf("expected code %s, got: %s", domainGitops.CodeGitOpsCircularSyncWave, prob.Code)
		}
	})
}

// TestReconcileDrift_ConflictAndAutoResolve tests 3-way merge drift reconciliation endpoint.
func TestReconcileDrift_ConflictAndAutoResolve(t *testing.T) {
	cluster := &mockClusterProvider{
		resources: map[string][]byte{
			"prod/Deployment/payment": []byte(`{
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "metadata": {
    "name": "payment",
    "namespace": "prod",
    "managedFields": [
      {
        "manager": "kubectl-edit",
        "fieldsV1": {
          "f:spec": {
            "f:template": {
              "f:spec": {
                "f:containers": {
                  "k:{\"name\":\"app\"}": {
                    "f:image": {}
                  }
                }
              }
            }
          }
        }
      }
    ]
  }
}`),
			"prod/Deployment/logger": []byte(`{
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "metadata": {
    "name": "logger",
    "namespace": "prod",
    "managedFields": [
      {
        "manager": "kubectl-edit",
        "fieldsV1": {
          "f:spec": {
            "f:replicas": {}
          }
        }
      }
    ]
  }
}`),
		},
	}

	engine := usecaseGitops.NewSyncWaveManager(usecaseGitops.WithLiveClusterStateProvider(cluster))
	router := setupGitOpsSyncWaveRouter(engine)

	t.Run("Conflict returns 409 RFC 7807", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"release_name": "payment",
			"namespace":    "prod",
			"manifest": `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payment
  namespace: prod
spec:
  template:
    spec:
      containers:
        - name: app
          image: payment:v2.1.0
`,
		}
		raw, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/gitops/drift/reconcile", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected HTTP 409, got %d: %s", rec.Code, rec.Body.String())
		}
		var prob domainGitops.RFC7807ProblemDetail
		_ = json.Unmarshal(rec.Body.Bytes(), &prob)
		if prob.Code != domainGitops.CodeGitOpsDriftConflict {
			t.Fatalf("expected code %s, got %s", domainGitops.CodeGitOpsDriftConflict, prob.Code)
		}
	})

	t.Run("Non-conflicting drift auto-resolves with 200 OK", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"release_name": "logger",
			"namespace":    "prod",
			"manifest": `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: logger
  namespace: prod
spec:
  template:
    spec:
      containers:
        - name: app
          image: logger:v1.0.0
`,
		}
		raw, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/gitops/drift/reconcile", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["has_drift"] != true || resp["conflict_detected"] != false || resp["auto_resolved"] != true {
			t.Fatalf("unexpected auto-resolved response: %v", resp)
		}
	})
}

// TestGitOpsSyncWaveHandler_ConcurrentRequests tests thread safety under concurrent HTTP traffic.
func TestGitOpsSyncWaveHandler_ConcurrentRequests(t *testing.T) {
	mockGit := &mockGitCommander{
		ancestors: map[string]bool{
			"c-1:c-2": true,
		},
	}
	engine := usecaseGitops.NewSyncWaveManager(usecaseGitops.WithGitCommander(mockGit))
	router := setupGitOpsSyncWaveRouter(engine)

	const workers = 30
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			reqBody := map[string]interface{}{
				"delivery_id":  "deliv-conc",
				"repo_path":    "/repos/web",
				"incoming_sha": "c-2",
				"current_sha":  "c-1",
			}
			raw, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/gitops/webhooks/verify-ordering", bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200 OK, got: %d", rec.Code)
			}
		}(i)
	}

	wg.Wait()
}
