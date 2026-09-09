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

	log.Info("🚀 Starting K8S Control Plane (standalone mode)",
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
