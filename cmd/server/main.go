// Package main is the entrypoint for the K8S Self-Healing server.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/datdt/k8sselfhost/internal/adapter/event"
	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	"github.com/datdt/k8sselfhost/internal/infrastructure/llm"
	infraNats "github.com/datdt/k8sselfhost/internal/infrastructure/nats"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	infraDocker "github.com/datdt/k8sselfhost/internal/infrastructure/provider/docker"
	infraRedis "github.com/datdt/k8sselfhost/internal/infrastructure/redis"
	infraCluster "github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
	"github.com/datdt/k8sselfhost/internal/usecase/ai"
	"github.com/datdt/k8sselfhost/internal/usecase/rca"
	usecaseAgent "github.com/datdt/k8sselfhost/internal/usecase/agent"
	"github.com/datdt/k8sselfhost/pkg/health"
	"github.com/datdt/k8sselfhost/pkg/logger"
	"github.com/datdt/k8sselfhost/pkg/telemetry"
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

	// 3. Initialize NATS Publisher
	publisher := infraNats.NewPublisher(natsClient)

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
	k8sClient, err := infraK8s.NewClient(kubeconfigPath)
	if err != nil {
		log.Warn("failed to initialize Kubernetes client, falling back to cached state", zap.Error(err))
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
	bridge := &wsBridge{hub: wsHub}
	rcaWorker := rca.NewWorker(natsClient, pipeline, incRepo, bridge)
	if err := rcaWorker.Start(egCtx); err != nil {
		return fmt.Errorf("starting RCA worker: %w", err)
	}
	defer rcaWorker.Stop()

	// 9. Setup HTTP Platform Handlers & Router
	dockerRepo, err := infraDocker.NewRealDockerRepo(cfg.Docker.Host, cfg.Docker.Version)
	if err != nil {
		log.Fatal("failed to initialize docker repo", zap.Error(err))
	}

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
	costRepo := postgres.NewCostRepo(pgClient.Pool())
	backupRepo := postgres.NewBackupRepo(pgClient.Pool())
	agentRepo := postgres.NewAgentRepo(pgClient.Pool())

	clientManager := infraCluster.NewClientManager(fleetRepo)

	defaultLLM, _ := registry.Default()
	orchestrator := usecaseAgent.NewOrchestrator(agentRepo, defaultLLM, bridge)

	platformHandlers := &adapthttp.PlatformHandlers{
		AI:            adapthttp.NewAIHandler(registry),
		Dashboard:     adapthttp.NewHandler(incRepo, reportRepo, prRepo, publisher),
		Docker:        adapthttp.NewDockerHandler(dockerRepo),
		Drift:         adapthttp.NewDriftHandler(driftRepo),
		Correlation:   adapthttp.NewCorrelationHandler(correlationRepo),
		Compliance:    adapthttp.NewComplianceHandler(complianceRepo),
		Tagging:       adapthttp.NewTaggingHandler(taggingRepo),
		Runbook:       adapthttp.NewRunbookHandler(runbookRepo, auditRepo),
		Observability: adapthttp.NewObservabilityHandler(obsRepo),
		Capacity:      adapthttp.NewCapacityHandler(infraK8s.NewCapacityRepo(k8sClient, clientManager)),
		Changes:       adapthttp.NewChangeHandler(changesRepo),
		Promotion:     adapthttp.NewPromotionHandler(promotionRepo),
		Explorer:      adapthttp.NewExplorerHandler(infraK8s.NewExplorerRepo(k8sClient, dockerRepo, clientManager)),
		Reporting:     adapthttp.NewReportingHandler(reportingRepo),
		HealthCenter:  adapthttp.NewHealthCenterHandler(infraK8s.NewHealthCenterRepo(k8sClient, clientManager)),
		Fleet:         adapthttp.NewFleetHandler(fleetRepo, auditRepo),
		Audit:         adapthttp.NewAuditHandler(auditRepo),
		Notification:  adapthttp.NewNotificationHandler(notificationRepo),
		Automation:    adapthttp.NewAutomationHandler(automationRepo),
		Timeline:      adapthttp.NewTimelineHandler(timelineRepo),
		Auth:          adapthttp.NewAuthHandler(pgClient.Pool()),
		Search:        adapthttp.NewSearchHandler(pgClient.Pool()),
		Cost:          adapthttp.NewCostHandler(costRepo),
		Backup:        adapthttp.NewBackupHandler(backupRepo, wsHub),
		Agents:        adapthttp.NewAgentHandler(agentRepo, orchestrator),
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

type wsBridge struct {
	hub *adapthttp.WSHub
}

func (b *wsBridge) Broadcast(msgType string, data interface{}) {
	b.hub.Broadcast(adapthttp.WSMessage{Type: msgType, Data: data})
}
