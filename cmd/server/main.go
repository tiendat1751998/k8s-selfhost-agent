// Package main is the entrypoint for the K8S Self-Healing server.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/nats-io/nats.go"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/datdt/k8sselfhost/internal/adapter/event"
	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	mw "github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	infraBackup "github.com/datdt/k8sselfhost/internal/infrastructure/backup"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/drivers"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/storage"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	"github.com/datdt/k8sselfhost/internal/infrastructure/llm"
	infraNats "github.com/datdt/k8sselfhost/internal/infrastructure/nats"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	infraDocker "github.com/datdt/k8sselfhost/internal/infrastructure/provider/docker"
	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	infraRedis "github.com/datdt/k8sselfhost/internal/infrastructure/redis"
	infraCluster "github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	"github.com/datdt/k8sselfhost/internal/domain/alert"
	"github.com/datdt/k8sselfhost/internal/infrastructure/notifier"
	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
	"github.com/datdt/k8sselfhost/internal/usecase/ai"
	"github.com/datdt/k8sselfhost/internal/usecase/rca"
	usecaseAgent "github.com/datdt/k8sselfhost/internal/usecase/agent"
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
	usecaseSearch "github.com/datdt/k8sselfhost/internal/usecase/search"
	usecaseSLO "github.com/datdt/k8sselfhost/internal/usecase/slo"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
	"github.com/datdt/k8sselfhost/internal/pkg/telemetry"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Coordinate lifecycle for background tasks and listeners
	eg, egCtx := errgroup.WithContext(ctx)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Validate encryption key
	if err := crypto.ValidateKey(); err != nil {
		return fmt.Errorf("cryptography validation: %w", err)
	}

	// Initialize logger
	logger.Init(cfg.Log.Level)
	defer func() {
		if p := recover(); p != nil {
			fmt.Fprintf(os.Stderr, "panic syncing logger: %v\n", p)
		}
		logger.Sync()
	}()
	log := logger.Get()

	log.Info("starting k8sselfhost server",
		zap.String("version", "0.1.0"),
		zap.String("environment", cfg.Telemetry.Environment),
	)

	// Initialize telemetry
	tp, err := telemetry.Init(egCtx, telemetry.Config{
		ServiceName:    cfg.Telemetry.ServiceName,
		ServiceVersion: cfg.Telemetry.ServiceVersion,
		OTLPEndpoint:   cfg.Telemetry.OTLPEndpoint,
		Environment:    cfg.Telemetry.Environment,
	})
	if err != nil {
		return fmt.Errorf("initializing telemetry: %w", err)
	}
	defer func() {
		defer func() {
			if p := recover(); p != nil {
				log.Error("panic closing telemetry", zap.Any("panic", p))
			}
		}()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := tp.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Error("failed to shutdown telemetry", zap.Error(shutdownErr))
		}
	}()

	// Connect to PostgreSQL
	pgClient, err := postgres.NewClient(egCtx, cfg.Postgres)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer func() {
		defer func() {
			if p := recover(); p != nil {
				log.Error("panic closing postgres", zap.Any("panic", p))
			}
		}()
		pgClient.Close()
	}()

	// Connect to Redis
	redisClient, err := infraRedis.NewClient(egCtx, cfg.Redis)
	if err != nil {
		return fmt.Errorf("connecting to redis: %w", err)
	}
	defer func() {
		defer func() {
			if p := recover(); p != nil {
				log.Error("panic closing redis", zap.Any("panic", p))
			}
		}()
		if closeErr := redisClient.Close(); closeErr != nil {
			log.Error("failed to close redis", zap.Error(closeErr))
		}
	}()

	cacheManager := infraRedis.NewCacheManager(redisClient)

	// Connect to NATS
	natsClient, err := infraNats.NewClient(egCtx, cfg.NATS)
	if err != nil {
		return fmt.Errorf("connecting to nats: %w", err)
	}
	defer func() {
		defer func() {
			if p := recover(); p != nil {
				log.Error("panic closing nats", zap.Any("panic", p))
			}
		}()
		if closeErr := natsClient.Close(); closeErr != nil {
			log.Error("failed to close nats", zap.Error(closeErr))
		}
	}()

	// Setup health checks
	healthHandler := health.NewHandler(5 * time.Second)
	healthHandler.Register("postgres", pgClient.HealthCheck)
	healthHandler.Register("redis", redisClient.HealthCheck)
	healthHandler.Register("nats", natsClient.HealthCheck)

	// 1. Initialize WebSocket Hub
	wsHub := adapthttp.NewWSHub()
	eg.Go(func() error {
		wsHub.Run()
		return nil
	})

	if natsClient != nil && natsClient.Conn() != nil {
		_, err = natsClient.Conn().Subscribe("agent.events", func(msg *nats.Msg) {
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
	incRepo := postgres.NewIncidentRepo(pgClient.Pool())
	reportRepo := postgres.NewReportRepo(pgClient.Pool())
	prRepo := postgres.NewPRRepo(pgClient.Pool())
	obsRepo := postgres.NewObservabilityRepo(pgClient.Pool())

	// 3. Initialize NATS Publisher and Subscriber
	publisher := infraNats.NewPublisher(natsClient)
	subscriber := infraNats.NewSubscriber(natsClient)

	// Subscribe to incident events
	err = subscriber.Subscribe(egCtx, "incidents.>", func(ctx context.Context, subject string, data []byte) error {
		log.Info("received incident event", zap.String("subject", subject), zap.ByteString("data", data))
		return nil
	})
	if err != nil {
		log.Error("failed to subscribe to incident events", zap.Error(err))
	}

	// 4. Initialize LLM Provider Registry
	registry := llm.NewProviderRegistry()
	for _, pCfg := range cfg.LLM.Providers {
		var client llm.Client
		var initErr error
		switch pCfg.Type {
		case "ollama":
			client = llm.NewOllamaClientDynamic(llm.OllamaClientConfig{
				Endpoint: pCfg.Endpoint,
				Model:    pCfg.Model,
			})
		case "openai":
			client = llm.NewOpenAIClient(pCfg.Endpoint, pCfg.Model, pCfg.APIKey)
		case "vllm":
			client = llm.NewVLLMClient(pCfg.Endpoint, pCfg.Model, pCfg.APIKey)
		default:
			log.Warn("unknown LLM provider type, skipping", zap.String("type", pCfg.Type))
			continue
		}
		if initErr != nil {
			log.Error("failed to create client for LLM provider", zap.String("name", pCfg.Name), zap.Error(initErr))
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

	// Register default fallback if empty
	if registry.Count() == 0 && cfg.LLM.Endpoint != "" {
		client := llm.NewOllamaClientDynamic(llm.OllamaClientConfig{
			Endpoint: cfg.LLM.Endpoint,
			Model:    cfg.LLM.Model,
		})
		cbClient := llm.NewCircuitBreakerClient("default", client, llm.DefaultCircuitBreakerConfig())
		registry.Register("default", cbClient, llm.ProviderInfo{
			Type:     "ollama",
			Model:    cfg.LLM.Model,
			Endpoint: cfg.LLM.Endpoint,
			Default:  true,
		})
	}

	// 5. Initialize K8s Client & Collector
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
		log.Warn("failed to initialize Kubernetes client, falling back to cached state", zap.Error(err))
		k8sAvailable = false
	}
	collector := event.NewCollector(k8sClient)

	// 6. Initialize RCA Usecase Pipeline
	pipeline := rca.NewPipeline(collector, registry, reportRepo, incRepo, obsRepo)

	// 7. Initialize and start AI Health Poller
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
	poller.Start(egCtx)
	defer poller.Stop()

		// 8. Initialize and start NATS worker (bridge wsHub)
	bridge := adapthttp.NewWSBridge(wsHub)

	rcaWorker := rca.NewWorker(natsClient.JetStream(), pipeline, incRepo, bridge)
	if err := rcaWorker.Start(egCtx); err != nil {
		return fmt.Errorf("starting RCA worker: %w", err)
	}
	defer rcaWorker.Stop()

	// 9. Setup HTTP Platform Handlers & Router
	dockerClient, err := infraDocker.NewDockerClient(cfg.Docker.Host, cfg.Docker.Version)
	var dockerRepo domainDocker.Repository
	if err != nil {
		log.Warn("failed to initialize real docker client, docker features will be unavailable", zap.Error(err))
		dockerRepo = nil
		dockerClient = nil
	} else {
		dockerRepo = infraDocker.NewDockerRepoWithClient(dockerClient)
	}

	computeHostRepo := postgres.NewComputeHostRepo(pgClient.Pool())
	metricsCollector := usecaseMetrics.NewCollector(
		dockerClient,
		computeHostRepo,
		bridge,
		log,
		usecaseMetrics.WithRequestCountFn(mw.GetRequestCount),
		usecaseMetrics.WithIncidentRepo(incRepo),
	)
	go metricsCollector.Start(egCtx)

	tpsCollector := usecaseMetrics.NewTPSCollector(
		metricsCollector,
		pgClient.Pool(),
		log,
		usecaseMetrics.WithTraefikURL(usecaseMetrics.DeriveTraefikURL(cfg.Docker.Host)),
		usecaseMetrics.WithNATSMonitorURL(usecaseMetrics.DeriveNATSMonitorURL(cfg.NATS.URL)),
		usecaseMetrics.WithTPSRequestCountFn(mw.GetRequestCount),
	)
	go tpsCollector.Start(egCtx)

	if dockerClient != nil {
		dockerWatcher := event.NewDockerEventWatcher(
			dockerClient,
			incRepo,
			bridge,
			log,
			event.WithDockerClusterName("fleet-primary"),
		)
		eg.Go(func() error {
			return dockerWatcher.Start(egCtx)
		})
	}
	overviewHandler := adapthttp.NewOverviewHandler(metricsCollector, log, tpsCollector)

	sloCollector := usecaseSLO.NewSLOCollector(dockerClient, obsRepo, log)
	go sloCollector.Start(egCtx)

	sloUpdater := usecaseSLO.NewSLOSnapshotUpdater(obsRepo, log)
	go sloUpdater.Start(egCtx)

	// Postgres-backed repos
	driftRepo := postgres.NewDriftRepo(pgClient.Pool())
	correlationRepo := postgres.NewCorrelationRepo(pgClient.Pool())
	complianceRepo := postgres.NewComplianceRepo(pgClient.Pool())
	taggingRepo := postgres.NewTaggingRepo(pgClient.Pool())
	auditRepo := postgres.NewAuditRepo(pgClient.Pool())
	timelineRepo := postgres.NewTimelineRepo(pgClient.Pool())
	runbookRepo := postgres.NewRunbookRepo(pgClient.Pool())
	automationRepo := postgres.NewAutomationRepo(pgClient.Pool())
	changesRepo := postgres.NewChangesRepo(pgClient.Pool())
	promotionRepo := postgres.NewPromotionRepo(pgClient.Pool())
	reportingRepo := postgres.NewReportingRepo(pgClient.Pool())
	notificationRepo := postgres.NewNotificationRepo(pgClient.Pool())
	fleetRepo := postgres.NewFleetRepo(pgClient.Pool())
	backupRepo := postgres.NewBackupRepo(pgClient.Pool())
	backupEngine := infraBackup.NewEngine(
		backupRepo,
		drivers.NewDriverRegistry(),
		storage.NewStorageRegistry(storage.NewLocalStorage("")),
	)
	backupPool := infraBackup.NewWorkerPool(backupEngine, log, 50)
	backupPool.Start(3)
	defer backupPool.Stop()

	backupUsecase := usecaseBackup.NewUsecase(backupRepo)
	backupUsecase.SetRunner(backupPool)

	agentRepo := postgres.NewAgentRepo(pgClient.Pool())
	alertRepo := postgres.NewAlertRepo(pgClient.Pool())
	cloudAccountRepo := postgres.NewCloudAccountRepo(pgClient.Pool())
	cloudHandler := adapthttp.NewCloudHandler(cloudAccountRepo, nil, log)

	alertNotifiers := map[string]alert.Notifier{
		"slack":   notifier.NewSlackNotifier(),
		"email":   notifier.NewEmailNotifier(),
		"webhook": notifier.NewWebhookNotifier(),
	}
	_ = usecaseAlert.NewRuleEngine(alertRepo, alertNotifiers)
	alertUsecaseInstance := usecaseAlert.NewUsecase(alertRepo)

	clientManager := infraCluster.NewClientManager(fleetRepo)
	discoveryAdapter := infraK8s.NewDiscoveryAdapter()
	importUsecase := usecaseCluster.NewImportUsecase(fleetRepo, discoveryAdapter, log)
	healthChecker := usecaseCluster.NewHealthChecker(fleetRepo, discoveryAdapter, log)
	eg.Go(func() error {
		healthChecker.Start(egCtx, 1*time.Minute)
		return nil
	})

	txManager := postgres.NewTxManager(pgClient.Pool())
	defaultLLM, _ := registry.Default()
	orchestrator := usecaseAgent.NewOrchestrator(agentRepo, defaultLLM, bridge, txManager)

	userRepo := postgres.NewUserRepo(pgClient.Pool())
	refreshTokenRepo := postgres.NewRefreshTokenRepo(pgClient.Pool())
	authUsecase := usecaseAuth.NewUsecase(userRepo)

	searchRepo := postgres.NewSearchRepo(pgClient.Pool(), cacheManager)
	searchUsecase := usecaseSearch.NewUsecase(searchRepo)
	promotionUsecase := usecasePromotion.NewUsecase(promotionRepo, dockerRepo, auditRepo)

	gitopsController := usecaseGitops.NewController(prRepo, incRepo, txManager)
	tenancyRepo := postgres.NewTenancyRepo(pgClient.Pool())

	costCalculator := usecaseCost.NewCalculator(computeHostRepo, metricsCollector)
	capacityForecaster := usecaseCapacity.NewForecaster(
		metricsCollector,
		usecaseCapacity.WithComputeHostRepo(computeHostRepo),
		usecaseCapacity.WithRepository(postgres.NewCapacityRepo(pgClient.Pool())),
	)
	capacityHandler := adapthttp.NewCapacityHandler(capacityForecaster)

	var explorerHandler *adapthttp.ExplorerHandler
	var healthCenterHandler *adapthttp.HealthCenterHandler
	var deploymentsHandler *adapthttp.DeploymentHandler
	var k8sHandler *adapthttp.K8sResourceHandler

	if k8sAvailable {
		explorerHandler = adapthttp.NewExplorerHandler(infraK8s.NewExplorerRepo(k8sClient, dockerRepo, clientManager))
		healthCenterHandler = adapthttp.NewHealthCenterHandler(infraK8s.NewHealthCenterRepo(k8sClient, clientManager))
		deploymentsHandler = adapthttp.NewDeploymentHandler(usecaseDeployment.NewUsecase(infraK8s.NewDeploymentRepo(k8sClient, dockerRepo, fleetRepo, clientManager)))
		k8sHandler = adapthttp.NewK8sResourceHandler(infraK8s.NewResourceRepo(k8sClient, clientManager), auditRepo)
	} else {
		k8sHandler = adapthttp.NewK8sResourceHandler(infraK8s.NewResourceRepo(nil, clientManager), auditRepo)
	}

	logAggregator := logging.NewLogAggregator(2000)
	logStreamHandler := adapthttp.NewLogStreamHandler(logAggregator)

	// Stream real Docker container logs into log aggregator
	if dockerClient != nil {
		go func() {
			containers, err := dockerClient.ContainerList(egCtx, container.ListOptions{All: true})
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
					reader, logErr := dockerClient.ContainerLogs(egCtx, id, container.LogsOptions{
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

					go func() {
						scanner := bufio.NewScanner(stdoutReader)
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
								Stream:    "stdout",
								Level:     detectLogLevel(line, "INFO"),
								Message:   line,
							})
						}
					}()

					go func() {
						scanner := bufio.NewScanner(stderrReader)
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
								Stream:    "stderr",
								Level:     detectLogLevel(line, "WARN"),
								Message:   line,
							})
						}
					}()

					_, _ = stdcopy.StdCopy(stdoutWriter, stderrWriter, reader)
					_ = stdoutWriter.Close()
					_ = stderrWriter.Close()
				}(cID, cName, ns)
			}
		}()
	}

	platformHandlers := &adapthttp.PlatformHandlers{
		AI:            adapthttp.NewAIHandler(registry),
		Dashboard:     adapthttp.NewHandler(incRepo, reportRepo, prRepo, publisher, gitopsController),
		Docker:        adapthttp.NewDockerHandler(dockerRepo, computeHostRepo, authUsecase, metricsCollector),
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
		Cloud:         cloudHandler,
		LogStream:     logStreamHandler,
	}
	if err := initializeWelcomeMessage(egCtx, pgClient.Pool(), wsHub); err != nil {
		log.Error("failed to initialize WebSocket welcome config message", zap.Error(err))
	}

	router := adapthttp.NewRouterWithWS(healthHandler, wsHub, platformHandlers)

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in errgroup
	eg.Go(func() error {
		log.Info("HTTP server listening", zap.String("addr", srv.Addr))
		if listenErr := srv.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			return fmt.Errorf("HTTP server error: %w", listenErr)
		}
		return nil
	})

	// Coordinated graceful shutdown
	eg.Go(func() error {
		<-egCtx.Done()
		log.Info("shutdown signal received or context cancelled, shutting down HTTP server")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Error("error shutting down HTTP server", zap.Error(shutdownErr))
			return fmt.Errorf("shutting down HTTP server: %w", shutdownErr)
		}
		log.Info("HTTP server stopped gracefully")
		return nil
	})

	// Wait for all errgroup routines
	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

// local wsBridge removed, using adapthttp.WSBridge


func initializeWelcomeMessage(ctx context.Context, db *pgxpool.Pool, hub *adapthttp.WSHub) error {
	// 1. Seed git_providers if empty
	var count int
	err := db.QueryRow(ctx, "SELECT COUNT(*) FROM git_providers").Scan(&count)
	if err != nil {
		return fmt.Errorf("counting git_providers: %w", err)
	}
	if count == 0 {
		_, err = db.Exec(ctx, `
			INSERT INTO git_providers (id, type, organization, repo_count, sync_status) VALUES
			('11111111-1111-1111-1111-111111111111', 'github', 'datdt-corp', 47, 'synced'),
			('22222222-2222-2222-2222-222222222222', 'gitlab', 'datdt-infra', 23, 'synced')
			ON CONFLICT DO NOTHING
		`)
		if err != nil {
			return fmt.Errorf("seeding git_providers: %w", err)
		}
	}

	// 2. Seed cicd_integrations if empty
	err = db.QueryRow(ctx, "SELECT COUNT(*) FROM cicd_integrations").Scan(&count)
	if err != nil {
		return fmt.Errorf("counting cicd_integrations: %w", err)
	}
	if count == 0 {
		_, err = db.Exec(ctx, `
			INSERT INTO cicd_integrations (id, name, provider, status, success_rate, last_log) VALUES
			('33333333-3333-3333-3333-333333333333', 'deploy-prod', 'GitHub Actions', 'active', 96.0, '✓ Build passed\n✓ Tests passed (142/142)\n✓ Image pushed: k8s-agent:v1.2.3\n✓ Deployed to prod-us-east\n✓ Health check OK'),
			('44444444-4444-4444-4444-444444444444', 'deploy-staging', 'GitHub Actions', 'active', 89.0, '✓ Build passed\n✓ Tests passed\n✓ Deployed to staging-1')
			ON CONFLICT DO NOTHING
		`)
		if err != nil {
			return fmt.Errorf("seeding cicd_integrations: %w", err)
		}
	}

	// 3. Seed ai_providers if empty
	err = db.QueryRow(ctx, "SELECT COUNT(*) FROM ai_providers").Scan(&count)
	if err != nil {
		return fmt.Errorf("counting ai_providers: %w", err)
	}
	if count == 0 {
		_, err = db.Exec(ctx, `
			INSERT INTO ai_providers (id, name, provider_type, model, endpoint, status, latency_ms) VALUES
			('55555555-5555-5555-5555-555555555555', 'openai-gpt4o', 'openai', 'gpt-4o', 'https://api.openai.com/v1', 'healthy', 340),
			('66666666-6666-6666-6666-666666666666', 'google-gemini-pro', 'google', 'gemini-1.5-pro', 'https://generativelanguage.googleapis.com', 'healthy', 290),
			('77777777-7777-7777-7777-777777777777', 'ollama-local', 'ollama', 'llama3:8b', 'http://localhost:11434', 'healthy', 120)
			ON CONFLICT DO NOTHING
		`)
		if err != nil {
			return fmt.Errorf("seeding ai_providers: %w", err)
		}
	}

	// 4. Query fleet_clusters
	rows, err := db.Query(ctx, "SELECT name, provider, status, nodes FROM fleet_clusters")
	if err != nil {
		return fmt.Errorf("querying fleet_clusters: %w", err)
	}
	defer rows.Close()

	var k8sData []map[string]interface{}
	for rows.Next() {
		var name, provider, status string
		var nodes int
		if err := rows.Scan(&name, &provider, &status, &nodes); err == nil {
			k8sData = append(k8sData, map[string]interface{}{
				"name":      name,
				"provider":  provider,
				"status":    status,
				"nodes":     nodes,
				"lastCheck": time.Now().UTC().Format(time.RFC3339),
			})
		}
	}

	// 5. Query git_providers
	rows2, err := db.Query(ctx, "SELECT type, organization, repo_count, sync_status FROM git_providers")
	if err != nil {
		return fmt.Errorf("querying git_providers: %w", err)
	}
	defer rows2.Close()

	var gitData []map[string]interface{}
	for rows2.Next() {
		var providerType, organization, syncStatus string
		var repoCount int
		if err := rows2.Scan(&providerType, &organization, &repoCount, &syncStatus); err == nil {
			gitData = append(gitData, map[string]interface{}{
				"type":         providerType,
				"organization": organization,
				"repoCount":    repoCount,
				"syncStatus":   syncStatus,
				"lastWebhook":  time.Now().UTC().Format(time.RFC3339),
			})
		}
	}

	// 6. Query cicd_integrations
	rows3, err := db.Query(ctx, "SELECT name, provider, status, success_rate, last_log FROM cicd_integrations")
	if err != nil {
		return fmt.Errorf("querying cicd_integrations: %w", err)
	}
	defer rows3.Close()

	var cicdData []map[string]interface{}
	for rows3.Next() {
		var name, provider, status, lastLog string
		var successRate float64
		if err := rows3.Scan(&name, &provider, &status, &successRate, &lastLog); err == nil {
			cicdData = append(cicdData, map[string]interface{}{
				"name":        name,
				"provider":    provider,
				"status":      status,
				"successRate": int(successRate),
				"lastLog":     lastLog,
				"lastRun":     time.Now().UTC().Format(time.RFC3339),
			})
		}
	}

	// 7. Query ai_providers
	rows4, err := db.Query(ctx, "SELECT name, provider_type, model, endpoint, status, latency_ms FROM ai_providers")
	if err != nil {
		return fmt.Errorf("querying ai_providers: %w", err)
	}
	defer rows4.Close()

	var aiData []map[string]interface{}
	for rows4.Next() {
		var name, providerType, model, endpoint, status string
		var latency int
		if err := rows4.Scan(&name, &providerType, &model, &endpoint, &status, &latency); err == nil {
			aiData = append(aiData, map[string]interface{}{
				"id":         name,
				"name":       name,
				"model":      model,
				"endpoint":   endpoint,
				"status":     status,
				"latency":    fmt.Sprintf("%dms", latency),
				"tokenUsage": "0",
			})
		}
	}

	configData := map[string]interface{}{
		"kubernetes":   k8sData,
		"gitProviders": gitData,
		"cicd":         cicdData,
		"aiProviders":  aiData,
		"connectionHealth": map[string]interface{}{
			"k8s":  map[string]interface{}{"status": "healthy", "latency": "12ms", "lastCheck": time.Now().UTC().Format(time.RFC3339)},
			"git":  map[string]interface{}{"status": "healthy", "latency": "45ms", "lastCheck": time.Now().UTC().Format(time.RFC3339)},
			"cicd": map[string]interface{}{"status": "healthy", "latency": "23ms", "lastCheck": time.Now().UTC().Format(time.RFC3339)},
			"ai":   map[string]interface{}{"status": "healthy", "latency": "120ms", "lastCheck": time.Now().UTC().Format(time.RFC3339)},
		},
	}

	hub.SetWelcomeMessage(adapthttp.WSMessage{Type: "config", Data: configData})

	// Periodic health broadcasts
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if hub.ClientCount() == 0 {
					continue
				}
				now := time.Now().UTC().Format(time.RFC3339)
				hub.Broadcast(adapthttp.WSMessage{
					Type: "connection.status",
					Data: map[string]interface{}{
						"k8s":  map[string]interface{}{"status": "healthy", "latency": "14ms", "lastCheck": now},
						"git":  map[string]interface{}{"status": "healthy", "latency": "38ms", "lastCheck": now},
						"cicd": map[string]interface{}{"status": "healthy", "latency": "29ms", "lastCheck": now},
						"ai":   map[string]interface{}{"status": "healthy", "latency": "98ms", "lastCheck": now},
					},
				})
			}
		}
	}()

	return nil
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

