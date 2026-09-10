package logengine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_Ingest20k_Compression_Pruning_Memory(t *testing.T) {
	InitEngineRuntime()

	tmpDir, err := os.MkdirTemp("", "logengine-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dict := NewLabelDictionary()
	writer, err := NewWriter(tmpDir, dict, WithMaxRowsPerBlock(2048), WithFlushInterval(5*time.Second))
	require.NoError(t, err)

	services := []string{"api-gateway", "auth-service", "billing-svc", "k8s-agent"}
	levels := []string{"info", "warn", "error", "debug"}

	baseTime := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	const totalRows = 20000

	var uncompressedBytes int64
	targetSecretToken := "CRITICAL_PAYMENT_FAILURE_XYZ_999"
	targetRowIndex := 12345

	for i := 0; i < totalRows; i++ {
		ts := baseTime.Add(time.Duration(i) * 10 * time.Millisecond)
		svc := services[i%len(services)]
		lvl := levels[i%len(levels)]

		var msg string
		if i == targetRowIndex {
			msg = fmt.Sprintf("transaction failed unexpectedly with token: %s at attempt %d", targetSecretToken, i)
		} else {
			msg = fmt.Sprintf("processed request id=%d status=200 client=10.0.0.%d duration=%dms details=success_log_entry_payload",
				i, i%255, 10+(i%50))
		}

		rawLen := int64(len(ts.Format(time.RFC3339Nano)) + len(svc) + len(lvl) + len(msg) + 10)
		uncompressedBytes += rawLen

		err := writer.Write(Entry{
			Timestamp: ts,
			Service:   svc,
			Level:     lvl,
			Message:   msg,
		})
		require.NoError(t, err)
	}

	err = writer.Close()
	require.NoError(t, err)

	// 1. Verify on-disk compression > 10x
	var diskBytes int64
	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			diskBytes += info.Size()
		}
		return nil
	})
	require.NoError(t, err)

	compressionRatio := float64(uncompressedBytes) / float64(diskBytes)
	t.Logf("Total uncompressed: %d bytes (%.2f MB)", uncompressedBytes, float64(uncompressedBytes)/(1024*1024))
	t.Logf("Total on disk: %d bytes (%.2f KB)", diskBytes, float64(diskBytes)/1024)
	t.Logf("Compression ratio: %.2fx", compressionRatio)

	assert.Greater(t, compressionRatio, 10.0, "on-disk compression must exceed 10x")

	// 2. Query with Reader
	reader, err := NewReader(tmpDir, dict)
	require.NoError(t, err)
	defer reader.Close()

	ctx := context.Background()

	// Search with ExactMatch: true (prunes blocks via Bloom filter)
	resExact, err := reader.Search(ctx, QueryParams{
		Query:      targetSecretToken,
		ExactMatch: true,
		Limit:      10,
	})
	require.NoError(t, err)
	require.Len(t, resExact, 1, "exact token search should find target entry")
	assert.Contains(t, resExact[0].Message, targetSecretToken)

	// Search with substring query (guarantees zero false negatives)
	resSub, err := reader.Search(ctx, QueryParams{
		Query:      "PAYMENT_FAILURE",
		ExactMatch: false,
		Limit:      10,
	})
	require.NoError(t, err)
	require.Len(t, resSub, 1, "substring search should find entry without false negatives")
	assert.Contains(t, resSub[0].Message, targetSecretToken)

	// Time range query
	startTime := baseTime.Add(100 * time.Second)
	endTime := baseTime.Add(105 * time.Second)
	resTime, err := reader.Search(ctx, QueryParams{
		Since: startTime,
		Until: endTime,
		Limit: 100,
	})
	require.NoError(t, err)
	for _, e := range resTime {
		assert.True(t, !e.Timestamp.Before(startTime) && !e.Timestamp.After(endTime),
			"entry timestamp %v must be within [%v, %v]", e.Timestamp, startTime, endTime)
	}

	// Service query
	resSvc, err := reader.Search(ctx, QueryParams{
		Service: "billing-svc",
		Limit:   50,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resSvc)
	for _, e := range resSvc {
		assert.Equal(t, "billing-svc", e.Service)
	}

	// 3. Verify Memory RSS stays < 60MB
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	allocMB := float64(m.Alloc) / (1024 * 1024)
	sysMB := float64(m.Sys) / (1024 * 1024)
	t.Logf("Memory stats: Alloc = %.2f MB, Sys = %.2f MB", allocMB, sysMB)
	assert.Less(t, sysMB, 60.0, "Engine memory Sys must remain under 60MB")
}
