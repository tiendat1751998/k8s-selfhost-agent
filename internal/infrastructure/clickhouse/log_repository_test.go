package clickhouse_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/infrastructure/clickhouse"
)

// Interface conformance compile-time check
var _ logging.LogRepository = (*clickhouse.LogRepository)(nil)
var _ logging.SurroundingContextQuerier = (*clickhouse.LogRepository)(nil)

func TestBuildLogQuery_SparseIndexPruning(t *testing.T) {
	now := time.Now().UTC()
	oneHourAgo := now.Add(-1 * time.Hour)

	filter := logging.LogFilter{
		TenantID:      "tenant-42",
		ClusterID:     "k8s-prod-1",
		Namespace:     "production",
		LogLevel:      logging.LogLevelError,
		StartTime:     oneHourAgo,
		EndTime:       now,
		SearchText:    "oom-killed",
		TraceID:       "trace-abc-123",
		PodName:       "api-worker-abc",
		ContainerName: "app",
		Stream:        "stderr",
		Limit:         50,
		Offset:        10,
		Attributes: map[string]string{
			"env":     "prod",
			"version": "v1.0.0",
		},
	}

	query, args := clickhouse.BuildLogQuery(filter, "cluster_logs")

	// Verify primary key sparse index order: tenant_id -> cluster_id -> namespace -> timestamp -> log_level
	tenantIdx := strings.Index(query, "tenant_id = ?")
	clusterIdx := strings.Index(query, "cluster_id = ?")
	namespaceIdx := strings.Index(query, "namespace = ?")
	timeIdx := strings.Index(query, "timestamp >= ?")
	levelIdx := strings.Index(query, "log_level = ?")

	require.True(t, tenantIdx != -1, "query must filter on tenant_id")
	require.True(t, clusterIdx != -1, "query must filter on cluster_id")
	require.True(t, namespaceIdx != -1, "query must filter on namespace")
	require.True(t, timeIdx != -1, "query must filter on timestamp range")
	require.True(t, levelIdx != -1, "query must filter on log_level")

	// Order must match sparse index hierarchy (timestamp before log_level)
	require.True(t, tenantIdx < clusterIdx, "tenant_id must precede cluster_id")
	require.True(t, clusterIdx < namespaceIdx, "cluster_id must precede namespace")
	require.True(t, namespaceIdx < timeIdx, "namespace must precede timestamp")
	require.True(t, timeIdx < levelIdx, "timestamp must precede log_level")

	// Ngram bloom filter substring search: positionCaseInsensitive
	require.Contains(t, query, "positionCaseInsensitive(message, ?) > 0")

	// Trace ID bloom filter
	require.Contains(t, query, "trace_id = ?")

	// Attribute filtering with sorted keys
	require.Contains(t, query, "attributes[?] = ?")

	// Pagination
	require.Contains(t, query, "LIMIT ? OFFSET ?")

	// Arguments order check
	require.Equal(t, "tenant-42", args[0])
	require.Equal(t, "k8s-prod-1", args[1])
	require.Equal(t, "production", args[2])
	require.Equal(t, oneHourAgo, args[3])
	require.Equal(t, now, args[4])
	require.Equal(t, "error", args[5])
}

func TestBuildLogQuery_CursorPagination(t *testing.T) {
	cursor := time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC)
	filter := logging.LogFilter{
		TenantID:        "tenant-cursor",
		CursorTimestamp: cursor,
		Limit:           100,
	}

	query, args := clickhouse.BuildLogQuery(filter, "cluster_logs")
	require.Contains(t, query, "timestamp < ?")

	foundCursor := false
	for _, a := range args {
		if ts, ok := a.(time.Time); ok && ts.Equal(cursor) {
			foundCursor = true
			break
		}
	}
	require.True(t, foundCursor, "cursor timestamp must be in query arguments")
}

func TestBuildSurroundingContextQueries(t *testing.T) {
	targetTime := time.Date(2026, 9, 16, 8, 30, 0, 0, time.UTC)
	service := "order-svc"
	window := 25

	// Test with explicit tenant ID
	beforeQuery, beforeArgs, afterQuery, afterArgs := clickhouse.BuildSurroundingContextQueries("tenant-alpha", service, targetTime, window)

	require.Contains(t, beforeQuery, "tenant_id = ?")
	require.Contains(t, beforeQuery, "<= ?")
	require.Contains(t, beforeQuery, "ORDER BY timestamp DESC LIMIT ?")
	require.Contains(t, afterQuery, "tenant_id = ?")
	require.Contains(t, afterQuery, "> ?")
	require.Contains(t, afterQuery, "ORDER BY timestamp ASC LIMIT ?")

	require.Equal(t, "tenant-alpha", beforeArgs[0], "tenant_id must be first argument in before query")
	require.Equal(t, "tenant-alpha", afterArgs[0], "tenant_id must be first argument in after query")
	require.Equal(t, 25, beforeArgs[len(beforeArgs)-1])
	require.Equal(t, 25, afterArgs[len(afterArgs)-1])

	// Test default tenant fallback when tenantID is empty
	bQuery, bArgs, aQuery, aArgs := clickhouse.BuildSurroundingContextQueries("", service, targetTime, window)
	require.Contains(t, bQuery, "tenant_id = ?")
	require.Contains(t, aQuery, "tenant_id = ?")
	require.Equal(t, "default-tenant", bArgs[0], "empty tenant must default to default-tenant in before query")
	require.Equal(t, "default-tenant", aArgs[0], "empty tenant must default to default-tenant in after query")
}

func TestBuildHistogramQuery(t *testing.T) {
	filter := logging.LogFilter{
		TenantID:  "tenant-42",
		ClusterID: "k8s-prod-1",
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
	}

	query, args := clickhouse.BuildHistogramQuery(filter, "cluster_logs", 60)
	require.Contains(t, query, "toStartOfInterval(timestamp, toIntervalSecond(?)) AS bucket")
	require.Contains(t, query, "GROUP BY bucket, log_level")
	require.Contains(t, query, "ORDER BY bucket ASC")
	require.Equal(t, 60, args[0])
}

func TestLogRepository_IngestBatch_Empty(t *testing.T) {
	repo := clickhouse.NewLogRepository(nil, nil)
	ctx := context.Background()

	// Empty entries is a no-op
	err := repo.IngestBatch(ctx, nil)
	require.NoError(t, err)
}
