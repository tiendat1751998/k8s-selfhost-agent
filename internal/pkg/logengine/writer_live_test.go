package logengine

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriter_LiveSnapshot_ZeroLatencyVisibility(t *testing.T) {
	InitEngineRuntime()
	tmpDir := t.TempDir()
	dict := NewLabelDictionary()

	// High flush interval and max rows to ensure logs stay in memory buffer
	writer, err := NewWriter(tmpDir, dict,
		WithMaxRowsPerBlock(1000),
		WithFlushInterval(1*time.Hour),
	)
	require.NoError(t, err)
	defer writer.Close()

	reader, err := NewReader(tmpDir, dict, WithLiveWriter(writer))
	require.NoError(t, err)
	defer reader.Close()

	now := time.Now().UTC()
	err = writer.Write(Entry{
		Timestamp:     now,
		Service:       "auth-service",
		Level:         "warn",
		Message:       "jwt token expired for user 42",
		Namespace:     "prod",
		PodName:       "auth-pod-1",
		ContainerName: "auth-container",
		Stream:        "stderr",
	})
	require.NoError(t, err)

	// Ingest is asynchronous via RingBuffer; wait briefly for drain to builder
	var results []Entry
	for i := 0; i < 50; i++ {
		results, err = reader.Search(context.Background(), QueryParams{
			Service: "auth-service",
		})
		require.NoError(t, err)
		if len(results) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	require.Len(t, results, 1, "uncommitted in-memory log should be immediately visible via LiveWriter")
	assert.Equal(t, "jwt token expired for user 42", results[0].Message)
	assert.Equal(t, "auth-service", results[0].Service)
	assert.Equal(t, "warn", results[0].Level)
	assert.Equal(t, "prod", results[0].Namespace)
	assert.Equal(t, "auth-pod-1", results[0].PodName)
	assert.Equal(t, "auth-container", results[0].ContainerName)
	assert.Equal(t, "stderr", results[0].Stream)
}

func TestWriter_LiveSnapshot_DeduplicationAcrossFlush(t *testing.T) {
	InitEngineRuntime()
	tmpDir := t.TempDir()
	dict := NewLabelDictionary()

	writer, err := NewWriter(tmpDir, dict,
		WithMaxRowsPerBlock(1000),
		WithFlushInterval(1*time.Hour),
	)
	require.NoError(t, err)
	defer writer.Close()

	reader, err := NewReader(tmpDir, dict, WithLiveWriter(writer))
	require.NoError(t, err)
	defer reader.Close()

	now := time.Now().UTC()
	err = writer.Write(Entry{
		Timestamp: now,
		Service:   "payment-service",
		Level:     "info",
		Message:   "charge successful",
		Namespace: "default",
		PodName:   "pay-1",
		Stream:    "stdout",
	})
	require.NoError(t, err)

	// Flush to disk
	require.NoError(t, writer.Flush())

	// Search should return exactly 1 entry (not duplicated)
	results, err := reader.Search(context.Background(), QueryParams{
		Service: "payment-service",
	})
	require.NoError(t, err)
	assert.Len(t, results, 1, "entry flushed to disk must not be duplicated by LiveWriter snapshot")
	assert.Equal(t, "charge successful", results[0].Message)
}

func TestWriter_LiveSnapshot_ConcurrentRace(t *testing.T) {
	InitEngineRuntime()
	tmpDir := t.TempDir()
	dict := NewLabelDictionary()

	writer, err := NewWriter(tmpDir, dict,
		WithMaxRowsPerBlock(500),
		WithFlushInterval(10*time.Millisecond),
	)
	require.NoError(t, err)
	defer writer.Close()

	const numProducers = 10
	const msgsPerProducer = 200
	const numReaders = 5

	var wg sync.WaitGroup
	stopReaders := make(chan struct{})

	// Launch concurrent readers repeatedly taking uncommitted snapshots
	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopReaders:
					return
				default:
					snaps := writer.SnapshotUncommitted()
					// Access snapshot fields to verify no memory corruption
					for _, s := range snaps {
						_ = s.Timestamp
						_ = len(s.Message)
						_ = s.Namespace
					}
					time.Sleep(1 * time.Millisecond)
				}
			}
		}()
	}

	// Launch concurrent producers writing entries
	var prodWg sync.WaitGroup
	for p := 0; p < numProducers; p++ {
		prodWg.Add(1)
		go func(pid int) {
			defer prodWg.Done()
			for i := 0; i < msgsPerProducer; i++ {
				_ = writer.Write(Entry{
					Timestamp:     time.Now().UTC(),
					Service:       "test-service",
					Level:         "info",
					Message:       fmt.Sprintf("producer-%d-msg-%d", pid, i),
					Namespace:     "prod",
					PodName:       fmt.Sprintf("pod-%d", pid),
					ContainerName: "main",
					Stream:        "stdout",
				})
			}
		}(p)
	}

	prodWg.Wait()
	close(stopReaders)
	wg.Wait()

	require.NoError(t, writer.Flush())
}
