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
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
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
	registry := llm.InitRegistry(cfg.LLM)

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
