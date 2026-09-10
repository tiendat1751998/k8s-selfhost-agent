package clickhouse

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Config defines connection parameters for the ClickHouse client.
type Config struct {
	Host         string        `json:"host"`
	Port         int           `json:"port"`
	Database     string        `json:"database"`
	Username     string        `json:"username"`
	Password     string        `json:"password"`
	Secure       bool          `json:"secure"`
	MaxOpenConns int           `json:"max_open_conns"`
	MaxIdleConns int           `json:"max_idle_conns"`
	ConnLifetime time.Duration `json:"conn_lifetime"`
	DialTimeout  time.Duration `json:"dial_timeout"`
}

// DefaultConfig provides production-ready defaults with capped pool sizes.
func DefaultConfig() Config {
	return Config{
		Host:         "127.0.0.1",
		Port:         9000,
		Database:     "default",
		Username:     "default",
		Password:     "",
		Secure:       false,
		MaxOpenConns: 10,
		MaxIdleConns: 5,
		ConnLifetime: time.Hour,
		DialTimeout:  5 * time.Second,
	}
}

// BuildOptions converts Config into clickhouse.Options.
func BuildOptions(cfg Config) *clickhouse.Options {
	if cfg.Port == 0 {
		cfg.Port = 9000
	}
	if cfg.MaxOpenConns <= 0 {
		cfg.MaxOpenConns = 10
	}
	if cfg.MaxIdleConns <= 0 {
		cfg.MaxIdleConns = 5
	}
	if cfg.ConnLifetime <= 0 {
		cfg.ConnLifetime = time.Hour
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 5 * time.Second
	}
	if cfg.Database == "" {
		cfg.Database = "default"
	}
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}

	opts := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnLifetime,
		DialTimeout:     cfg.DialTimeout,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
	}
	if cfg.Secure {
		opts.TLS = &tls.Config{InsecureSkipVerify: false}
	}
	return opts
}

// Client wraps driver.Conn with connection pooling, health checks, and auto-reconnection.
type Client struct {
	cfg    Config
	mu     sync.RWMutex
	conn   driver.Conn
	closed bool
}

// NewClient creates and initializes a ClickHouse client connection pool.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	opts := BuildOptions(cfg)
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("opening clickhouse connection: %w", err)
	}

	if err := conn.Ping(ctx); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			return nil, fmt.Errorf("ping failed (%w) and cleanup failed: %v", err, closeErr)
		}
		return nil, fmt.Errorf("pinging clickhouse: %w", err)
	}

	return &Client{
		cfg:  cfg,
		conn: conn,
	}, nil
}

// NewClientWithConn wraps an existing driver.Conn for testing and dependency injection.
func NewClientWithConn(conn driver.Conn, cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		conn: conn,
	}
}

// Ping verifies that the ClickHouse connection is responsive.
func (c *Client) Ping(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return errors.New("clickhouse client is closed")
	}
	if c.conn == nil {
		return errors.New("clickhouse connection is nil")
	}
	return c.conn.Ping(ctx)
}

// Conn returns the active driver.Conn pool without per-call ping overhead.
func (c *Client) Conn(ctx context.Context) (driver.Conn, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, errors.New("clickhouse client is closed")
	}
	if c.conn == nil {
		return nil, errors.New("clickhouse connection is nil")
	}
	return c.conn, nil
}

// Reconnect establishes a fresh connection pool and replaces the current connection.
func (c *Client) Reconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return errors.New("clickhouse client is closed")
	}

	if c.conn != nil {
		if closeErr := c.conn.Close(); closeErr != nil {
			// Record error if close fails during reconnect
			return fmt.Errorf("closing previous clickhouse connection: %w", closeErr)
		}
		c.conn = nil
	}

	opts := BuildOptions(c.cfg)
	newConn, err := clickhouse.Open(opts)
	if err != nil {
		return fmt.Errorf("reopening clickhouse connection: %w", err)
	}

	if err := newConn.Ping(ctx); err != nil {
		if closeErr := newConn.Close(); closeErr != nil {
			return fmt.Errorf("ping failed after reconnect (%w) and cleanup failed: %v", err, closeErr)
		}
		return fmt.Errorf("ping failed after reconnect: %w", err)
	}

	c.conn = newConn
	return nil
}

// Close gracefully terminates the ClickHouse connection pool.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		if err != nil {
			return fmt.Errorf("closing clickhouse connection: %w", err)
		}
	}
	return nil
}