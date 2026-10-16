package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	mw "github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
	usecaseGitops "github.com/datdt/k8sselfhost/internal/usecase/gitops"
)

func TestParseDockerLogLine(t *testing.T) {
	t.Run("RFC3339Nano timestamp prefix", func(t *testing.T) {
		input := "2026-09-16T05:14:12.123456789Z database system is ready"
		parsedTime, msg := parseDockerLogLine(input)

		expectedTime := time.Date(2026, 9, 16, 5, 14, 12, 123456789, time.UTC)
		assert.Equal(t, expectedTime, parsedTime)
		assert.Equal(t, "database system is ready", msg)
	})

	t.Run("RFC3339 standard timestamp prefix", func(t *testing.T) {
		input := "2026-09-16T05:14:12Z worker active"
		parsedTime, msg := parseDockerLogLine(input)

		expectedTime := time.Date(2026, 9, 16, 5, 14, 12, 0, time.UTC)
		assert.Equal(t, expectedTime, parsedTime)
		assert.Equal(t, "worker active", msg)
	})

	t.Run("RFC3339Nano with offset", func(t *testing.T) {
		input := "2026-09-16T07:14:12.123456789+02:00 server started"
		parsedTime, msg := parseDockerLogLine(input)

		expectedTime := time.Date(2026, 9, 16, 5, 14, 12, 123456789, time.UTC)
		assert.Equal(t, expectedTime, parsedTime)
		assert.Equal(t, "server started", msg)
	})

	t.Run("no timestamp prefix fallback", func(t *testing.T) {
		input := "plain log line without docker timestamp"
		before := time.Now().UTC().Add(-time.Second)
		parsedTime, msg := parseDockerLogLine(input)
		after := time.Now().UTC().Add(time.Second)

		assert.True(t, parsedTime.After(before) && parsedTime.Before(after))
		assert.Equal(t, input, msg)
	})
}

func TestStandalone_PostgresPoolFromConfig(t *testing.T) {
	// Verify that standalone pool settings come from config (cfg.Postgres)
	pgCfg := config.PostgresConfig{
		Host:     "10.0.0.5",
		Port:     5433,
		User:     "k8s_user",
		Password: "secret",
		DBName:   "production_db",
		SSLMode:  "disable",
		MaxConns: 35,
		MinConns: 5,
	}

	dsn := pgCfg.DSN()
	assert.Contains(t, dsn, "postgres://k8s_user:secret@10.0.0.5:5433/production_db")
	assert.Equal(t, int32(35), pgCfg.MaxConns)
	assert.Equal(t, int32(5), pgCfg.MinConns)
}

func TestStandalone_AuxAndSREHandlersWired(t *testing.T) {
	cfg := &config.Config{
		GitOps: config.GitOpsConfig{
			GitHubToken: "gh_test_token",
			GitLabToken: "gl_test_token",
			GitLabURL:   "https://gitlab.example.com",
			GiteaToken:  "gt_test_token",
			GiteaURL:    "https://gitea.example.com",
		},
	}
	gitopsCtrl := usecaseGitops.NewController(nil, nil, nil)
	registerGitopsProviders(gitopsCtrl, cfg)

	aux := initStandaloneAuxHandlers(cfg, nil, nil, nil, nil, gitopsCtrl)

	assert.NotNil(t, aux.Canary, "Canary handler must be wired")
	assert.NotNil(t, aux.Mesh, "Mesh handler must be wired")
	assert.NotNil(t, aux.GitOps, "GitOps handler must be wired")
	assert.NotNil(t, aux.IaC, "IaC handler must be wired")
	assert.NotNil(t, aux.Telegram, "Telegram handler must be wired")
	assert.NotNil(t, aux.Admission, "Admission handler must be wired")
	assert.NotNil(t, aux.MessagingSRE, "MessagingSRE handler must be wired")
	assert.NotNil(t, aux.SRE, "SRE resilience handlers must be wired")

	// Verify all critical SRE resilience sub-handlers are initialized
	assert.NotNil(t, aux.SRE.StorageResiliency)
	assert.NotNil(t, aux.SRE.StorageHealing)
	assert.NotNil(t, aux.SRE.CGroup)
	assert.NotNil(t, aux.SRE.Vault)
	assert.NotNil(t, aux.SRE.JWKS)
	assert.NotNil(t, aux.SRE.Fencing)
	assert.NotNil(t, aux.SRE.SecretDrift)
	assert.NotNil(t, aux.SRE.EBPFWatch)
	assert.NotNil(t, aux.SRE.NetGuard)
	assert.NotNil(t, aux.SRE.IngestGuard)
	assert.NotNil(t, aux.SRE.RuntimeGuard)
	assert.NotNil(t, aux.SRE.GPUWatch)
	assert.NotNil(t, aux.SRE.DRHealing)
	assert.NotNil(t, aux.SRE.FinOpsGuard)
	assert.NotNil(t, aux.SRE.Netscaler)
	assert.NotNil(t, aux.SRE.VFSWatchdog)
	assert.NotNil(t, aux.SRE.K8sResilience)
	assert.NotNil(t, aux.SRE.DBSafeguard)
	assert.NotNil(t, aux.SRE.SecDefense)
	assert.NotNil(t, aux.SRE.CloudResilience)
	assert.NotNil(t, aux.SRE.ServiceMesh)
	assert.NotNil(t, aux.SRE.NUMAMemory)
	assert.NotNil(t, aux.SRE.Hardware)
	assert.NotNil(t, aux.SRE.Crypto)
	assert.NotNil(t, aux.SRE.GitOpsSyncWave)
	assert.NotNil(t, aux.SRE.RDMAWatch)
	assert.NotNil(t, aux.SRE.APFGuard)
	assert.NotNil(t, aux.SRE.TransitNet)
	assert.NotNil(t, aux.SRE.SPIFFEGuard)
	assert.NotNil(t, aux.SRE.Consensus)
	assert.NotNil(t, aux.SRE.WorkloadGuard)
	assert.NotNil(t, aux.SRE.Messaging)
}

func TestStandalone_SettingsWired(t *testing.T) {
	settingsRepo := postgres.NewSettingsRepo(nil)
	settingsHandler := adapthttp.NewSettingsHandler(settingsRepo, nil)
	assert.NotNil(t, settingsHandler, "SettingsHandler must not be nil")

	platform := &adapthttp.PlatformHandlers{
		Settings: settingsHandler,
	}
	healthHandler := health.NewHandler(time.Second)
	router := adapthttp.NewRouterWithPlatform(healthHandler, nil, platform)
	assert.NotNil(t, router)

	token, err := mw.GenerateToken("admin", "platform_admin", "default-tenant")
	assert.NoError(t, err)

	endpoints := []string{
		"/api/v1/settings",
		"/api/v1/settings/defaults",
		"/api/v1/settings/api-keys",
		"/api/v1/settings/keys",
		"/api/v1/apikeys",
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(http.MethodGet, ep, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.NotEqual(t, http.StatusNotFound, rec.Code, "endpoint %s must be mounted", ep)
	}
}

type standaloneMockHostRepo struct {
	host docker.ComputeHost
}

func (m *standaloneMockHostRepo) Create(ctx context.Context, host *docker.ComputeHost) error { return nil }
func (m *standaloneMockHostRepo) GetByID(ctx context.Context, id string) (*docker.ComputeHost, error) {
	return &m.host, nil
}
func (m *standaloneMockHostRepo) List(ctx context.Context, tenantID string) ([]docker.ComputeHost, error) {
	return []docker.ComputeHost{m.host}, nil
}
func (m *standaloneMockHostRepo) ListAll(ctx context.Context) ([]docker.ComputeHost, error) {
	return []docker.ComputeHost{m.host}, nil
}
func (m *standaloneMockHostRepo) Update(ctx context.Context, host *docker.ComputeHost) error { return nil }
func (m *standaloneMockHostRepo) Delete(ctx context.Context, id string) error               { return nil }
func (m *standaloneMockHostRepo) UpdateStatus(ctx context.Context, id string, status string, lastHealthCheck time.Time) error {
	return nil
}

func TestStandalone_RemediationEndpointsMounted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/k8s/health":
			_, _ = w.Write([]byte(`{"node_name":"test-node","kubelet_healthy":true,"containerd_healthy":true}`))
		case "/remediate":
			_, _ = w.Write([]byte(`{"success":true,"action":"restart_service","details":"done"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	mockRepo := &standaloneMockHostRepo{
		host: docker.ComputeHost{
			ID:       "host-1",
			Name:     "node-1",
			Endpoint: server.URL,
		},
	}

	remHandler := adapthttp.NewHostRemediationHandler(mockRepo, nil, nil)
	dockerHandler := adapthttp.NewDockerHandler(nil, mockRepo, remHandler)
	platform := &adapthttp.PlatformHandlers{
		Docker:          dockerHandler,
		HostRemediation: remHandler,
	}
	healthHandler := health.NewHandler(time.Second)
	router := adapthttp.NewRouterWithPlatform(healthHandler, nil, platform)
	assert.NotNil(t, router)

	token, err := mw.GenerateToken("admin", "platform_admin", "default-tenant")
	assert.NoError(t, err)

	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/hosts/host-1/k8s-health", ""},
		{http.MethodPost, "/api/v1/hosts/host-1/remediate", `{"action":"restart_service","target":"kubelet"}`},
		{http.MethodPost, "/api/v1/k8s/nodes/node-1/remediate", `{"action":"restart_service","target":"kubelet"}`},
	}

	for _, ep := range endpoints {
		var req *http.Request
		if ep.body != "" {
			req = httptest.NewRequest(ep.method, ep.path, strings.NewReader(ep.body))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req = httptest.NewRequest(ep.method, ep.path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "endpoint %s %s must return 200 OK, got body: %s", ep.method, ep.path, rec.Body.String())
	}
}


