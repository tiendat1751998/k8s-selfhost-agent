// Package nats provides NATS JetStream connection management.
package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// Client wraps a NATS connection with JetStream support, health check, and lifecycle management.
type Client struct {
	conn   *nats.Conn
	js     jetstream.JetStream
	stream jetstream.Stream
	cfg    config.NATSConfig
}

// NewClient creates a new NATS client with JetStream enabled.
// It creates the configured stream if it does not already exist.
func NewClient(ctx context.Context, cfg config.NATSConfig) (*Client, error) {
	log := logger.WithContext(ctx)

	opts := []nats.Option{
		nats.MaxReconnects(cfg.MaxReconnects),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				log.Warn("NATS disconnected", zap.Error(err))
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Info("NATS reconnected", zap.String("url", nc.ConnectedUrl()))
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			log.Info("NATS connection closed")
		}),
	}

	var conn *nats.Conn
	var err error
	maxRetries := 5
	for attempt := 1; attempt <= maxRetries; attempt++ {
		conn, err = nats.Connect(cfg.URL, opts...)
		if err == nil {
			break
		}

		if attempt == maxRetries {
			return nil, fmt.Errorf("connecting to NATS after %d attempts: %w", maxRetries, err)
		}

		backoff := time.Duration(attempt*attempt) * time.Second
		log.Warn("failed to connect to NATS, retrying",
			zap.Int("attempt", attempt),
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)
		time.Sleep(backoff)
	}

	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("creating JetStream context: %w", err)
	}

	stream, err := ensureStream(ctx, js, cfg)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ensuring stream: %w", err)
	}

	log.Info("connected to NATS JetStream",
		zap.String("url", cfg.URL),
		zap.String("stream", cfg.StreamName),
	)

	return &Client{
		conn:   conn,
		js:     js,
		stream: stream,
		cfg:    cfg,
	}, nil
}

// JetStream returns the JetStream context for publish/subscribe operations.
func (c *Client) JetStream() jetstream.JetStream {
	return c.js
}

// Conn returns the underlying raw NATS connection.
func (c *Client) Conn() *nats.Conn {
	return c.conn
}

// Stream returns the configured JetStream stream.
func (c *Client) Stream() jetstream.Stream {
	return c.stream
}

// HealthCheck verifies the NATS connection is alive.
func (c *Client) HealthCheck(_ context.Context) error {
	if !c.conn.IsConnected() {
		return fmt.Errorf("NATS is not connected")
	}
	return nil
}

// Close drains and closes the NATS connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Drain()
	}
	return nil
}

func ensureStream(ctx context.Context, js jetstream.JetStream, cfg config.NATSConfig) (jetstream.Stream, error) {
	streamCfg := jetstream.StreamConfig{
		Name:        cfg.StreamName,
		Subjects:    cfg.StreamSubjects,
		Retention:   jetstream.WorkQueuePolicy,
		MaxAge:      72 * time.Hour,
		Storage:     jetstream.FileStorage,
		Replicas:    1,
		Description: "K8s Self-Healing incident events",
	}

	stream, err := js.CreateOrUpdateStream(ctx, streamCfg)
	if err != nil {
		return nil, fmt.Errorf("creating/updating stream %s: %w", cfg.StreamName, err)
	}

	return stream, nil
}
