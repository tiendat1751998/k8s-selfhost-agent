package logengine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkipIndex_DualBloom_TokenAndNGram4Pruning(t *testing.T) {
	InitEngineRuntime()
	tmpDir := t.TempDir()
	dict := NewLabelDictionary()

	// 1 row per block so every entry gets its own block and bloom filter
	writer, err := NewWriter(tmpDir, dict, WithMaxRowsPerBlock(1))
	require.NoError(t, err)

	now := time.Now().UTC()

	// Block 0: auth failure
	require.NoError(t, writer.Write(Entry{
		Timestamp: now,
		Service:   "auth-api",
		Level:     "warn",
		Message:   "unauthorized login attempt from 192.168.1.5",
	}))
	require.NoError(t, writer.Flush())

	// Block 1: checkout transaction
	require.NoError(t, writer.Write(Entry{
		Timestamp: now.Add(1 * time.Second),
		Service:   "checkout-svc",
		Level:     "info",
		Message:   "order processed with payment_token_secure_99",
	}))
	require.NoError(t, writer.Flush())

	// Block 2: redis cache
	require.NoError(t, writer.Write(Entry{
		Timestamp: now.Add(2 * time.Second),
		Service:   "cache-svc",
		Level:     "debug",
		Message:   "redis cluster eviction completed",
	}))
	require.NoError(t, writer.Flush())
	require.NoError(t, writer.Close())

	reader, err := NewReader(tmpDir, dict)
	require.NoError(t, err)
	defer reader.Close()

	// 1. Substring search with 4-gram: "payment"
	subResults, err := reader.Search(context.Background(), QueryParams{
		Query:      "payment",
		ExactMatch: false,
	})
	require.NoError(t, err)
	require.Len(t, subResults, 1)
	assert.Equal(t, "checkout-svc", subResults[0].Service)
	assert.Contains(t, subResults[0].Message, "payment_token_secure_99")

	// 2. Substring search with 4-gram inside a token: "evict" (inside "eviction")
	evictResults, err := reader.Search(context.Background(), QueryParams{
		Query:      "evict",
		ExactMatch: false,
	})
	require.NoError(t, err)
	require.Len(t, evictResults, 1)
	assert.Equal(t, "cache-svc", evictResults[0].Service)

	// 3. Substring search with 4-gram that does NOT exist anywhere
	missingResults, err := reader.Search(context.Background(), QueryParams{
		Query:      "foobarqux",
		ExactMatch: false,
	})
	require.NoError(t, err)
	assert.Empty(t, missingResults)

	// 4. ExactMatch search with tokenbf_v1
	exactResults, err := reader.Search(context.Background(), QueryParams{
		Query:      "unauthorized",
		ExactMatch: true,
	})
	require.NoError(t, err)
	require.Len(t, exactResults, 1)
	assert.Equal(t, "auth-api", exactResults[0].Service)
}
