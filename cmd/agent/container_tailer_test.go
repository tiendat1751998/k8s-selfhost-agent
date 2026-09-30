package main

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

type mockDockerClientForTailer struct {
	mu         sync.Mutex
	containers []types.Container
	logStreams map[string]string
}

func (m *mockDockerClientForTailer) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.containers, nil
}

func (m *mockDockerClientForTailer) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	content := m.logStreams[containerID]
	return io.NopCloser(strings.NewReader(content)), nil
}

func (m *mockDockerClientForTailer) Ping(ctx context.Context) (types.Ping, error) {
	return types.Ping{}, nil
}

func (m *mockDockerClientForTailer) Close() error {
	return nil
}

func TestContainerTailer_IngestsAndStopsCleanly(t *testing.T) {
	tempDir := t.TempDir()
	dict := logengine.NewLabelDictionary()
	writer, err := logengine.NewWriter(tempDir, dict)
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer writer.Close()

	mockCli := &mockDockerClientForTailer{
		containers: []types.Container{
			{
				ID:    "c123456789012",
				Names: []string{"/web-api"},
				Labels: map[string]string{
					"com.docker.compose.service": "web-api",
				},
			},
		},
		logStreams: map[string]string{
			"c123456789012": "2026-09-14T12:00:00Z [INFO] Server started\n2026-09-14T12:00:01Z [ERROR] DB connection failed\n",
		},
	}

	tailer := NewContainerTailer(mockCli, writer, slog.Default())
	tailer.pollInterval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		tailer.Start(ctx)
		close(done)
	}()

	// Wait for tailer to process stream
	time.Sleep(200 * time.Millisecond)

	// Flush writer to disk
	if err := writer.Flush(); err != nil {
		t.Fatalf("failed to flush writer: %v", err)
	}

	// Read back entries via Reader
	reader, err := logengine.NewReader(tempDir, dict)
	if err != nil {
		t.Fatalf("failed to create log reader: %v", err)
	}
	defer reader.Close()

	results, err := reader.Search(context.Background(), logengine.QueryParams{
		Service: "web-api",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 log entries ingested, got %d", len(results))
	}

	if results[0].Service != "web-api" {
		t.Errorf("expected service 'web-api', got '%s'", results[0].Service)
	}

	// Cancel context to test clean termination
	cancel()

	select {
	case <-done:
		// Clean termination
	case <-time.After(2 * time.Second):
		t.Fatalf("ContainerTailer did not terminate cleanly on context cancellation")
	}
}
