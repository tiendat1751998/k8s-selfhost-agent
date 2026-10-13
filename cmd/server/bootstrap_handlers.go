package main

import (
	"time"

	"go.uber.org/zap"

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
}

func initPlatformHandlers(p platformHandlerParams) *adapthttp.PlatformHandlers {
	helmHome := p.cfg.Helm.HomeDir
	helmManager := helm.NewReleaseManager(p.clientManager, nil, helmHome)
	helmHandler := adapthttp.NewHelmHandler(helmManager, p.auditRepo)
	storageUsecase := usecaseStorage.NewVolumeUsecase(p.infra.k8sClient, p.clientManager, p.log)
	storageHandler := adapthttp.NewStorageHandler(storageUsecase)
	drUsecase := dr.NewDRUsecase(p.infra.k8sClient, p.clientManager, p.log)
	remediationController := sre.NewRemediationController(p.infra.k8sClient, p.clientManager, p.log)
	drHandler := adapthttp.NewDRHandler(drUsecase, remediationController, p.log)
	gitopsHandler := adapthttp.NewGitOpsHandler(p.gitopsController)
	iacHandler := adapthttp.NewIaCHandler(iac.NewTerraformRunner(""), iac.NewAnsibleRunner(""))
	telegramHandler := adapthttp.NewTelegramHandler(p.telegramBot)

	tenancyRepo := postgres.NewTenancyRepo(p.infra.pgClient.Pool())

	var healthCenterHandler *adapthttp.HealthCenterHandler
	if p.infra.k8sAvailable {
		healthCenterHandler = adapthttp.NewHealthCenterHandler(infraK8s.NewHealthCenterRepo(p.infra.k8sClient, p.clientManager))
	}
	k8sResourceRepo := infraK8s.NewResourceRepo(p.infra.k8sClient, p.clientManager)
	k8sHandler := adapthttp.NewK8sResourceHandler(k8sResourceRepo, p.auditRepo)
	k8sExecHandler := adapthttp.NewK8sExecHandler(p.infra.k8sClient, p.clientManager)
	k8sLogsHandler := adapthttp.NewK8sLogsHandler(p.infra.k8sClient, p.clientManager)

	bootstrapUsecase := usecaseCluster.NewBootstrapUsecase(k8sResourceRepo, p.clientManager, p.log)
	k8sBootstrapHandler := adapthttp.NewK8sBootstrapHandler(bootstrapUsecase)

	settingsRepo := postgres.NewSettingsRepo(p.infra.pgClient.Pool())
	settingsHandler := adapthttp.NewSettingsHandler(settingsRepo, p.log)

	catalogRepo := postgres.NewCatalogRepo(p.infra.pgClient.Pool())
	catalogHandler := adapthttp.NewCatalogHandler(catalogRepo, p.log)

	scaffoldRepo := postgres.NewScaffoldRepo(p.infra.pgClient.Pool())
	scaffoldService := usecaseScaffold.NewService(scaffoldRepo, catalogRepo, usecaseScaffold.NewEngine())
	scaffolderHandler := adapthttp.NewScaffoldHandler(scaffoldService, p.log)

	pluginRepo := postgres.NewPluginRepo(p.infra.pgClient.Pool())
	pluginHandler := adapthttp.NewPluginHandler(pluginRepo, p.log)

	ecosystemRepo := postgres.NewEcosystemRepo(p.infra.pgClient.Pool())
	var ecoOpts []usecaseEcosystem.Option
	if p.infra.dockerClient != nil {
		ecoOpts = append(ecoOpts, usecaseEcosystem.WithDockerClient(p.infra.dockerClient))
	}
	ecosystemUsecase := usecaseEcosystem.NewUsecase(ecosystemRepo, settingsRepo, httputil.NewSafeHTTPClient(5*time.Second), p.log, ecoOpts...)
	ecosystemHandler := adapthttp.NewEcosystemHandler(ecosystemUsecase, p.log)

	canaryEngine := usecasePromotion.NewCanaryEngine(usecasePromotion.WithLogger(p.log))
	canaryHandler := adapthttp.NewCanaryHandler(canaryEngine)

	return &adapthttp.PlatformHandlers{
		AI:              adapthttp.NewAIHandler(p.registry),
		Dashboard:       adapthttp.NewHandler(p.incRepo, p.reportRepo, p.prRepo, p.publisher, p.gitopsController),
		Docker:          adapthttp.NewDockerHandler(p.infra.dockerRepo, p.computeHostRepo, p.authUsecase, p.metricsCollector),
		Overview:        p.overviewHandler,
		Drift:           adapthttp.NewDriftHandler(postgres.NewDriftRepo(p.infra.pgClient.Pool())),
		Correlation:     adapthttp.NewCorrelationHandler(postgres.NewCorrelationRepo(p.infra.pgClient.Pool())),
		Compliance:      adapthttp.NewComplianceHandler(postgres.NewComplianceRepo(p.infra.pgClient.Pool())),
		Tagging:         adapthttp.NewTaggingHandler(postgres.NewTaggingRepo(p.infra.pgClient.Pool())),
		Runbook:         adapthttp.NewRunbookHandler(postgres.NewRunbookRepo(p.infra.pgClient.Pool()), p.auditRepo),
		Observability:   adapthttp.NewObservabilityHandler(p.obsRepo),
		Capacity:        adapthttp.NewCapacityHandler(usecaseCapacity.NewForecaster(p.metricsCollector, usecaseCapacity.WithComputeHostRepo(p.computeHostRepo), usecaseCapacity.WithRepository(postgres.NewCapacityRepo(p.infra.pgClient.Pool())))),
		Changes:         adapthttp.NewChangeHandler(postgres.NewChangesRepo(p.infra.pgClient.Pool())),
		Promotion:       adapthttp.NewPromotionHandler(p.promotionUsecase),
		Canary:          canaryHandler,
		Explorer:        adapthttp.NewExplorerHandler(infraK8s.NewExplorerRepo(p.infra.k8sClient, p.infra.dockerRepo, p.clientManager)),
		Reporting:       adapthttp.NewReportingHandler(postgres.NewReportingRepo(p.infra.pgClient.Pool())),
		HealthCenter:    healthCenterHandler,
		Fleet:           adapthttp.NewFleetHandler(p.fleetRepo, p.auditRepo, p.importUsecase),
		Audit:           adapthttp.NewAuditHandler(p.auditRepo),
		Notification:    adapthttp.NewNotificationHandler(postgres.NewNotificationRepo(p.infra.pgClient.Pool())),
		Automation:      adapthttp.NewAutomationHandler(postgres.NewAutomationRepo(p.infra.pgClient.Pool())),
		Timeline:        adapthttp.NewTimelineHandler(postgres.NewTimelineRepo(p.infra.pgClient.Pool())),
		Auth:            adapthttp.NewAuthHandler(p.authUsecase, p.userRepo, p.refreshTokenRepo, adapthttp.WithCookieSecure(p.cfg.Server.CookieSecure)),
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
	}
}
