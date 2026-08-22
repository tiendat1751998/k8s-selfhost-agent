package ecosystem_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/ecosystem"
	"github.com/datdt/k8sselfhost/internal/domain/settings"
	usecaseEcosystem "github.com/datdt/k8sselfhost/internal/usecase/ecosystem"
)

// In-memory mock repositories for usecase unit testing
type mockEcosystemRepo struct {
	tools   map[string]map[string]ecosystem.DetectedTool // tenantID -> id -> tool
	deleted []string
}

func newMockEcosystemRepo() *mockEcosystemRepo {
	return &mockEcosystemRepo{
		tools: make(map[string]map[string]ecosystem.DetectedTool),
	}
}

func (m *mockEcosystemRepo) GetAll(ctx context.Context, tenantID string) ([]ecosystem.DetectedTool, error) {
	tenantTools, ok := m.tools[tenantID]
	if !ok {
		return []ecosystem.DetectedTool{}, nil
	}
	var res []ecosystem.DetectedTool
	for _, t := range tenantTools {
		res = append(res, t)
	}
	return res, nil
}

func (m *mockEcosystemRepo) GetByID(ctx context.Context, tenantID, id string) (*ecosystem.DetectedTool, error) {
	tenantTools, ok := m.tools[tenantID]
	if !ok {
		return nil, nil
	}
	tool, ok := tenantTools[id]
	if !ok {
		return nil, nil
	}
	return &tool, nil
}

func (m *mockEcosystemRepo) Create(ctx context.Context, tool *ecosystem.DetectedTool) error {
	if tool.ID == "" {
		tool.ID = "generated-" + tool.Name
	}
	if _, ok := m.tools[tool.TenantID]; !ok {
		m.tools[tool.TenantID] = make(map[string]ecosystem.DetectedTool)
	}
	m.tools[tool.TenantID][tool.ID] = *tool
	return nil
}

func (m *mockEcosystemRepo) BulkUpsert(ctx context.Context, tools []ecosystem.DetectedTool) error {
	for _, t := range tools {
		if t.ID == "" {
			t.ID = "generated-" + t.Name
		}
		if _, ok := m.tools[t.TenantID]; !ok {
			m.tools[t.TenantID] = make(map[string]ecosystem.DetectedTool)
		}
		// Match by name or ID
		found := false
		for k, existing := range m.tools[t.TenantID] {
			if existing.Name == t.Name {
				m.tools[t.TenantID][k] = t
				found = true
				break
			}
		}
		if !found {
			m.tools[t.TenantID][t.ID] = t
		}
	}
	return nil
}

func (m *mockEcosystemRepo) Delete(ctx context.Context, tenantID, id string) error {
	if tenantTools, ok := m.tools[tenantID]; ok {
		delete(tenantTools, id)
	}
	m.deleted = append(m.deleted, id)
	return nil
}

func (m *mockEcosystemRepo) GetSummary(ctx context.Context, tenantID string) (*ecosystem.EcosystemSummary, error) {
	all, _ := m.GetAll(ctx, tenantID)
	summary := &ecosystem.EcosystemSummary{
		Total:      len(all),
		Healthy:    0,
		Degraded:   0,
		ByCategory: make(map[string]int),
	}
	for _, t := range all {
		summary.ByCategory[t.Category]++
		if t.Health == ecosystem.HealthHealthy {
			summary.Healthy++
		} else if t.Health == ecosystem.HealthDegraded || t.Status == ecosystem.StatusUnreachable {
			summary.Degraded++
		}
	}
	return summary, nil
}

type mockSettingsRepo struct {
	settings []settings.Setting
}

func (m *mockSettingsRepo) GetByCategory(ctx context.Context, tenantID, category string) ([]settings.Setting, error) {
	var res []settings.Setting
	for _, s := range m.settings {
		if (tenantID == "" || s.TenantID == tenantID) && s.Category == category {
			res = append(res, s)
		}
	}
	return res, nil
}

func (m *mockSettingsRepo) GetByKey(ctx context.Context, tenantID, category, key string) (*settings.Setting, error) {
	for _, s := range m.settings {
		if (tenantID == "" || s.TenantID == tenantID) && s.Category == category && s.Key == key {
			return &s, nil
		}
	}
	return nil, nil
}

func (m *mockSettingsRepo) Upsert(ctx context.Context, s *settings.Setting) error {
	m.settings = append(m.settings, *s)
	return nil
}

func (m *mockSettingsRepo) BulkUpsert(ctx context.Context, ss []settings.Setting) error {
	m.settings = append(m.settings, ss...)
	return nil
}

func (m *mockSettingsRepo) GetAll(ctx context.Context, tenantID string) ([]settings.Setting, error) {
	var res []settings.Setting
	for _, s := range m.settings {
		if tenantID == "" || s.TenantID == tenantID {
			res = append(res, s)
		}
	}
	return res, nil
}

func TestDetector_ScanAndProbes(t *testing.T) {
	// Mock servers for tools
	argocdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/version" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"Version": "v2.10.4"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer argocdServer.Close()

	trivyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer trivyServer.Close()

	grafanaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"version":  "10.2.1",
				"database": "ok",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer grafanaServer.Close()

	vaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"initialized": true,
				"sealed":      false,
				"version":     "1.15.4",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer vaultServer.Close()

	promServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/-/healthy" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Prometheus Server is Healthy."))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer promServer.Close()

	ecoRepo := newMockEcosystemRepo()
	settingsRepo := &mockSettingsRepo{
		settings: []settings.Setting{
			{Category: settings.CategoryIntegrations, Key: "argocd_url", Value: argocdServer.URL, TenantID: "test-tenant"},
			{Category: settings.CategoryIntegrations, Key: "trivy_url", Value: trivyServer.URL, TenantID: "test-tenant"},
			{Category: settings.CategoryIntegrations, Key: "grafana_url", Value: grafanaServer.URL, TenantID: "test-tenant"},
			{Category: settings.CategoryIntegrations, Key: "vault_url", Value: vaultServer.URL, TenantID: "test-tenant"},
			{Category: settings.CategoryIntegrations, Key: "prometheus_url", Value: promServer.URL, TenantID: "test-tenant"},
		},
	}

	uc := usecaseEcosystem.NewUsecase(ecoRepo, settingsRepo, http.DefaultClient, zap.NewNop())

	ctx := context.Background()
	tools, err := uc.Scan(ctx, "test-tenant")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(tools) == 0 {
		t.Fatalf("expected detected tools, got 0")
	}

	toolMap := make(map[string]ecosystem.DetectedTool)
	for _, tool := range tools {
		toolMap[tool.Name] = tool
	}

	// Verify ArgoCD
	argo, ok := toolMap["ArgoCD"]
	if !ok {
		t.Fatalf("ArgoCD tool missing")
	}
	if argo.Status != ecosystem.StatusDetected || argo.Health != ecosystem.HealthHealthy || argo.Version != "v2.10.4" {
		t.Errorf("unexpected ArgoCD: %+v", argo)
	}

	// Verify Trivy
	trivy, ok := toolMap["Trivy"]
	if !ok {
		t.Fatalf("Trivy tool missing")
	}
	if trivy.Status != ecosystem.StatusDetected || trivy.Health != ecosystem.HealthHealthy {
		t.Errorf("unexpected Trivy: %+v", trivy)
	}

	// Verify Grafana
	grafana, ok := toolMap["Grafana"]
	if !ok {
		t.Fatalf("Grafana tool missing")
	}
	if grafana.Status != ecosystem.StatusDetected || grafana.Version != "10.2.1" {
		t.Errorf("unexpected Grafana: %+v", grafana)
	}

	// Verify Vault
	vault, ok := toolMap["Vault"]
	if !ok {
		t.Fatalf("Vault tool missing")
	}
	if vault.Status != ecosystem.StatusDetected || vault.Version != "1.15.4" || vault.Metadata["sealed"] != "false" {
		t.Errorf("unexpected Vault: %+v", vault)
	}

	// Verify Prometheus
	prom, ok := toolMap["Prometheus"]
	if !ok {
		t.Fatalf("Prometheus tool missing")
	}
	if prom.Status != ecosystem.StatusDetected || prom.Health != ecosystem.HealthHealthy {
		t.Errorf("unexpected Prometheus: %+v", prom)
	}

	// Check Summary
	summary, err := uc.GetSummary(ctx, "test-tenant")
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}
	if summary.Healthy < 5 {
		t.Errorf("expected at least 5 healthy tools, got %d", summary.Healthy)
	}
}

func TestDetector_Cache5Minutes(t *testing.T) {
	ecoRepo := newMockEcosystemRepo()
	settingsRepo := &mockSettingsRepo{}
	uc := usecaseEcosystem.NewUsecase(ecoRepo, settingsRepo, http.DefaultClient, zap.NewNop())

	ctx := context.Background()
	// First scan loads tools
	tools1, err := uc.Scan(ctx, "tenant-cache")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// GetAll should return cached tools
	tools2, err := uc.GetAll(ctx, "tenant-cache")
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}

	if len(tools1) != len(tools2) {
		t.Errorf("cached tools length mismatch: %d vs %d", len(tools1), len(tools2))
	}
}

func TestDetector_ManualToolCRUD(t *testing.T) {
	ecoRepo := newMockEcosystemRepo()
	settingsRepo := &mockSettingsRepo{}
	uc := usecaseEcosystem.NewUsecase(ecoRepo, settingsRepo, http.DefaultClient, zap.NewNop())

	ctx := context.Background()
	manualTool := &ecosystem.DetectedTool{
		Name:     "CustomTool",
		Category: ecosystem.CategorySecurity,
		Endpoint: "https://custom.internal",
		Source:   ecosystem.SourceManual,
		TenantID: "tenant-manual",
	}

	if err := uc.CreateTool(ctx, manualTool); err != nil {
		t.Fatalf("CreateTool failed: %v", err)
	}

	tools, err := uc.GetAll(ctx, "tenant-manual")
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(tools) == 0 {
		t.Fatalf("expected tool in GetAll")
	}

	if err := uc.DeleteTool(ctx, "tenant-manual", manualTool.ID); err != nil {
		t.Fatalf("DeleteTool failed: %v", err)
	}
}

type mockDockerClient struct {
	serverVer  string
	containers []container.Summary
}

func (m *mockDockerClient) ServerVersion(ctx context.Context) (types.Version, error) {
	return types.Version{Version: m.serverVer}, nil
}

func (m *mockDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	return m.containers, nil
}

func TestDetector_DockerLiveContainers(t *testing.T) {
	mockDocker := &mockDockerClient{
		serverVer: "28.5.2",
		containers: []container.Summary{
			{
				ID:     "c111111111111",
				Image:  "postgres:16.4-alpine",
				Names:  []string{"/k8sselfhost-postgres-1"},
				State:  "running",
				Status: "Up 4 hours",
			},
			{
				ID:     "c222222222222",
				Image:  "redis:8.0-alpine",
				Names:  []string{"/k8sselfhost-redis-1"},
				State:  "running",
				Status: "Up 4 hours",
			},
			{
				ID:     "c333333333333",
				Image:  "nats:2.10.19-alpine",
				Names:  []string{"/nats-jetstream"},
				State:  "running",
				Status: "Up 4 hours",
			},
			{
				ID:     "c444444444444",
				Image:  "traefik:v3.2.0",
				Names:  []string{"/traefik-proxy"},
				State:  "running",
				Status: "Up 4 hours",
			},
			{
				ID:     "c555555555555",
				Image:  "drone/drone:2.24.1",
				Names:  []string{"/drone-server"},
				State:  "running",
				Status: "Up 4 hours",
			},
			{
				ID:     "c666666666666",
				Image:  "custom/kafka-broker:3.6.1",
				Names:  []string{"/custom-kafka"},
				State:  "running",
				Status: "Up 4 hours",
			},
		},
	}

	ecoRepo := newMockEcosystemRepo()
	settingsRepo := &mockSettingsRepo{
		settings: []settings.Setting{
			{Category: settings.CategoryIntegrations, Key: "vault_url", Value: "http://127.0.0.1:59999", TenantID: "docker-tenant"},
		},
	}
	uc := usecaseEcosystem.NewUsecase(
		ecoRepo,
		settingsRepo,
		http.DefaultClient,
		zap.NewNop(),
		usecaseEcosystem.WithDockerClient(mockDocker),
	)

	ctx := context.Background()
	tools, err := uc.Scan(ctx, "docker-tenant")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	toolMap := make(map[string]ecosystem.DetectedTool)
	for _, tool := range tools {
		toolMap[tool.Name] = tool
	}

	// 1. Docker Engine
	dockerTool, ok := toolMap["Docker Engine"]
	if !ok {
		t.Fatalf("Docker Engine missing")
	}
	if dockerTool.Version != "28.5.2" || dockerTool.Status != ecosystem.StatusDetected || dockerTool.Health != ecosystem.HealthHealthy {
		t.Errorf("expected Docker Engine version 28.5.2 healthy, got %+v", dockerTool)
	}

	// 2. PostgreSQL
	pgTool, ok := toolMap["PostgreSQL 16"]
	if !ok {
		t.Fatalf("PostgreSQL 16 missing")
	}
	if pgTool.Version != "16.4-alpine" || pgTool.Status != ecosystem.StatusDetected || pgTool.Health != ecosystem.HealthHealthy {
		t.Errorf("expected PostgreSQL 16 version 16.4-alpine healthy, got %+v", pgTool)
	}

	// 3. Redis
	redisTool, ok := toolMap["Redis 8"]
	if !ok {
		t.Fatalf("Redis 8 missing")
	}
	if redisTool.Version != "8.0-alpine" || redisTool.Status != ecosystem.StatusDetected || redisTool.Health != ecosystem.HealthHealthy {
		t.Errorf("expected Redis 8 version 8.0-alpine healthy, got %+v", redisTool)
	}

	// 4. NATS
	natsTool, ok := toolMap["NATS JetStream"]
	if !ok {
		t.Fatalf("NATS JetStream missing")
	}
	if natsTool.Version != "2.10.19-alpine" || natsTool.Status != ecosystem.StatusDetected || natsTool.Health != ecosystem.HealthHealthy {
		t.Errorf("expected NATS version 2.10.19-alpine healthy, got %+v", natsTool)
	}

	// 5. Traefik
	traefikTool, ok := toolMap["Traefik v3.1"]
	if !ok {
		t.Fatalf("Traefik v3.1 missing")
	}
	if traefikTool.Version != "v3.2.0" || traefikTool.Status != ecosystem.StatusDetected || traefikTool.Health != ecosystem.HealthHealthy {
		t.Errorf("expected Traefik version v3.2.0 healthy, got %+v", traefikTool)
	}

	// 6. Drone
	droneTool, ok := toolMap["Drone CI"]
	if !ok {
		t.Fatalf("Drone CI missing")
	}
	if droneTool.Version != "2.24.1" || droneTool.Status != ecosystem.StatusDetected || droneTool.Health != ecosystem.HealthHealthy {
		t.Errorf("expected Drone CI version 2.24.1 healthy, got %+v", droneTool)
	}

	// 7. Unmatched Kafka container
	kafkaTool, ok := toolMap["custom-kafka"]
	if !ok {
		t.Fatalf("custom-kafka container missing from detected tools")
	}
	if kafkaTool.Version != "3.6.1" || kafkaTool.Category != ecosystem.CategoryMessaging || kafkaTool.Status != ecosystem.StatusDetected {
		t.Errorf("expected custom-kafka version 3.6.1 messaging category, got %+v", kafkaTool)
	}

	// 8. Non-running tool with failed probe (Vault)
	vaultTool, ok := toolMap["Vault"]
	if !ok {
		t.Fatalf("Vault missing")
	}
	if vaultTool.Version != "unknown" || vaultTool.Status != ecosystem.StatusUnreachable || vaultTool.Health != ecosystem.HealthDegraded {
		t.Errorf("expected non-running Vault to be unreachable with version unknown, got %+v", vaultTool)
	}

	// 9. Unconfigured tool (Istio)
	istioTool, ok := toolMap["Istio"]
	if !ok {
		t.Fatalf("Istio missing")
	}
	if istioTool.Version != "unknown" || istioTool.Status != ecosystem.StatusNotConfigured || istioTool.Health != ecosystem.HealthUnknown {
		t.Errorf("expected unconfigured Istio to be not_configured with version unknown, got %+v", istioTool)
	}
}

func TestDetector_ExtractImageTag(t *testing.T) {
	mockDocker := &mockDockerClient{
		serverVer: "27.0.0",
		containers: []container.Summary{
			{Image: "redis:8-alpine", Names: []string{"/redis"}},
			{Image: "docker.io/library/postgres:16.3-alpine", Names: []string{"/postgres"}},
			{Image: "registry.example.com:5000/org/app:v1.2.3@sha256:abcdef", Names: []string{"/app"}},
			{Image: "nats", Names: []string{"/nats"}},
		},
	}

	ecoRepo := newMockEcosystemRepo()
	settingsRepo := &mockSettingsRepo{}
	uc := usecaseEcosystem.NewUsecase(
		ecoRepo,
		settingsRepo,
		http.DefaultClient,
		zap.NewNop(),
		usecaseEcosystem.WithDockerClient(mockDocker),
	)

	ctx := context.Background()
	tools, err := uc.Scan(ctx, "tag-tenant")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	toolMap := make(map[string]ecosystem.DetectedTool)
	for _, tool := range tools {
		toolMap[tool.Name] = tool
	}

	if r, ok := toolMap["Redis 8"]; !ok || r.Version != "8-alpine" {
		t.Errorf("expected Redis 8 version 8-alpine, got %+v", r)
	}
	if p, ok := toolMap["PostgreSQL 16"]; !ok || p.Version != "16.3-alpine" {
		t.Errorf("expected PostgreSQL 16 version 16.3-alpine, got %+v", p)
	}
	if a, ok := toolMap["app"]; !ok || a.Version != "v1.2.3" {
		t.Errorf("expected app version v1.2.3, got %+v", a)
	}
	if n, ok := toolMap["NATS JetStream"]; !ok || n.Version != "unknown" {
		t.Errorf("expected untagged NATS version unknown, got %+v", n)
	}
}
