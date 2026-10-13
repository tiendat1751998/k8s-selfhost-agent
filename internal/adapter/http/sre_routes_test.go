package http_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	mw "github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	domainIngestGuard "github.com/datdt/k8sselfhost/internal/domain/ingestguard"
	usecaseAPFGuard "github.com/datdt/k8sselfhost/internal/usecase/apfguard"
	usecaseAuth "github.com/datdt/k8sselfhost/internal/usecase/auth"
	usecaseCloudResilience "github.com/datdt/k8sselfhost/internal/usecase/cloudresilience"
	usecaseCluster "github.com/datdt/k8sselfhost/internal/usecase/cluster"
	usecaseConsensus "github.com/datdt/k8sselfhost/internal/usecase/consensus"
	usecaseCrypto "github.com/datdt/k8sselfhost/internal/usecase/crypto"
	usecaseDBSafeguard "github.com/datdt/k8sselfhost/internal/usecase/dbsafeguard"
	usecaseDRHealing "github.com/datdt/k8sselfhost/internal/usecase/drhealing"
	usecaseEBPFWatch "github.com/datdt/k8sselfhost/internal/usecase/ebpfwatch"
	usecaseFinOpsGuard "github.com/datdt/k8sselfhost/internal/usecase/finopsguard"
	usecaseGitops "github.com/datdt/k8sselfhost/internal/usecase/gitops"
	usecaseGPUWatch "github.com/datdt/k8sselfhost/internal/usecase/gpuwatch"
	usecaseHardware "github.com/datdt/k8sselfhost/internal/usecase/hardware"
	usecaseIngestGuard "github.com/datdt/k8sselfhost/internal/usecase/ingestguard"
	usecaseK8sResilience "github.com/datdt/k8sselfhost/internal/usecase/k8sresilience"
	usecaseMessaging "github.com/datdt/k8sselfhost/internal/usecase/messaging"
	usecaseNetGuard "github.com/datdt/k8sselfhost/internal/usecase/netguard"
	usecaseNetscaler "github.com/datdt/k8sselfhost/internal/usecase/netscaler"
	usecaseNodeMetrics "github.com/datdt/k8sselfhost/internal/usecase/nodemetrics"
	usecaseNUMAMemory "github.com/datdt/k8sselfhost/internal/usecase/numamemory"
	usecaseRDMAWatch "github.com/datdt/k8sselfhost/internal/usecase/rdmawatch"
	usecaseRuntimeGuard "github.com/datdt/k8sselfhost/internal/usecase/runtimeguard"
	usecaseSecDefense "github.com/datdt/k8sselfhost/internal/usecase/secdefense"
	usecaseSecretGuard "github.com/datdt/k8sselfhost/internal/usecase/secretguard"
	usecaseServiceMesh "github.com/datdt/k8sselfhost/internal/usecase/servicemesh"
	usecaseSPIFFEGuard "github.com/datdt/k8sselfhost/internal/usecase/spiffeguard"
	usecaseStorage "github.com/datdt/k8sselfhost/internal/usecase/storage"
	usecaseTransitNet "github.com/datdt/k8sselfhost/internal/usecase/transitnet"
	usecaseVFSWatchdog "github.com/datdt/k8sselfhost/internal/usecase/vfswatchdog"
	usecaseWorkloadGuard "github.com/datdt/k8sselfhost/internal/usecase/workloadguard"
)

func init() {
	const secret = "this-is-a-test-secret-at-least-32-chars-long"
	if os.Getenv("JWT_SECRET") == "" {
		_ = os.Setenv("JWT_SECRET", secret)
	}
	_ = mw.SetJWTSecret(secret)
}

func setupTestSREResilienceHandlers() *adapthttp.SREResilienceHandlers {
	nopLogger := zap.NewNop()

	storageMgr := usecaseStorage.NewResiliencyManager(nopLogger)
	cgroupScaler := usecaseNodeMetrics.NewCGroupScalerService(usecaseNodeMetrics.WithLogger(nopLogger))
	jwksShield := usecaseAuth.NewJWKSShieldManager()
	fencingMgr := usecaseCluster.NewFencingManager(nil, nil, nil, nil, nopLogger)
	driftReconciler := usecaseSecretGuard.NewDriftReconciler(nil, nil)
	ebpfMgr := usecaseEBPFWatch.NewWatchdogManager(usecaseEBPFWatch.WithLogger(nopLogger))
	netguardMgr := usecaseNetGuard.NewTunnelManager(usecaseNetGuard.WithLogger(nopLogger))
	ingestMgr := usecaseIngestGuard.NewIngestManager(domainIngestGuard.DefaultBatchFlushConfig())
	runtimeMgr := usecaseRuntimeGuard.NewResilienceManager()
	gpuMgr := usecaseGPUWatch.NewGPUManagerService(nil, nil)
	drHealingMgr := usecaseDRHealing.NewHealingManager(nil, nil, nil, nil, nil)
	finopsMgr := usecaseFinOpsGuard.NewFinOpsManager(nil, nil, nil, nil)
	netscalerMgr := usecaseNetscaler.NewScalerManager(usecaseNetscaler.WithLogger(nopLogger))
	vfsMgr := usecaseVFSWatchdog.NewVFSManager(usecaseVFSWatchdog.WithLogger(nopLogger))
	k8sResilienceMgr := usecaseK8sResilience.NewResilienceManager(nil, nopLogger)
	dbSafeguardMgr := usecaseDBSafeguard.NewSafeguardManager(nil, nil, nil, nil)
	secDefenseMgr := usecaseSecDefense.NewDefenseManager(nil)
	cloudResilienceMgr := usecaseCloudResilience.NewResilienceManager(usecaseCloudResilience.WithLogger(nopLogger))
	meshResilienceMgr := usecaseServiceMesh.NewMeshManager(usecaseServiceMesh.WithLogger(nopLogger))
	numaMgr := usecaseNUMAMemory.NewNUMAMemoryManager(usecaseNUMAMemory.WithLogger(nopLogger))
	hardwareMgr := usecaseHardware.NewHardwareManager(usecaseHardware.DefaultConfig(), nil, nil, nil, nil, nil, nil, nopLogger)
	cryptoMgr := usecaseCrypto.NewShieldManager(usecaseCrypto.ShieldManagerOptions{})
	syncwaveMgr := usecaseGitops.NewSyncWaveManager()
	rdmaMgr := usecaseRDMAWatch.NewFabricManager(nil, nil, nil, nil)
	apfMgr := usecaseAPFGuard.NewAPFManager(nil, nopLogger)
	transitNetMgr := usecaseTransitNet.NewRoutingManager(usecaseTransitNet.WithLogger(nopLogger))
	spiffeMgr := usecaseSPIFFEGuard.NewIdentityManager(usecaseSPIFFEGuard.IdentityManagerConfig{})
	consensusMgr := usecaseConsensus.NewCoordinatorManager(nil, nil)
	workloadMgr := usecaseWorkloadGuard.NewArtifactManager()
	messagingMgr := usecaseMessaging.NewManager(nil, nil, nil, usecaseMessaging.DefaultSREManagerConfig())

	storageHandler := adapthttp.NewStorageResiliencyHandler(storageMgr)

	return &adapthttp.SREResilienceHandlers{
		StorageResiliency: storageHandler,
		StorageHealing:    storageHandler,
		CGroup:            adapthttp.NewCGroupHandler(cgroupScaler, nopLogger),
		Vault:             adapthttp.NewJWKSHandler(jwksShield),
		JWKS:              adapthttp.NewJWKSHandler(jwksShield),
		Fencing:           adapthttp.NewFencingHandler(fencingMgr, nopLogger),
		SecretDrift:       adapthttp.NewSecretDriftHandler(driftReconciler, nil),
		EBPFWatch:         adapthttp.NewEBPFWatchHandler(ebpfMgr, nopLogger),
		NetGuard:          adapthttp.NewNetGuardHandler(netguardMgr, nopLogger),
		IngestGuard:       adapthttp.NewIngestGuardHandler(ingestMgr, nopLogger),
		RuntimeGuard:      adapthttp.NewRuntimeGuardHandler(runtimeMgr, nopLogger),
		GPUWatch:          adapthttp.NewGPUWatchHandler(gpuMgr, nopLogger),
		DRHealing:         adapthttp.NewDRHealingHandler(drHealingMgr, nopLogger),
		FinOpsGuard:       adapthttp.NewFinOpsGuardHandler(finopsMgr),
		Netscaler:         adapthttp.NewNetscalerHandler(netscalerMgr, nopLogger),
		VFSWatchdog:       adapthttp.NewVFSWatchdogHandler(vfsMgr, nopLogger),
		K8sResilience:     adapthttp.NewK8sResilienceHandler(k8sResilienceMgr, nopLogger),
		DBSafeguard:       adapthttp.NewDBSafeguardHandler(dbSafeguardMgr),
		SecDefense:        adapthttp.NewSecDefenseHandler(secDefenseMgr),
		CloudResilience:   adapthttp.NewCloudResilienceHandler(cloudResilienceMgr, nopLogger),
		ServiceMesh:       adapthttp.NewServiceMeshHandler(meshResilienceMgr, nopLogger),
		NUMAMemory:        adapthttp.NewNUMAMemoryHandler(numaMgr, nopLogger),
		Hardware:          adapthttp.NewHardwareHandler(hardwareMgr, nopLogger),
		Crypto:            adapthttp.NewCryptoHandler(cryptoMgr),
		GitOpsSyncWave:    adapthttp.NewGitOpsSyncWaveHandler(syncwaveMgr),
		RDMAWatch:         adapthttp.NewRDMAWatchHandler(rdmaMgr, nopLogger),
		APFGuard:          adapthttp.NewAPFGuardHandler(apfMgr, nopLogger),
		TransitNet:        adapthttp.NewTransitNetHandler(transitNetMgr, nopLogger),
		SPIFFEGuard:       adapthttp.NewSPIFFEGuardHandler(spiffeMgr),
		Consensus:         adapthttp.NewConsensusHandler(consensusMgr),
		WorkloadGuard:     adapthttp.NewWorkloadGuardHandler(workloadMgr, nopLogger),
		Messaging:         adapthttp.NewMessagingHandler(messagingMgr),
	}
}

func TestSRERoutes_All30HandlersMounted(t *testing.T) {
	r := chi.NewRouter()
	handlers := setupTestSREResilienceHandlers()

	r.Route("/api/v1", func(sub chi.Router) {
		adapthttp.RegisterSRERoutes(sub, handlers)
	})

	endpoints := []struct {
		method string
		path   string
		body   string
		desc   string
	}{
		{http.MethodGet, "/api/v1/clusters/c1/storage/ceph/osds", "", "Storage Resiliency"},
		{http.MethodGet, "/api/v1/kernel/nodes/node-1/cgroup-events", "", "CGroup v2"},
		{http.MethodGet, "/api/v1/auth/jwks/status", "", "JWKS Shield"},
		{http.MethodGet, "/api/v1/vault/unseal-status", "", "Vault Unseal"},
		{http.MethodGet, "/api/v1/clusters/c1/fencing/readiness", "", "Multi-DC Fencing"},
		{http.MethodGet, "/api/v1/secrets/drift/audit", "", "Secret Store Drift"},
		{http.MethodGet, "/api/v1/ebpf/probes/pressure", "", "eBPF RingBuffer Watchdog"},
		{http.MethodGet, "/api/v1/network/tunnels/health", "", "Hybrid WAN NetGuard"},
		{http.MethodGet, "/api/v1/analytics/clickhouse/parts-health", "", "ClickHouse Parts IngestGuard"},
		{http.MethodGet, "/api/v1/runtime/nodes/storage-health", "", "RuntimeGuard Storage"},
		{http.MethodGet, "/api/v1/gpu/nodes/gpu-1/telemetry", "", "GPU AI Workload"},
		{http.MethodGet, "/api/v1/dr/backup/locks", "", "DR Velero Locks"},
		{http.MethodGet, "/api/v1/finops/services/topology-audit", "", "FinOps Cross-AZ Optimizer"},
		{http.MethodGet, "/api/v1/kernel/network/socket-metrics", "", "Kernel Socket Netscaler"},
		{http.MethodGet, "/api/v1/vfs/inodes/report", "", "VFS Inode Watchdog"},
		{http.MethodGet, "/api/v1/k8s/webhooks/audit", "", "K8s Webhook Deadlock Breaker"},
		{http.MethodGet, "/api/v1/db/postgres/xid-health", "", "Database XID Safeguard"},
		{http.MethodPost, "/api/v1/security/ssrf/validate-url", `{"url":"https://example.com"}`, "Security SSRF Defense"},
		{http.MethodGet, "/api/v1/cloud/storage/burst-status", "", "Cloud Burst Resilience"},
		{http.MethodGet, "/api/v1/mesh/sds/socket-status", "", "Service Mesh SDS"},
		{http.MethodGet, "/api/v1/kernel/memory/numa-status", "", "NUMA Memory Tuning"},
		{http.MethodGet, "/api/v1/hardware/ecc/dimm-status", "", "Bare-Metal Hardware EDAC"},
		{http.MethodGet, "/api/v1/crypto/hsm/session-status", "", "Crypto HSM Session Pool"},
		{http.MethodPost, "/api/v1/gitops/sync-waves/audit-cycle", `{"resources":[]}`, "GitOps Sync-Wave DAG"},
		{http.MethodGet, "/api/v1/ai/rdma/fabric-bandwidth", "", "RoCEv2 PFC RDMA Watch"},
		{http.MethodGet, "/api/v1/k8s/apf/queue-status", "", "K8s APF Guard"},
		{http.MethodGet, "/api/v1/cloud/network/nlb/cross-zone-balance", "", "Transit Network NLB"},
		{http.MethodGet, "/api/v1/identity/spiffe/svid-status", "", "SPIFFE Identity Guard"},
		{http.MethodPost, "/api/v1/consensus/locks/acquire-fencing", `{"cluster_id":"c1","lock_key":"l1","fence_token":1,"lease_duration_ms":1000}`, "Consensus STONITH Lock"},
		{http.MethodPost, "/api/v1/workload/storage/validate-ephemeral", `{"pod_name":"p1","namespace":"default","volumes":[]}`, "Workload Ephemeral Storage Guard"},
		{http.MethodGet, "/api/v1/messaging/tenants/t-1/backlog-alerts", "", "Messaging SRE"},
	}

	for _, ep := range endpoints {
		t.Run(ep.desc, func(t *testing.T) {
			var bodyReader *bytes.Reader
			if ep.body != "" {
				bodyReader = bytes.NewReader([]byte(ep.body))
			} else {
				bodyReader = bytes.NewReader(nil)
			}
			req := httptest.NewRequest(ep.method, ep.path, bodyReader)
			if ep.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.NotEqualf(t, http.StatusNotFound, rec.Code,
				"expected %s %s to be mounted, but got 404 Not Found", ep.method, ep.path)
			assert.NotEqualf(t, http.StatusMethodNotAllowed, rec.Code,
				"expected %s %s to match method, but got 405 Method Not Allowed", ep.method, ep.path)
		})
	}

	t.Run("nil handlers does not panic", func(t *testing.T) {
		nilR := chi.NewRouter()
		require.NotPanics(t, func() {
			adapthttp.RegisterSRERoutes(nilR, nil)
		})
	})
}
