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

	// Token bloom filter index: hasToken
	require.Contains(t, query, "hasToken(message, ?)")

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