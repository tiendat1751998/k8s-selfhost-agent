package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func setupHandler(collector *SystemCollector, authToken string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if authToken != "" {
			authHeader := r.Header.Get("Authorization")
			expected := "Bearer " + authToken
			if authHeader != expected {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}
		}

		metrics := collector.GetLastMetrics()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(metrics)
	})

	return mux
}

func main() {
	var (
		port      int
		interval  time.Duration
		authToken string
	)

	flag.IntVar(&port, "port", 9100, "Port for metrics HTTP server")
	flag.DurationVar(&interval, "interval", 5*time.Second, "Metrics collection interval")
	flag.StringVar(&authToken, "auth-token", "", "Optional Bearer token for authentication")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	logger.Info("Starting k8s monitoring agent",
		slog.Int("port", port),
		slog.Duration("interval", interval),
		slog.Bool("auth_enabled", authToken != ""),
	)

	collector := NewSystemCollector("", "", nil, WithCollectionInterval(interval))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go collector.Start(ctx)

	handler := setupHandler(collector, authToken)
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Agent HTTP server failed", slog.String("error", err.Error()))
			cancel()
		}
	}()

	logger.Info("Monitoring agent listening", slog.String("addr", srv.Addr))

	<-ctx.Done()
	logger.Info("Shutting down monitoring agent...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	collector.Stop()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error during agent shutdown", slog.String("error", err.Error()))
	}
	logger.Info("Monitoring agent exited cleanly")
}
