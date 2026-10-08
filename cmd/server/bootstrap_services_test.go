package main

import (
	"context"
	"io"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
)

type serverBlockingReadCloser struct {
	pr     *io.PipeReader
	pw     *io.PipeWriter
	closed chan struct{}
	once   sync.Once
}

func newServerBlockingReadCloser() *serverBlockingReadCloser {
	pr, pw := io.Pipe()
	return &serverBlockingReadCloser{
		pr:     pr,
		pw:     pw,
		closed: make(chan struct{}),
	}
}

func (b *serverBlockingReadCloser) Read(p []byte) (int, error) {
	return b.pr.Read(p)
}

func (b *serverBlockingReadCloser) Close() error {
	b.once.Do(func() {
		close(b.closed)
	})
	return b.pr.Close()
}

type fakeServerDockerClient struct {
	mu         sync.Mutex
	containers []container.Summary
	logsFn     func(ctx context.Context, c string, options container.LogsOptions) (io.ReadCloser, error)
}

func (f *fakeServerDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.containers, nil
}

func (f *fakeServerDockerClient) ContainerLogs(ctx context.Context, c string, options container.LogsOptions) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.logsFn != nil {
		return f.logsFn(ctx, c, options)
	}
	return newServerBlockingReadCloser(), nil
}

func TestServerDockerLogStreamer_GoroutineExitOnCancel(t *testing.T) {
	agg := logging.NewLogAggregator(100)
	logger := zap.NewNop()

	// Capture baseline goroutines after aggregator is initialized
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseGoroutines := runtime.NumGoroutine()

	blockingBody := newServerBlockingReadCloser()
	fakeClient := &fakeServerDockerClient{
		containers: []container.Summary{
			{ID: "server-c-123456", Names: []string{"/server-test-container"}},
		},
		logsFn: func(ctx context.Context, c string, options container.LogsOptions) (io.ReadCloser, error) {
			return blockingBody, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	startDockerLogStreamer(ctx, fakeClient, agg, nil, logger)

	// Verify streamer goroutines spawned
	require.Eventually(t, func() bool {
		return runtime.NumGoroutine() > baseGoroutines
	}, 1*time.Second, 10*time.Millisecond, "expected server streamer goroutines to spawn")

	// Cancel context - must close reader and terminate all streamer goroutines
	cancel()

	select {
	case <-blockingBody.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("blocking body reader was not closed on ctx cancellation")
	}

	// Verify goroutines return to baseline
	deadline := time.Now().Add(2 * time.Second)
	var finalCount int
	for time.Now().Before(deadline) {
		runtime.GC()
		finalCount = runtime.NumGoroutine()
		if finalCount <= baseGoroutines {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.LessOrEqual(t, finalCount, baseGoroutines, "expected goroutine count to return to baseline (%d), got %d", baseGoroutines, finalCount)
}
