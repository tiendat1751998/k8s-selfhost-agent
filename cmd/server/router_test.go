package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	mw "github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	gitproviderGitea "github.com/datdt/k8sselfhost/internal/infrastructure/gitprovider/gitea"
	gitproviderGithub "github.com/datdt/k8sselfhost/internal/infrastructure/gitprovider/github"
	gitproviderGitlab "github.com/datdt/k8sselfhost/internal/infrastructure/gitprovider/gitlab"
	"github.com/datdt/k8sselfhost/internal/infrastructure/helm"
	"github.com/datdt/k8sselfhost/internal/infrastructure/iac"
	"github.com/datdt/k8sselfhost/internal/infrastructure/telegram"
	usecaseDR "github.com/datdt/k8sselfhost/internal/usecase/dr"
	usecaseGitops "github.com/datdt/k8sselfhost/internal/usecase/gitops"
	usecaseSRE "github.com/datdt/k8sselfhost/internal/usecase/sre"
	usecasesec "github.com/datdt/k8sselfhost/internal/usecase/security"
	usecaseStorage "github.com/datdt/k8sselfhost/internal/usecase/storage"
	usecaseMesh "github.com/datdt/k8sselfhost/internal/usecase/mesh"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
)

func init() {
	const secret = "this-is-a-test-secret-at-least-32-chars-long"
	if os.Getenv("JWT_SECRET") == "" {
		_ = os.Setenv("JWT_SECRET", secret)
	}
	_ = mw.SetJWTSecret(secret)
}

func setupTestRouter() http.Handler {
	healthHandler := health.NewHandler(5 * time.Second)

	gitopsCtrl := usecaseGitops.NewController(nil, nil, nil)
	helmMgr := helm.NewReleaseManager(nil, nil)
	storageUc := usecaseStorage.NewVolumeUsecase(nil, nil, nil)
	drUc := usecaseDR.NewDRUsecase(nil, nil, nil)
	remediationCtrl := usecaseSRE.NewRemediationController(nil, nil, nil)
	tfRunner := iac.NewTerraformRunner("")
	ansibleRunner := iac.NewAnsibleRunner("")
	bot := telegram.NewSREBot(telegram.TelegramConfig{BotToken: "test-token"}, nil, nil, nil, nil)

	admissionEnforcer := usecasesec.NewAdmissionEnforcer()
	meshService := usecaseMesh.NewLifecycleService()

	handlers := &adapthttp.PlatformHandlers{
		Helm:      adapthttp.NewHelmHandler(helmMgr, nil),
		Storage:   adapthttp.NewStorageHandler(storageUc),
		DR:        adapthttp.NewDRHandler(drUc, remediationCtrl, nil),
		GitOps:    adapthttp.NewGitOpsHandler(gitopsCtrl),
		IaC:       adapthttp.NewIaCHandler(tfRunner, ansibleRunner),
		Telegram:  adapthttp.NewTelegramHandler(bot),
		Admission: adapthttp.NewAdmissionHandler(admissionEnforcer),
		Mesh:      adapthttp.NewMeshHandler(meshService),
		SRE:       bootstrapSREHandlers(platformHandlerParams{log: zap.NewNop()}),
	}
	handlers.SRE.Mesh = handlers.Mesh

	return newRouter(healthHandler, nil, handlers)
}

func TestRouter_MountedRoutes_UnauthorizedNotNotFound(t *testing.T) {
	router := setupTestRouter()

	endpoints := []struct {
		method string
		path   string
		desc   string
	}{
		{http.MethodGet, "/api/v1/helm/releases", "Helm releases route"},
		{http.MethodGet, "/api/v1/storage/volumes", "Storage volumes route"},
		{http.MethodGet, "/api/v1/dr/backups", "DR backups route"},
		{http.MethodGet, "/api/v1/gitops/prs", "GitOps pull requests route"},
		{http.MethodGet, "/api/v1/iac/status", "IaC status route"},
		{http.MethodGet, "/api/v1/telegram/status", "Telegram status route"},
		{http.MethodPost, "/api/v1/mesh/sidecar/inject-guard", "Mesh sidecar inject-guard route"},
		{http.MethodPost, "/api/v1/mesh/waf/verify-regex", "Mesh WAF verify-regex route"},
		{http.MethodPost, "/api/v1/mesh/tls/drain", "Mesh TLS drain route"},
		// SRE Resilience routes
		{http.MethodGet, "/api/v1/clusters/c1/storage/ceph/osds", "SRE Storage Resiliency"},
		{http.MethodGet, "/api/v1/kernel/nodes/node-1/cgroup-events", "SRE CGroup v2"},
		{http.MethodGet, "/api/v1/auth/jwks/status", "SRE JWKS Shield"},
		{http.MethodGet, "/api/v1/clusters/c1/fencing/readiness", "SRE Multi-DC Fencing"},
		{http.MethodGet, "/api/v1/secrets/drift/audit", "SRE Secret Drift"},
		{http.MethodGet, "/api/v1/ebpf/probes/pressure", "SRE eBPF Watchdog"},
		{http.MethodGet, "/api/v1/network/tunnels/health", "SRE NetGuard"},
		{http.MethodGet, "/api/v1/analytics/clickhouse/parts-health", "SRE IngestGuard"},
		{http.MethodGet, "/api/v1/runtime/nodes/storage-health", "SRE RuntimeGuard"},
		{http.MethodGet, "/api/v1/gpu/nodes/gpu-1/telemetry", "SRE GPUWatch"},
		{http.MethodGet, "/api/v1/dr/backup/locks", "SRE DRHealing"},
		{http.MethodGet, "/api/v1/finops/services/topology-audit", "SRE FinOpsGuard"},
		{http.MethodGet, "/api/v1/kernel/network/socket-metrics", "SRE Netscaler"},
		{http.MethodGet, "/api/v1/vfs/inodes/report", "SRE VFSWatchdog"},
		{http.MethodGet, "/api/v1/k8s/webhooks/audit", "SRE K8sResilience"},
		{http.MethodGet, "/api/v1/db/postgres/xid-health", "SRE DBSafeguard"},
		{http.MethodPost, "/api/v1/security/ssrf/validate-url", "SRE SecDefense"},
		{http.MethodGet, "/api/v1/cloud/storage/burst-status", "SRE CloudResilience"},
		{http.MethodGet, "/api/v1/mesh/sds/socket-status", "SRE ServiceMesh"},
		{http.MethodGet, "/api/v1/kernel/memory/numa-status", "SRE NUMAMemory"},
		{http.MethodGet, "/api/v1/hardware/ecc/dimm-status", "SRE Hardware"},
		{http.MethodGet, "/api/v1/crypto/hsm/session-status", "SRE Crypto"},
		{http.MethodPost, "/api/v1/gitops/sync-waves/audit-cycle", "SRE GitOpsSyncWave"},
		{http.MethodGet, "/api/v1/ai/rdma/fabric-bandwidth", "SRE RDMAWatch"},
		{http.MethodGet, "/api/v1/k8s/apf/queue-status", "SRE APFGuard"},
		{http.MethodGet, "/api/v1/cloud/network/nlb/cross-zone-balance", "SRE TransitNet"},
		{http.MethodGet, "/api/v1/identity/spiffe/svid-status", "SRE SPIFFEGuard"},
		{http.MethodPost, "/api/v1/consensus/locks/acquire-fencing", "SRE Consensus"},
		{http.MethodPost, "/api/v1/workload/storage/validate-ephemeral", "SRE WorkloadGuard"},
		{http.MethodGet, "/api/v1/messaging/tenants/t-1/backlog-alerts", "SRE Messaging"},
	}

	for _, ep := range endpoints {
		t.Run(ep.desc, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.NotEqualf(t, http.StatusNotFound, rec.Code,
				"expected %s %s to be mounted, but got 404 Not Found", ep.method, ep.path)
			assert.Equalf(t, http.StatusUnauthorized, rec.Code,
				"expected %s %s to reject unauthenticated request with 401 Unauthorized, got %d", ep.method, ep.path, rec.Code)
		})
	}

	t.Run("unmapped route returns 404", func(t *testing.T) {
		token, err := mw.GenerateToken("test-user", "platform_admin", "default")
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/nonexistent-endpoint-xyz", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestGitOpsController_ProvidersRegistered(t *testing.T) {
	controller := usecaseGitops.NewController(nil, nil, nil)

	ghProvider := gitproviderGithub.NewProvider("test-gh-token")
	glProvider := gitproviderGitlab.NewProvider("test-gl-token", "https://gitlab.example.com")
	giteaProvider := gitproviderGitea.NewProvider("test-gitea-token", "https://gitea.example.com")

	controller.RegisterProvider(domainGitops.ProviderGitHub, ghProvider)
	controller.RegisterProvider(domainGitops.ProviderGitLab, glProvider)
	controller.RegisterProvider(domainGitops.ProviderGitea, giteaProvider)

	assert.True(t, controller.HasProvider(domainGitops.ProviderGitHub), "expected GitHub provider to be registered")
	assert.True(t, controller.HasProvider(domainGitops.ProviderGitLab), "expected GitLab provider to be registered")
	assert.True(t, controller.HasProvider(domainGitops.ProviderGitea), "expected Gitea provider to be registered")
}

func TestPlatformHandlers_WiringFieldsParity(t *testing.T) {
	gitopsCtrl := usecaseGitops.NewController(nil, nil, nil)
	helmMgr := helm.NewReleaseManager(nil, nil)
	storageUc := usecaseStorage.NewVolumeUsecase(nil, nil, nil)
	drUc := usecaseDR.NewDRUsecase(nil, nil, nil)
	remediationCtrl := usecaseSRE.NewRemediationController(nil, nil, nil)
	tfRunner := iac.NewTerraformRunner("")
	ansibleRunner := iac.NewAnsibleRunner("")
	bot := telegram.NewSREBot(telegram.TelegramConfig{BotToken: "test-token"}, nil, nil, nil, nil)

	handlers := &adapthttp.PlatformHandlers{
		Helm:      adapthttp.NewHelmHandler(helmMgr, nil),
		Storage:   adapthttp.NewStorageHandler(storageUc),
		DR:        adapthttp.NewDRHandler(drUc, remediationCtrl, nil),
		GitOps:    adapthttp.NewGitOpsHandler(gitopsCtrl),
		IaC:       adapthttp.NewIaCHandler(tfRunner, ansibleRunner),
		Telegram:  adapthttp.NewTelegramHandler(bot),
		Admission: adapthttp.NewAdmissionHandler(usecasesec.NewAdmissionEnforcer()),
	}

	require.NotNil(t, handlers.Helm, "Helm handler must be initialized")
	require.NotNil(t, handlers.Storage, "Storage handler must be initialized")
	require.NotNil(t, handlers.DR, "DR handler must be initialized")
	require.NotNil(t, handlers.GitOps, "GitOps handler must be initialized")
	require.NotNil(t, handlers.IaC, "IaC handler must be initialized")
	require.NotNil(t, handlers.Telegram, "Telegram handler must be initialized")
	require.NotNil(t, handlers.Admission, "Admission handler must be initialized")
}

func TestRouter_AdmissionWebhook_Mounted(t *testing.T) {
	router := setupTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/admission/validate", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	// Route must be mounted and respond with 400 Bad Request on empty body (not 404 Not Found)
	assert.NotEqual(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
