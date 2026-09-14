package clickhouse_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/infrastructure/clickhouse"
)

func TestDefaultConfig(t *testing.T) {
	cfg := clickhouse.DefaultConfig()
	require.Equal(t, "127.0.0.1", cfg.Host)
	require.Equal(t, 9000, cfg.Port)
	require.Equal(t, "default", cfg.Database)
	require.Equal(t, 10, cfg.MaxOpenConns)
	require.Equal(t, 5, cfg.MaxIdleConns)
	require.Equal(t, time.Hour, cfg.ConnLifetime)
	require.Equal(t, 5*time.Second, cfg.DialTimeout)
}

func TestBuildOptions(t *testing.T) {
	cfg := clickhouse.Config{
		Host:     "ch-server",
		Port:     9000,
		Database: "analytics",
		Username: "admin",
		Password: "secret",
		Secure:   true,
	}
	opts := clickhouse.BuildOptions(cfg)
	require.Equal(t, []string{"ch-server:9000"}, opts.Addr)
	require.Equal(t, "analytics", opts.Auth.Database)
	require.Equal(t, "admin", opts.Auth.Username)
	require.Equal(t, "secret", opts.Auth.Password)
	require.Equal(t, 10, opts.MaxOpenConns)
	require.Equal(t, 5, opts.MaxIdleConns)
	require.NotNil(t, opts.TLS)
}

func TestClient_ClosedState(t *testing.T) {
	client := clickhouse.NewClientWithConn(nil, clickhouse.DefaultConfig())
	ctx := context.Background()

	// Conn with nil connection returns error
	_, err := client.Conn(ctx)
	require.Error(t, err)

	// Close client
	err = client.Close()
	require.NoError(t, err)

	// Second close is idempotent
	err = client.Close()
	require.NoError(t, err)

	// Ping returns error when closed
	err = client.Ping(ctx)
	require.Error(t, err)

	// Conn returns error when closed
	_, err = client.Conn(ctx)
	require.Error(t, err)

	// Reconnect returns error when closed
	err = client.Reconnect(ctx)
	require.Error(t, err)
}