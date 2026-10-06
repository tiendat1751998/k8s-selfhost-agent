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
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
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

	log.Info("Starting K8S Control Plane (standalone mode)",
		zap.String("version", "0.1.0"),
		zap.String("mode", "standalone-enterprise"),
	)

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("failed to load config", zap.Error(err))
	}

	router, cleanup, err := wireStandalone(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer cleanup()

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	if cfg.Server.Port == 0 {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("Dashboard server started", zap.String("addr", addr))
		log.Info("WebSocket endpoint registered", zap.String("endpoint", "/ws"))
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
