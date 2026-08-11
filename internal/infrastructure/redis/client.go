// Package redis provides Redis connection management using go-redis.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// Client wraps a go-redis client with health check and lifecycle management.
type Client struct {
	rdb *redis.Client
	cfg config.RedisConfig
}

// NewClient creates a new Redis client with retry logic.
func NewClient(ctx context.Context, cfg config.RedisConfig) (*Client, error) {
	log := logger.WithContext(ctx)

	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     20,
		MinIdleConns: 5,
	})

	var err error
	maxRetries := 5
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = rdb.Ping(ctx).Err()
		if err == nil {
			break
		}

		if attempt == maxRetries {
			return nil, fmt.Errorf("connecting to redis after %d attempts: %w", maxRetries, err)
		}

		backoff := time.Duration(attempt*attempt) * time.Second
		log.Warn("failed to connect to redis, retrying",
			zap.Int("attempt", attempt),
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)
		time.Sleep(backoff)
	}

	log.Info("connected to Redis",
		zap.String("addr", cfg.Addr()),
		zap.Int("db", cfg.DB),
	)

	return &Client{rdb: rdb, cfg: cfg}, nil
}

// Conn returns the underlying redis.Client for command execution.
func (c *Client) Conn() *redis.Client {
	return c.rdb
}

// Set stores a key-value pair with an expiration time.
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.rdb.Set(ctx, key, value, expiration).Err()
}

// Get retrieves the value for a given key.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

// Delete removes keys from the store.
func (c *Client) Delete(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

// Exists checks if a key exists.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// HealthCheck verifies the Redis connection is alive.
func (c *Client) HealthCheck(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	if c.rdb != nil {
		return c.rdb.Close()
	}
	return nil
}
