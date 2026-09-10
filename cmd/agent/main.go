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
	"path/filepath"
	"syscall"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

func setupHandler(collector *SystemCollector, authToken string, logServers ...*LogServer) http.Handler {
	var ls *LogServer
	if len(logServers) > 0 && logServers[0] != nil {
		ls = logServers[0]
	} else {
		ls = NewLogServer()
	}

	mux := http.NewServeMux()

	isAuthorized := func(r *http.Request) bool {
		if authToken == "" {
			return true
		}
		authHeader := r.Header.Get("Authorization")
		expected := "Bearer " + authToken
		return authHeader == expected
	}

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthorized(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}

		metrics := collector.GetLastMetrics()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(metrics)
	})

	mux.HandleFunc("/logs/services", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
			return
		}
		if !isAuthorized(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		ls.HandleGetServices(w, r)
	})

	mux.HandleFunc("/logs/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
			return
		}
		if !isAuthorized(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		ls.HandleSearchLogs(w, r)
	})

	mux.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
			return
		}
		if !isAuthorized(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		ls.HandleGetLogs(w, r)
	})

	return mux
}

func main() {
	var (
		port      int
		interval  time.Duration
		authToken string
		logDir    string
		engineDir string
	)

	flag.IntVar(&port, "port", 9100, "Port for metrics and logs HTTP server")
	flag.DurationVar(&interval, "interval", 5*time.Second, "Metrics collection interval")
	flag.StringVar(&authToken, "auth-token", "", "Optional Bearer token for authentication")
	flag.StringVar(&logDir, "log-dir", "/var/log", "Directory for local log file scanning")
	flag.StringVar(&engineDir, "engine-dir", "", "Directory for columnar log engine data")
	flag.Parse()

	logengine.InitEngineRuntime()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	logger.Info("Starting k8s monitoring agent",
		slog.Int("port", port),
		slog.Duration("interval", interval),
		slog.Bool("auth_enabled", authToken != ""),
		slog.String("log_dir", logDir),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	collector := NewSystemCollector("", "", nil, WithCollectionInterval(interval))
	go collector.Start(ctx)

	if engineDir == "" {
		engineDir = filepath.Join(logDir, ".logengine")
	}

	logServer := NewLogServer(WithLogDir(logDir))
	var engineSrc *EngineLogSource

	if eng, err := NewEngineLogSource(engineDir); err == nil {
		engineSrc = eng
		logger.Info("Columnar log engine initialized", slog.String("engine_dir", engineDir))
		logServer.AddSource(engineSrc)

		fileSrc := &FileLogSource{logDir: logDir}
		go func() {
			if err := fileSrc.IngestToWriter(ctx, engineSrc.Writer()); err != nil && ctx.Err() == nil {
				logger.Warn("Initial log ingestion into log engine encountered warning", slog.String("error", err.Error()))
			}
		}()
	} else {
		logger.Warn("Failed to initialize columnar log engine, falling back to standard sources", slog.String("error", err.Error()))
	}

	handler := setupHandler(collector, authToken, logServer)
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
	if engineSrc != nil {
		if err := engineSrc.Close(); err != nil {
			logger.Error("Error closing log engine", slog.String("error", err.Error()))
		}
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error during agent shutdown", slog.String("error", err.Error()))
	}
	logger.Info("Monitoring agent exited cleanly")
}
