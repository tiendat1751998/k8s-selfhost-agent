package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
	infraClickhouse "github.com/datdt/k8sselfhost/internal/infrastructure/clickhouse"
	usecaseLogging "github.com/datdt/k8sselfhost/internal/usecase/logging"
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
	settingsRepo := postgres.NewSettingsRepo(pgClient)
	catalogRepo := postgres.NewCatalogRepo(pgClient)
	scaffoldService := usecaseScaffold.NewService(postgres.NewScaffoldRepo(pgClient), catalogRepo, usecaseScaffold.NewEngine())
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
	ruleEngine, err := usecaseAlert.NewRuleEngine(alertRepo, map[string]alert.Notifier{
		"slack":   notifier.NewSlackNotifier(),
		"email":   notifier.NewEmailNotifier(),
		"webhook": notifier.NewWebhookNotifier(),
	}, usecaseAlert.WithLogger(log))
	if err != nil {
		return nil, nil, fmt.Errorf("initializing alert rule engine: %w", err)
	}
	_ = ruleEngine
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
		Timestamp: time.Now().UTC(), Namespace: "system", Pod: "control-plane",
		Container: "standalone", Service: "standalone", Node: "control-plane",
		Stream: "stdout", Level: "INFO",
		Message: "K8SCONTROL Hybrid Control Plane online — real-time telemetry log aggregator initialized",
	})
	logStreamHandler := adapthttp.NewLogStreamHandler(logAggregator)
	centralizedLogs, chCleanup := wireCentralizedLogging(ctx, log)

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
		Capacity:      adapthttp.NewCapacityHandler(usecaseCapacity.NewForecaster(metricsCollector, usecaseCapacity.WithComputeHostRepo(computeHostRepo), usecaseCapacity.WithRepository(postgres.NewCapacityRepo(pgClient)))),
		Changes:       adapthttp.NewChangeHandler(postgres.NewChangesRepo(pgClient)),
		Promotion:     adapthttp.NewPromotionHandler(usecasePromotion.NewUsecase(postgres.NewPromotionRepo(pgClient), dockerRepo, auditRepo)),
		Explorer:      adapthttp.NewExplorerHandler(infraK8s.NewExplorerRepo(k8sClient, dockerRepo, clientManager)),
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
		Cost:          adapthttp.NewCostHandler(usecaseCost.NewCalculator(computeHostRepo, metricsCollector)),
		Backup:        adapthttp.NewBackupHandler(usecaseBackup.NewUsecase(backupRepo)),
		Agents:        adapthttp.NewAgentHandler(agentRepo, orchestrator),
		Deployments:   adapthttp.NewDeploymentHandler(usecaseDeployment.NewUsecase(infraK8s.NewDeploymentRepo(k8sClient, dockerRepo, fleetRepo, clientManager))),
		Tenancy:       adapthttp.NewTenancyHandler(tenancyRepo),
		Alert:         adapthttp.NewAlertHandler(alertUsecaseInstance),
		K8s:           adapthttp.NewK8sResourceHandler(resourceRepo, auditRepo),
		K8sExec:       adapthttp.NewK8sExecHandler(k8sClient, clientManager),
		K8sLogs:       adapthttp.NewK8sLogsHandler(k8sClient, clientManager),
		K8sBootstrap:  adapthttp.NewK8sBootstrapHandler(usecaseCluster.NewBootstrapUsecase(resourceRepo, clientManager, log)),
		Storage:       adapthttp.NewStorageHandler(usecaseStorage.NewVolumeUsecase(k8sClient, clientManager, log)),
		DR:            adapthttp.NewDRHandler(usecaseDR.NewDRUsecase(k8sClient, clientManager, log), remediationCtrl, log),
		Cloud:         adapthttp.NewCloudHandler(postgres.NewCloudAccountRepo(pgClient), nil, log),
		Settings:      adapthttp.NewSettingsHandler(settingsRepo, log),
		Catalog:       adapthttp.NewCatalogHandler(catalogRepo, log),
		Scaffolder:    adapthttp.NewScaffoldHandler(scaffoldService, log),
		Plugin:        adapthttp.NewPluginHandler(postgres.NewPluginRepo(pgClient), log),
		Ecosystem:     ecosystemHandler,
		LogStream:       logStreamHandler,
		CentralizedLogs: centralizedLogs,
		Helm:            helmHandler,
	}

	router := adapthttp.NewRouterWithWS(healthHandler, wsHub, platformHandlers)
	cleanup := func() {
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
	u := strings.ToUpper(msg)
	switch {
	case strings.Contains(u, "ERROR") || strings.Contains(u, "FATAL") || strings.Contains(u, "PANIC"):
		return "ERROR"
	case strings.Contains(u, "WARN"):
		return "WARN"
	case strings.Contains(u, "DEBUG") || strings.Contains(u, "TRACE"):
		return "DEBUG"
	case strings.Contains(u, "INFO"):
		return "INFO"
	default:
		return defaultLvl
	}
}

func wireCentralizedLogging(ctx context.Context, log *zap.Logger) (*adapthttp.LogHandler, func()) {
	chHost := os.Getenv("CLICKHOUSE_HOST")
	if chHost == "" {
		chHost = os.Getenv("K8S_CLICKHOUSE_HOST")
	}
	chDSN := os.Getenv("CLICKHOUSE_DSN")
	if chDSN == "" {
		chDSN = os.Getenv("K8S_CLICKHOUSE_DSN")
	}

	if chHost != "" || chDSN != "" {
		cfg := infraClickhouse.DefaultConfig()
		cfg.Host = chHost
		if p, err := strconv.Atoi(os.Getenv("CLICKHOUSE_PORT")); err == nil {
			cfg.Port = p
		}
		if db := os.Getenv("CLICKHOUSE_DB"); db != "" {
			cfg.Database = db
		}
		if u := os.Getenv("CLICKHOUSE_USER"); u != "" {
			cfg.Username = u
		}
		cfg.Password = os.Getenv("CLICKHOUSE_PASSWORD")
		cfg.DialTimeout = 3 * time.Second

		cCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		client, err := infraClickhouse.NewClient(cCtx, cfg)
		cancel()
		if err == nil {
			log.Info("ClickHouse centralized logging connected", zap.String("host", cfg.Host))
			service := usecaseLogging.NewService(infraClickhouse.NewLogRepository(client, nil))
			return adapthttp.NewLogHandler(service, &chStatusProvider{client: client}), func() { _ = client.Close() }
		}
		log.Warn("ClickHouse connection failed, using in-memory ringbuffer fallback", zap.Error(err))
	} else {
		log.Info("ClickHouse not configured, using resilient in-memory ringbuffer fallback")
	}

	memRepo := logging.NewMemoryLogRepo(5000)
	return adapthttp.NewLogHandler(usecaseLogging.NewService(memRepo), memRepo), nil
}

type chStatusProvider struct {
	client *infraClickhouse.Client
}

func (p *chStatusProvider) GetStatus(ctx context.Context) (*adapthttp.LogEngineStatus, error) {
	start := time.Now()
	err := p.client.Ping(ctx)
	latency := float64(time.Since(start).Microseconds()) / 1000.0
	status := "connected"
	if err != nil {
		status = "fallback"
	}
	var total int64
	if conn, cErr := p.client.Conn(ctx); cErr == nil {
		var cnt uint64
		if scanErr := conn.QueryRow(ctx, "SELECT count() FROM cluster_logs").Scan(&cnt); scanErr == nil {
			total = int64(cnt)
		}
	}
	return &adapthttp.LogEngineStatus{
		Engine:        "ClickHouse MergeTree",
		Status:        status,
		LatencyMS:     latency,
		TotalRecords:  total,
		RetentionDays: 30,
	}, nil
}




