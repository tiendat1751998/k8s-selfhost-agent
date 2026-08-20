package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/datdt/k8sselfhost/internal/domain/audit"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
)

type mockK8sAuditRepo struct {
	actions []string
}

func (m *mockK8sAuditRepo) ListFindings(ctx context.Context, status string) ([]audit.AuditFinding, error) {
	return nil, nil
}
func (m *mockK8sAuditRepo) GetFinding(ctx context.Context, id string) (*audit.AuditFinding, error) {
	return nil, nil
}
func (m *mockK8sAuditRepo) ResolveFinding(ctx context.Context, id string) error {
	return nil
}
func (m *mockK8sAuditRepo) RecordRun(ctx context.Context, run *audit.AuditRun) error {
	return nil
}
func (m *mockK8sAuditRepo) GetLastRun(ctx context.Context) (*audit.AuditRun, error) {
	return nil, nil
}
func (m *mockK8sAuditRepo) RecordAction(ctx context.Context, actor, action, targetType, targetID, targetName, result string, details map[string]interface{}, ipAddress, userAgent string) error {
	m.actions = append(m.actions, action+":"+targetType+":"+targetName)
	return nil
}

func TestK8sResourceHandler_OfflineGraceful(t *testing.T) {
	// Handler with nil repo should return 503
	handler := NewK8sResourceHandler(nil, nil)
	r := chi.NewRouter()
	r.Route("/k8s/{cluster}", handler.RegisterRoutes)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/k8s/local/namespaces"},
		{"POST", "/k8s/local/namespaces"},
		{"DELETE", "/k8s/local/namespaces/foo"},
		{"GET", "/k8s/local/resources/configmaps"},
		{"GET", "/k8s/local/resources/configmaps/bar"},
		{"POST", "/k8s/local/resources/configmaps"},
		{"PUT", "/k8s/local/resources/configmaps/bar"},
		{"DELETE", "/k8s/local/resources/configmaps/bar"},
		{"POST", "/k8s/local/apply"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, bytes.NewBufferString(`{}`))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("[%s %s] expected status 503, got %d", ep.method, ep.path, rec.Code)
		}

		var res map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Errorf("[%s %s] failed to unmarshal response: %v", ep.method, ep.path, err)
		}
		if res["error"] != "Kubernetes cluster not connected or unconfigured" {
			t.Errorf("[%s %s] expected error 'Kubernetes cluster not connected or unconfigured', got '%s'", ep.method, ep.path, res["error"])
		}
		if res["code"] != "K8S_UNAVAILABLE" {
			t.Errorf("[%s %s] expected code 'K8S_UNAVAILABLE', got '%s'", ep.method, ep.path, res["code"])
		}
		if res["message"] != "Import a kubeconfig via Fleet to enable this feature" {
			t.Errorf("[%s %s] expected message 'Import a kubeconfig via Fleet to enable this feature', got '%s'", ep.method, ep.path, res["message"])
		}
	}
}

func TestK8sResourceHandler_UnconfiguredClient(t *testing.T) {
	// Handler with a repo that has no kubernetes client configured
	repo := infraK8s.NewResourceRepoWithInterface(nil, nil)
	handler := NewK8sResourceHandler(repo, nil)
	r := chi.NewRouter()
	r.Route("/k8s/{cluster}", handler.RegisterRoutes)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/k8s/unconfigured-cluster/namespaces"},
		{"POST", "/k8s/unconfigured-cluster/namespaces"},
		{"DELETE", "/k8s/unconfigured-cluster/namespaces/foo"},
		{"GET", "/k8s/unconfigured-cluster/resources/configmaps"},
		{"GET", "/k8s/unconfigured-cluster/resources/configmaps/bar"},
		{"POST", "/k8s/unconfigured-cluster/resources/configmaps"},
		{"PUT", "/k8s/unconfigured-cluster/resources/configmaps/bar"},
		{"DELETE", "/k8s/unconfigured-cluster/resources/configmaps/bar"},
		{"POST", "/k8s/unconfigured-cluster/apply"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, bytes.NewBufferString(`{"name":"test","apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test"}}`))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("[%s %s] expected status 503, got %d (body: %s)", ep.method, ep.path, rec.Code, rec.Body.String())
		}

		var res map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Errorf("[%s %s] failed to unmarshal response: %v", ep.method, ep.path, err)
		}
		if res["error"] != "Kubernetes cluster not connected or unconfigured" {
			t.Errorf("[%s %s] expected error 'Kubernetes cluster not connected or unconfigured', got '%s'", ep.method, ep.path, res["error"])
		}
		if res["code"] != "K8S_UNAVAILABLE" {
			t.Errorf("[%s %s] expected code 'K8S_UNAVAILABLE', got '%s'", ep.method, ep.path, res["code"])
		}
	}
}

func TestK8sResourceHandler_LiveOperations(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "db-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"password": []byte("super-secret-password"),
		},
	}
	fakeClient := fake.NewSimpleClientset(secret)
	repo := infraK8s.NewResourceRepoWithInterface(fakeClient, nil)
	auditMock := &mockK8sAuditRepo{}
	handler := NewK8sResourceHandler(repo, auditMock)

	r := chi.NewRouter()
	r.Route("/k8s/{cluster}", handler.RegisterRoutes)

	// 1. Create Namespace
	nsBody := `{"name": "test-zone"}`
	req := httptest.NewRequest("POST", "/k8s/local/namespaces", bytes.NewBufferString(nsBody))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. List Namespaces
	req = httptest.NewRequest("GET", "/k8s/local/namespaces", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// 3. Get Secret (verifying masked data)
	req = httptest.NewRequest("GET", "/k8s/local/resources/secrets/db-secret?ns=default", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	var secretResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &secretResp); err != nil {
		t.Fatalf("unmarshaling secret resp: %v", err)
	}
	dataMap := secretResp["data"].(map[string]interface{})
	if dataMap["password"] != "***" {
		t.Fatalf("expected masked secret value, got %v", dataMap["password"])
	}

	// 4. Create ConfigMap
	cmBody := `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"app-config","namespace":"default"},"data":{"PORT":"8080"}}`
	req = httptest.NewRequest("POST", "/k8s/local/resources/configmaps?ns=default", bytes.NewBufferString(cmBody))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5. Update ConfigMap
	cmUpdateBody := `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"app-config","namespace":"default"},"data":{"PORT":"9090"}}`
	req = httptest.NewRequest("PUT", "/k8s/local/resources/configmaps/app-config?ns=default", bytes.NewBufferString(cmUpdateBody))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// 6. Delete ConfigMap
	req = httptest.NewRequest("DELETE", "/k8s/local/resources/configmaps/app-config?ns=default", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// 7. Apply YAML
	applyBody := `{"yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: applied-cm\n  namespace: default\ndata:\n  test: val"}`
	req = httptest.NewRequest("POST", "/k8s/local/apply?ns=default", bytes.NewBufferString(applyBody))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// 8. Delete Namespace
	req = httptest.NewRequest("DELETE", "/k8s/local/namespaces/test-zone", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify Audit Actions were recorded
	if len(auditMock.actions) == 0 {
		t.Fatalf("expected audit actions to be recorded, got none")
	}
}
