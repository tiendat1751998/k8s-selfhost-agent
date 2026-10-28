package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
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
	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	infraLB "github.com/datdt/k8sselfhost/internal/infrastructure/loadbalancer"
	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	gitproviderGitea "github.com/datdt/k8sselfhost/internal/infrastructure/gitprovider/gitea"
	gitproviderGithub "github.com/datdt/k8sselfhost/internal/infrastructure/gitprovider/github"
	gitproviderGitlab "github.com/datdt/k8sselfhost/internal/infrastructure/gitprovider/gitlab"
	"github.com/datdt/k8sselfhost/internal/infrastructure/llm"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	infraNats "github.com/datdt/k8sselfhost/internal/infrastructure/nats"
	"github.com/datdt/k8sselfhost/internal/infrastructure/notifier"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	"github.com/datdt/k8sselfhost/internal/infrastructure/telegram"
	usecaseAgent "github.com/datdt/k8sselfhost/internal/usecase/agent"
	"github.com/datdt/k8sselfhost/internal/usecase/ai"
	usecaseAlert "github.com/datdt/k8sselfhost/internal/usecase/alert"
	usecaseAuth "github.com/datdt/k8sselfhost/internal/usecase/auth"
	usecaseBackup "github.com/datdt/k8sselfhost/internal/usecase/backup"
	usecaseCluster "github.com/datdt/k8sselfhost/internal/usecase/cluster"
	usecaseDeployment "github.com/datdt/k8sselfhost/internal/usecase/deployment"
	usecaseGitops "github.com/datdt/k8sselfhost/internal/usecase/gitops"
	usecaseMetrics "github.com/datdt/k8sselfhost/internal/usecase/metrics"
	usecasePromotion "github.com/datdt/k8sselfhost/internal/usecase/promotion"
	"github.com/datdt/k8sselfhost/internal/usecase/rca"
	usecaseSearch "github.com/datdt/k8sselfhost/internal/usecase/search"
	usecaseSLO "github.com/datdt/k8sselfhost/internal/usecase/slo"
	"errors"
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
	stopFns = append(stopFns, mw.CloseDefaultTokenDenylist)

	// 1. Initialize WebSocket Hub
	wsHub := adapthttp.NewWSHub()
	eg.Go(func() error {
		wsHub.Run(ctx)
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
		usecaseMetrics.WithHostProbe(os.Getenv("ENABLE_LOCAL_HOST_PROBE") == "true"),
	)
	go metricsCollector.Start(ctx)

	lbURL := cfg.LoadBalancer.URL
	if lbURL == "" {
		lbURL = usecaseMetrics.DeriveTraefikURL(cfg.Docker.Host)
	}
	if usecaseMetrics.IsSelfServerURL(lbURL, cfg.Server.Port) {
		lbURL = ""
	}
	var lbProvider domainLB.Provider
	if (cfg.LoadBalancer.Provider == "traefik" || cfg.LoadBalancer.Provider == "") && lbURL != "" {
		lbProvider = infraLB.NewTraefikProvider(lbURL)
	}
	if provider, ok := lbProvider.(*infraLB.TraefikProvider); ok && lbURL != "" {
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
	nodeHistoryWorker := usecaseMetrics.NewNodeHistoryWorker(
		metricsCollector, nodeMetricsRepo, incRepo, log,
		usecaseMetrics.WithComputeHostLister(computeHostRepo),
	)
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
		for i := len(stopFns) - 1; i >= 0; i-- { stopFns[i]() }
		return nil, fmt.Errorf("initializing alert rule engine: %w", err)
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

	clientManager := infraCluster.NewClientManager(fleetRepo)
	discoveryAdapter := infraK8s.NewDiscoveryAdapter()
	if err := crypto.ValidateKey(); err != nil {
		for i := len(stopFns) - 1; i >= 0; i-- { stopFns[i]() }
		return nil, fmt.Errorf("cluster encryption key validation failed: %w", err)
	}
	importUsecase := usecaseCluster.NewImportUsecase(fleetRepo, discoveryAdapter, log)
	healthChecker := usecaseCluster.NewHealthChecker(fleetRepo, discoveryAdapter, log,
		usecaseCluster.WithComputeHostRepo(computeHostRepo),
	)
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
	authUsecase := usecaseAuth.NewUsecase(
		userRepo,
		usecaseAuth.WithRefreshTokenRepo(refreshTokenRepo),
		usecaseAuth.WithTxManager(txManager),
	)

	searchRepo := postgres.NewSearchRepo(infra.pgClient.Pool())
	searchUsecase := usecaseSearch.NewUsecase(searchRepo)
	promotionUsecase := usecasePromotion.NewUsecase(postgres.NewPromotionRepo(infra.pgClient.Pool()), infra.dockerRepo, auditRepo)
	gitopsController := usecaseGitops.NewController(prRepo, incRepo, txManager)
	if cfg.GitOps.GitHubToken != "" {
		gitopsController.RegisterProvider(domainGitops.ProviderGitHub, gitproviderGithub.NewProvider(cfg.GitOps.GitHubToken))
	}
	if cfg.GitOps.GitLabToken != "" {
		gitopsController.RegisterProvider(domainGitops.ProviderGitLab, gitproviderGitlab.NewProvider(cfg.GitOps.GitLabToken, cfg.GitOps.GitLabURL))
	}
	if cfg.GitOps.GiteaToken != "" {
		gitopsController.RegisterProvider(domainGitops.ProviderGitea, gitproviderGitea.NewProvider(cfg.GitOps.GiteaToken, cfg.GitOps.GiteaURL))
	}

	deploymentUsecaseInstance := usecaseDeployment.NewUsecase(infraK8s.NewDeploymentRepo(infra.k8sClient, infra.dockerRepo, fleetRepo, clientManager))
	telegramBot := telegram.NewSREBot(telegram.TelegramConfig{
		BotToken:            cfg.Telegram.BotToken,
		AdminChatIDs:        cfg.Telegram.AdminChatIDs,
		EnableDeduplication: true,
		DebounceWindow:      30 * time.Second,
	}, nil, deploymentUsecaseInstance, backupUsecase, log)

	if cfg.Telegram.BotToken != "" {
		go func() {
			log.Info("Starting Telegram SRE Copilot background bot...")
			if err := telegramBot.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("Telegram bot listener exited with error", zap.Error(err))
			}
		}()
	}

	logAggregator := logging.NewLogAggregator(2000)
	logStreamHandler := adapthttp.NewLogStreamHandler(logAggregator, computeHostRepo)
	centralizedLogs, chCleanup := wireCentralizedLogging(ctx, log)
	if chCleanup != nil {
		stopFns = append(stopFns, chCleanup)
	}

	if infra.dockerClient != nil {
		startDockerLogStreamer(ctx, infra.dockerClient, logAggregator, centralizedLogs, log)
	}

	platformHandlers := initPlatformHandlers(platformHandlerParams{
		cfg:                       cfg,
		infra:                     infra,
		clientManager:             clientManager,
		auditRepo:                 auditRepo,
		gitopsController:          gitopsController,
		telegramBot:               telegramBot,
		computeHostRepo:           computeHostRepo,
		logStreamHandler:          logStreamHandler,
		centralizedLogs:           centralizedLogs,
		registry:                  registry,
		incRepo:                   incRepo,
		reportRepo:                reportRepo,
		prRepo:                    prRepo,
		publisher:                 publisher,
		authUsecase:               authUsecase,
		metricsCollector:          metricsCollector,
		overviewHandler:           overviewHandler,
		obsRepo:                   obsRepo,
		promotionUsecase:          promotionUsecase,
		fleetRepo:                 fleetRepo,
		importUsecase:             importUsecase,
		userRepo:                  userRepo,
		refreshTokenRepo:          refreshTokenRepo,
		searchUsecase:             searchUsecase,
		backupUsecase:             backupUsecase,
		agentRepo:                 agentRepo,
		orchestrator:              orchestrator,
		deploymentUsecaseInstance:  deploymentUsecaseInstance,
		alertUsecaseInstance:      alertUsecaseInstance,
		cloudHandler:              cloudHandler,
		log:                       log,
	})

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

type serverDockerLogClient interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerLogs(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error)
}

func startDockerLogStreamer(ctx context.Context, dockerClient serverDockerLogClient, logAggregator *logging.LogAggregator, centralizedLogs *adapthttp.LogHandler, log *zap.Logger) {
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

				done := make(chan struct{})
				go func() {
					select {
					case <-ctx.Done(): _ = reader.Close()
					case <-done:
					}
				}()
				defer close(done)

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
							entries := []domainLogging.LogEntry{{Timestamp: time.Now().UTC(), TenantID: "default-tenant", ClusterID: "default", Namespace: namespace, PodName: name, ContainerName: name, Stream: stream, LogLevel: domainLogging.LogLevel(strings.ToLower(lvl)), Message: line, TraceID: extractTraceID(line), Attributes: map[string]string{"service": name}}}
							if err := centralizedLogs.Ingest(ctx, entries); err != nil {
								logger.Get().Debug("log ingestion warning", zap.Error(err))
							}
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

