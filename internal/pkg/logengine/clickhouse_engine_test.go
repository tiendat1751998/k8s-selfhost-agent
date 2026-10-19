package logengine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClickHouse_LabelDictionary_OnlyExportsServices(t *testing.T) {
	tmpDir := t.TempDir()
	dict := NewLabelDictionary()
	w, err := NewWriter(tmpDir, dict, WithMaxRowsPerBlock(10))
	require.NoError(t, err)

	levels := []string{"info", "warn", "error", "debug"}
	for i := 0; i < 20; i++ {
		err := w.Write(Entry{
			Timestamp: time.Now().UTC(),
			Service:   "payment-service",
			Level:     levels[i%len(levels)],
			Message:   fmt.Sprintf("Transaction %d processed", i),
		})
		require.NoError(t, err)
	}
	require.NoError(t, w.Flush())
	require.NoError(t, w.Close())

	exported := dict.Export()
	assert.Contains(t, exported, "payment-service")
	assert.NotContains(t, exported, "info", "Export MUST NOT contain log level 'info'")
	assert.NotContains(t, exported, "warn", "Export MUST NOT contain log level 'warn'")
	assert.NotContains(t, exported, "error", "Export MUST NOT contain log level 'error'")
	assert.NotContains(t, exported, "debug", "Export MUST NOT contain log level 'debug'")
}

func TestClickHouse_MultiBlock_AppendsToSinglePart(t *testing.T) {
	tmpDir := t.TempDir()
	dict := NewLabelDictionary()
	// Each block flushes at 5 rows
	w, err := NewWriter(tmpDir, dict, WithMaxRowsPerBlock(5))
	require.NoError(t, err)

	// Write 15 rows (3 blocks)
	baseTime := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 15; i++ {
		err := w.Write(Entry{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Service:   "user-service",
			Level:     "info",
			Message:   fmt.Sprintf("User action %d", i),
		})
		require.NoError(t, err)
	}
	require.NoError(t, w.Flush())
	require.NoError(t, w.Close())

	// Verify parts directory: should have ONLY 1 part containing 3 blocks, avoiding inode bomb!
	partsDir := filepath.Join(tmpDir, "parts")
	entries, err := os.ReadDir(partsDir)
	require.NoError(t, err)

	partDirs := 0
	for _, e := range entries {
		if e.IsDir() && filepath.HasPrefix(e.Name(), "part_") {
			partDirs++
		}
	}
	assert.Equal(t, 1, partDirs, "expected exactly 1 multi-block part directory instead of 3 single-block dirs")

	// Verify reader reads all 15 rows
	reader, err := NewReader(tmpDir, dict)
	require.NoError(t, err)
	defer reader.Close()

	assert.Equal(t, 3, reader.BlockCount(), "reader should report 3 blocks in the single part")

	results, err := reader.Search(context.Background(), QueryParams{Limit: 50})
	require.NoError(t, err)
	assert.Len(t, results, 15)
}

func TestClickHouse_BlockMark_LevelAndServicePruning(t *testing.T) {
	tmpDir := t.TempDir()
	dict := NewLabelDictionary()
	w, err := NewWriter(tmpDir, dict, WithMaxRowsPerBlock(10))
	require.NoError(t, err)

	baseTime := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

	// Block 1: only info logs for service-A
	for i := 0; i < 10; i++ {
		_ = w.Write(Entry{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Service:   "service-A",
			Level:     "info",
			Message:   fmt.Sprintf("service-A info %d", i),
		})
	}
	require.NoError(t, w.Flush())

	// Block 2: only error logs for service-B
	for i := 10; i < 20; i++ {
		_ = w.Write(Entry{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Service:   "service-B",
			Level:     "error",
			Message:   fmt.Sprintf("service-B error %d", i),
		})
	}
	require.NoError(t, w.Flush())
	require.NoError(t, w.Close())

	reader, err := NewReader(tmpDir, dict)
	require.NoError(t, err)
	defer reader.Close()

	// Query for error level only: should return only service-B error logs
	errResults, err := reader.Search(context.Background(), QueryParams{
		Level: "error",
		Limit: 50,
	})
	require.NoError(t, err)
	assert.Len(t, errResults, 10)
	for _, e := range errResults {
		assert.Equal(t, "error", e.Level)
		assert.Equal(t, "service-B", e.Service)
	}

	// Query for service-A: should return only service-A info logs
	svcA_Results, err := reader.Search(context.Background(), QueryParams{
		Service: "service-A",
		Limit:   50,
	})
	require.NoError(t, err)
	assert.Len(t, svcA_Results, 10)
	for _, e := range svcA_Results {
		assert.Equal(t, "service-A", e.Service)
	}
}

func TestClickHouse_RetentionAndDiskQuotaPruning(t *testing.T) {
	tmpDir := t.TempDir()
	dict := NewLabelDictionary()
	// Set 500 bytes disk quota to force pruning of older parts
	w, err := NewWriter(tmpDir, dict,
		WithMaxRowsPerBlock(5),
		WithMaxDiskBytes(500),
	)
	require.NoError(t, err)

	// Force sealing multiple parts by reaching threshold or calling Close/re-opening
	for p := 0; p < 5; p++ {
		for i := 0; i < 5; i++ {
			_ = w.Write(Entry{
				Timestamp: time.Now().UTC().Add(time.Duration(p*10+i) * time.Second),
				Service:   "churn-service",
				Level:     "info",
				Message:   "Large message to consume disk space fast " + string(make([]byte, 100)),
			})
		}
		_ = w.Flush()
		w.SealActivePart()
	}
	require.NoError(t, w.Close())

	partsDir := filepath.Join(tmpDir, "parts")
	entries, err := os.ReadDir(partsDir)
	require.NoError(t, err)

	// Due to 500 byte limit, older parts should have been pruned
	assert.LessOrEqual(t, len(entries), 3, "older parts should be pruned by disk quota")
}

func TestClickHouse_PruneOldParts_HardCap128(t *testing.T) {
	tmpDir := t.TempDir()
	partsDir := filepath.Join(tmpDir, "parts")
	require.NoError(t, os.MkdirAll(partsDir, 0755))

	// Create 135 fake part directories
	for i := 1; i <= 135; i++ {
		pDir := filepath.Join(partsDir, fmt.Sprintf("part_%016d", i))
		require.NoError(t, os.MkdirAll(pDir, 0755))
		_ = os.WriteFile(filepath.Join(pDir, "primary.idx"), []byte{0}, 0644)
	}

	err := pruneOldParts(partsDir, "", 0, 0)
	require.NoError(t, err)

	entries, err := os.ReadDir(partsDir)
	require.NoError(t, err)
	assert.Equal(t, 128, len(entries), "should enforce hard cap of 128 sealed parts")

	// Oldest parts 1..7 must have been deleted
	for i := 1; i <= 7; i++ {
		oldPart := filepath.Join(partsDir, fmt.Sprintf("part_%016d", i))
		_, statErr := os.Stat(oldPart)
		assert.True(t, os.IsNotExist(statErr), "part %d should have been pruned", i)
	}

	// Part 135 must still exist
	newestPart := filepath.Join(partsDir, fmt.Sprintf("part_%016d", 135))
	_, statErr := os.Stat(newestPart)
	assert.False(t, os.IsNotExist(statErr), "newest part 135 should still exist")
}

func TestClickHouse_Reader_BlockCount_TTLAndHasBlocks(t *testing.T) {
	tmpDir := t.TempDir()
	dict := NewLabelDictionary()
	w, err := NewWriter(tmpDir, dict, WithMaxRowsPerBlock(5))
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		_ = w.Write(Entry{
			Timestamp: time.Now().UTC(),
			Service:   "test-svc",
			Level:     "info",
			Message:   fmt.Sprintf("msg %d", i),
		})
	}
	require.NoError(t, w.Flush())
	require.NoError(t, w.Close())

	r, err := NewReader(tmpDir, dict)
	require.NoError(t, err)
	defer r.Close()

	assert.True(t, r.HasBlocks(), "HasBlocks should return true")
	count1 := r.BlockCount()
	assert.Equal(t, 1, count1)

	// Second call should return cached result
	count2 := r.BlockCount()
	assert.Equal(t, count1, count2)
}

