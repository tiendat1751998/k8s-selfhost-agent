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

	adapterhttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domain "github.com/datdt/k8sselfhost/internal/domain/runtimeguard"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/runtimeguard"
)

type mockStorageProbe struct {
	nodes []domain.NodeStorageHealth
	err   error
}

func (m *mockStorageProbe) GetNodeStorageHealth(ctx context.Context) ([]domain.NodeStorageHealth, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.nodes, nil
}

func setupTestRouter(mgr *usecase.ResilienceManager) *chi.Mux {
	r := chi.NewRouter()
	handler := adapterhttp.NewRuntimeGuardHandler(mgr)
	handler.RegisterRoutes(r)
	return r
}

func TestAuditStorageHealth_Endpoint(t *testing.T) {
	probe := &mockStorageProbe{
		nodes: []domain.NodeStorageHealth{
			{
				NodeName:       "node-01",
				MountPoint:     domain.DefaultContainerdMountPoint,
				TotalBytes:     1000,
				UsedBytesPct:   50.0,
				TotalInodes:    1000,
				UsedInodesPct:  60.0,
				DanglingImages: 2,
			},
		},
	}
	mgr := usecase.NewResilienceManager(usecase.WithStorageProbe(probe))
	r := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime/nodes/storage-health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	nodes, ok := resp["nodes"].([]interface{})
	if !ok || len(nodes) != 1 {
		t.Fatalf("expected 1 node in response, got %v", resp["nodes"])
	}
}

func TestTriggerDanglingPrune_Endpoint(t *testing.T) {
	mgr := usecase.NewResilienceManager()
	r := setupTestRouter(mgr)

	// Valid node
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/nodes/worker-01/prune", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "pruned" || resp["node"] != "worker-01" {
		t.Fatalf("unexpected prune response: %v", resp)
	}

	// Missing node (empty whitespace)
	reqEmpty := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/nodes/%20/prune", nil)
	wEmpty := httptest.NewRecorder()
	r.ServeHTTP(wEmpty, reqEmpty)

	if wEmpty.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for empty node, got %d", wEmpty.Code)
	}
}

func TestMutateImageMirror_Endpoint(t *testing.T) {
	mgr := usecase.NewResilienceManager()
	r := setupTestRouter(mgr)

	// Valid image mutation
	body := bytes.NewBufferString(`{"image": "nginx:1.25"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/images/mutate-mirror", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["mutated_image"] != "harbor.corp.local/proxy-cache/library/nginx:1.25" {
		t.Fatalf("unexpected mutated_image: %v", resp["mutated_image"])
	}

	// Invalid image format -> 422 Unprocessable Entity
	badBody := bytes.NewBufferString(`{"image": "http://invalid-url"}`)
	badReq := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/images/mutate-mirror", badBody)
	badReq.Header.Set("Content-Type", "application/json")
	wBad := httptest.NewRecorder()
	r.ServeHTTP(wBad, badReq)

	if wBad.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422 for invalid image, got %d: %s", wBad.Code, wBad.Body.String())
	}

	// Empty request body -> 400 Bad Request
	emptyReq := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/images/mutate-mirror", bytes.NewBufferString(``))
	wEmpty := httptest.NewRecorder()
	r.ServeHTTP(wEmpty, emptyReq)

	if wEmpty.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for empty body, got %d", wEmpty.Code)
	}
}

func TestMutateImageMirror_RateLimitExceeded(t *testing.T) {
	mgr := usecase.NewResilienceManager()
	_ = mgr.ConfigureRegistryMirror(context.Background(), domain.RegistryMirrorConfig{
		PrimaryRegistry: "docker.io",
		MirrorURL:       "harbor.corp.local/proxy-cache",
		FallbackTimeout: 15 * time.Second,
		RateLimitFloor:  1,
	})
	r := setupTestRouter(mgr)

	// First pull: succeeds
	body1 := bytes.NewBufferString(`{"image": "redis:alpine"}`)
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/images/mutate-mirror", body1)
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w1.Code)
	}

	// Second pull: 429 Too Many Requests
	body2 := bytes.NewBufferString(`{"image": "redis:alpine"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/images/mutate-mirror", body2)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d: %s", w2.Code, w2.Body.String())
	}

	var problem adapterhttp.ProblemDetails
	_ = json.Unmarshal(w2.Body.Bytes(), &problem)
	if problem.Code != domain.CodeRegistryRateLimitExceeded {
		t.Fatalf("expected problem code %s, got %s", domain.CodeRegistryRateLimitExceeded, problem.Code)
	}
}

func TestConfigureMirror_Endpoint(t *testing.T) {
	mgr := usecase.NewResilienceManager()
	r := setupTestRouter(mgr)

	// Valid config
	payload := `{"primary_registry": "docker.io", "mirror_url": "harbor.private.local/cache", "fallback_timeout_seconds": 10, "rate_limit_floor": 200}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/registries/configure-mirror", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "configured" || resp["mirror_url"] != "harbor.private.local/cache" {
		t.Fatalf("unexpected configure response: %v", resp)
	}

	// Invalid config (missing mirror_url)
	invalidPayload := `{"primary_registry": "docker.io", "mirror_url": ""}`
	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/registries/configure-mirror", bytes.NewBufferString(invalidPayload))
	reqBad.Header.Set("Content-Type", "application/json")
	wBad := httptest.NewRecorder()
	r.ServeHTTP(wBad, reqBad)

	if wBad.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for invalid config, got %d", wBad.Code)
	}
}
