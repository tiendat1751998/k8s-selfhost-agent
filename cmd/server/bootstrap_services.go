package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/datdt/k8sselfhost/internal/adapter/event"
	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	mw "github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/alert"
	domainLB "github.com/datdt/k8sselfhost/internal/domain/loadbalancer"
	infraBackup "github.com/datdt/k8sselfhost/internal/infrastructure/backup"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/drivers"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/storage"
	infraCluster "github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	infraLB "github.com/datdt/k8sselfhost/internal/infrastructure/loadbalancer"
	"github.com/datdt/k8sselfhost/internal/infrastructure/llm"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	infraNats "github.com/datdt/k8sselfhost/internal/infrastructure/nats"
	"github.com/datdt/k8sselfhost/internal/infrastructure/notifier"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	usecaseAgent "github.com/datdt/k8sselfhost/internal/usecase/agent"
	"github.com/datdt/k8sselfhost/internal/usecase/ai"
	usecaseAlert "github.com/datdt/k8sselfhost/internal/usecase/alert"
	usecaseAuth "github.com/datdt/k8sselfhost/internal/usecase/auth"
	usecaseBackup "github.com/datdt/k8sselfhost/internal/usecase/backup"
	usecaseCapacity "github.com/datdt/k8sselfhost/internal/usecase/capacity"
	usecaseCluster "github.com/datdt/k8sselfhost/internal/usecase/cluster"
	usecaseCost "github.com/datdt/k8sselfhost/internal/usecase/cost"
	usecaseDeployment "github.com/datdt/k8sselfhost/internal/usecase/deployment"
	usecaseGitops "github.com/datdt/k8sselfhost/internal/usecase/gitops"
	usecaseMetrics "github.com/datdt/k8sselfhost/internal/usecase/metrics"
	usecasePromotion "github.com/datdt/k8sselfhost/internal/usecase/promotion"
	"github.com/datdt/k8sselfhost/internal/usecase/rca"
	usecaseSearch "github.com/datdt/k8sselfhost/internal/usecase/search"
	usecaseSLO "github.com/datdt/k8sselfhost/internal/usecase/slo"
)

type appServices struct {
	wsHub            *adapthttp.WSHub
	platformHandlers *adapthttp.PlatformHandlers
	stopFns          []func()
}

func (s *appServices) stop() {
	for i := len(s.stopFns) - 1; i >= 0; i-- {
		s.stopFns[i]()
	}
}

func initServices(ctx context.Context, eg *errgroup.Group, cfg *config.Config, infra *infrastructure, log *zap.Logger) (*appServices, error) {
	var stopFns []func()

	// 1. Initialize WebSocket Hub
	wsHub := adapthttp.NewWSHub()
	eg.Go(func() error {
		wsHub.Run()
		return nil
	})

	if infra.natsClient != nil && infra.natsClient.Conn() != nil {
		_, err := infra.natsClient.Conn().Subscribe("agent.events", func(msg *nats.Msg) {
			var wsMsg adapthttp.WSMessage
			if err := json.Unmarshal(msg.Data, &wsMsg); err == nil {
				wsHub.Broadcast(wsMsg)
			}
		})
		if err != nil {
			log.Error("failed to subscribe to NATS agent events", zap.Error(err))
		}
	}

	// 2. Initialize Repositories
	incRepo := postgres.NewIncidentRepo(infra.pgClient.Pool())
	reportRepo := postgres.NewReportRepo(infra.pgClient.Pool())
	prRepo := postgres.NewPRRepo(infra.pgClient.Pool())
	obsRepo := postgres.NewObservabilityRepo(infra.pgClient.Pool())

	// 3. Initialize NATS Publisher and Subscriber
	publisher := infraNats.NewPublisher(infra.natsClient)
	subscriber := infraNats.NewSubscriber(infra.natsClient)

	// Subscribe to incident events
	err := subscriber.Subscribe(ctx, "incidents.>", func(eventCtx context.Context, subject string, data []byte) error {
		log.Info("received incident event", zap.String("subject", subject), zap.ByteString("data", data))
		return nil
	})
	if err != nil {
		log.Error("failed to subscribe to incident events", zap.Error(err))
	}

	// 4. Initialize LLM Provider Registry
	registry := initLLMRegistry(cfg, log)

	// 5. Initialize K8s Collector & RCA Usecase Pipeline
	collector := event.NewCollector(infra.k8sClient)
	pipeline := rca.NewPipeline(collector, registry, reportRepo, incRepo, obsRepo)

	// 6. Initialize and start AI Health Poller
	poller := ai.NewHealthPoller(registry, 60*time.Second, func(name string, result llm.ProviderHealthResult) {
		wsHub.Broadcast(adapthttp.WSMessage{
			Type: "ai_provider_status",
			Data: map[string]interface{}{
				"name":    name,
				"status":  result.Status,
				"latency": result.Latency.String(),
			},
		})
	})
	poller.Start(ctx)
	stopFns = append(stopFns, poller.Stop)

	// 7. Initialize and start NATS worker (bridge wsHub)
	bridge := adapthttp.NewWSBridge(wsHub)

	rcaWorker := rca.NewWorker(infra.natsClient.JetStream(), pipeline, incRepo, bridge)
	if err := rcaWorker.Start(ctx); err != nil {
		for i := len(stopFns) - 1; i >= 0; i-- {
			stopFns[i]()
		}
		return nil, fmt.Errorf("starting RCA worker: %w", err)
	}
	stopFns = append(stopFns, rcaWorker.Stop)

	// 8. Setup HTTP Platform Handlers & Background Workers
	computeHostRepo := postgres.NewComputeHostRepo(infra.pgClient.Pool())
	metricsCollector := usecaseMetrics.NewCollector(
		infra.dockerClient,
		computeHostRepo,
		bridge,
		log,
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
		metricsCollector,
		infra.pgClient.Pool(),
		log,
		usecaseMetrics.WithLoadBalancerProvider(lbProvider),
		usecaseMetrics.WithTraefikURL(lbURL),
		usecaseMetrics.WithNATSMonitorURL(usecaseMetrics.DeriveNATSMonitorURL(cfg.NATS.URL)),
		usecaseMetrics.WithTPSRequestCountFn(mw.GetRequestCount),
	)
	go tpsCollector.Start(ctx)

	if infra.dockerClient != nil {
		dockerWatcher := event.NewDockerEventWatcher(
			infra.dockerClient,
			incRepo,
			bridge,
			log,
			event.WithDockerClusterName("fleet-primary"),
		)
		eg.Go(func() error {
			return dockerWatcher.Start(ctx)
		})
	}

	nodeMetricsRepo := postgres.NewNodeMetricsRepo(infra.pgClient.Pool())
	nodeHistoryWorker := usecaseMetrics.NewNodeHistoryWorker(metricsCollector, nodeMetricsRepo, incRepo, log)
	go func() {
		if err := nodeHistoryWorker.Start(ctx); err != nil {
			log.Error("node history worker stopped with error", zap.Error(err))
		}
	}()

	overviewHandler := adapthttp.NewOverviewHandler(metricsCollector, log, tpsCollector)
	overviewHandler.SetNodeMetricsRepo(nodeMetricsRepo)
	overviewHandler.SetIncidentRepo(incRepo)

	sloCollector := usecaseSLO.NewSLOCollector(infra.dockerClient, obsRepo, log)
	go sloCollector.Start(ctx)

	sloUpdater := usecaseSLO.NewSLOSnapshotUpdater(obsRepo, log)
	go sloUpdater.Start(ctx)

	// Postgres-backed repos
	driftRepo := postgres.NewDriftRepo(infra.pgClient.Pool())
	correlationRepo := postgres.NewCorrelationRepo(infra.pgClient.Pool())
	complianceRepo := postgres.NewComplianceRepo(infra.pgClient.Pool())
	taggingRepo := postgres.NewTaggingRepo(infra.pgClient.Pool())
	auditRepo := postgres.NewAuditRepo(infra.pgClient.Pool())
	timelineRepo := postgres.NewTimelineRepo(infra.pgClient.Pool())
	runbookRepo := postgres.NewRunbookRepo(infra.pgClient.Pool())
	automationRepo := postgres.NewAutomationRepo(infra.pgClient.Pool())
	changesRepo := postgres.NewChangesRepo(infra.pgClient.Pool())
	promotionRepo := postgres.NewPromotionRepo(infra.pgClient.Pool())
	reportingRepo := postgres.NewReportingRepo(infra.pgClient.Pool())
	notificationRepo := postgres.NewNotificationRepo(infra.pgClient.Pool())
	fleetRepo := postgres.NewFleetRepo(infra.pgClient.Pool())
	backupRepo := postgres.NewBackupRepo(infra.pgClient.Pool())
	backupEngine := infraBackup.NewEngine(
		backupRepo,
		drivers.NewDriverRegistry(),
		storage.NewStorageRegistry(storage.NewLocalStorage("")),
	)
	backupPool := infraBackup.NewWorkerPool(backupEngine, log, 50)
	backupPool.Start(3)
	stopFns = append(stopFns, backupPool.Stop)

	backupUsecase := usecaseBackup.NewUsecase(backupRepo)
	backupUsecase.SetRunner(backupPool)

	agentRepo := postgres.NewAgentRepo(infra.pgClient.Pool())
	alertRepo := postgres.NewAlertRepo(infra.pgClient.Pool())
	cloudAccountRepo := postgres.NewCloudAccountRepo(infra.pgClient.Pool())
	cloudHandler := adapthttp.NewCloudHandler(cloudAccountRepo, nil, log)

	alertNotifiers := map[string]alert.Notifier{
		"slack":   notifier.NewSlackNotifier(),
		"email":   notifier.NewEmailNotifier(),
		"webhook": notifier.NewWebhookNotifier(),
	}
	ruleEngine, err := usecaseAlert.NewRuleEngine(alertRepo, alertNotifiers, usecaseAlert.WithLogger(log))
	if err != nil {
		for i := len(stopFns) - 1; i >= 0; i-- {
			stopFns[i]()
		}
		return nil, fmt.Errorf("initializing alert rule engine: %w", err)
	}
	_ = ruleEngine
	alertUsecaseInstance := usecaseAlert.NewUsecase(alertRepo)

	clientManager := infraCluster.NewClientManager(fleetRepo)
	discoveryAdapter := infraK8s.NewDiscoveryAdapter()
	importUsecase := usecaseCluster.NewImportUsecase(fleetRepo, discoveryAdapter, log)
	healthChecker := usecaseCluster.NewHealthChecker(fleetRepo, discoveryAdapter, log)
	eg.Go(func() error {
		healthChecker.Start(ctx, 1*time.Minute)
		return nil
	})

	txManager := postgres.NewTxManager(infra.pgClient.Pool())
	defaultLLM, err := registry.Default()
	if err != nil {
		log.Warn("no default LLM provider available, proceeding with caution", zap.Error(err))
	}
	orchestrator := usecaseAgent.NewOrchestrator(agentRepo, defaultLLM, bridge, txManager)

	userRepo := postgres.NewUserRepo(infra.pgClient.Pool())
	refreshTokenRepo := postgres.NewRefreshTokenRepo(infra.pgClient.Pool())
	authUsecase := usecaseAuth.NewUsecase(userRepo)

	searchRepo := postgres.NewSearchRepo(infra.pgClient.Pool(), infra.cacheManager)
	searchUsecase := usecaseSearch.NewUsecase(searchRepo)
	promotionUsecase := usecasePromotion.NewUsecase(promotionRepo, infra.dockerRepo, auditRepo)

	gitopsController := usecaseGitops.NewController(prRepo, incRepo, txManager)
	tenancyRepo := postgres.NewTenancyRepo(infra.pgClient.Pool())

	costCalculator := usecaseCost.NewCalculator(computeHostRepo, metricsCollector)
	capacityForecaster := usecaseCapacity.NewForecaster(
		metricsCollector,
		usecaseCapacity.WithComputeHostRepo(computeHostRepo),
		usecaseCapacity.WithRepository(postgres.NewCapacityRepo(infra.pgClient.Pool())),
	)
	capacityHandler := adapthttp.NewCapacityHandler(capacityForecaster)

	deploymentsHandler := adapthttp.NewDeploymentHandler(usecaseDeployment.NewUsecase(infraK8s.NewDeploymentRepo(infra.k8sClient, infra.dockerRepo, fleetRepo, clientManager)))
	explorerHandler := adapthttp.NewExplorerHandler(infraK8s.NewExplorerRepo(infra.k8sClient, infra.dockerRepo, clientManager))

	var healthCenterHandler *adapthttp.HealthCenterHandler
	var k8sHandler *adapthttp.K8sResourceHandler
	var k8sExecHandler *adapthttp.K8sExecHandler
	var k8sLogsHandler *adapthttp.K8sLogsHandler

	if infra.k8sAvailable {
		healthCenterHandler = adapthttp.NewHealthCenterHandler(infraK8s.NewHealthCenterRepo(infra.k8sClient, clientManager))
		k8sHandler = adapthttp.NewK8sResourceHandler(infraK8s.NewResourceRepo(infra.k8sClient, clientManager), auditRepo)
		k8sExecHandler = adapthttp.NewK8sExecHandler(infra.k8sClient, clientManager)
		k8sLogsHandler = adapthttp.NewK8sLogsHandler(infra.k8sClient, clientManager)
	} else {
		k8sHandler = adapthttp.NewK8sResourceHandler(infraK8s.NewResourceRepo(nil, clientManager), auditRepo)
		k8sExecHandler = adapthttp.NewK8sExecHandler(nil, clientManager)
		k8sLogsHandler = adapthttp.NewK8sLogsHandler(nil, clientManager)
	}

	logAggregator := logging.NewLogAggregator(2000)
	logStreamHandler := adapthttp.NewLogStreamHandler(logAggregator)

	if infra.dockerClient != nil {
		startDockerLogStreamer(ctx, infra.dockerClient, logAggregator, log)
	}

	platformHandlers := &adapthttp.PlatformHandlers{
		AI:            adapthttp.NewAIHandler(registry),
		Dashboard:     adapthttp.NewHandler(incRepo, reportRepo, prRepo, publisher, gitopsController),
		Docker:        adapthttp.NewDockerHandler(infra.dockerRepo, computeHostRepo, authUsecase, metricsCollector),
		Overview:      overviewHandler,
		Drift:         adapthttp.NewDriftHandler(driftRepo),
		Correlation:   adapthttp.NewCorrelationHandler(correlationRepo),
		Compliance:    adapthttp.NewComplianceHandler(complianceRepo),
		Tagging:       adapthttp.NewTaggingHandler(taggingRepo),
		Runbook:       adapthttp.NewRunbookHandler(runbookRepo, auditRepo),
		Observability: adapthttp.NewObservabilityHandler(obsRepo),
		Capacity:      capacityHandler,
		Changes:       adapthttp.NewChangeHandler(changesRepo),
		Promotion:     adapthttp.NewPromotionHandler(promotionUsecase),
		Explorer:      explorerHandler,
		Reporting:     adapthttp.NewReportingHandler(reportingRepo),
		HealthCenter:  healthCenterHandler,
		Fleet:         adapthttp.NewFleetHandler(fleetRepo, auditRepo, importUsecase),
		Audit:         adapthttp.NewAuditHandler(auditRepo),
		Notification:  adapthttp.NewNotificationHandler(notificationRepo),
		Automation:    adapthttp.NewAutomationHandler(automationRepo),
		Timeline:      adapthttp.NewTimelineHandler(timelineRepo),
		Auth:          adapthttp.NewAuthHandler(authUsecase, userRepo, refreshTokenRepo),
		Search:        adapthttp.NewSearchHandler(searchUsecase),
		Cost:          adapthttp.NewCostHandler(costCalculator),
		Backup:        adapthttp.NewBackupHandler(backupUsecase),
		Agents:        adapthttp.NewAgentHandler(agentRepo, orchestrator),
		Deployments:   deploymentsHandler,
		Tenancy:       adapthttp.NewTenancyHandler(tenancyRepo),
		Alert:         adapthttp.NewAlertHandler(alertUsecaseInstance),
		K8s:           k8sHandler,
		K8sExec:       k8sExecHandler,
		K8sLogs:       k8sLogsHandler,
		Cloud:         cloudHandler,
		LogStream:     logStreamHandler,
	}

	return &appServices{
		wsHub:            wsHub,
		platformHandlers: platformHandlers,
		stopFns:          stopFns,
	}, nil
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
		registry.Register(pCfg.Name, cbClient, llm.ProviderInfo{
			Type:     pCfg.Type,
			Model:    pCfg.Model,
			Endpoint: pCfg.Endpoint,
			Default:  pCfg.Default,
		})
	}

	if registry.Count() == 0 && cfg.LLM.Endpoint != "" {
		client := llm.NewOllamaClientDynamic(llm.OllamaClientConfig{Endpoint: cfg.LLM.Endpoint, Model: cfg.LLM.Model})
		cbClient := llm.NewCircuitBreakerClient("default", client, llm.DefaultCircuitBreakerConfig())
		registry.Register("default", cbClient, llm.ProviderInfo{
			Type:     "ollama",
			Model:    cfg.LLM.Model,
			Endpoint: cfg.LLM.Endpoint,
			Default:  true,
		})
	}

	return registry
}

func startDockerLogStreamer(ctx context.Context, dockerClient *dockerclient.Client, logAggregator *logging.LogAggregator, log *zap.Logger) {
	go func() {
		containers, err := dockerClient.ContainerList(ctx, container.ListOptions{All: true})
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
			ns := getContainerNamespace(cName)

			go func(id, name, namespace string) {
				reader, logErr := dockerClient.ContainerLogs(ctx, id, container.LogsOptions{
					ShowStdout: true,
					ShowStderr: true,
					Follow:     true,
					Tail:       "50",
				})
				if logErr != nil {
					log.Debug("Failed to open Docker log stream", zap.String("container", name), zap.Error(logErr))
					return
				}
				defer reader.Close()

				stdoutReader, stdoutWriter := io.Pipe()
				stderrReader, stderrWriter := io.Pipe()

				pipeStream := func(r io.Reader, stream, defaultLvl string) {
					scanner := bufio.NewScanner(r)
					for scanner.Scan() {
						line := strings.TrimSpace(scanner.Text())
						if line == "" {
							continue
						}
						logAggregator.Ingest(logging.LogEntry{
							Timestamp: time.Now().UTC(),
							Namespace: namespace,
							Pod:       name,
							Container: name,
							Stream:    stream,
							Level:     detectLogLevel(line, defaultLvl),
							Message:   line,
						})
					}
				}

				go pipeStream(stdoutReader, "stdout", "INFO")
				go pipeStream(stderrReader, "stderr", "WARN")

				_, _ = stdcopy.StdCopy(stdoutWriter, stderrWriter, reader)
				_ = stdoutWriter.Close()
				_ = stderrWriter.Close()
			}(cID, cName, ns)
		}
	}()
}

func detectLogLevel(msg string, defaultLvl string) string {
	upper := strings.ToUpper(msg)
	switch {
	case strings.Contains(upper, "ERROR") || strings.Contains(upper, "FATAL") || strings.Contains(upper, "PANIC") || strings.Contains(upper, "ERR"):
		return "ERROR"
	case strings.Contains(upper, "WARN"):
		return "WARN"
	case strings.Contains(upper, "DEBUG") || strings.Contains(upper, "TRACE"):
		return "DEBUG"
	case strings.Contains(upper, "INFO"):
		return "INFO"
	default:
		return defaultLvl
	}
}

func getContainerNamespace(name string) string {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "log") || strings.Contains(lower, "nats") {
		return "logging"
	}
	if strings.Contains(lower, "vault") {
		return "vault"
	}
	if strings.Contains(lower, "stage") || strings.Contains(lower, "staging") {
		return "staging"
	}
	return "production"
}
