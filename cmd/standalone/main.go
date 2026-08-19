// Package main is a standalone development server for the K8S Self-Healing dashboard.
// It runs with PostgreSQL but WITHOUT Redis or NATS — perfect for frontend development.
// Usage: go run ./cmd/standalone
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"go.uber.org/zap"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/infrastructure/llm"
	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	infraDocker "github.com/datdt/k8sselfhost/internal/infrastructure/provider/docker"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	infraCluster "github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
	usecaseAgent "github.com/datdt/k8sselfhost/internal/usecase/agent"
	usecaseAuth "github.com/datdt/k8sselfhost/internal/usecase/auth"
	usecaseDeployment "github.com/datdt/k8sselfhost/internal/usecase/deployment"
	usecaseGitops "github.com/datdt/k8sselfhost/internal/usecase/gitops"
	usecaseSearch "github.com/datdt/k8sselfhost/internal/usecase/search"
	usecaseBackup "github.com/datdt/k8sselfhost/internal/usecase/backup"
	usecasePromotion "github.com/datdt/k8sselfhost/internal/usecase/promotion"
	usecaseAlert "github.com/datdt/k8sselfhost/internal/usecase/alert"
	usecaseCluster "github.com/datdt/k8sselfhost/internal/usecase/cluster"
	"github.com/datdt/k8sselfhost/internal/domain/alert"
	"github.com/datdt/k8sselfhost/internal/infrastructure/notifier"
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

	dockerRepo, err := infraDocker.NewRealDockerRepo(cfg.Docker.Host, cfg.Docker.Version)
	if err != nil {
		log.Warn("failed to initialize real docker client, docker features will be unavailable", zap.Error(err))
		dockerRepo = nil
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
	costRepo := postgres.NewCostRepo(pgClient)
	backupRepo := postgres.NewBackupRepo(pgClient)
	cloudAccountRepo := postgres.NewCloudAccountRepo(pgClient)
	cloudHandler := adapthttp.NewCloudHandler(cloudAccountRepo, nil, log)

	txManager := postgres.NewTxManager(pgClient)
	defaultLLM, _ := registry.Default()
	bridge := adapthttp.NewWSBridge(wsHub)
	orchestrator := usecaseAgent.NewOrchestrator(agentRepo, defaultLLM, bridge, txManager)

	userRepo := postgres.NewUserRepo(pgClient)
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

	var capacityHandler *adapthttp.CapacityHandler
	var explorerHandler *adapthttp.ExplorerHandler
	var healthCenterHandler *adapthttp.HealthCenterHandler
	var deploymentsHandler *adapthttp.DeploymentHandler
	var k8sHandler *adapthttp.K8sResourceHandler

	if k8sAvailable {
		capacityHandler = adapthttp.NewCapacityHandler(infraK8s.NewCapacityRepo(k8sClient, clientManager))
		explorerHandler = adapthttp.NewExplorerHandler(infraK8s.NewExplorerRepo(k8sClient, dockerRepo, clientManager))
		healthCenterHandler = adapthttp.NewHealthCenterHandler(infraK8s.NewHealthCenterRepo(k8sClient, clientManager))
		deploymentsHandler = adapthttp.NewDeploymentHandler(usecaseDeployment.NewUsecase(infraK8s.NewDeploymentRepo(k8sClient, dockerRepo, fleetRepo, clientManager)))
		k8sHandler = adapthttp.NewK8sResourceHandler(infraK8s.NewResourceRepo(k8sClient, clientManager), auditRepo)
	} else {
		k8sHandler = adapthttp.NewK8sResourceHandler(infraK8s.NewResourceRepo(nil, clientManager), auditRepo)
	}

	discoveryAdapter := infraK8s.NewDiscoveryAdapter()
	importUsecase := usecaseCluster.NewImportUsecase(fleetRepo, discoveryAdapter, logger.Get())
	healthChecker := usecaseCluster.NewHealthChecker(fleetRepo, discoveryAdapter, logger.Get())
	go healthChecker.Start(ctx, 1*time.Minute)

	platformHandlers := &adapthttp.PlatformHandlers{
		Dashboard:     adapthttp.NewHandler(incRepo, reportRepo, prRepo, nil, gitopsController),
		Docker:        adapthttp.NewDockerHandler(dockerRepo),
		Drift:         adapthttp.NewDriftHandler(postgres.NewDriftRepo(pgClient)),
		Correlation:   adapthttp.NewCorrelationHandler(postgres.NewCorrelationRepo(pgClient)),
		Compliance:    adapthttp.NewComplianceHandler(postgres.NewComplianceRepo(pgClient)),
		Tagging:       adapthttp.NewTaggingHandler(postgres.NewTaggingRepo(pgClient)),
		Runbook:       adapthttp.NewRunbookHandler(postgres.NewRunbookRepo(pgClient), auditRepo),
		Observability: adapthttp.NewObservabilityHandler(postgres.NewObservabilityRepo(pgClient)),
		Capacity:      capacityHandler,
		Changes:       adapthttp.NewChangeHandler(postgres.NewChangesRepo(pgClient)),
		Promotion:     adapthttp.NewPromotionHandler(usecasePromotion.NewUsecase(postgres.NewPromotionRepo(pgClient))),
		Explorer:      explorerHandler,
		Reporting:     adapthttp.NewReportingHandler(postgres.NewReportingRepo(pgClient)),
		HealthCenter:  healthCenterHandler,
		Fleet:         adapthttp.NewFleetHandler(fleetRepo, auditRepo, importUsecase),
		Audit:         adapthttp.NewAuditHandler(auditRepo),
		Notification:  adapthttp.NewNotificationHandler(postgres.NewNotificationRepo(pgClient)),
		Automation:    adapthttp.NewAutomationHandler(postgres.NewAutomationRepo(pgClient)),
		Timeline:      adapthttp.NewTimelineHandler(postgres.NewTimelineRepo(pgClient)),
		AI:            adapthttp.NewAIHandler(registry),
		Auth:          adapthttp.NewAuthHandler(authUsecase),
		Search:        adapthttp.NewSearchHandler(searchUsecase),
		Cost:          adapthttp.NewCostHandler(costRepo),
		Backup:        adapthttp.NewBackupHandler(usecaseBackup.NewUsecase(backupRepo)),
		Agents:        adapthttp.NewAgentHandler(agentRepo, orchestrator),
		Deployments:   deploymentsHandler,
		Tenancy:       adapthttp.NewTenancyHandler(tenancyRepo),
		Alert:         adapthttp.NewAlertHandler(alertUsecaseInstance),
		K8s:           k8sHandler,
		Cloud:         cloudHandler,
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



// local wsBridge removed, using adapthttp.WSBridge

