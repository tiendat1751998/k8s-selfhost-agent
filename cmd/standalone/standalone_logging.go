package main

import (
	"context"
	"os"
	"strconv"
	"time"

	"go.uber.org/zap"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	infraAgent "github.com/datdt/k8sselfhost/internal/infrastructure/agent"
	infraClickhouse "github.com/datdt/k8sselfhost/internal/infrastructure/clickhouse"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	usecaseLogging "github.com/datdt/k8sselfhost/internal/usecase/logging"
)

func wireCentralizedLogging(ctx context.Context, log *zap.Logger, computeHostRepo domainDocker.ComputeHostRepository) (*adapthttp.LogHandler, func()) {
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
			batchWriter := infraClickhouse.NewBatchWriter(client, infraClickhouse.BatchConfig{BatchSize: 5000, FlushInterval: 2 * time.Second, BufferCap: 50000})
			batchWriter.Start(ctx)
			repo := infraClickhouse.NewLogRepository(client, batchWriter)
			service := usecaseLogging.NewService(repo)
			return adapthttp.NewLogHandler(service, &chStatusProvider{client: client, repo: repo}), func() {
				_ = batchWriter.Stop()
				_ = client.Close()
			}
		}
		log.Warn("ClickHouse connection failed, falling back to distributed edge log engine", zap.Error(err))
	} else {
		log.Info("ClickHouse not configured, activating distributed edge log engine")
	}

	memRepo := logging.NewMemoryLogRepo(50000)
	distRepo := infraAgent.NewDistributedAgentLogRepo(computeHostRepo, infraAgent.NewAgentLogClient(), memRepo)
	return adapthttp.NewLogHandler(usecaseLogging.NewService(distRepo), distRepo), nil
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
	return &adapthttp.LogEngineStatus{
		Engine:        "ClickHouse MergeTree",
		Status:        status,
		LatencyMS:     latency,
		TotalRecords:  total,
		RetentionDays: 30,
	}, nil
}
