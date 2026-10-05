package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	dockerclient "github.com/docker/docker/client"

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

// AgentConfig holds the configuration settings for the agent.
type AgentConfig struct {
	Port      int
	BindAddr  string
	Interval  time.Duration
	AuthToken string
	LogDir    string
	EngineDir string
}

// validateConfig validates that required configuration options are present.
// The agent refuses to start if no authentication token is provided or if port is invalid.
func validateConfig(cfg AgentConfig) error {
	if strings.TrimSpace(cfg.AuthToken) == "" {
		return fmt.Errorf("agent authentication token is required: provide via --auth-token flag or AGENT_AUTH_TOKEN environment variable")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("invalid port %d: must be between 1 and 65535", cfg.Port)
	}
	return nil
}

// resolveListenAddr formats the server bind address based on bindAddr and port.
// Defaults to ":<port>" when bindAddr is empty, preserving existing behavior.
func resolveListenAddr(bindAddr string, port int) string {
	bindAddr = strings.TrimSpace(bindAddr)
	if bindAddr == "" {
		return fmt.Sprintf(":%d", port)
	}
	if strings.Contains(bindAddr, ":") {
		if _, _, err := net.SplitHostPort(bindAddr); err == nil {
			return bindAddr
		}
	}
	return net.JoinHostPort(bindAddr, strconv.Itoa(port))
}

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
			return false
		}
		authHeader := r.Header.Get("Authorization")
		expected := "Bearer " + authToken
		return subtle.ConstantTimeCompare([]byte(authHeader), []byte(expected)) == 1
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

	mux.HandleFunc("/logs/status", func(w http.ResponseWriter, r *http.Request) {
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
		ls.HandleEngineStatus(w, r)
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

	mux.HandleFunc("/logs/tail", func(w http.ResponseWriter, r *http.Request) {
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
		ls.HandleTailLogs(w, r)
	})

	return mux
}

// applyEnvOverrides updates configuration parameters from environment variables
// when their respective flags remain at default or empty values.
func applyEnvOverrides(port int, bindAddr, logDir, engineDir, authToken string) (int, string, string, string, string) {
	if port == 9100 {
		if p := os.Getenv("AGENT_PORT"); p != "" {
			if parsedPort, err := strconv.Atoi(p); err == nil && parsedPort > 0 {
				port = parsedPort
			}
		}
	}
	if bindAddr == "" {
		if envBind := os.Getenv("AGENT_BIND_ADDR"); envBind != "" {
			bindAddr = envBind
		}
	}
	if logDir == "/var/log" {
		if envLogDir := os.Getenv("AGENT_LOG_DIR"); envLogDir != "" {
			logDir = envLogDir
		}
	}
	if engineDir == "" {
		if envEngineDir := os.Getenv("AGENT_ENGINE_DIR"); envEngineDir != "" {
			engineDir = envEngineDir
		}
	}
	if authToken == "" {
		if envToken := os.Getenv("AGENT_AUTH_TOKEN"); envToken != "" {
			authToken = envToken
		}
	}
	return port, bindAddr, logDir, engineDir, authToken
}

// isDirWritable checks if the specified directory can be created and written to.
func isDirWritable(dir string) bool {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false
	}
	probe, err := os.CreateTemp(dir, ".probe-*")
	if err != nil {
		return false
	}
	_ = probe.Close()
	_ = os.Remove(probe.Name())
	return true
}

// resolveEngineDir selects a writable directory for the columnar log engine using a prioritized
// candidate list with permission-aware fallback:
// Candidate 1: requestedEngineDir if specified
// Candidate 2: filepath.Join(logDir, ".logengine")
// Candidate 3: filepath.Join(homeDir, ".k8s-agent", "logengine")
// Candidate 4: filepath.Join(os.TempDir(), "k8s-agent-logengine") as ultimate fallback
func resolveEngineDir(logDir, requestedEngineDir string, logger *slog.Logger) string {
	candidates := make([]string, 0, 4)
	if requestedEngineDir != "" {
		candidates = append(candidates, requestedEngineDir)
	}
	if logDir != "" {
		candidates = append(candidates, filepath.Join(logDir, ".logengine"))
	}
	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		candidates = append(candidates, filepath.Join(homeDir, ".k8s-agent", "logengine"))
	}
	tempFallback := filepath.Join(os.TempDir(), "k8s-agent-logengine")
	candidates = append(candidates, tempFallback)

	for _, candidate := range candidates {
		if isDirWritable(candidate) {
			if logger != nil {
				logger.Info("Columnar engine directory selected", slog.String("engine_dir", candidate))
			}
			return candidate
		}
		if logger != nil {
			logger.Warn("Candidate engine directory not writable, trying next fallback", slog.String("candidate", candidate))
		}
	}

	if logger != nil {
		logger.Warn("All engine directory candidates failed writability check, defaulting to temp", slog.String("engine_dir", tempFallback))
	}
	return tempFallback
}

func main() {
	var (
		port      int
		bindAddr  string
		interval  time.Duration
		authToken string
		logDir    string
		engineDir string
	)

	flag.IntVar(&port, "port", 9100, "Port for metrics and logs HTTP server")
	flag.StringVar(&bindAddr, "bind", "", "Optional bind address or host:port for HTTP server (default \":<port>\")")
	flag.DurationVar(&interval, "interval", 5*time.Second, "Metrics collection interval")
	flag.StringVar(&authToken, "auth-token", "", "Required Bearer token for authentication")
	flag.StringVar(&logDir, "log-dir", "/var/log", "Directory for local log file scanning")
	flag.StringVar(&engineDir, "engine-dir", "", "Directory for columnar log engine data")
	flag.Parse()

	port, bindAddr, logDir, engineDir, authToken = applyEnvOverrides(port, bindAddr, logDir, engineDir, authToken)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := AgentConfig{
		Port:      port,
		BindAddr:  bindAddr,
		Interval:  interval,
		AuthToken: authToken,
		LogDir:    logDir,
		EngineDir: engineDir,
	}

	if err := validateConfig(cfg); err != nil {
		logger.Error("Agent startup failed: invalid configuration", slog.String("error", err.Error()))
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	logengine.InitEngineRuntime()

	listenAddr := resolveListenAddr(bindAddr, port)

	logger.Info("Starting k8s monitoring agent",
		slog.Int("port", port),
		slog.String("bind", listenAddr),
		slog.Duration("interval", interval),
		slog.Bool("auth_enabled", authToken != ""),
		slog.String("log_dir", logDir),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	collector := NewSystemCollector("", "", nil, WithCollectionInterval(interval))
	go collector.Start(ctx)

	engineDir = resolveEngineDir(logDir, engineDir, logger)

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

		// Continuous container log tailing
		if dockerCli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation()); err == nil {
			logServer.SetDockerClient(dockerCli)
			containerTailer := NewContainerTailer(dockerCli, engineSrc.Writer(), logger)
			go containerTailer.Start(ctx)
		}

		// Continuous journalctl log tailing
		journalTailer := NewJournalTailer(engineSrc.Writer(), logger)
		go journalTailer.Start(ctx)
	} else {
		logger.Warn("Failed to initialize columnar log engine, falling back to standard sources", slog.String("error", err.Error()))
	}

	handler := setupHandler(collector, authToken, logServer)
	srv := &http.Server{
		Addr:         listenAddr,
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
