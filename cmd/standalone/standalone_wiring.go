package main

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/jackc/pgx/v5/pgxpool"
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
	go wsHub.Run()

	// Initialize LLM Provider Registry
	registry := initLLMRegistry(cfg, log)

	dsn := cfg.Postgres.DSN()
	pgClient, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal("failed to connect to postgres", zap.Error(err))
	}

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
	auditRepo := postgres.NewAuditRepo(pgClient)

	fleetRepo := postgres.NewFleetRepo(pgClient)
	clientManager := infraCluster.NewClientManager(fleetRepo)

	agentRepo := postgres.NewAgentRepo(pgClient)
	incRepo := postgres.NewIncidentRepo(pgClient)
	reportRepo := postgres.NewReportRepo(pgClient)
	prRepo := postgres.NewPRRepo(pgClient)
	backupRepo := postgres.NewBackupRepo(pgClient)
	cloudAccountRepo := postgres.NewCloudAccountRepo(pgClient)
	cloudHandler := adapthttp.NewCloudHandler(cloudAccountRepo, nil, log)
	settingsRepo := postgres.NewSettingsRepo(pgClient)
	settingsHandler := adapthttp.NewSettingsHandler(settingsRepo, log)
	catalogRepo := postgres.NewCatalogRepo(pgClient)
	catalogHandler := adapthttp.NewCatalogHandler(catalogRepo, log)
	pluginRepo := postgres.NewPluginRepo(pgClient)
	pluginHandler := adapthttp.NewPluginHandler(pluginRepo, log)
	scaffoldRepo := postgres.NewScaffoldRepo(pgClient)
	scaffoldService := usecaseScaffold.NewService(scaffoldRepo, catalogRepo, usecaseScaffold.NewEngine())
	scaffoldHandler := adapthttp.NewScaffoldHandler(scaffoldService, log)
	ecosystemRepo := postgres.NewEcosystemRepo(pgClient)
	var ecoOpts []usecaseEcosystem.Option
	if dockerClient != nil {
		ecoOpts = append(ecoOpts, usecaseEcosystem.WithDockerClient(dockerClient))
	}
	ecosystemUsecase := usecaseEcosystem.NewUsecase(ecosystemRepo, settingsRepo, httputil.NewSafeHTTPClient(5*time.Second), log, ecoOpts...)
	ecosystemHandler := adapthttp.NewEcosystemHandler(ecosystemUsecase, log)
	helmReleaseManager := infraHelm.NewReleaseManager(clientManager, infraK8s.GetLastConfig())
	helmHandler := adapthttp.NewHelmHandler(helmReleaseManager, auditRepo)
	computeHostRepo := postgres.NewComputeHostRepo(pgClient)

	txManager := postgres.NewTxManager(pgClient)
	defaultLLM, _ := registry.Default()
	bridge := adapthttp.NewWSBridge(wsHub)
	orchestrator := usecaseAgent.NewOrchestrator(agentRepo, defaultLLM, bridge, txManager)

	metricsCollector := usecaseMetrics.NewCollector(
		dockerClient, computeHostRepo, bridge, log,
		usecaseMetrics.WithRequestCountFn(mw.GetRequestCount),
		usecaseMetrics.WithIncidentRepo(incRepo),
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
		metricsCollector, pgClient, log,
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

	nodeMetricsRepo := postgres.NewNodeMetricsRepo(pgClient)
	nodeHistoryWorker := usecaseMetrics.NewNodeHistoryWorker(metricsCollector, nodeMetricsRepo, incRepo, log)
	go func() {
		if err := nodeHistoryWorker.Start(ctx); err != nil {
			log.Error("node history worker stopped with error", zap.Error(err))
		}
	}()

	overviewHandler := adapthttp.NewOverviewHandler(metricsCollector, log, tpsCollector)
	overviewHandler.SetNodeMetricsRepo(nodeMetricsRepo)
	overviewHandler.SetIncidentRepo(incRepo)

	userRepo := postgres.NewUserRepo(pgClient)
	refreshTokenRepo := postgres.NewRefreshTokenRepo(pgClient)
	authUsecase := usecaseAuth.NewUsecase(userRepo)

	searchRepo := postgres.NewSearchRepo(pgClient, nil)
	searchUsecase := usecaseSearch.NewUsecase(searchRepo)

	gitopsController := usecaseGitops.NewController(prRepo, incRepo, txManager)
	tenancyRepo := postgres.NewTenancyRepo(pgClient)

	alertRepo := postgres.NewAlertRepo(pgClient)
	_ = usecaseAlert.NewRuleEngine(alertRepo, map[string]alert.Notifier{
		"slack":   notifier.NewSlackNotifier(),
		"email":   notifier.NewEmailNotifier(),
		"webhook": notifier.NewWebhookNotifier(),
	})
	alertUsecaseInstance := usecaseAlert.NewUsecase(alertRepo)

	costCalculator := usecaseCost.NewCalculator(computeHostRepo, metricsCollector)
	capacityForecaster := usecaseCapacity.NewForecaster(
		metricsCollector,
		usecaseCapacity.WithComputeHostRepo(computeHostRepo),
		usecaseCapacity.WithRepository(postgres.NewCapacityRepo(pgClient)),
	)
	capacityHandler := adapthttp.NewCapacityHandler(capacityForecaster)

	deploymentsHandler := adapthttp.NewDeploymentHandler(usecaseDeployment.NewUsecase(infraK8s.NewDeploymentRepo(k8sClient, dockerRepo, fleetRepo, clientManager)))
	explorerHandler := adapthttp.NewExplorerHandler(infraK8s.NewExplorerRepo(k8sClient, dockerRepo, clientManager))
	var healthCenterHandler *adapthttp.HealthCenterHandler
	if k8sAvailable {
		healthCenterHandler = adapthttp.NewHealthCenterHandler(infraK8s.NewHealthCenterRepo(k8sClient, clientManager))
	}
	k8sHandler := adapthttp.NewK8sResourceHandler(infraK8s.NewResourceRepo(k8sClient, clientManager), auditRepo)
	k8sExecHandler := adapthttp.NewK8sExecHandler(k8sClient, clientManager)
	k8sLogsHandler := adapthttp.NewK8sLogsHandler(k8sClient, clientManager)

	resourceRepo := infraK8s.NewResourceRepo(k8sClient, clientManager)
	k8sBootstrapHandler := adapthttp.NewK8sBootstrapHandler(usecaseCluster.NewBootstrapUsecase(resourceRepo, clientManager, log))
	storageHandler := adapthttp.NewStorageHandler(usecaseStorage.NewVolumeUsecase(k8sClient, clientManager, log))
	var k8sIface kubernetes.Interface
	if k8sClient != nil {
		k8sIface = k8sClient
	}
	remediationCtrl := usecaseSRE.NewRemediationController(k8sIface, clientManager, log)
	drHandler := adapthttp.NewDRHandler(usecaseDR.NewDRUsecase(k8sClient, clientManager, log), remediationCtrl, log)

	discoveryAdapter := infraK8s.NewDiscoveryAdapter()
	importUsecase := usecaseCluster.NewImportUsecase(fleetRepo, discoveryAdapter, logger.Get())
	healthChecker := usecaseCluster.NewHealthChecker(fleetRepo, discoveryAdapter, logger.Get())
	go healthChecker.Start(ctx, 1*time.Minute)

	obsRepo := postgres.NewObservabilityRepo(pgClient)
	if obsRepo != nil {
		defs, err := obsRepo.ListSLODefinitions(ctx)
		if err != nil {
			log.Warn("Failed to check existing SLO definitions", zap.Error(err))
		} else if len(defs) == 0 {
			if seedErr := usecaseSLO.SeedDefaultSLODefinitions(ctx, obsRepo, log); seedErr != nil {
				log.Warn("Failed to seed default enterprise SLO definitions", zap.Error(seedErr))
			}
		}
	}
	sloCollector := usecaseSLO.NewSLOCollector(dockerClient, obsRepo, log)
	go sloCollector.Start(ctx)

	sloUpdater := usecaseSLO.NewSLOSnapshotUpdater(obsRepo, log)
	go sloUpdater.Start(ctx)

	rcaCollector := event.NewCollector(k8sClient)
	rcaPipeline := usecaseRCA.NewPipeline(rcaCollector, registry, reportRepo, incRepo, obsRepo)

	dashboardHandler := adapthttp.NewHandler(incRepo, reportRepo, prRepo, nil, gitopsController)
	dashboardHandler.SetRCAPipeline(rcaPipeline)

	logAggregator := logging.NewLogAggregator(2000)
	logAggregator.Ingest(logging.LogEntry{
		Timestamp: time.Now().UTC(),
		Namespace: "system",
		Pod:       "control-plane",
		Container: "standalone",
		Service:   "standalone",
		Node:      "control-plane",
		Stream:    "stdout",
		Level:     "INFO",
		Message:   "K8SCONTROL Hybrid Control Plane online — real-time telemetry log aggregator initialized",
	})
	logStreamHandler := adapthttp.NewLogStreamHandler(logAggregator)

	if dockerClient != nil {
		startDockerLogStreamer(ctx, dockerClient, logAggregator, log)
	}

	platformHandlers := &adapthttp.PlatformHandlers{
		Dashboard:     dashboardHandler,
		Docker:        adapthttp.NewDockerHandler(dockerRepo, computeHostRepo, authUsecase, metricsCollector),
		Overview:      overviewHandler,
		Drift:         adapthttp.NewDriftHandler(postgres.NewDriftRepo(pgClient)),
		Correlation:   adapthttp.NewCorrelationHandler(postgres.NewCorrelationRepo(pgClient)),
		Compliance:    adapthttp.NewComplianceHandler(postgres.NewComplianceRepo(pgClient)),
		Tagging:       adapthttp.NewTaggingHandler(postgres.NewTaggingRepo(pgClient)),
		Runbook:       adapthttp.NewRunbookHandler(postgres.NewRunbookRepo(pgClient), auditRepo),
		Observability: adapthttp.NewObservabilityHandler(obsRepo),
		Capacity:      capacityHandler,
		Changes:       adapthttp.NewChangeHandler(postgres.NewChangesRepo(pgClient)),
		Promotion:     adapthttp.NewPromotionHandler(usecasePromotion.NewUsecase(postgres.NewPromotionRepo(pgClient), dockerRepo, auditRepo)),
		Explorer:      explorerHandler,
		Reporting:     adapthttp.NewReportingHandler(postgres.NewReportingRepo(pgClient)),
		HealthCenter:  healthCenterHandler,
		Fleet:         adapthttp.NewFleetHandler(fleetRepo, auditRepo, importUsecase),
		Audit:         adapthttp.NewAuditHandler(auditRepo),
		Notification:  adapthttp.NewNotificationHandler(postgres.NewNotificationRepo(pgClient)),
		Automation:    adapthttp.NewAutomationHandler(postgres.NewAutomationRepo(pgClient)),
		Timeline:      adapthttp.NewTimelineHandler(postgres.NewTimelineRepo(pgClient)),
		AI:            adapthttp.NewAIHandler(registry),
		Auth:          adapthttp.NewAuthHandler(authUsecase, userRepo, refreshTokenRepo),
		Search:        adapthttp.NewSearchHandler(searchUsecase),
		Cost:          adapthttp.NewCostHandler(costCalculator),
		Backup:        adapthttp.NewBackupHandler(usecaseBackup.NewUsecase(backupRepo)),
		Agents:        adapthttp.NewAgentHandler(agentRepo, orchestrator),
		Deployments:   deploymentsHandler,
		Tenancy:       adapthttp.NewTenancyHandler(tenancyRepo),
		Alert:         adapthttp.NewAlertHandler(alertUsecaseInstance),
		K8s:           k8sHandler,
		K8sExec:       k8sExecHandler,
		K8sLogs:       k8sLogsHandler,
		K8sBootstrap:  k8sBootstrapHandler,
		Storage:       storageHandler,
		DR:            drHandler,
		Cloud:         cloudHandler,
		Settings:      settingsHandler,
		Catalog:       catalogHandler,
		Scaffolder:    scaffoldHandler,
		Plugin:        pluginHandler,
		Ecosystem:     ecosystemHandler,
		LogStream:     logStreamHandler,
		Helm:          helmHandler,
	}

	router := adapthttp.NewRouterWithWS(healthHandler, wsHub, platformHandlers)
	cleanup := func() {
		pgClient.Close()
	}
	return router, cleanup, nil
}

func initLLMRegistry(cfg *config.Config, log *zap.Logger) *llm.ProviderRegistry {
	registry := llm.NewProviderRegistry()
	for _, pCfg := range cfg.LLM.Providers {
		var client llm.Client
		switch pCfg.Type {
		case "ollama":
			client = llm.NewOllamaClientDynamic(llm.OllamaClientConfig{Endpoint: pCfg.Endpoint, Model: pCfg.Model})
		case "openai":
			client = llm.NewOpenAIClient(pCfg.Endpoint, pCfg.Model, pCfg.APIKey)
		case "vllm":
			client = llm.NewVLLMClient(pCfg.Endpoint, pCfg.Model, pCfg.APIKey)
		default:
			log.Warn("unknown LLM provider type, skipping", zap.String("type", pCfg.Type))
			continue
		}
		cbClient := llm.NewCircuitBreakerClient(pCfg.Name, client, llm.DefaultCircuitBreakerConfig())
		registry.Register(pCfg.Name, cbClient, llm.ProviderInfo{Type: pCfg.Type, Model: pCfg.Model, Endpoint: pCfg.Endpoint, Default: pCfg.Default})
	}

	if registry.Count() == 0 && cfg.LLM.Endpoint != "" {
		client := llm.NewOllamaClientDynamic(llm.OllamaClientConfig{Endpoint: cfg.LLM.Endpoint, Model: cfg.LLM.Model})
		cbClient := llm.NewCircuitBreakerClient("default", client, llm.DefaultCircuitBreakerConfig())
		registry.Register("default", cbClient, llm.ProviderInfo{Type: "ollama", Model: cfg.LLM.Model, Endpoint: cfg.LLM.Endpoint, Default: true})
	}
	return registry
}

func startDockerLogStreamer(ctx context.Context, dockerClient *dockerclient.Client, logAggregator *logging.LogAggregator, log *zap.Logger) {
	go func() {
		containers, err := dockerClient.ContainerList(ctx, container.ListOptions{})
		if err != nil {
			log.Warn("Failed to list Docker containers for log streaming", zap.Error(err))
			return
		}

		for _, c := range containers {
			cID := c.ID
			cName := cID
			if len(cID) > 12 {
				cName = cID[:12]
			}
			if len(c.Names) > 0 {
				cName = strings.TrimPrefix(c.Names[0], "/")
			}

			go func(id, name string) {
				reader, logErr := dockerClient.ContainerLogs(ctx, id, container.LogsOptions{
					ShowStdout: true,
					ShowStderr: true,
					Follow:     true,
					Tail:       "25",
				})
				if logErr != nil {
					log.Debug("Failed to open Docker log stream", zap.String("container", name), zap.Error(logErr))
					return
				}
				defer reader.Close()

				stdoutReader, stdoutWriter := io.Pipe()
				stderrReader, stderrWriter := io.Pipe()

				streamPipe := func(r io.Reader, stream, defaultLvl string) {
					scanner := bufio.NewScanner(r)
					for scanner.Scan() {
						line := strings.TrimSpace(scanner.Text())
						if line == "" {
							continue
						}
						logAggregator.Ingest(logging.LogEntry{
							Timestamp: time.Now().UTC(),
							Namespace: "docker",
							Pod:       name,
							Container: name,
							Service:   name,
							Node:      "standalone-host",
							Stream:    stream,
							Level:     detectLogLevel(line, defaultLvl),
							Message:   line,
						})
					}
				}

				go streamPipe(stdoutReader, "stdout", "INFO")
				go streamPipe(stderrReader, "stderr", "WARN")

				_, _ = stdcopy.StdCopy(stdoutWriter, stderrWriter, reader)
				_ = stdoutWriter.Close()
				_ = stderrWriter.Close()
			}(cID, cName)
		}
	}()
}

func detectLogLevel(msg string, defaultLvl string) string {
	upper := strings.ToUpper(msg)
	if strings.Contains(upper, "ERROR") || strings.Contains(upper, "FATAL") || strings.Contains(upper, "PANIC") {
		return "ERROR"
	}
	if strings.Contains(upper, "WARN") {
		return "WARN"
	}
	if strings.Contains(upper, "DEBUG") || strings.Contains(upper, "TRACE") {
		return "DEBUG"
	}
	if strings.Contains(upper, "INFO") {
		return "INFO"
	}
	return defaultLvl
}
