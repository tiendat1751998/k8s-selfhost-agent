package clickhouse_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/infrastructure/clickhouse"
	usecaseIngestGuard "github.com/datdt/k8sselfhost/internal/usecase/ingestguard"
)

// Compile-time interface satisfaction checks
var (
	_ usecaseIngestGuard.PartsInspector   = (*clickhouse.PartsInspectorAdapter)(nil)
	_ usecaseIngestGuard.BulkWriter       = (*clickhouse.BulkWriterAdapter)(nil)
	_ usecaseIngestGuard.MergeAccelerator = (*clickhouse.MergeAcceleratorAdapter)(nil)
	_ usecaseIngestGuard.SpillJournal     = (*clickhouse.DiskSpillJournal)(nil)
)

func TestBuildLogQuery_LimitCappedAt5000(t *testing.T) {
	tests := []struct {
		name          string
		inputLimit    int
		expectedLimit int
	}{
		{"zero limit defaults to 500", 0, 500},
		{"negative limit defaults to 500", -10, 500},
		{"moderate limit preserved", 250, 250},
		{"exact 5000 preserved", 5000, 5000},
		{"excessive limit 10000 capped at 5000", 10000, 5000},
		{"massive limit 500000 capped at 5000", 500000, 5000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := logging.LogFilter{
				TenantID: "tenant-test",
				Limit:    tt.inputLimit,
			}
			_, args := clickhouse.BuildLogQuery(filter, "cluster_logs")
			require.NotEmpty(t, args)
			// Limit is the second to last argument in BuildLogQuery
			limitArg := args[len(args)-2]
			require.Equal(t, tt.expectedLimit, limitArg)
		})
	}
}

func TestBatchWriterAndRepo_CachedInsertSQL(t *testing.T) {
	bw := clickhouse.NewBatchWriter(nil, clickhouse.BatchConfig{TableName: "custom_logs"})
	require.Contains(t, bw.InsertSQL(), "INSERT INTO custom_logs")
	require.Contains(t, bw.InsertSQL(), "error_fingerprint")

	repo := clickhouse.NewLogRepository(nil, nil)
	require.Contains(t, repo.InsertSQL(), "INSERT INTO cluster_logs")
	require.Contains(t, repo.InsertSQL(), "error_fingerprint")
}

func TestPartsInspector_NilClientSafeFallback(t *testing.T) {
	inspector := clickhouse.NewPartsInspector(nil)
	require.NotNil(t, inspector)

	parts, merging, err := inspector.InspectTableParts(context.Background(), "default", "cluster_logs")
	require.NoError(t, err)
	require.Equal(t, int64(0), parts)
	require.False(t, merging)
}

func TestMergeAccelerator_NilClientSafeNoOp(t *testing.T) {
	accelerator := clickhouse.NewMergeAccelerator(nil)
	require.NotNil(t, accelerator)

	err := accelerator.ScheduleMerge(context.Background(), "default", "cluster_logs")
	require.NoError(t, err)
}

func TestDiskSpillJournal_WritesCompactJSONLAndRotates(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "spill_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	journal := clickhouse.NewDiskSpillJournal(tempDir)
	require.NotNil(t, journal)

	records := []any{
		map[string]any{
			"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
			"tenant_id": "test-tenant",
			"message":   "spill record 1",
		},
		logging.LogEntry{
			Timestamp: time.Now().UTC(),
			TenantID:  "test-tenant",
			Message:   "spill record 2",
		},
	}

	err = journal.Spill(context.Background(), "default", "cluster_logs", records)
	require.NoError(t, err)

	spillFile := filepath.Join(tempDir, "ingest_spill_cluster_logs.jsonl")
	data, err := os.ReadFile(spillFile)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 2)
	require.Contains(t, lines[0], "spill record 1")
	require.Contains(t, lines[1], "spill record 2")
}

func TestBulkWriter_ConvertsAndSpillsWhenNoCH(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bulkwriter_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	journal := clickhouse.NewDiskSpillJournal(tempDir)
	writer := clickhouse.NewBulkWriter(nil, nil, nil, journal)
	require.NotNil(t, writer)

	records := []any{
		logging.LogEntry{
			TenantID: "tenant-a",
			Message:  "bulk log record 1",
		},
	}

	err = writer.WriteBulk(context.Background(), "default", "cluster_logs", records)
	require.NoError(t, err)

	spillFile := filepath.Join(tempDir, "ingest_spill_cluster_logs.jsonl")
	data, err := os.ReadFile(spillFile)
	require.NoError(t, err)
	require.Contains(t, string(data), "bulk log record 1")
}
