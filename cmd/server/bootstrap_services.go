package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
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
	infraClickhouse "github.com/datdt/k8sselfhost/internal/infrastructure/clickhouse"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	infraLB "github.com/datdt/k8sselfhost/internal/infrastructure/loadbalancer"
	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	usecaseLogging "github.com/datdt/k8sselfhost/internal/usecase/logging"
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
		metricsCollector, infra.pgClient.Pool(), log,
		usecaseMetrics.WithLoadBalancerProvider(lbProvider), usecaseMetrics.WithTraefikURL(lbURL),
		usecaseMetrics.WithNATSMonitorURL(usecaseMetrics.DeriveNATSMonitorURL(cfg.NATS.URL)),
		usecaseMetrics.WithTPSRequestCountFn(mw.GetRequestCount),
	)
	go tpsCollector.Start(ctx)

	if infra.dockerClient != nil {
		dockerWatcher := event.NewDockerEventWatcher(infra.dockerClient, incRepo, bridge, log, event.WithDockerClusterName("fleet-primary"))
		eg.Go(func() error { return dockerWatcher.Start(ctx) })
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

	auditRepo := postgres.NewAuditRepo(infra.pgClient.Pool())
	fleetRepo := postgres.NewFleetRepo(infra.pgClient.Pool())
	backupRepo := postgres.NewBackupRepo(infra.pgClient.Pool())
	backupEngine := infraBackup.NewEngine(backupRepo, drivers.NewDriverRegistry(), storage.NewStorageRegistry(storage.NewLocalStorage("")))
	backupPool := infraBackup.NewWorkerPool(backupEngine, log, 50)
	backupPool.Start(3)
	stopFns = append(stopFns, backupPool.Stop)

	backupUsecase := usecaseBackup.NewUsecase(backupRepo)
	backupUsecase.SetRunner(backupPool)

	agentRepo := postgres.NewAgentRepo(infra.pgClient.Pool())
	alertRepo := postgres.NewAlertRepo(infra.pgClient.Pool())
	cloudAccountRepo := postgres.NewCloudAccountRepo(infra.pgClient.Pool())
	cloudHandler := adapthttp.NewCloudHandler(cloudAccountRepo, nil, log)

	alertNotifiers := map[string]alert.Notifier{"slack": notifier.NewSlackNotifier(), "email": notifier.NewEmailNotifier(), "webhook": notifier.NewWebhookNotifier()}
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
	promotionUsecase := usecasePromotion.NewUsecase(postgres.NewPromotionRepo(infra.pgClient.Pool()), infra.dockerRepo, auditRepo)
	gitopsController := usecaseGitops.NewController(prRepo, incRepo, txManager)
	tenancyRepo := postgres.NewTenancyRepo(infra.pgClient.Pool())

	var healthCenterHandler *adapthttp.HealthCenterHandler
	if infra.k8sAvailable {
		healthCenterHandler = adapthttp.NewHealthCenterHandler(infraK8s.NewHealthCenterRepo(infra.k8sClient, clientManager))
	}
	k8sHandler := adapthttp.NewK8sResourceHandler(infraK8s.NewResourceRepo(infra.k8sClient, clientManager), auditRepo)
	k8sExecHandler := adapthttp.NewK8sExecHandler(infra.k8sClient, clientManager)
	k8sLogsHandler := adapthttp.NewK8sLogsHandler(infra.k8sClient, clientManager)

	logAggregator := logging.NewLogAggregator(2000)
	logStreamHandler := adapthttp.NewLogStreamHandler(logAggregator)
	centralizedLogs, chCleanup := wireCentralizedLogging(ctx, log)
	if chCleanup != nil {
		stopFns = append(stopFns, chCleanup)
	}

	if infra.dockerClient != nil {
		startDockerLogStreamer(ctx, infra.dockerClient, logAggregator, centralizedLogs, log)
	}

	platformHandlers := &adapthttp.PlatformHandlers{
		AI:            adapthttp.NewAIHandler(registry),
		Dashboard:     adapthttp.NewHandler(incRepo, reportRepo, prRepo, publisher, gitopsController),
		Docker:        adapthttp.NewDockerHandler(infra.dockerRepo, computeHostRepo, authUsecase, metricsCollector),
		Overview:      overviewHandler,
		Drift:         adapthttp.NewDriftHandler(postgres.NewDriftRepo(infra.pgClient.Pool())),
		Correlation:   adapthttp.NewCorrelationHandler(postgres.NewCorrelationRepo(infra.pgClient.Pool())),
		Compliance:    adapthttp.NewComplianceHandler(postgres.NewComplianceRepo(infra.pgClient.Pool())),
		Tagging:       adapthttp.NewTaggingHandler(postgres.NewTaggingRepo(infra.pgClient.Pool())),
		Runbook:       adapthttp.NewRunbookHandler(postgres.NewRunbookRepo(infra.pgClient.Pool()), auditRepo),
		Observability: adapthttp.NewObservabilityHandler(obsRepo),
		Capacity:      adapthttp.NewCapacityHandler(usecaseCapacity.NewForecaster(metricsCollector, usecaseCapacity.WithComputeHostRepo(computeHostRepo), usecaseCapacity.WithRepository(postgres.NewCapacityRepo(infra.pgClient.Pool())))),
		Changes:       adapthttp.NewChangeHandler(postgres.NewChangesRepo(infra.pgClient.Pool())),
		Promotion:     adapthttp.NewPromotionHandler(promotionUsecase),
		Explorer:      adapthttp.NewExplorerHandler(infraK8s.NewExplorerRepo(infra.k8sClient, infra.dockerRepo, clientManager)),
		Reporting:     adapthttp.NewReportingHandler(postgres.NewReportingRepo(infra.pgClient.Pool())),
		HealthCenter:  healthCenterHandler,
		Fleet:         adapthttp.NewFleetHandler(fleetRepo, auditRepo, importUsecase),
		Audit:         adapthttp.NewAuditHandler(auditRepo),
		Notification:  adapthttp.NewNotificationHandler(postgres.NewNotificationRepo(infra.pgClient.Pool())),
		Automation:    adapthttp.NewAutomationHandler(postgres.NewAutomationRepo(infra.pgClient.Pool())),
		Timeline:      adapthttp.NewTimelineHandler(postgres.NewTimelineRepo(infra.pgClient.Pool())),
		Auth:          adapthttp.NewAuthHandler(authUsecase, userRepo, refreshTokenRepo),
		Search:        adapthttp.NewSearchHandler(searchUsecase),
		Cost:          adapthttp.NewCostHandler(usecaseCost.NewCalculator(computeHostRepo, metricsCollector)),
		Backup:        adapthttp.NewBackupHandler(backupUsecase),
		Agents:        adapthttp.NewAgentHandler(agentRepo, orchestrator),
		Deployments:   adapthttp.NewDeploymentHandler(usecaseDeployment.NewUsecase(infraK8s.NewDeploymentRepo(infra.k8sClient, infra.dockerRepo, fleetRepo, clientManager))),
		Tenancy:       adapthttp.NewTenancyHandler(tenancyRepo),
		Alert:         adapthttp.NewAlertHandler(alertUsecaseInstance),
		K8s:           k8sHandler,
		K8sExec:       k8sExecHandler,
		K8sLogs:       k8sLogsHandler,
		Cloud:         cloudHandler,
		LogStream:       logStreamHandler,
		CentralizedLogs: centralizedLogs,
	}

	return &appServices{
		wsHub:            wsHub,
		platformHandlers: platformHandlers,
		stopFns:          stopFns,
	}, nil
}

func initLLMRegistry(cfg *config.Config, log *zap.Logger) *llm.ProviderRegistry {
	registry := llm.NewProviderRegistry()
	for _, p := range cfg.LLM.Providers {
		var c llm.Client
		switch p.Type {
		case "ollama": c = llm.NewOllamaClientDynamic(llm.OllamaClientConfig{Endpoint: p.Endpoint, Model: p.Model})
		case "openai": c = llm.NewOpenAIClient(p.Endpoint, p.Model, p.APIKey)
		case "vllm":   c = llm.NewVLLMClient(p.Endpoint, p.Model, p.APIKey)
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

func startDockerLogStreamer(ctx context.Context, dockerClient *dockerclient.Client, logAggregator *logging.LogAggregator, centralizedLogs *adapthttp.LogHandler, log *zap.Logger) {
	go func() {
		containers, err := dockerClient.ContainerList(ctx, container.ListOptions{All: true})
		if err != nil {
			log.Warn("Failed to list Docker containers for log streaming", zap.Error(err))
			return
		}
		for _, c := range containers {
			cID := c.ID
			cName := cID
			if len(cID) > 12 { cName = cID[:12] }
			if len(c.Names) > 0 { cName = strings.TrimPrefix(c.Names[0], "/") }
			ns := getContainerNamespace(cName)
			go func(id, name, namespace string) {
				reader, logErr := dockerClient.ContainerLogs(ctx, id, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true, Tail: "50"})
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
						if line == "" { continue }
						lvl := detectLogLevel(line, defaultLvl)
						logAggregator.Ingest(logging.LogEntry{Timestamp: time.Now().UTC(), Namespace: namespace, Pod: name, Container: name, Stream: stream, Level: lvl, Message: line})
						if centralizedLogs != nil {
							_ = centralizedLogs.Ingest(ctx, []domainLogging.LogEntry{{Timestamp: time.Now().UTC(), TenantID: "default-tenant", ClusterID: "default", Namespace: namespace, PodName: name, ContainerName: name, Stream: stream, LogLevel: domainLogging.LogLevel(strings.ToLower(lvl)), Message: line, TraceID: extractTraceID(line), Attributes: map[string]string{"service": name}}})
						}
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

func detectLogLevel(msg, defaultLvl string) string {
	u := strings.ToUpper(msg)
	switch {
	case strings.Contains(u, "ERROR") || strings.Contains(u, "FATAL") || strings.Contains(u, "PANIC") || strings.Contains(u, "ERR"): return "ERROR"
	case strings.Contains(u, "WARN"): return "WARN"
	case strings.Contains(u, "DEBUG") || strings.Contains(u, "TRACE"): return "DEBUG"
	case strings.Contains(u, "INFO"): return "INFO"
	default: return defaultLvl
	}
}

func getContainerNamespace(name string) string {
	l := strings.ToLower(name)
	switch {
	case strings.Contains(l, "log") || strings.Contains(l, "nats"): return "logging"
	case strings.Contains(l, "vault"): return "vault"
	case strings.Contains(l, "stage") || strings.Contains(l, "staging"): return "staging"
	default: return "production"
	}
}

func extractTraceID(msg string) string {
	for _, p := range []string{`"trace_id":`, `"traceId":`, `trace_id=`, `traceId=`, `trace-id=`} {
		if idx := strings.Index(msg, p); idx != -1 {
			rest := strings.TrimSpace(msg[idx+len(p):])
			if len(rest) > 0 {
				if rest[0] == '"' || rest[0] == 0x27 {
					if end := strings.IndexByte(rest[1:], rest[0]); end != -1 { return strings.TrimSpace(rest[1 : end+1]) }
				} else if end := strings.IndexAny(rest, " \t\r\n,;{}]"); end != -1 { return strings.TrimSpace(rest[:end])
				} else { return rest }
			}
		}
	}
	return ""
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
			if conn, cErr := client.Conn(ctx); cErr == nil {
				if mErr := infraClickhouse.RunMigrations(ctx, conn); mErr != nil {
					log.Warn("ClickHouse migration failed", zap.Error(mErr))
				}
			}
			bw := infraClickhouse.NewBatchWriter(client, infraClickhouse.BatchConfig{BatchSize: 5000, FlushInterval: 2 * time.Second, BufferCap: 50000})
			bw.Start(ctx)
			repo := infraClickhouse.NewLogRepository(client, bw)
			service := usecaseLogging.NewService(repo)
			return adapthttp.NewLogHandler(service, &chStatusProvider{client: client, repo: repo}), func() {
				_ = bw.Stop()
				_ = client.Close()
			}
		}
		log.Warn("ClickHouse connection failed, using in-memory ringbuffer fallback", zap.Error(err))
	} else {
		log.Info("ClickHouse not configured, using resilient in-memory ringbuffer fallback")
	}

	memRepo := logging.NewMemoryLogRepo(50000)
	return adapthttp.NewLogHandler(usecaseLogging.NewService(memRepo), memRepo), nil
}

type chStatusProvider struct {
	client *infraClickhouse.Client
	repo   *infraClickhouse.LogRepository
}

func (p *chStatusProvider) QuerySurroundingContext(ctx context.Context, service string, timestamp time.Time, window int) ([]domainLogging.LogEntry, error) {
	if p.repo != nil { return p.repo.QuerySurroundingContext(ctx, service, timestamp, window) }
	return nil, fmt.Errorf("clickhouse repository unavailable")
}

func (p *chStatusProvider) GetStatus(ctx context.Context) (*adapthttp.LogEngineStatus, error) {
	start := time.Now()
	err := p.client.Ping(ctx)
	latency := float64(time.Since(start).Microseconds()) / 1000.0
	status := "connected"
	if err != nil { status = "fallback" }
	var total int64
	if conn, cErr := p.client.Conn(ctx); cErr == nil {
		var cnt uint64
		if scanErr := conn.QueryRow(ctx, "SELECT count() FROM cluster_logs").Scan(&cnt); scanErr == nil { total = int64(cnt) }
	}
	return &adapthttp.LogEngineStatus{Engine: "ClickHouse MergeTree", Status: status, LatencyMS: latency, TotalRecords: total, RetentionDays: 30}, nil
}
