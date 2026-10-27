package main

import (
	"os"
	"time"

	dockerclient "github.com/docker/docker/client"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/domain/agent"
	"github.com/datdt/k8sselfhost/internal/domain/audit"
	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/domain/fleet"
	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/observability"
	"github.com/datdt/k8sselfhost/internal/domain/report"
	"github.com/datdt/k8sselfhost/internal/domain/user"
	infraCluster "github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	"github.com/datdt/k8sselfhost/internal/infrastructure/helm"
	"github.com/datdt/k8sselfhost/internal/infrastructure/iac"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	"github.com/datdt/k8sselfhost/internal/infrastructure/llm"
	infraNats "github.com/datdt/k8sselfhost/internal/infrastructure/nats"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	"github.com/datdt/k8sselfhost/internal/infrastructure/telegram"
	"github.com/datdt/k8sselfhost/internal/pkg/httputil"
	usecaseAgent "github.com/datdt/k8sselfhost/internal/usecase/agent"
	usecaseAlert "github.com/datdt/k8sselfhost/internal/usecase/alert"
	usecaseAuth "github.com/datdt/k8sselfhost/internal/usecase/auth"
	usecaseBackup "github.com/datdt/k8sselfhost/internal/usecase/backup"
	usecaseCapacity "github.com/datdt/k8sselfhost/internal/usecase/capacity"
	usecaseCluster "github.com/datdt/k8sselfhost/internal/usecase/cluster"
	usecaseCost "github.com/datdt/k8sselfhost/internal/usecase/cost"
	usecaseDeployment "github.com/datdt/k8sselfhost/internal/usecase/deployment"
	"github.com/datdt/k8sselfhost/internal/usecase/dr"
	usecaseEcosystem "github.com/datdt/k8sselfhost/internal/usecase/ecosystem"
	usecaseGitops "github.com/datdt/k8sselfhost/internal/usecase/gitops"
	usecaseMetrics "github.com/datdt/k8sselfhost/internal/usecase/metrics"
	usecasePromotion "github.com/datdt/k8sselfhost/internal/usecase/promotion"
	usecaseScaffold "github.com/datdt/k8sselfhost/internal/usecase/scaffold"
	usecaseSearch "github.com/datdt/k8sselfhost/internal/usecase/search"
	usecaseSecurity "github.com/datdt/k8sselfhost/internal/usecase/security"
	"github.com/datdt/k8sselfhost/internal/usecase/sre"
	usecaseStorage "github.com/datdt/k8sselfhost/internal/usecase/storage"
	usecaseMesh "github.com/datdt/k8sselfhost/internal/usecase/mesh"
	usecaseMessaging "github.com/datdt/k8sselfhost/internal/usecase/messaging"
)

type platformHandlerParams struct {
	cfg                       *config.Config
	infra                     *infrastructure
	clientManager             *infraCluster.ClientManager
	auditRepo                 audit.Repository
	gitopsController          *usecaseGitops.Controller
	telegramBot               *telegram.SREBot
	computeHostRepo           domainDocker.ComputeHostRepository
	logStreamHandler          *adapthttp.LogStreamHandler
	centralizedLogs           *adapthttp.LogHandler
	registry                  *llm.ProviderRegistry
	incRepo                   incident.Repository
	reportRepo                report.Repository
	prRepo                    domainGitops.Repository
	publisher                 *infraNats.Publisher
	authUsecase               *usecaseAuth.Usecase
	metricsCollector          *usecaseMetrics.Collector
	overviewHandler           *adapthttp.OverviewHandler
	obsRepo                   observability.Repository
	promotionUsecase          *usecasePromotion.Usecase
	fleetRepo                 fleet.Repository
	importUsecase             *usecaseCluster.ImportUsecase
	userRepo                  user.Repository
	refreshTokenRepo          user.RefreshTokenRepository
	searchUsecase             *usecaseSearch.Usecase
	backupUsecase             *usecaseBackup.Usecase
	agentRepo                 agent.Repository
	orchestrator              *usecaseAgent.Orchestrator
	deploymentUsecaseInstance *usecaseDeployment.Usecase
	alertUsecaseInstance      *usecaseAlert.Usecase
	cloudHandler              *adapthttp.CloudHandler
	log                       *zap.Logger
	meshHandler               *adapthttp.MeshHandler
}

func initPlatformHandlers(p platformHandlerParams) *adapthttp.PlatformHandlers {
	var pgPool postgres.DBTX
	var k8sClient *kubernetes.Clientset
	var k8sAvailable bool
	var dockerClient *dockerclient.Client
	var dockerRepo domainDocker.Repository
	if p.infra != nil {
		if p.infra.pgClient != nil {
			pgPool = p.infra.pgClient.Pool()
		}
		k8sClient = p.infra.k8sClient
		k8sAvailable = p.infra.k8sAvailable
		dockerClient = p.infra.dockerClient
		dockerRepo = p.infra.dockerRepo
	}

	var helmHome string
	var cookieSecure bool
	if p.cfg != nil {
		helmHome = p.cfg.Helm.HomeDir
		cookieSecure = p.cfg.Server.CookieSecure
	}

	helmManager := helm.NewReleaseManager(p.clientManager, nil, helmHome)
	helmHandler := adapthttp.NewHelmHandler(helmManager, p.auditRepo)
	storageUsecase := usecaseStorage.NewVolumeUsecase(
		k8sClient,
		p.clientManager,
		p.log,
		usecaseStorage.WithMetricsProvider(infraK8s.NewKubeletVolumeMetricsProvider()),
	)
	storageHandler := adapthttp.NewStorageHandler(storageUsecase)
	drUsecase := dr.NewDRUsecase(k8sClient, p.clientManager, p.log)
	remediationController := sre.NewRemediationController(k8sClient, p.clientManager, p.log)
	drHandler := adapthttp.NewDRHandler(drUsecase, remediationController, p.log)

	var gitopsOpts []adapthttp.GitOpsHandlerOption
	webhookSecret := os.Getenv("GITOPS_WEBHOOK_SECRET")
	if webhookSecret == "" && p.cfg != nil {
		webhookSecret = p.cfg.GitOps.WebhookSecret
	}
	if webhookSecret != "" {
		gitopsOpts = append(gitopsOpts, adapthttp.WithWebhookSecret([]byte(webhookSecret)))
	}
	gitopsHandler := adapthttp.NewGitOpsHandler(p.gitopsController, gitopsOpts...)

	tfRunner := iac.NewTerraformRunner("")
	ansibleRunner := iac.NewAnsibleRunner("")
	if iacWorkspaceDir := os.Getenv("IAC_WORKSPACE_DIR"); iacWorkspaceDir != "" {
		tfRunner.SetBaseDir(iacWorkspaceDir)
		ansibleRunner.SetBaseDir(iacWorkspaceDir)
	}
	iacHandler := adapthttp.NewIaCHandler(tfRunner, ansibleRunner)
	telegramHandler := adapthttp.NewTelegramHandler(p.telegramBot)

	tenancyRepo := postgres.NewTenancyRepo(pgPool)

	var healthCenterHandler *adapthttp.HealthCenterHandler
	if k8sAvailable {
		healthCenterHandler = adapthttp.NewHealthCenterHandler(infraK8s.NewHealthCenterRepo(k8sClient, p.clientManager))
	}
	k8sResourceRepo := infraK8s.NewResourceRepo(k8sClient, p.clientManager)
	k8sHandler := adapthttp.NewK8sResourceHandler(k8sResourceRepo, p.auditRepo)
	k8sExecHandler := adapthttp.NewK8sExecHandler(k8sClient, p.clientManager)
	k8sLogsHandler := adapthttp.NewK8sLogsHandler(k8sClient, p.clientManager)

	bootstrapUsecase := usecaseCluster.NewBootstrapUsecase(k8sResourceRepo, p.clientManager, p.log)
	k8sBootstrapHandler := adapthttp.NewK8sBootstrapHandler(bootstrapUsecase)

	settingsRepo := postgres.NewSettingsRepo(pgPool)
	settingsHandler := adapthttp.NewSettingsHandler(settingsRepo, p.log)

	catalogRepo := postgres.NewCatalogRepo(pgPool)
	catalogHandler := adapthttp.NewCatalogHandler(catalogRepo, p.log)

	scaffoldRepo := postgres.NewScaffoldRepo(pgPool)
	scaffoldService := usecaseScaffold.NewService(scaffoldRepo, catalogRepo, usecaseScaffold.NewEngine())
	scaffolderHandler := adapthttp.NewScaffoldHandler(scaffoldService, p.log)

	pluginRepo := postgres.NewPluginRepo(pgPool)
	pluginHandler := adapthttp.NewPluginHandler(pluginRepo, p.log)

	ecosystemRepo := postgres.NewEcosystemRepo(pgPool)
	var ecoOpts []usecaseEcosystem.Option
	if dockerClient != nil {
		ecoOpts = append(ecoOpts, usecaseEcosystem.WithDockerClient(dockerClient))
	}
	ecosystemUsecase := usecaseEcosystem.NewUsecase(ecosystemRepo, settingsRepo, httputil.NewSafeHTTPClient(5*time.Second), p.log, ecoOpts...)
	ecosystemHandler := adapthttp.NewEcosystemHandler(ecosystemUsecase, p.log)

	canaryEngine := usecasePromotion.NewCanaryEngine(usecasePromotion.WithLogger(p.log))
	canaryHandler := adapthttp.NewCanaryHandler(canaryEngine)

	var meshOpts []usecaseMesh.Option
	if k8sClient != nil {
		meshOpts = append(meshOpts, usecaseMesh.WithKubernetesClient(k8sClient))
	}
	if p.log != nil {
		meshOpts = append(meshOpts, usecaseMesh.WithLogger(p.log))
	}
	meshService := usecaseMesh.NewLifecycleService(meshOpts...)
	meshHandler := adapthttp.NewMeshHandler(meshService, p.log)

	messagingManager := usecaseMessaging.NewManager(nil, nil, nil, usecaseMessaging.DefaultSREManagerConfig())
	messagingHandler := adapthttp.NewMessagingHandler(messagingManager)
	p.meshHandler = meshHandler
	sreHandlers := bootstrapSREHandlers(p)
	if sreHandlers != nil && sreHandlers.Crypto != nil && settingsRepo != nil {
		sreHandlers.Crypto.SetSettingsRepo(settingsRepo)
	}

	return &adapthttp.PlatformHandlers{
		AI:              adapthttp.NewAIHandler(p.registry),
		Dashboard:       adapthttp.NewHandler(p.incRepo, p.reportRepo, p.prRepo, p.publisher, p.gitopsController),
		Docker:          adapthttp.NewDockerHandler(dockerRepo, p.computeHostRepo, p.authUsecase, p.metricsCollector),
		Overview:        p.overviewHandler,
		Drift:           adapthttp.NewDriftHandler(postgres.NewDriftRepo(pgPool)),
		Correlation:     adapthttp.NewCorrelationHandler(postgres.NewCorrelationRepo(pgPool)),
		Compliance:      adapthttp.NewComplianceHandler(postgres.NewComplianceRepo(pgPool)),
		Tagging:         adapthttp.NewTaggingHandler(postgres.NewTaggingRepo(pgPool)),
		Runbook:         adapthttp.NewRunbookHandler(postgres.NewRunbookRepo(pgPool), p.auditRepo),
		Observability:   adapthttp.NewObservabilityHandler(p.obsRepo, adapthttp.WithComputeHostRepo(p.computeHostRepo)),
		Capacity:        adapthttp.NewCapacityHandler(usecaseCapacity.NewForecaster(p.metricsCollector, usecaseCapacity.WithComputeHostRepo(p.computeHostRepo), usecaseCapacity.WithRepository(postgres.NewCapacityRepo(pgPool)))),
		Changes:         adapthttp.NewChangeHandler(postgres.NewChangesRepo(pgPool)),
		Promotion:       adapthttp.NewPromotionHandler(p.promotionUsecase),
		Canary:          canaryHandler,
		Mesh:            meshHandler,
		Explorer:        adapthttp.NewExplorerHandler(infraK8s.NewExplorerRepo(k8sClient, dockerRepo, p.clientManager)),
		Reporting:       adapthttp.NewReportingHandler(postgres.NewReportingRepo(pgPool)),
		HealthCenter:    healthCenterHandler,
		Fleet:           adapthttp.NewFleetHandler(p.fleetRepo, p.auditRepo, p.importUsecase),
		Audit:           adapthttp.NewAuditHandler(p.auditRepo),
		Notification:    adapthttp.NewNotificationHandler(postgres.NewNotificationRepo(pgPool)),
		Automation:      adapthttp.NewAutomationHandler(postgres.NewAutomationRepo(pgPool)),
		Timeline:        adapthttp.NewTimelineHandler(postgres.NewTimelineRepo(pgPool)),
		Auth:            adapthttp.NewAuthHandler(p.authUsecase, p.userRepo, p.refreshTokenRepo, adapthttp.WithCookieSecure(cookieSecure)),
		Search:          adapthttp.NewSearchHandler(p.searchUsecase),
		Cost:            adapthttp.NewCostHandler(usecaseCost.NewCalculator(p.computeHostRepo, p.metricsCollector)),
		Backup:          adapthttp.NewBackupHandler(p.backupUsecase),
		Agents:          adapthttp.NewAgentHandler(p.agentRepo, p.orchestrator),
		Deployments:     adapthttp.NewDeploymentHandler(p.deploymentUsecaseInstance),
		Tenancy:         adapthttp.NewTenancyHandler(tenancyRepo),
		Alert:           adapthttp.NewAlertHandler(p.alertUsecaseInstance),
		K8s:             k8sHandler,
		K8sExec:         k8sExecHandler,
		K8sLogs:         k8sLogsHandler,
		K8sBootstrap:    k8sBootstrapHandler,
		Settings:        settingsHandler,
		Catalog:         catalogHandler,
		Scaffolder:      scaffolderHandler,
		Plugin:          pluginHandler,
		Ecosystem:       ecosystemHandler,
		Cloud:           p.cloudHandler,
		LogStream:       p.logStreamHandler,
		CentralizedLogs: p.centralizedLogs,
		Helm:            helmHandler,
		Storage:         storageHandler,
		DR:              drHandler,
		GitOps:          gitopsHandler,
		IaC:             iacHandler,
		Telegram:        telegramHandler,
		Admission:       adapthttp.NewAdmissionHandler(usecaseSecurity.NewAdmissionEnforcer()),
		MessagingSRE:    messagingHandler,
		SRE:             sreHandlers,
	}
}
