package main

import (
	"os"

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domainBackup "github.com/datdt/k8sselfhost/internal/domain/backup"
	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	domainIngestGuard "github.com/datdt/k8sselfhost/internal/domain/ingestguard"
	infraBackup "github.com/datdt/k8sselfhost/internal/infrastructure/backup"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/drivers"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/storage"
	infraClickhouse "github.com/datdt/k8sselfhost/internal/infrastructure/clickhouse"
	infraCluster "github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	gitproviderGitea "github.com/datdt/k8sselfhost/internal/infrastructure/gitprovider/gitea"
	gitproviderGithub "github.com/datdt/k8sselfhost/internal/infrastructure/gitprovider/github"
	gitproviderGitlab "github.com/datdt/k8sselfhost/internal/infrastructure/gitprovider/gitlab"
	"github.com/datdt/k8sselfhost/internal/infrastructure/iac"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
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
	usecaseMesh "github.com/datdt/k8sselfhost/internal/usecase/mesh"
	usecaseMessaging "github.com/datdt/k8sselfhost/internal/usecase/messaging"
	usecaseNetGuard "github.com/datdt/k8sselfhost/internal/usecase/netguard"
	usecaseNetscaler "github.com/datdt/k8sselfhost/internal/usecase/netscaler"
	usecaseNodeMetrics "github.com/datdt/k8sselfhost/internal/usecase/nodemetrics"
	usecaseNUMAMemory "github.com/datdt/k8sselfhost/internal/usecase/numamemory"
	usecasePromotion "github.com/datdt/k8sselfhost/internal/usecase/promotion"
	usecaseRDMAWatch "github.com/datdt/k8sselfhost/internal/usecase/rdmawatch"
	usecaseRuntimeGuard "github.com/datdt/k8sselfhost/internal/usecase/runtimeguard"
	usecaseSecDefense "github.com/datdt/k8sselfhost/internal/usecase/secdefense"
	usecaseSecretGuard "github.com/datdt/k8sselfhost/internal/usecase/secretguard"
	usecaseSecurity "github.com/datdt/k8sselfhost/internal/usecase/security"
	usecaseServiceMesh "github.com/datdt/k8sselfhost/internal/usecase/servicemesh"
	usecaseSPIFFEGuard "github.com/datdt/k8sselfhost/internal/usecase/spiffeguard"
	usecaseStorage "github.com/datdt/k8sselfhost/internal/usecase/storage"
	usecaseTransitNet "github.com/datdt/k8sselfhost/internal/usecase/transitnet"
	usecaseVFSWatchdog "github.com/datdt/k8sselfhost/internal/usecase/vfswatchdog"
	usecaseWorkloadGuard "github.com/datdt/k8sselfhost/internal/usecase/workloadguard"
)

func initBackupRunner(backupRepo domainBackup.Repository, log *zap.Logger) (*infraBackup.WorkerPool, func()) {
	backupEngine := infraBackup.NewEngine(backupRepo, drivers.NewDriverRegistry(), storage.NewStorageRegistry(storage.NewLocalStorage("")))
	backupPool := infraBackup.NewWorkerPool(backupEngine, log, 50)
	backupPool.Start(3)
	return backupPool, backupPool.Stop
}

func registerGitopsProviders(controller *usecaseGitops.Controller, cfg *config.Config) {
	if cfg == nil || controller == nil {
		return
	}
	if cfg.GitOps.GitHubToken != "" {
		controller.RegisterProvider(domainGitops.ProviderGitHub, gitproviderGithub.NewProvider(cfg.GitOps.GitHubToken))
	}
	if cfg.GitOps.GitLabToken != "" {
		controller.RegisterProvider(domainGitops.ProviderGitLab, gitproviderGitlab.NewProvider(cfg.GitOps.GitLabToken, cfg.GitOps.GitLabURL))
	}
	if cfg.GitOps.GiteaToken != "" {
		controller.RegisterProvider(domainGitops.ProviderGitea, gitproviderGitea.NewProvider(cfg.GitOps.GiteaToken, cfg.GitOps.GiteaURL))
	}
}

type standaloneAuxHandlers struct {
	Canary       *adapthttp.CanaryHandler
	Mesh         *adapthttp.MeshHandler
	GitOps       *adapthttp.GitOpsHandler
	IaC          *adapthttp.IaCHandler
	Telegram     *adapthttp.TelegramHandler
	Admission    *adapthttp.AdmissionHandler
	MessagingSRE *adapthttp.MessagingHandler
	SRE          *adapthttp.SREResilienceHandlers
}

func initStandaloneAuxHandlers(
	cfg *config.Config,
	log *zap.Logger,
	k8sClient kubernetes.Interface,
	clientManager *infraCluster.ClientManager,
	dbPool postgres.DBTX,
	gitopsController *usecaseGitops.Controller,
) standaloneAuxHandlers {
	canaryEngine := usecasePromotion.NewCanaryEngine(usecasePromotion.WithLogger(log))
	canaryHandler := adapthttp.NewCanaryHandler(canaryEngine)

	var meshOpts []usecaseMesh.Option
	if k8sClient != nil {
		meshOpts = append(meshOpts, usecaseMesh.WithKubernetesClient(k8sClient))
	}
	if log != nil {
		meshOpts = append(meshOpts, usecaseMesh.WithLogger(log))
	}
	meshService := usecaseMesh.NewLifecycleService(meshOpts...)
	meshHandler := adapthttp.NewMeshHandler(meshService, log)

	var gitopsOpts []adapthttp.GitOpsHandlerOption
	webhookSecret := os.Getenv("GITOPS_WEBHOOK_SECRET")
	if webhookSecret == "" && cfg != nil {
		webhookSecret = cfg.GitOps.WebhookSecret
	}
	if webhookSecret != "" {
		gitopsOpts = append(gitopsOpts, adapthttp.WithWebhookSecret([]byte(webhookSecret)))
	}
	gitopsHandler := adapthttp.NewGitOpsHandler(gitopsController, gitopsOpts...)

	tfRunner := iac.NewTerraformRunner("")
	ansibleRunner := iac.NewAnsibleRunner("")
	if iacWorkspaceDir := os.Getenv("IAC_WORKSPACE_DIR"); iacWorkspaceDir != "" {
		tfRunner.SetBaseDir(iacWorkspaceDir)
		ansibleRunner.SetBaseDir(iacWorkspaceDir)
	}
	iacHandler := adapthttp.NewIaCHandler(tfRunner, ansibleRunner)
	telegramHandler := adapthttp.NewTelegramHandler(nil)
	admissionHandler := adapthttp.NewAdmissionHandler(usecaseSecurity.NewAdmissionEnforcer())

	messagingManager := usecaseMessaging.NewManager(nil, nil, nil, usecaseMessaging.DefaultSREManagerConfig())
	messagingHandler := adapthttp.NewMessagingHandler(messagingManager)

	sreHandlers := bootstrapStandaloneSREHandlers(log, k8sClient, clientManager, dbPool, meshHandler)

	return standaloneAuxHandlers{
		Canary:       canaryHandler,
		Mesh:         meshHandler,
		GitOps:       gitopsHandler,
		IaC:          iacHandler,
		Telegram:     telegramHandler,
		Admission:    admissionHandler,
		MessagingSRE: messagingHandler,
		SRE:          sreHandlers,
	}
}

func bootstrapStandaloneSREHandlers(
	log *zap.Logger,
	k8sClient kubernetes.Interface,
	clientManager *infraCluster.ClientManager,
	dbPool postgres.DBTX,
	meshHandler *adapthttp.MeshHandler,
) *adapthttp.SREResilienceHandlers {
	storageMgr := usecaseStorage.NewResiliencyManager(log)
	var cgroupOpts []usecaseNodeMetrics.Option
	if log != nil {
		cgroupOpts = append(cgroupOpts, usecaseNodeMetrics.WithLogger(log))
	}
	if k8sClient != nil {
		cgroupOpts = append(cgroupOpts, usecaseNodeMetrics.WithKubernetesClient(k8sClient))
	}
	if clientManager != nil {
		cgroupOpts = append(cgroupOpts, usecaseNodeMetrics.WithClientManager(clientManager))
	}
	cgroupScaler := usecaseNodeMetrics.NewCGroupScalerService(cgroupOpts...)
	jwksShield := usecaseAuth.NewJWKSShieldManager()
	var fencingVerifier usecaseCluster.FencingChannelVerifier
	if k8sClient != nil {
		fencingVerifier = usecaseCluster.NewK8sNodeVerifier(k8sClient)
	}
	fencingMgr := usecaseCluster.NewFencingManager(nil, fencingVerifier, nil, nil, log)
	driftReconciler := usecaseSecretGuard.NewDriftReconciler(nil, nil)
	ebpfMgr := usecaseEBPFWatch.NewWatchdogManager(usecaseEBPFWatch.WithLogger(log))
	netguardMgr := usecaseNetGuard.NewTunnelManager(usecaseNetGuard.WithLogger(log))
	spillJournal := infraClickhouse.NewDiskSpillJournal("")
	partsInspector := infraClickhouse.NewPartsInspector(nil)
	bulkWriter := infraClickhouse.NewBulkWriter(nil, nil, nil, spillJournal)
	mergeAccelerator := infraClickhouse.NewMergeAccelerator(nil)
	ingestMgr := usecaseIngestGuard.NewIngestManager(
		domainIngestGuard.DefaultBatchFlushConfig(),
		usecaseIngestGuard.WithBulkWriter(bulkWriter),
		usecaseIngestGuard.WithPartsInspector(partsInspector),
		usecaseIngestGuard.WithMergeAccelerator(mergeAccelerator),
		usecaseIngestGuard.WithSpillJournal(spillJournal),
	)
	runtimeMgr := usecaseRuntimeGuard.NewResilienceManager()
	gpuMgr := usecaseGPUWatch.NewGPUManagerService(nil, nil)
	drHealingMgr := usecaseDRHealing.NewHealingManager(nil, nil, nil, nil, nil)
	finopsMgr := usecaseFinOpsGuard.NewFinOpsManager(nil, nil, nil, nil)
	netscalerMgr := usecaseNetscaler.NewScalerManager(usecaseNetscaler.WithLogger(log))
	vfsMgr := usecaseVFSWatchdog.NewVFSManager(usecaseVFSWatchdog.WithLogger(log))
	k8sResilienceMgr := usecaseK8sResilience.NewResilienceManager(nil, log)
	dbSafeguardMgr := usecaseDBSafeguard.NewSafeguardManager(nil, nil, nil, nil)
	secDefenseMgr := usecaseSecDefense.NewDefenseManager(nil)
	cloudResilienceMgr := usecaseCloudResilience.NewResilienceManager(usecaseCloudResilience.WithLogger(log))
	meshResilienceMgr := usecaseServiceMesh.NewMeshManager(usecaseServiceMesh.WithLogger(log))
	numaMgr := usecaseNUMAMemory.NewNUMAMemoryManager(usecaseNUMAMemory.WithLogger(log))
	hardwareMgr := usecaseHardware.NewHardwareManager(usecaseHardware.DefaultConfig(), nil, nil, nil, nil, nil, nil, log)
	cryptoMgr := usecaseCrypto.NewShieldManager(usecaseCrypto.ShieldManagerOptions{})
	syncwaveMgr := usecaseGitops.NewSyncWaveManager()
	rdmaMgr := usecaseRDMAWatch.NewFabricManager(nil, nil, nil, nil)
	apfMgr := usecaseAPFGuard.NewAPFManager(nil, log)
	transitNetMgr := usecaseTransitNet.NewRoutingManager(usecaseTransitNet.WithLogger(log))
	spiffeMgr := usecaseSPIFFEGuard.NewIdentityManager(usecaseSPIFFEGuard.IdentityManagerConfig{})
	var consensusOpts []usecaseConsensus.CoordinatorOption
	if dbPool != nil {
		leaseStore := postgres.NewConsensusLeaseRepo(dbPool)
		consensusOpts = append(consensusOpts, usecaseConsensus.WithLeaseStore(leaseStore))
	}
	consensusMgr := usecaseConsensus.NewCoordinatorManager(nil, nil, consensusOpts...)
	workloadMgr := usecaseWorkloadGuard.NewArtifactManager()
	messagingMgr := usecaseMessaging.NewManager(nil, nil, nil, usecaseMessaging.DefaultSREManagerConfig())

	storageResiliencyHandler := adapthttp.NewStorageResiliencyHandler(storageMgr)

	return &adapthttp.SREResilienceHandlers{
		StorageResiliency: storageResiliencyHandler,
		StorageHealing:    storageResiliencyHandler,
		CGroup:            adapthttp.NewCGroupHandler(cgroupScaler, log),
		Vault:             adapthttp.NewJWKSHandler(jwksShield),
		JWKS:              adapthttp.NewJWKSHandler(jwksShield),
		Fencing:           adapthttp.NewFencingHandler(fencingMgr, log),
		SecretDrift:       adapthttp.NewSecretDriftHandler(driftReconciler, nil),
		EBPFWatch:         adapthttp.NewEBPFWatchHandler(ebpfMgr, log),
		NetGuard:          adapthttp.NewNetGuardHandler(netguardMgr, log),
		IngestGuard:       adapthttp.NewIngestGuardHandler(ingestMgr, log),
		RuntimeGuard:      adapthttp.NewRuntimeGuardHandler(runtimeMgr, log),
		GPUWatch:          adapthttp.NewGPUWatchHandler(gpuMgr, log),
		DRHealing:         adapthttp.NewDRHealingHandler(drHealingMgr, log),
		FinOpsGuard:       adapthttp.NewFinOpsGuardHandler(finopsMgr),
		Netscaler:         adapthttp.NewNetscalerHandler(netscalerMgr, log),
		VFSWatchdog:       adapthttp.NewVFSWatchdogHandler(vfsMgr, log),
		K8sResilience:     adapthttp.NewK8sResilienceHandler(k8sResilienceMgr, log),
		DBSafeguard:       adapthttp.NewDBSafeguardHandler(dbSafeguardMgr),
		SecDefense:        adapthttp.NewSecDefenseHandler(secDefenseMgr),
		CloudResilience:   adapthttp.NewCloudResilienceHandler(cloudResilienceMgr, log),
		ServiceMesh:       adapthttp.NewServiceMeshHandler(meshResilienceMgr, log),
		Mesh:              meshHandler,
		NUMAMemory:        adapthttp.NewNUMAMemoryHandler(numaMgr, log),
		Hardware:          adapthttp.NewHardwareHandler(hardwareMgr, log),
		Crypto:            adapthttp.NewCryptoHandler(cryptoMgr),
		GitOpsSyncWave:    adapthttp.NewGitOpsSyncWaveHandler(syncwaveMgr),
		RDMAWatch:         adapthttp.NewRDMAWatchHandler(rdmaMgr, log),
		APFGuard:          adapthttp.NewAPFGuardHandler(apfMgr, log),
		TransitNet:        adapthttp.NewTransitNetHandler(transitNetMgr, log),
		SPIFFEGuard:       adapthttp.NewSPIFFEGuardHandler(spiffeMgr),
		Consensus:         adapthttp.NewConsensusHandler(consensusMgr),
		WorkloadGuard:     adapthttp.NewWorkloadGuardHandler(workloadMgr, log),
		Messaging:         adapthttp.NewMessagingHandler(messagingMgr),
	}
}
