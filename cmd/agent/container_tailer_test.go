package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
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
	mu              sync.Mutex
	containers      []types.Container
	logStreams      map[string]string
	callCounts      map[string]int
	lastLogsOptions map[string]container.LogsOptions
	streamGenerators map[string]func(call int, opts container.LogsOptions) string
	binaryLogStreams map[string][]byte
}

func (m *mockDockerClientForTailer) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.containers, nil
}

func (m *mockDockerClientForTailer) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.callCounts == nil {
		m.callCounts = make(map[string]int)
	}
	if m.lastLogsOptions == nil {
		m.lastLogsOptions = make(map[string]container.LogsOptions)
	}
	m.callCounts[containerID]++
	m.lastLogsOptions[containerID] = options

	if m.streamGenerators != nil && m.streamGenerators[containerID] != nil {
		content := m.streamGenerators[containerID](m.callCounts[containerID], options)
		return io.NopCloser(strings.NewReader(content)), nil
	}
	if m.binaryLogStreams != nil && m.binaryLogStreams[containerID] != nil {
		return io.NopCloser(bytes.NewReader(m.binaryLogStreams[containerID])), nil
	}

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

func TestContainerTailer_CatchesUpInitialLogsOnDiscovery(t *testing.T) {
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
				ID:     "c999888777666",
				Names:  []string{"/initial-svc"},
				Labels: map[string]string{"com.docker.compose.service": "initial-svc"},
			},
		},
		logStreams: map[string]string{
			"c999888777666": "2026-09-14T12:00:00Z [INFO] init log\n",
		},
	}

	tailer := NewContainerTailer(mockCli, writer, slog.Default())
	tailer.pollInterval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go tailer.Start(ctx)
	time.Sleep(150 * time.Millisecond)
	tailer.Stop()

	mockCli.mu.Lock()
	opts := mockCli.lastLogsOptions["c999888777666"]
	mockCli.mu.Unlock()

	// Initial discovery must catch up logs on container creation (Tail: "all" or empty, NOT "100")
	if opts.Tail != "all" && opts.Tail != "" {
		t.Fatalf("expected Tail option to be 'all' or empty to catch up on container creation, got %q", opts.Tail)
	}
}

func TestContainerTailer_RecoversOnEOFWhenContainerRotates(t *testing.T) {
	tempDir := t.TempDir()
	dict := logengine.NewLabelDictionary()
	writer, err := logengine.NewWriter(tempDir, dict)
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer writer.Close()

	const containerID = "c_rotate_12345"
	mockCli := &mockDockerClientForTailer{
		containers: []types.Container{
			{
				ID:     containerID,
				Names:  []string{"/rotate-svc"},
				Labels: map[string]string{"com.docker.compose.service": "rotate-svc"},
			},
		},
		streamGenerators: map[string]func(call int, opts container.LogsOptions) string{
			containerID: func(call int, opts container.LogsOptions) string {
				if call == 1 {
					// First stream: reaches EOF (e.g. rotation event)
					return "2026-09-14T12:00:00Z [INFO] line before rotation\n"
				}
				// Subsequent stream after EOF/rotation: resumes with new logs
				return "2026-09-14T12:00:01Z [INFO] line after rotation\n"
			},
		},
	}

	tailer := NewContainerTailer(mockCli, writer, slog.Default())
	tailer.pollInterval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go tailer.Start(ctx)

	// Wait enough for first stream to hit EOF and reconnect/recover
	time.Sleep(400 * time.Millisecond)
	tailer.Stop()

	mockCli.mu.Lock()
	callCount := mockCli.callCounts[containerID]
	mockCli.mu.Unlock()

	if callCount < 2 {
		t.Fatalf("expected tailer to reconnect on EOF after log rotation (callCount >= 2), got %d calls", callCount)
	}

	// Verify both logs were ingested
	_ = writer.Flush()
	reader, err := logengine.NewReader(tempDir, dict)
	if err != nil {
		t.Fatalf("failed to create log reader: %v", err)
	}
	defer reader.Close()

	results, err := reader.Search(context.Background(), logengine.QueryParams{
		Service: "rotate-svc",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 entries ingested across rotations, got %d", len(results))
	}
}

func createStdCopyFrame(stream byte, payload string) []byte {
	b := []byte(payload)
	hdr := make([]byte, 8+len(b))
	hdr[0] = stream
	binary.BigEndian.PutUint32(hdr[4:8], uint32(len(b)))
	copy(hdr[8:], b)
	return hdr
}

func TestContainerTailer_MultiplexedStdCopy_ConcurrentNoDataRace(t *testing.T) {
	tempDir := t.TempDir()
	dict := logengine.NewLabelDictionary()
	writer, err := logengine.NewWriter(tempDir, dict)
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer writer.Close()

	const containerID = "c_stdcopy_race"
	var buf bytes.Buffer
	for i := 0; i < 50; i++ {
		stdoutLine := fmt.Sprintf("2026-09-14T12:00:%02d.000Z [INFO] stdout line %d\n", i%60, i)
		stderrLine := fmt.Sprintf("2026-09-14T12:00:%02d.000Z [WARN] stderr line %d\n", i%60, i)
		buf.Write(createStdCopyFrame(1, stdoutLine))
		buf.Write(createStdCopyFrame(2, stderrLine))
	}

	mockCli := &mockDockerClientForTailer{
		containers: []types.Container{
			{
				ID:     containerID,
				Names:  []string{"/race-svc"},
				Labels: map[string]string{"com.docker.compose.service": "race-svc"},
			},
		},
		binaryLogStreams: map[string][]byte{
			containerID: buf.Bytes(),
		},
	}

	tailer := NewContainerTailer(mockCli, writer, slog.Default())
	tailer.pollInterval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go tailer.Start(ctx)

	time.Sleep(200 * time.Millisecond)
	tailer.Stop()
}

