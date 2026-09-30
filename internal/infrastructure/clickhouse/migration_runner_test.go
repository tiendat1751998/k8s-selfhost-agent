package clickhouse_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/infrastructure/clickhouse"
)

type mockDriverConn struct {
	driver.Conn
	mu       sync.Mutex
	executed []string
}

func (m *mockDriverConn) Exec(ctx context.Context, query string, args ...any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executed = append(m.executed, query)
	return nil
}

func (m *mockDriverConn) Close() error {
	return nil
}

func TestRunMigrations_ExecutesAllIdempotentDDL(t *testing.T) {
	conn := &mockDriverConn{}
	ctx := context.Background()

	err := clickhouse.RunMigrations(ctx, conn)
	require.NoError(t, err)

	require.NotEmpty(t, conn.executed, "RunMigrations should have executed statements")

	joined := strings.Join(conn.executed, "\n")
	require.Contains(t, joined, "trace_id LowCardinality(String)")
	require.Contains(t, joined, "span_id String")
	require.Contains(t, joined, "error_fingerprint LowCardinality(String)")
	require.Contains(t, joined, "idx_ngram")
	require.Contains(t, joined, "idx_trace")
}

func TestRunMigrations_NilConnReturnsError(t *testing.T) {
	err := clickhouse.RunMigrations(context.Background(), nil)
	require.Error(t, err)
}
