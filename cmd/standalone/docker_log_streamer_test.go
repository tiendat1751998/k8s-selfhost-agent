package main

import (
	"context"
	"errors"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
)

type standaloneBlockingReadCloser struct {
	pr     *io.PipeReader
	pw     *io.PipeWriter
	closed chan struct{}
	once   sync.Once
}

func newStandaloneBlockingReadCloser() *standaloneBlockingReadCloser {
	pr, pw := io.Pipe()
	return &standaloneBlockingReadCloser{
		pr:     pr,
		pw:     pw,
		closed: make(chan struct{}),
	}
}

func (b *standaloneBlockingReadCloser) Read(p []byte) (int, error) {
	return b.pr.Read(p)
}

func (b *standaloneBlockingReadCloser) Close() error {
	b.once.Do(func() {
		close(b.closed)
	})
	return b.pr.Close()
}

type fakeStandaloneDockerClient struct {
	mu          sync.Mutex
	containers  []container.Summary
	eventsCalls int
	eventsFn    func(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error)
	logsFn      func(ctx context.Context, c string, options container.LogsOptions) (io.ReadCloser, error)
}

func (f *fakeStandaloneDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.containers, nil
}

func (f *fakeStandaloneDockerClient) ContainerLogs(ctx context.Context, c string, options container.LogsOptions) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.logsFn != nil {
		return f.logsFn(ctx, c, options)
	}
	return newStandaloneBlockingReadCloser(), nil
}

func (f *fakeStandaloneDockerClient) Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.eventsCalls++
	if f.eventsFn != nil {
		return f.eventsFn(ctx, options)
	}
	msgCh := make(chan events.Message)
	errCh := make(chan error)
	return msgCh, errCh
}

func TestExtractTraceID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "logfmt with trace_id=",
			input:    "2026-09-16 08:00:00 [ERROR] trace_id=c0ffee1234 service error",
			expected: "c0ffee1234",
		},
		{
			name:     "logfmt with traceId quotes",
			input:    `level=info traceId="9876-abcd" msg="hello"`,
			expected: "9876-abcd",
		},
		{
			name:     "json trace_id",
			input:    `{"timestamp":"2026-09-16T08:00:00Z","trace_id":"xyz-123","message":"user login"}`,
			expected: "xyz-123",
		},
		{
			name:     "json traceId",
			input:    `{"traceId":"abc-789","level":"warn"}`,
			expected: "abc-789",
		},
		{
			name:     "hyphenated trace-id=",
			input:    "trace-id=feedbeef99 container started",
			expected: "feedbeef99",
		},
		{
			name:     "colon formatted trace_id",
			input:    `trace_id: "token-456", detail: "ok"`,
			expected: "token-456",
		},
		{
			name:     "no trace present",
			input:    "standard nginx access log without tracing 200 OK",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := extractTraceID(tc.input)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestDockerLogStreamer_GoroutineExitOnCancel(t *testing.T) {
	agg := logging.NewLogAggregator(100)
	logger := zap.NewNop()

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseGoroutines := runtime.NumGoroutine()

	blockingBody := newStandaloneBlockingReadCloser()
	fakeClient := &fakeStandaloneDockerClient{
		containers: []container.Summary{
			{ID: "c-standalone-1234", Names: []string{"/standalone-test-container"}},
		},
		logsFn: func(ctx context.Context, c string, options container.LogsOptions) (io.ReadCloser, error) {
			return blockingBody, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	startDockerLogStreamer(ctx, fakeClient, agg, nil, logger)

	require.Eventually(t, func() bool {
		return runtime.NumGoroutine() > baseGoroutines
	}, 1*time.Second, 10*time.Millisecond, "expected streamer goroutines to spawn")

	cancel()

	select {
	case <-blockingBody.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("blocking body reader was not closed on ctx cancellation")
	}

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

func TestDockerLogStreamer_EventWatcherRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agg := logging.NewLogAggregator(100)
	logger := zap.NewNop()

	var eventsCalls int
	var mu sync.Mutex
	reSubscribed := make(chan struct{}, 1)

	fakeClient := &fakeStandaloneDockerClient{
		containers: []container.Summary{},
		logsFn: func(ctx context.Context, c string, options container.LogsOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("")), nil
		},
		eventsFn: func(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error) {
			mu.Lock()
			eventsCalls++
			callNum := eventsCalls
			mu.Unlock()

			msgCh := make(chan events.Message)
			errCh := make(chan error, 1)

			if callNum == 1 {
				// Emit error on first call to trigger retry logic
				errCh <- errors.New("simulated docker daemon connection error")
			} else {
				// Re-subscribed successfully!
				select {
				case reSubscribed <- struct{}{}:
				default:
				}
			}
			return msgCh, errCh
		},
	}

	startDockerLogStreamer(ctx, fakeClient, agg, nil, logger)

	select {
	case <-reSubscribed:
		// Succeeded: watcher re-subscribed after error
	case <-time.After(3 * time.Second):
		t.Fatalf("expected event watcher to re-subscribe after error, but it did not within deadline")
	}

	mu.Lock()
	calls := eventsCalls
	mu.Unlock()
	require.GreaterOrEqual(t, calls, 2, "expected at least 2 Events calls (initial + re-subscribe)")
}
