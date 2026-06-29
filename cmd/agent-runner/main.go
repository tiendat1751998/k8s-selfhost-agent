// Package main is the entrypoint for the Agent Runner.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	"github.com/datdt/k8sselfhost/internal/infrastructure/llm"
	infraNats "github.com/datdt/k8sselfhost/internal/infrastructure/nats"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	usecaseAgent "github.com/datdt/k8sselfhost/internal/usecase/agent"
	"github.com/datdt/k8sselfhost/pkg/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

type natsBridge struct {
	client *infraNats.Client
}

func (b *natsBridge) Broadcast(msgType string, data interface{}) {
	if b.client != nil && b.client.Conn() != nil {
		wsMsg := adapthttp.WSMessage{
			Type: msgType,
			Data: data,
		}
		payload, err := json.Marshal(wsMsg)
		if err == nil {
			_ = b.client.Conn().Publish("agent.events", payload)
		}
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Initialize logger
	logger.Init(cfg.Log.Level)
	defer logger.Sync()
	log := logger.Get()

	log.Info("starting agent-runner",
		zap.String("version", "0.1.0"),
	)

	// Connect to PostgreSQL
	pgClient, err := postgres.NewClient(ctx, cfg.Postgres)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer pgClient.Close()

	// Connect to NATS
	natsClient, err := infraNats.NewClient(ctx, cfg.NATS)
	if err != nil {
		return fmt.Errorf("connecting to nats: %w", err)
	}
	defer natsClient.Close()

	// Initialize LLM Provider Registry
	registry := llm.NewProviderRegistry()
	for _, pCfg := range cfg.LLM.Providers {
		var client llm.Client
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

	// Setup repositories & usecases
	agentRepo := postgres.NewAgentRepo(pgClient.Pool())
	defaultLLM, err := registry.Default()
	if err != nil {
		log.Warn("no default LLM provider available, proceeding with caution", zap.Error(err))
	}

	bridge := &natsBridge{client: natsClient}
	orchestrator := usecaseAgent.NewOrchestrator(agentRepo, defaultLLM, bridge)
	runner := usecaseAgent.NewRunner(agentRepo, orchestrator, 5*time.Second)

	// Start active polling loop
	log.Info("agent-runner ready, starting runner loop")
	runner.Start(ctx)
	defer runner.Stop()

	<-ctx.Done()
	log.Info("agent-runner shutting down")

	return nil
}
