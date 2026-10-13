package main

import (
	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
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

// bootstrapSREHandlers instantiates all 30 SRE resilience managers and handlers.
func bootstrapSREHandlers(p platformHandlerParams) *adapthttp.SREResilienceHandlers {
	storageMgr := usecaseStorage.NewResiliencyManager(p.log)
	cgroupScaler := usecaseNodeMetrics.NewCGroupScalerService(usecaseNodeMetrics.WithLogger(p.log))
	jwksShield := usecaseAuth.NewJWKSShieldManager()
	fencingMgr := usecaseCluster.NewFencingManager(nil, nil, nil, nil, p.log)
	driftReconciler := usecaseSecretGuard.NewDriftReconciler(nil, nil)
	ebpfMgr := usecaseEBPFWatch.NewWatchdogManager(usecaseEBPFWatch.WithLogger(p.log))
	netguardMgr := usecaseNetGuard.NewTunnelManager(usecaseNetGuard.WithLogger(p.log))
	ingestMgr := usecaseIngestGuard.NewIngestManager(domainIngestGuard.DefaultBatchFlushConfig())
	runtimeMgr := usecaseRuntimeGuard.NewResilienceManager()
	gpuMgr := usecaseGPUWatch.NewGPUManagerService(nil, nil)
	drHealingMgr := usecaseDRHealing.NewHealingManager(nil, nil, nil, nil, nil)
	finopsMgr := usecaseFinOpsGuard.NewFinOpsManager(nil, nil, nil, nil)
	netscalerMgr := usecaseNetscaler.NewScalerManager(usecaseNetscaler.WithLogger(p.log))
	vfsMgr := usecaseVFSWatchdog.NewVFSManager(usecaseVFSWatchdog.WithLogger(p.log))
	k8sResilienceMgr := usecaseK8sResilience.NewResilienceManager(nil, p.log)
	dbSafeguardMgr := usecaseDBSafeguard.NewSafeguardManager(nil, nil, nil, nil)
	secDefenseMgr := usecaseSecDefense.NewDefenseManager(nil)
	cloudResilienceMgr := usecaseCloudResilience.NewResilienceManager(usecaseCloudResilience.WithLogger(p.log))
	meshResilienceMgr := usecaseServiceMesh.NewMeshManager(usecaseServiceMesh.WithLogger(p.log))
	numaMgr := usecaseNUMAMemory.NewNUMAMemoryManager(usecaseNUMAMemory.WithLogger(p.log))
	hardwareMgr := usecaseHardware.NewHardwareManager(usecaseHardware.DefaultConfig(), nil, nil, nil, nil, nil, nil, p.log)
	cryptoMgr := usecaseCrypto.NewShieldManager(usecaseCrypto.ShieldManagerOptions{})
	syncwaveMgr := usecaseGitops.NewSyncWaveManager()
	rdmaMgr := usecaseRDMAWatch.NewFabricManager(nil, nil, nil, nil)
	apfMgr := usecaseAPFGuard.NewAPFManager(nil, p.log)
	transitNetMgr := usecaseTransitNet.NewRoutingManager(usecaseTransitNet.WithLogger(p.log))
	spiffeMgr := usecaseSPIFFEGuard.NewIdentityManager(usecaseSPIFFEGuard.IdentityManagerConfig{})
	consensusMgr := usecaseConsensus.NewCoordinatorManager(nil, nil)
	workloadMgr := usecaseWorkloadGuard.NewArtifactManager()
	messagingMgr := usecaseMessaging.NewManager(nil, nil, nil, usecaseMessaging.DefaultSREManagerConfig())

	storageResiliencyHandler := adapthttp.NewStorageResiliencyHandler(storageMgr)

	return &adapthttp.SREResilienceHandlers{
		StorageResiliency: storageResiliencyHandler,
		StorageHealing:    storageResiliencyHandler,
		CGroup:            adapthttp.NewCGroupHandler(cgroupScaler, p.log),
		Vault:             adapthttp.NewJWKSHandler(jwksShield),
		JWKS:              adapthttp.NewJWKSHandler(jwksShield),
		Fencing:           adapthttp.NewFencingHandler(fencingMgr, p.log),
		SecretDrift:       adapthttp.NewSecretDriftHandler(driftReconciler, nil),
		EBPFWatch:         adapthttp.NewEBPFWatchHandler(ebpfMgr, p.log),
		NetGuard:          adapthttp.NewNetGuardHandler(netguardMgr, p.log),
		IngestGuard:       adapthttp.NewIngestGuardHandler(ingestMgr, p.log),
		RuntimeGuard:      adapthttp.NewRuntimeGuardHandler(runtimeMgr, p.log),
		GPUWatch:          adapthttp.NewGPUWatchHandler(gpuMgr, p.log),
		DRHealing:         adapthttp.NewDRHealingHandler(drHealingMgr, p.log),
		FinOpsGuard:       adapthttp.NewFinOpsGuardHandler(finopsMgr),
		Netscaler:         adapthttp.NewNetscalerHandler(netscalerMgr, p.log),
		VFSWatchdog:       adapthttp.NewVFSWatchdogHandler(vfsMgr, p.log),
		K8sResilience:     adapthttp.NewK8sResilienceHandler(k8sResilienceMgr, p.log),
		DBSafeguard:       adapthttp.NewDBSafeguardHandler(dbSafeguardMgr),
		SecDefense:        adapthttp.NewSecDefenseHandler(secDefenseMgr),
		CloudResilience:   adapthttp.NewCloudResilienceHandler(cloudResilienceMgr, p.log),
		ServiceMesh:       adapthttp.NewServiceMeshHandler(meshResilienceMgr, p.log),
		Mesh:              p.meshHandler,
		NUMAMemory:        adapthttp.NewNUMAMemoryHandler(numaMgr, p.log),
		Hardware:          adapthttp.NewHardwareHandler(hardwareMgr, p.log),
		Crypto:            adapthttp.NewCryptoHandler(cryptoMgr),
		GitOpsSyncWave:    adapthttp.NewGitOpsSyncWaveHandler(syncwaveMgr),
		RDMAWatch:         adapthttp.NewRDMAWatchHandler(rdmaMgr, p.log),
		APFGuard:          adapthttp.NewAPFGuardHandler(apfMgr, p.log),
		TransitNet:        adapthttp.NewTransitNetHandler(transitNetMgr, p.log),
		SPIFFEGuard:       adapthttp.NewSPIFFEGuardHandler(spiffeMgr),
		Consensus:         adapthttp.NewConsensusHandler(consensusMgr),
		WorkloadGuard:     adapthttp.NewWorkloadGuardHandler(workloadMgr, p.log),
		Messaging:         adapthttp.NewMessagingHandler(messagingMgr),
	}
}
