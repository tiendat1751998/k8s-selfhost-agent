package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"

	"github.com/datdt/k8sselfhost/internal/adapter/event"
	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	mw "github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/alert"
	domainLB "github.com/datdt/k8sselfhost/internal/domain/loadbalancer"
	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	infraCluster "github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	infraHelm "github.com/datdt/k8sselfhost/internal/infrastructure/helm"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	infraLB "github.com/datdt/k8sselfhost/internal/infrastructure/loadbalancer"
	"github.com/datdt/k8sselfhost/internal/infrastructure/llm"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	"github.com/datdt/k8sselfhost/internal/infrastructure/notifier"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	infraDocker "github.com/datdt/k8sselfhost/internal/infrastructure/provider/docker"
	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
	"github.com/datdt/k8sselfhost/internal/pkg/httputil"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
	usecaseAgent "github.com/datdt/k8sselfhost/internal/usecase/agent"
	usecaseAlert "github.com/datdt/k8sselfhost/internal/usecase/alert"
	usecaseAuth "github.com/datdt/k8sselfhost/internal/usecase/auth"
	usecaseBackup "github.com/datdt/k8sselfhost/internal/usecase/backup"
	usecaseCapacity "github.com/datdt/k8sselfhost/internal/usecase/capacity"
	usecaseCluster "github.com/datdt/k8sselfhost/internal/usecase/cluster"
	usecaseCost "github.com/datdt/k8sselfhost/internal/usecase/cost"
	usecaseDeployment "github.com/datdt/k8sselfhost/internal/usecase/deployment"
	usecaseDR "github.com/datdt/k8sselfhost/internal/usecase/dr"
	usecaseEcosystem "github.com/datdt/k8sselfhost/internal/usecase/ecosystem"
	usecaseGitops "github.com/datdt/k8sselfhost/internal/usecase/gitops"
	usecaseMetrics "github.com/datdt/k8sselfhost/internal/usecase/metrics"
	usecasePromotion "github.com/datdt/k8sselfhost/internal/usecase/promotion"
	usecaseRCA "github.com/datdt/k8sselfhost/internal/usecase/rca"
	usecaseScaffold "github.com/datdt/k8sselfhost/internal/usecase/scaffold"
	usecaseSearch "github.com/datdt/k8sselfhost/internal/usecase/search"
	usecaseSLO "github.com/datdt/k8sselfhost/internal/usecase/slo"
	usecaseSRE "github.com/datdt/k8sselfhost/internal/usecase/sre"
	usecaseStorage "github.com/datdt/k8sselfhost/internal/usecase/storage"
)

func wireStandalone(ctx context.Context, cfg *config.Config, log *zap.Logger) (http.Handler, func(), error) {
	healthHandler := health.NewHandler(5 * time.Second)
	wsHub := adapthttp.NewWSHub()
	go wsHub.Run(ctx)

	// Initialize LLM Provider Registry
	registry := initLLMRegistry(cfg, log)

	pgClient, err := postgres.NewClient(ctx, cfg.Postgres)
	if err != nil {
		log.Fatal("failed to connect to postgres", zap.Error(err))
	}
	healthHandler.Register("postgres", pgClient.HealthCheck)
	dbPool := pgClient.Pool()

	dockerClient, err := infraDocker.NewDockerClient(cfg.Docker.Host, cfg.Docker.Version)
	var dockerRepo domainDocker.Repository
	if err != nil {
		log.Warn("failed to initialize real docker client, docker features will be unavailable", zap.Error(err))
		dockerRepo = nil
		dockerClient = nil
	} else {
		dockerRepo = infraDocker.NewDockerRepoWithClient(dockerClient)
	}

	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		if home := os.Getenv("USERPROFILE"); home != "" {
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		} else if home = os.Getenv("HOME"); home != "" {
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		}
	}
	k8sAvailable := true
	k8sClient, err := infraK8s.NewClient(kubeconfigPath)
	if err != nil {
		log.Warn("failed to initialize Kubernetes client, kubernetes features will be unavailable", zap.Error(err))
		k8sAvailable = false
	}
	auditRepo := postgres.NewAuditRepo(dbPool)

	fleetRepo := postgres.NewFleetRepo(dbPool)
	clientManager := infraCluster.NewClientManager(fleetRepo)

	agentRepo := postgres.NewAgentRepo(dbPool)
	incRepo := postgres.NewIncidentRepo(dbPool)
	reportRepo := postgres.NewReportRepo(dbPool)
	prRepo := postgres.NewPRRepo(dbPool)
	backupRepo := postgres.NewBackupRepo(dbPool)
	settingsRepo := postgres.NewSettingsRepo(dbPool)
	catalogRepo := postgres.NewCatalogRepo(dbPool)
	scaffoldService := usecaseScaffold.NewService(postgres.NewScaffoldRepo(dbPool), catalogRepo, usecaseScaffold.NewEngine())
	ecosystemRepo := postgres.NewEcosystemRepo(dbPool)
	var ecoOpts []usecaseEcosystem.Option
	if dockerClient != nil {
		ecoOpts = append(ecoOpts, usecaseEcosystem.WithDockerClient(dockerClient))
	}
	ecosystemUsecase := usecaseEcosystem.NewUsecase(ecosystemRepo, settingsRepo, httputil.NewSafeHTTPClient(5*time.Second), log, ecoOpts...)
	ecosystemHandler := adapthttp.NewEcosystemHandler(ecosystemUsecase, log)
	helmReleaseManager := infraHelm.NewReleaseManager(clientManager, infraK8s.GetLastConfig())
	helmHandler := adapthttp.NewHelmHandler(helmReleaseManager, auditRepo)
	computeHostRepo := postgres.NewComputeHostRepo(dbPool)

	txManager := postgres.NewTxManager(dbPool)
	defaultLLM, err := registry.Default()
	if err != nil {
		log.Warn("no default LLM provider available, proceeding with caution", zap.Error(err))
	}
	bridge := adapthttp.NewWSBridge(wsHub)
	orchestrator := usecaseAgent.NewOrchestrator(agentRepo, defaultLLM, bridge, txManager)

	metricsCollector := usecaseMetrics.NewCollector(
		dockerClient, computeHostRepo, bridge, log,
		usecaseMetrics.WithRequestCountFn(mw.GetRequestCount),
		usecaseMetrics.WithIncidentRepo(incRepo),
		usecaseMetrics.WithHostProbe(os.Getenv("ENABLE_LOCAL_HOST_PROBE") == "true"),
	)
	go metricsCollector.Start(ctx)

	lbURL := cfg.LoadBalancer.URL
	if lbURL == "" {
		lbURL = usecaseMetrics.DeriveTraefikURL(cfg.Docker.Host)
	}
	var lbProvider domainLB.Provider
	if cfg.LoadBalancer.Provider == "traefik" || cfg.LoadBalancer.Provider == "" {
		lbProvider = infraLB.NewTraefikProvider(lbURL)
	}

	if provider, ok := lbProvider.(*infraLB.TraefikProvider); ok {
		go provider.StartBackgroundScraper(ctx)
	}

	tpsCollector := usecaseMetrics.NewTPSCollector(
		metricsCollector, dbPool, log,
		usecaseMetrics.WithLoadBalancerProvider(lbProvider),
		usecaseMetrics.WithTraefikURL(lbURL),
		usecaseMetrics.WithNATSMonitorURL(usecaseMetrics.DeriveNATSMonitorURL(cfg.NATS.URL)),
		usecaseMetrics.WithTPSRequestCountFn(mw.GetRequestCount),
	)
	go tpsCollector.Start(ctx)

	if dockerClient != nil {
		dockerEventWatcher := event.NewDockerEventWatcher(dockerClient, incRepo, bridge, log, event.WithDockerClusterName("fleet-primary"))
		go func() {
			if err := dockerEventWatcher.Start(ctx); err != nil {
				log.Error("docker event watcher stopped with error", zap.Error(err))
			}
		}()
	}

	nodeMetricsRepo := postgres.NewNodeMetricsRepo(dbPool)
	nodeHistoryWorker := usecaseMetrics.NewNodeHistoryWorker(metricsCollector, nodeMetricsRepo, incRepo, log)
	go func() {
		if err := nodeHistoryWorker.Start(ctx); err != nil {
			log.Error("node history worker stopped with error", zap.Error(err))
		}
	}()

	overviewHandler := adapthttp.NewOverviewHandler(metricsCollector, log, tpsCollector)
	overviewHandler.SetNodeMetricsRepo(nodeMetricsRepo)
	overviewHandler.SetIncidentRepo(incRepo)

	userRepo := postgres.NewUserRepo(dbPool)
	refreshTokenRepo := postgres.NewRefreshTokenRepo(dbPool)
	authUsecase := usecaseAuth.NewUsecase(userRepo)

	searchRepo := postgres.NewSearchRepo(dbPool)
	searchUsecase := usecaseSearch.NewUsecase(searchRepo)

	gitopsController := usecaseGitops.NewController(prRepo, incRepo, txManager)
	registerGitopsProviders(gitopsController, cfg)
	tenancyRepo := postgres.NewTenancyRepo(dbPool)

	alertRepo := postgres.NewAlertRepo(dbPool)
	ruleEngine, err := usecaseAlert.NewRuleEngine(alertRepo, map[string]alert.Notifier{
		"slack":   notifier.NewSlackNotifier(),
		"email":   notifier.NewEmailNotifier(),
		"webhook": notifier.NewWebhookNotifier(),
	}, usecaseAlert.WithLogger(log))
	if err != nil {
		return nil, nil, fmt.Errorf("initializing alert rule engine: %w", err)
	}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if snap := metricsCollector.GetLastSnapshot(); snap != nil {
					for _, tenant := range []string{"default", ""} {
						_ = ruleEngine.EvaluateRule(ctx, "cpu_usage", snap.TotalCPUPercent, tenant)
						_ = ruleEngine.EvaluateRule(ctx, "memory_usage", snap.TotalMemPercent, tenant)
						_ = ruleEngine.EvaluateRule(ctx, "disk_usage", snap.TotalDiskPercent, tenant)
						_ = ruleEngine.EvaluateRule(ctx, "requests_per_sec", snap.RequestsPerSec, tenant)
					}
				}
			}
		}
	}()
	alertUsecaseInstance := usecaseAlert.NewUsecase(alertRepo)

	var healthCenterHandler *adapthttp.HealthCenterHandler
	if k8sAvailable {
		healthCenterHandler = adapthttp.NewHealthCenterHandler(infraK8s.NewHealthCenterRepo(k8sClient, clientManager))
	}
	resourceRepo := infraK8s.NewResourceRepo(k8sClient, clientManager)
	var k8sIface kubernetes.Interface
	if k8sClient != nil {
		k8sIface = k8sClient
	}
	remediationCtrl := usecaseSRE.NewRemediationController(k8sIface, clientManager, log)
	remediationCtrl.SetComputeHostRepo(computeHostRepo)

	discoveryAdapter := infraK8s.NewDiscoveryAdapter()
	if err := crypto.ValidateKey(); err != nil {
		return nil, nil, fmt.Errorf("cluster encryption key validation failed: %w", err)
	}
	importUsecase := usecaseCluster.NewImportUsecase(fleetRepo, discoveryAdapter, logger.Get())
	healthChecker := usecaseCluster.NewHealthChecker(fleetRepo, discoveryAdapter, logger.Get())
	go healthChecker.Start(ctx, 1*time.Minute)

	obsRepo := postgres.NewObservabilityRepo(dbPool)
	sloCollector := usecaseSLO.NewSLOCollector(dockerClient, obsRepo, log,
		usecaseSLO.WithCollectorInterval(30*time.Second),
		usecaseSLO.WithFleetDependencies(computeHostRepo, dbPool.Ping, obsRepo),
	)
	sloCollector.CollectOnce(ctx)
	go sloCollector.Start(ctx)

	sloUpdater := usecaseSLO.NewSLOSnapshotUpdater(obsRepo, log)
	go sloUpdater.Start(ctx)

	rcaCollector := event.NewCollector(k8sClient)
	rcaPipeline := usecaseRCA.NewPipeline(rcaCollector, registry, reportRepo, incRepo, obsRepo)

	dashboardHandler := adapthttp.NewHandler(incRepo, reportRepo, prRepo, nil, gitopsController)
	dashboardHandler.SetRCAPipeline(rcaPipeline)

	logAggregator := logging.NewLogAggregator(2000)
	logStreamHandler := adapthttp.NewLogStreamHandler(logAggregator, computeHostRepo)
	centralizedLogs, chCleanup := wireCentralizedLogging(ctx, log, computeHostRepo)

	if dockerClient != nil {
		startDockerLogStreamer(ctx, dockerClient, logAggregator, centralizedLogs, log)
	}

	backupRunner, stopBackup := initBackupRunner(backupRepo, log)
	backupUC := usecaseBackup.NewUsecase(backupRepo)
	backupUC.SetRunner(backupRunner)

	aux := initStandaloneAuxHandlers(cfg, log, k8sClient, clientManager, dbPool, gitopsController)
	hostRemediationHandler := adapthttp.NewHostRemediationHandler(computeHostRepo, nil, log)
	dockerHandler := adapthttp.NewDockerHandler(dockerRepo, computeHostRepo, authUsecase, metricsCollector, hostRemediationHandler)

	platformHandlers := &adapthttp.PlatformHandlers{
		Dashboard:       dashboardHandler,
		Docker:          dockerHandler,
		HostRemediation: hostRemediationHandler,
		Overview:        overviewHandler,
		Drift:           adapthttp.NewDriftHandler(postgres.NewDriftRepo(dbPool)),
		Correlation:     adapthttp.NewCorrelationHandler(postgres.NewCorrelationRepo(dbPool)),
		Compliance:      adapthttp.NewComplianceHandler(postgres.NewComplianceRepo(dbPool)),
		Tagging:         adapthttp.NewTaggingHandler(postgres.NewTaggingRepo(dbPool)),
		Runbook:         adapthttp.NewRunbookHandler(postgres.NewRunbookRepo(dbPool), auditRepo),
		Observability:   adapthttp.NewObservabilityHandler(obsRepo),
		Capacity:        adapthttp.NewCapacityHandler(usecaseCapacity.NewForecaster(metricsCollector, usecaseCapacity.WithComputeHostRepo(computeHostRepo), usecaseCapacity.WithRepository(postgres.NewCapacityRepo(dbPool)))),
		Changes:         adapthttp.NewChangeHandler(postgres.NewChangesRepo(dbPool)),
		Promotion:       adapthttp.NewPromotionHandler(usecasePromotion.NewUsecase(postgres.NewPromotionRepo(dbPool), dockerRepo, auditRepo)),
		Canary:          aux.Canary,
		Mesh:            aux.Mesh,
		Explorer:        adapthttp.NewExplorerHandler(infraK8s.NewExplorerRepo(k8sClient, dockerRepo, clientManager)),
		Reporting:       adapthttp.NewReportingHandler(postgres.NewReportingRepo(dbPool)),
		HealthCenter:    healthCenterHandler,
		Fleet:           adapthttp.NewFleetHandler(fleetRepo, auditRepo, importUsecase),
		Audit:           adapthttp.NewAuditHandler(auditRepo),
		Notification:    adapthttp.NewNotificationHandler(postgres.NewNotificationRepo(dbPool)),
		Automation:      adapthttp.NewAutomationHandler(postgres.NewAutomationRepo(dbPool)),
		Timeline:        adapthttp.NewTimelineHandler(postgres.NewTimelineRepo(dbPool)),
		AI:              adapthttp.NewAIHandler(registry),
		Auth:            adapthttp.NewAuthHandler(authUsecase, userRepo, refreshTokenRepo, adapthttp.WithCookieSecure(cfg.Server.CookieSecure)),
		Search:          adapthttp.NewSearchHandler(searchUsecase),
		Cost:            adapthttp.NewCostHandler(usecaseCost.NewCalculator(computeHostRepo, metricsCollector)),
		Backup:          adapthttp.NewBackupHandler(backupUC),
		Agents:          adapthttp.NewAgentHandler(agentRepo, orchestrator),
		Deployments:     adapthttp.NewDeploymentHandler(usecaseDeployment.NewUsecase(infraK8s.NewDeploymentRepo(k8sClient, dockerRepo, fleetRepo, clientManager, infraK8s.WithComputeHostRepo(computeHostRepo)))),
		Tenancy:         adapthttp.NewTenancyHandler(tenancyRepo),
		Alert:           adapthttp.NewAlertHandler(alertUsecaseInstance),
		K8s:             adapthttp.NewK8sResourceHandler(resourceRepo, auditRepo),
		K8sExec:         adapthttp.NewK8sExecHandler(k8sClient, clientManager),
		K8sLogs:         adapthttp.NewK8sLogsHandler(k8sClient, clientManager),
		K8sBootstrap:    adapthttp.NewK8sBootstrapHandler(usecaseCluster.NewBootstrapUsecase(resourceRepo, clientManager, log)),
		Storage:         adapthttp.NewStorageHandler(usecaseStorage.NewVolumeUsecase(k8sClient, clientManager, log)),
		DR:              adapthttp.NewDRHandler(usecaseDR.NewDRUsecase(k8sClient, clientManager, log), remediationCtrl, log),
		Cloud:           adapthttp.NewCloudHandler(postgres.NewCloudAccountRepo(dbPool), nil, log),
		Settings:        adapthttp.NewSettingsHandler(settingsRepo, log),
		Catalog:         adapthttp.NewCatalogHandler(catalogRepo, log),
		Scaffolder:      adapthttp.NewScaffoldHandler(scaffoldService, log),
		Plugin:          adapthttp.NewPluginHandler(postgres.NewPluginRepo(dbPool), log),
		Ecosystem:       ecosystemHandler,
		LogStream:       logStreamHandler,
		CentralizedLogs: centralizedLogs,
		Helm:            helmHandler,
		GitOps:          aux.GitOps,
		IaC:             aux.IaC,
		Telegram:        aux.Telegram,
		Admission:       aux.Admission,
		MessagingSRE:    aux.MessagingSRE,
		SRE:             aux.SRE,
	}

	router := adapthttp.NewRouterWithWS(healthHandler, wsHub, platformHandlers)
	cleanup := func() {
		stopBackup()
		pgClient.Close()
		if chCleanup != nil {
			chCleanup()
		}
	}
	return router, cleanup, nil
}

func initLLMRegistry(cfg *config.Config, log *zap.Logger) *llm.ProviderRegistry {
	registry := llm.NewProviderRegistry()
	for _, p := range cfg.LLM.Providers {
		var c llm.Client
		switch p.Type {
		case "ollama":
			c = llm.NewOllamaClientDynamic(llm.OllamaClientConfig{Endpoint: p.Endpoint, Model: p.Model})
		case "openai":
			c = llm.NewOpenAIClient(p.Endpoint, p.Model, p.APIKey)
		case "vllm":
			c = llm.NewVLLMClient(p.Endpoint, p.Model, p.APIKey)
		default:
			log.Warn("unknown LLM provider type, skipping", zap.String("type", p.Type))
			continue
		}
		cb := llm.NewCircuitBreakerClient(p.Name, c, llm.DefaultCircuitBreakerConfig())
		registry.Register(p.Name, cb, llm.ProviderInfo{Type: p.Type, Model: p.Model, Endpoint: p.Endpoint, Default: p.Default})
	}
	if registry.Count() == 0 && cfg.LLM.Endpoint != "" {
		c := llm.NewOllamaClientDynamic(llm.OllamaClientConfig{Endpoint: cfg.LLM.Endpoint, Model: cfg.LLM.Model})
		cb := llm.NewCircuitBreakerClient("default", c, llm.DefaultCircuitBreakerConfig())
		registry.Register("default", cb, llm.ProviderInfo{Type: "ollama", Model: cfg.LLM.Model, Endpoint: cfg.LLM.Endpoint, Default: true})
	}
	return registry
}
