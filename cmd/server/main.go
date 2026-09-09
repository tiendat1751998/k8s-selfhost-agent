// Package main is the entrypoint for the K8S Self-Healing server.
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
	"golang.org/x/sync/errgroup"

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

	eg, egCtx := errgroup.WithContext(ctx)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := crypto.ValidateKey(); err != nil {
		return fmt.Errorf("cryptography validation: %w", err)
	}

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

	infra, err := initInfrastructure(egCtx, cfg, log)
	if err != nil {
		return err
	}
	defer infra.close(log)

	services, err := initServices(egCtx, eg, cfg, infra, log)
	if err != nil {
		return err
	}
	defer services.stop()

	srv := newHTTPServer(egCtx, cfg, infra, services, log)

	eg.Go(func() error {
		log.Info("HTTP server listening", zap.String("addr", srv.Addr))
		if listenErr := srv.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			return fmt.Errorf("HTTP server error: %w", listenErr)
		}
		return nil
	})

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

	return eg.Wait()
}
