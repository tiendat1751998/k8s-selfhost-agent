package logengine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompactor_ConsolidatesSmallPartsWithoutZstdRecompression(t *testing.T) {
	InitEngineRuntime()
	tmpDir := t.TempDir()
	dict := NewLabelDictionary()

	writer, err := NewWriter(tmpDir, dict, WithMaxRowsPerBlock(2))
	require.NoError(t, err)

	now := time.Now().UTC()

	// Write 3 parts by sealing after each batch
	for part := 0; part < 3; part++ {
		for row := 0; row < 4; row++ {
			err = writer.Write(Entry{
				Timestamp: now.Add(time.Duration(part*10+row) * time.Second),
				Service:   fmt.Sprintf("svc-%d", part),
				Level:     "info",
				Message:   fmt.Sprintf("log message part %d row %d", part, row),
				Namespace: "kube-system",
				Stream:    "stdout",
			})
			require.NoError(t, err)
		}
		require.NoError(t, writer.Flush())
		writer.SealActivePart()
	}
	require.NoError(t, writer.Close())

	// Verify 3 parts exist before compaction
	partsDir := filepath.Join(tmpDir, "parts")
	entries, err := os.ReadDir(partsDir)
	require.NoError(t, err)

	partCount := 0
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "part_") {
			partCount++
		}
	}
	require.Equal(t, 3, partCount, "expected 3 small parts before compaction")

	// Run compactor
	compactor := NewCompactor(partsDir, nil, nil, CompactorOptions{
		MinPartsToMerge: 2,
		MaxPartsToMerge: 10,
	})
	mergedCount, err := compactor.CompactOnce()
	require.NoError(t, err)
	assert.Equal(t, 3, mergedCount, "compactor should merge all 3 small parts")

	// Verify only 1 part remains
	entriesAfter, err := os.ReadDir(partsDir)
	require.NoError(t, err)
	partCountAfter := 0
	for _, e := range entriesAfter {
		if e.IsDir() && strings.HasPrefix(e.Name(), "part_") {
			partCountAfter++
		}
	}
	assert.Equal(t, 1, partCountAfter, "only 1 consolidated part must remain after compaction")

	// Verify all 12 rows are searchable via Reader
	reader, err := NewReader(tmpDir, dict)
	require.NoError(t, err)
	defer reader.Close()

	results, err := reader.Search(context.Background(), QueryParams{
		Limit: 100,
	})
	require.NoError(t, err)
	assert.Len(t, results, 12, "all 12 rows across the 3 original parts must be searchable")

	// Verify service names filter works across rebased offsets
	for p := 0; p < 3; p++ {
		pResults, err := reader.Search(context.Background(), QueryParams{
			Service: fmt.Sprintf("svc-%d", p),
			Limit:   10,
		})
		require.NoError(t, err)
		assert.Len(t, pResults, 4, "each service should have 4 rows in compacted part")
	}

	// Verify dictionary exports ONLY services
	exported := dict.Export()
	for _, s := range exported {
		assert.True(t, strings.HasPrefix(s, "svc-"), "dict.Export must only contain service names, got: %s", s)
	}
}
