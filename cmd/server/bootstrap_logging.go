package main

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	infraClickhouse "github.com/datdt/k8sselfhost/internal/infrastructure/clickhouse"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	usecaseLogging "github.com/datdt/k8sselfhost/internal/usecase/logging"
)

func wireCentralizedLogging(ctx context.Context, log *zap.Logger) (*adapthttp.LogHandler, func()) {
	chHost := os.Getenv("CLICKHOUSE_HOST")
	if chHost == "" {
		chHost = os.Getenv("K8S_CLICKHOUSE_HOST")
	}
	chDSN := os.Getenv("CLICKHOUSE_DSN")
	if chDSN == "" {
		chDSN = os.Getenv("K8S_CLICKHOUSE_DSN")
	}

	if chHost != "" || chDSN != "" {
		cfg := infraClickhouse.DefaultConfig()
		cfg.Host = chHost
		if p, err := strconv.Atoi(os.Getenv("CLICKHOUSE_PORT")); err == nil {
			cfg.Port = p
		}
		if db := os.Getenv("CLICKHOUSE_DB"); db != "" {
			cfg.Database = db
		}
		if u := os.Getenv("CLICKHOUSE_USER"); u != "" {
			cfg.Username = u
		}
		cfg.Password = os.Getenv("CLICKHOUSE_PASSWORD")
		cfg.DialTimeout = 3 * time.Second

		cCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		client, err := infraClickhouse.NewClient(cCtx, cfg)
		cancel()
		if err == nil {
			log.Info("ClickHouse centralized logging connected", zap.String("host", cfg.Host))
			if conn, cErr := client.Conn(ctx); cErr == nil {
				if mErr := infraClickhouse.RunMigrations(ctx, conn); mErr != nil {
					log.Warn("ClickHouse migration failed", zap.Error(mErr))
				}
			}
			bw := infraClickhouse.NewBatchWriter(client, infraClickhouse.BatchConfig{BatchSize: 5000, FlushInterval: 2 * time.Second, BufferCap: 50000})
			bw.Start(ctx)
			repo := infraClickhouse.NewLogRepository(client, bw)
			service := usecaseLogging.NewService(repo)
			return adapthttp.NewLogHandler(service, &chStatusProvider{client: client, repo: repo}), func() {
				_ = bw.Stop()
				_ = client.Close()
			}
		}
		log.Warn("ClickHouse connection failed, using in-memory ringbuffer fallback", zap.Error(err))
	} else {
		log.Info("ClickHouse not configured, using resilient in-memory ringbuffer fallback")
	}

	memRepo := logging.NewMemoryLogRepo(50000)
	return adapthttp.NewLogHandler(usecaseLogging.NewService(memRepo), memRepo), nil
}

type chStatusProvider struct {
	client *infraClickhouse.Client
	repo   *infraClickhouse.LogRepository
}

func (p *chStatusProvider) GetStatus(ctx context.Context) (*adapthttp.LogEngineStatus, error) {
	start := time.Now()
	err := p.client.Ping(ctx)
	latency := float64(time.Since(start).Microseconds()) / 1000.0
	status := "connected"
	if err != nil {
		status = "fallback"
	}
	var total int64
	if conn, cErr := p.client.Conn(ctx); cErr == nil {
		var cnt uint64
		if scanErr := conn.QueryRow(ctx, "SELECT count() FROM cluster_logs").Scan(&cnt); scanErr == nil {
			total = int64(cnt)
		}
	}
	return &adapthttp.LogEngineStatus{Engine: "ClickHouse MergeTree", Status: status, LatencyMS: latency, TotalRecords: total, RetentionDays: 30}, nil
}

func detectLogLevel(msg, defaultLvl string) string {
	u := strings.ToUpper(msg)
	if strings.Contains(u, "ERROR") || strings.Contains(u, "FATAL") || strings.Contains(u, "PANIC") || strings.Contains(u, "ERR") {
		return "ERROR"
	}
	if strings.Contains(u, "WARN") { return "WARN" }
	if strings.Contains(u, "DEBUG") || strings.Contains(u, "TRACE") { return "DEBUG" }
	if strings.Contains(u, "INFO") { return "INFO" }
	return defaultLvl
}

func getContainerNamespace(name string) string {
	l := strings.ToLower(name)
	if strings.Contains(l, "log") || strings.Contains(l, "nats") { return "logging" }
	if strings.Contains(l, "vault") { return "vault" }
	if strings.Contains(l, "stage") || strings.Contains(l, "staging") { return "staging" }
	return "production"
}

func extractTraceID(msg string) string {
	for _, p := range []string{`"trace_id":`, `"traceId":`, `trace_id=`, `traceId=`, `trace-id=`} {
		if idx := strings.Index(msg, p); idx != -1 {
			rest := strings.TrimSpace(msg[idx+len(p):])
			if len(rest) > 0 {
				if rest[0] == '"' || rest[0] == 0x27 {
					if end := strings.IndexByte(rest[1:], rest[0]); end != -1 { return strings.TrimSpace(rest[1 : end+1]) }
				} else if end := strings.IndexAny(rest, " \t\r\n,;{}]"); end != -1 { return strings.TrimSpace(rest[:end]) }
				return rest
			}
		}
	}
	return ""
}
