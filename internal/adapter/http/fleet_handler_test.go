package http

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/audit"
	"github.com/datdt/k8sselfhost/internal/domain/fleet"
	clusterUsecase "github.com/datdt/k8sselfhost/internal/usecase/cluster"
)

type mockFleetHandlerRepo struct {
	clusters map[string]*fleet.Cluster
}

func (m *mockFleetHandlerRepo) ListClusters(ctx context.Context) ([]fleet.Cluster, error) {
	var list []fleet.Cluster
	for _, c := range m.clusters {
		list = append(list, *c)
	}
	return list, nil
}

func (m *mockFleetHandlerRepo) GetCluster(ctx context.Context, id string) (*fleet.Cluster, error) {
	if c, ok := m.clusters[id]; ok {
		return c, nil
	}
	return nil, nil
}

func (m *mockFleetHandlerRepo) RegisterCluster(ctx context.Context, c *fleet.Cluster) error {
	m.clusters[c.ID] = c
	return nil
}

func (m *mockFleetHandlerRepo) UpdateClusterStatus(ctx context.Context, id, status, version string, nodes int) error {
	return nil
}

func (m *mockFleetHandlerRepo) UpdateHealth(ctx context.Context, id, healthStatus string, lastCheck time.Time) error {
	return nil
}

func (m *mockFleetHandlerRepo) UpdateDiscoveredResources(ctx context.Context, id string, resources map[string]interface{}) error {
	return nil
}

func (m *mockFleetHandlerRepo) RemoveCluster(ctx context.Context, id string) error {
	delete(m.clusters, id)
	return nil
}

type mockFleetAuditRepo struct{}

func (m *mockFleetAuditRepo) ListFindings(ctx context.Context, status string) ([]audit.AuditFinding, error) {
	return nil, nil
}
func (m *mockFleetAuditRepo) GetFinding(ctx context.Context, id string) (*audit.AuditFinding, error) {
	return nil, nil
}
func (m *mockFleetAuditRepo) ResolveFinding(ctx context.Context, id string) error {
	return nil
}
func (m *mockFleetAuditRepo) RecordRun(ctx context.Context, run *audit.AuditRun) error {
	return nil
}
func (m *mockFleetAuditRepo) GetLastRun(ctx context.Context) (*audit.AuditRun, error) {
	return nil, nil
}
func (m *mockFleetAuditRepo) RecordAction(ctx context.Context, actor, action, targetType, targetID, targetName, result string, details map[string]interface{}, ipAddress, userAgent string) error {
	return nil
}

type mockDiscoveryPort struct{}

func (d *mockDiscoveryPort) DiscoverResources(ctx context.Context, kubeconfig []byte) (map[string]interface{}, error) {
	return map[string]interface{}{"version": "v1.30.0", "nodes_count": 3}, nil
}

func (d *mockDiscoveryPort) ValidateConnectivity(ctx context.Context, kubeconfig []byte) error {
	return nil
}

func setupFleetTestRouter(repo fleet.Repository) http.Handler {
	auditRepo := &mockFleetAuditRepo{}
	discovery := &mockDiscoveryPort{}
	importUc := clusterUsecase.NewImportUsecase(repo, discovery, zap.NewNop())
	h := NewFleetHandler(repo, auditRepo, importUc)

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

func TestFleetHandler_ListClusters_NoEncryptedTokenLeaked(t *testing.T) {
	sensitiveToken := "sensitive-cluster-kubeconfig-secret-token-xyz"
	repo := &mockFleetHandlerRepo{
		clusters: map[string]*fleet.Cluster{
			"c-1": {
				ID:             "c-1",
				Name:           "prod-cluster",
				Region:         "us-east-1",
				Provider:       "aws",
				EncryptedToken: sensitiveToken,
			},
		},
	}

	router := setupFleetTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if strings.Contains(body, sensitiveToken) {
		t.Fatalf("CRITICAL SECURITY LEAK: sensitive token %q found in response body: %s", sensitiveToken, body)
	}
	if strings.Contains(body, "encrypted_token") {
		t.Fatalf("encrypted_token JSON field should not be serialized in response: %s", body)
	}
}

func TestFleetHandler_RegisterCluster_NoEncryptedTokenLeaked(t *testing.T) {
	sensitiveToken := "secret-registration-token-123"
	repo := &mockFleetHandlerRepo{
		clusters: make(map[string]*fleet.Cluster),
	}

	router := setupFleetTestRouter(repo)

	payload := `{"id":"c-2","name":"stage-cluster","region":"eu-west-1","provider":"gcp","encrypted_token":"` + sensitiveToken + `"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if strings.Contains(body, sensitiveToken) {
		t.Fatalf("CRITICAL SECURITY LEAK: sensitive token %q found in response body: %s", sensitiveToken, body)
	}
	if strings.Contains(body, "encrypted_token") {
		t.Fatalf("encrypted_token JSON field should not be serialized in response: %s", body)
	}
}

func TestFleetHandler_ImportCluster_NoEncryptedTokenLeaked(t *testing.T) {
	sensitiveKubeconfig := "sensitive-kubeconfig-content-super-secret-xyz"
	repo := &mockFleetHandlerRepo{
		clusters: make(map[string]*fleet.Cluster),
	}

	router := setupFleetTestRouter(repo)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("id", "c-3")
	_ = writer.WriteField("name", "imported-cluster")
	_ = writer.WriteField("group", "default")
	_ = writer.WriteField("region", "ap-southeast-1")
	_ = writer.WriteField("provider", "onprem")

	part, err := writer.CreateFormFile("kubeconfig", "kubeconfig.yaml")
	if err != nil {
		t.Fatalf("creating form file: %v", err)
	}
	_, _ = part.Write([]byte(sensitiveKubeconfig))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/import", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if strings.Contains(body, sensitiveKubeconfig) {
		t.Fatalf("CRITICAL SECURITY LEAK: kubeconfig data %q found in response body: %s", sensitiveKubeconfig, body)
	}
	if strings.Contains(body, "encrypted_token") {
		t.Fatalf("encrypted_token JSON field should not be serialized in response: %s", body)
	}
}
