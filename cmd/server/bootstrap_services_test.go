package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
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

func TestBootstrapSREHandlers_CompilationAndConsensusWiring(t *testing.T) {
	log := zap.NewNop()
	params := platformHandlerParams{
		log:   log,
		infra: &infrastructure{},
	}

	handlers := bootstrapSREHandlers(params)
	require.NotNil(t, handlers)
	assert.NotNil(t, handlers.Consensus)
	assert.NotNil(t, handlers.StorageResiliency)
	assert.NotNil(t, handlers.CGroup)
	assert.NotNil(t, handlers.Fencing)
	assert.NotNil(t, handlers.APFGuard)
	assert.NotNil(t, handlers.K8sResilience)
}

func TestInitPlatformHandlers_SecurityAndIaCWiring(t *testing.T) {
	t.Setenv("GITOPS_WEBHOOK_SECRET", "test-secret-key-123")
	t.Setenv("IAC_WORKSPACE_DIR", "/tmp/iac-test-dir")

	cfg := &config.Config{}
	params := platformHandlerParams{
		cfg:   cfg,
		infra: &infrastructure{},
		log:   zap.NewNop(),
	}

	handlers := initPlatformHandlers(params)
	require.NotNil(t, handlers)
	require.NotNil(t, handlers.GitOps)
	require.NotNil(t, handlers.IaC)

	// Verify webhook secret wiring: request without HMAC header should return 401 Unauthorized
	// instead of 503 Service Unavailable (which occurs when webhook secret is unset).
	req := httptest.NewRequest(http.MethodPost, "/api/v1/gitops/webhook", strings.NewReader(`{"action":"opened"}`))
	rec := httptest.NewRecorder()
	handlers.GitOps.HandleWebhook(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "expected 401 Unauthorized when webhook secret is configured but header is missing")
}

