// Package main is a standalone development server for the K8S Self-Healing dashboard.
// It runs with PostgreSQL but WITHOUT Redis or NATS — perfect for frontend development.
// Usage: go run ./cmd/standalone
package main

import (
	"bufio"
	"context"
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
	"go.uber.org/zap"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	mw "github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/infrastructure/llm"
	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	infraDocker "github.com/datdt/k8sselfhost/internal/infrastructure/provider/docker"
	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	domainLB "github.com/datdt/k8sselfhost/internal/domain/loadbalancer"
	infraLB "github.com/datdt/k8sselfhost/internal/infrastructure/loadbalancer"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	infraCluster "github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
	usecaseAgent "github.com/datdt/k8sselfhost/internal/usecase/agent"
	usecaseAuth "github.com/datdt/k8sselfhost/internal/usecase/auth"
	usecaseCapacity "github.com/datdt/k8sselfhost/internal/usecase/capacity"
	usecaseCost "github.com/datdt/k8sselfhost/internal/usecase/cost"
	usecaseDeployment "github.com/datdt/k8sselfhost/internal/usecase/deployment"
	usecaseGitops "github.com/datdt/k8sselfhost/internal/usecase/gitops"
	usecaseMetrics "github.com/datdt/k8sselfhost/internal/usecase/metrics"
	usecaseSearch "github.com/datdt/k8sselfhost/internal/usecase/search"
	usecaseBackup "github.com/datdt/k8sselfhost/internal/usecase/backup"
	usecasePromotion "github.com/datdt/k8sselfhost/internal/usecase/promotion"
	usecaseAlert "github.com/datdt/k8sselfhost/internal/usecase/alert"
	usecaseCluster "github.com/datdt/k8sselfhost/internal/usecase/cluster"
	usecaseScaffold "github.com/datdt/k8sselfhost/internal/usecase/scaffold"
	usecaseEcosystem "github.com/datdt/k8sselfhost/internal/usecase/ecosystem"
	usecaseRCA "github.com/datdt/k8sselfhost/internal/usecase/rca"
	usecaseSLO "github.com/datdt/k8sselfhost/internal/usecase/slo"
	"github.com/datdt/k8sselfhost/internal/adapter/event"
	"github.com/datdt/k8sselfhost/internal/pkg/httputil"
	"github.com/datdt/k8sselfhost/internal/domain/alert"
	"github.com/datdt/k8sselfhost/internal/infrastructure/notifier"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
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

	// Validate encryption key
	if err := crypto.ValidateKey(); err != nil {
		return fmt.Errorf("cryptography validation: %w", err)
	}

	logger.Init("debug")
	defer logger.Sync()
	log := logger.Get()

	log.Info("🚀 Starting K8S Control Plane (standalone mode)",
		zap.String("version", "0.1.0"),
		zap.String("mode", "standalone-enterprise"),
	)

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("failed to load config", zap.Error(err))
	}

	healthHandler := health.NewHandler(5 * time.Second)
	wsHub := adapthttp.NewWSHub()
	go wsHub.Run()

	// Initialize LLM Provider Registry
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

	dsn := cfg.Postgres.DSN()
	pgClient, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer pgClient.Close()

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
	computeHostRepo := postgres.NewComputeHostRepo(pgClient)

	txManager := postgres.NewTxManager(pgClient)
	defaultLLM, _ := registry.Default()
	bridge := adapthttp.NewWSBridge(wsHub)
	orchestrator := usecaseAgent.NewOrchestrator(agentRepo, defaultLLM, bridge, txManager)

	metricsCollector := usecaseMetrics.NewCollector(
		dockerClient,
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
		pgClient,
		log,
		usecaseMetrics.WithLoadBalancerProvider(lbProvider),
		usecaseMetrics.WithTraefikURL(lbURL),
		usecaseMetrics.WithNATSMonitorURL(usecaseMetrics.DeriveNATSMonitorURL(cfg.NATS.URL)),
		usecaseMetrics.WithTPSRequestCountFn(mw.GetRequestCount),
	)
	go tpsCollector.Start(ctx)

	if dockerClient != nil {
		dockerEventWatcher := event.NewDockerEventWatcher(
			dockerClient,
			incRepo,
			bridge,
			log,
			event.WithDockerClusterName("fleet-primary"),
		)
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
	alertNotifiers := map[string]alert.Notifier{
		"slack":   notifier.NewSlackNotifier(),
		"email":   notifier.NewEmailNotifier(),
		"webhook": notifier.NewWebhookNotifier(),
	}
	_ = usecaseAlert.NewRuleEngine(alertRepo, alertNotifiers)
	alertUsecaseInstance := usecaseAlert.NewUsecase(alertRepo)

	costCalculator := usecaseCost.NewCalculator(computeHostRepo, metricsCollector)
	capacityForecaster := usecaseCapacity.NewForecaster(
		metricsCollector,
		usecaseCapacity.WithComputeHostRepo(computeHostRepo),
		usecaseCapacity.WithRepository(postgres.NewCapacityRepo(pgClient)),
	)
	capacityHandler := adapthttp.NewCapacityHandler(capacityForecaster)

	var explorerHandler *adapthttp.ExplorerHandler
	var healthCenterHandler *adapthttp.HealthCenterHandler
	var deploymentsHandler *adapthttp.DeploymentHandler
	var k8sHandler *adapthttp.K8sResourceHandler

	deploymentsHandler = adapthttp.NewDeploymentHandler(usecaseDeployment.NewUsecase(infraK8s.NewDeploymentRepo(k8sClient, dockerRepo, fleetRepo, clientManager)))
	explorerHandler = adapthttp.NewExplorerHandler(infraK8s.NewExplorerRepo(k8sClient, dockerRepo, clientManager))

	if k8sAvailable {
		healthCenterHandler = adapthttp.NewHealthCenterHandler(infraK8s.NewHealthCenterRepo(k8sClient, clientManager))
		k8sHandler = adapthttp.NewK8sResourceHandler(infraK8s.NewResourceRepo(k8sClient, clientManager), auditRepo)
	} else {
		k8sHandler = adapthttp.NewK8sResourceHandler(infraK8s.NewResourceRepo(nil, clientManager), auditRepo)
	}

	discoveryAdapter := infraK8s.NewDiscoveryAdapter()
	importUsecase := usecaseCluster.NewImportUsecase(fleetRepo, discoveryAdapter, logger.Get())
	healthChecker := usecaseCluster.NewHealthChecker(fleetRepo, discoveryAdapter, logger.Get())
	go healthChecker.Start(ctx, 1*time.Minute)

	obsRepo := postgres.NewObservabilityRepo(pgClient)
	sloCollector := usecaseSLO.NewSLOCollector(dockerClient, obsRepo, log)
	go sloCollector.Start(ctx)

	sloUpdater := usecaseSLO.NewSLOSnapshotUpdater(obsRepo, log)
	go sloUpdater.Start(ctx)

	rcaCollector := event.NewCollector(k8sClient)
	rcaPipeline := usecaseRCA.NewPipeline(rcaCollector, registry, reportRepo, incRepo, obsRepo)

	dashboardHandler := adapthttp.NewHandler(incRepo, reportRepo, prRepo, nil, gitopsController)
	dashboardHandler.SetRCAPipeline(rcaPipeline)

	logAggregator := logging.NewLogAggregator(2000)
	logStreamHandler := adapthttp.NewLogStreamHandler(logAggregator)

	// Stream real Docker container logs into log aggregator
	if dockerClient != nil {
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

					go func() {
						scanner := bufio.NewScanner(stdoutReader)
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
								Namespace: "docker",
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
				}(cID, cName)
			}
		}()
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
		Cloud:         cloudHandler,
		Settings:      settingsHandler,
		Catalog:       catalogHandler,
		Scaffolder:    scaffoldHandler,
		Plugin:        pluginHandler,
		Ecosystem:     ecosystemHandler,
		LogStream:     logStreamHandler,
	}

	router := adapthttp.NewRouterWithWS(healthHandler, wsHub, platformHandlers)


	addr := ":8080"
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("✅ Dashboard available at http://localhost" + addr)
		log.Info("📡 WebSocket endpoint: ws://localhost" + addr + "/ws")
		if listenErr := srv.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTP server error: %w", listenErr)
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down HTTP server: %w", err)
	}
	log.Info("server stopped gracefully")
	return nil
}



func detectLogLevel(msg string, defaultLvl string) string {
	upper := strings.ToUpper(msg)
	switch {
	case strings.Contains(upper, "ERROR") || strings.Contains(upper, "FATAL") || strings.Contains(upper, "PANIC"):
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

