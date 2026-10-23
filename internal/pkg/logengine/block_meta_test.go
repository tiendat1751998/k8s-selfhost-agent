package logengine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockMeta_K8sColumnarEncodingAndFiltering(t *testing.T) {
	InitEngineRuntime()
	tmpDir := t.TempDir()
	dict := NewLabelDictionary()

	writer, err := NewWriter(tmpDir, dict, WithMaxRowsPerBlock(10))
	require.NoError(t, err)

	now := time.Now().UTC()

	// Entry 1: namespace=ingress, pod=nginx-1, container=controller, stream=stdout
	require.NoError(t, writer.Write(Entry{
		Timestamp:     now,
		Service:       "ingress-nginx",
		Level:         "info",
		Message:       "GET /api/v1/health 200 OK",
		Namespace:     "ingress-system",
		PodName:       "nginx-controller-abc",
		ContainerName: "controller",
		Stream:        "stdout",
	}))

	// Entry 2: namespace=ingress, pod=nginx-1, container=controller, stream=stderr
	require.NoError(t, writer.Write(Entry{
		Timestamp:     now.Add(10 * time.Millisecond),
		Service:       "ingress-nginx",
		Level:         "warn",
		Message:       "upstream response time 250ms exceeded threshold",
		Namespace:     "ingress-system",
		PodName:       "nginx-controller-abc",
		ContainerName: "controller",
		Stream:        "stderr",
	}))

	// Entry 3: namespace=backend, pod=api-1, container=app, stream=stdout
	require.NoError(t, writer.Write(Entry{
		Timestamp:     now.Add(20 * time.Millisecond),
		Service:       "user-api",
		Level:         "info",
		Message:       "user profile fetched successfully",
		Namespace:     "production",
		PodName:       "user-api-xyz",
		ContainerName: "app",
		Stream:        "stdout",
	}))

	require.NoError(t, writer.Flush())
	require.NoError(t, writer.Close())

	reader, err := NewReader(tmpDir, dict)
	require.NoError(t, err)
	defer reader.Close()

	// Filter by Namespace
	nsResults, err := reader.Search(context.Background(), QueryParams{
		Namespace: "ingress-system",
	})
	require.NoError(t, err)
	assert.Len(t, nsResults, 2)
	for _, r := range nsResults {
		assert.Equal(t, "ingress-system", r.Namespace)
	}

	// Filter by Stream
	stderrResults, err := reader.Search(context.Background(), QueryParams{
		Stream: "stderr",
	})
	require.NoError(t, err)
	require.Len(t, stderrResults, 1)
	assert.Equal(t, "stderr", stderrResults[0].Stream)
	assert.Contains(t, stderrResults[0].Message, "threshold")

	// Filter by Container
	appResults, err := reader.Search(context.Background(), QueryParams{
		ContainerName: "app",
	})
	require.NoError(t, err)
	require.Len(t, appResults, 1)
	assert.Equal(t, "production", appResults[0].Namespace)
	assert.Equal(t, "user-api-xyz", appResults[0].PodName)
	assert.Equal(t, "app", appResults[0].ContainerName)
}
