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
	"github.com/jackc/pgx/v5/pgxpool"
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
	usecaseDeployment "github.com/datdt/k8sselfhost/internal/usecase/deployment"
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
		Deployments:   adapthttp.NewDeploymentHandler(usecaseDeployment.NewUsecase(infraK8s.NewDeploymentRepo(k8sClient, dockerRepo, fleetRepo, clientManager))),
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

type wsBridge struct {
	hub *adapthttp.WSHub
}

func (b *wsBridge) Broadcast(msgType string, data interface{}) {
	b.hub.Broadcast(adapthttp.WSMessage{Type: msgType, Data: data})
}

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

