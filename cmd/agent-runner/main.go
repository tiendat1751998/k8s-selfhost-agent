// Package main is the entrypoint for the Agent Runner.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
			if pubErr := b.client.Conn().Publish("agent.events", payload); pubErr != nil {
				logger.Get().Error("failed to publish agent event", zap.Error(pubErr))
			}
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
	txManager := postgres.NewTxManager(pgClient.Pool())
	orchestrator := usecaseAgent.NewOrchestrator(agentRepo, defaultLLM, bridge, txManager)
	runner := usecaseAgent.NewRunner(agentRepo, orchestrator, 5*time.Second)

	// Start health server
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})
		mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
			if bridge.client != nil && bridge.client.Conn() != nil && bridge.client.Conn().IsConnected() {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("NATS disconnected"))
			}
		})
		srv := &http.Server{
			Addr:    ":8081",
			Handler: mux,
		}
		log.Info("starting health server on :8081")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("health server failed", zap.Error(err))
		}
	}()

	// Start active polling loop
	log.Info("agent-runner ready, starting runner loop")
	runner.Start(ctx)
	defer runner.Stop()

	<-ctx.Done()
	log.Info("agent-runner shutting down")

	return nil
}
