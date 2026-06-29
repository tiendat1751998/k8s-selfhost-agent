// Package postgres provides PostgreSQL connection management using pgxpool.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	"github.com/datdt/k8sselfhost/pkg/logger"
)

// Client wraps a pgxpool.Pool with health check and lifecycle management.
type Client struct {
	pool *pgxpool.Pool
	cfg  config.PostgresConfig
}

// NewClient creates a new PostgreSQL client with connection pooling.
// It retries connection attempts with exponential backoff.
func NewClient(ctx context.Context, cfg config.PostgresConfig) (*Client, error) {
	log := logger.WithContext(ctx)

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parsing postgres DSN: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.HealthCheckPeriod = 30 * time.Second
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.MaxConnIdleTime = 5 * time.Minute
	poolCfg.ConnConfig.ConnectTimeout = 5 * time.Second

	var pool *pgxpool.Pool
	maxRetries := 5
	for attempt := 1; attempt <= maxRetries; attempt++ {
		pool, err = pgxpool.NewWithConfig(ctx, poolCfg)
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				break
			} else {
				pool.Close()
				err = pingErr
			}
		}

		if attempt == maxRetries {
			return nil, fmt.Errorf("connecting to postgres after %d attempts: %w", maxRetries, err)
		}

		backoff := time.Duration(attempt*attempt) * time.Second
		log.Warn("failed to connect to postgres, retrying",
			zap.Int("attempt", attempt),
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)
		time.Sleep(backoff)
	}

	log.Info("connected to PostgreSQL",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.DBName),
	)

	return &Client{pool: pool, cfg: cfg}, nil
}

// Pool returns the underlying pgxpool.Pool for query execution.
func (c *Client) Pool() *pgxpool.Pool {
	return c.pool
}

// HealthCheck verifies the database connection is alive.
func (c *Client) HealthCheck(ctx context.Context) error {
	return c.pool.Ping(ctx)
}

// Close closes all connections in the pool.
func (c *Client) Close() {
	if c.pool != nil {
		c.pool.Close()
	}
}
