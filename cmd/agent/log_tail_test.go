package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTailDockerClient struct {
	containers []types.Container
	logOutput  string
	lastTail   string
}

func (m *mockTailDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	return m.containers, nil
}

func (m *mockTailDockerClient) ContainerLogs(ctx context.Context, c string, options container.LogsOptions) (io.ReadCloser, error) {
	m.lastTail = options.Tail
	return io.NopCloser(bytes.NewBufferString(m.logOutput)), nil
}

func (m *mockTailDockerClient) Ping(ctx context.Context) (types.Ping, error) {
	return types.Ping{}, nil
}

func (m *mockTailDockerClient) Close() error {
	return nil
}

func TestLogServer_HandleTailLogs(t *testing.T) {
	mockCli := &mockTailDockerClient{
		containers: []types.Container{
			{
				ID:    "c123456789012",
				Names: []string{"/web-api"},
				Labels: map[string]string{
					"com.docker.compose.service": "web-api",
				},
			},
		},
		logOutput: "2026-09-16T05:00:00Z line 1\n2026-09-16T05:00:01Z line 2\n",
	}

	logServer := NewLogServer(WithDockerClient(mockCli))
	collector := NewSystemCollector("", "", nil)
	handler := setupHandler(collector, "", logServer)

	t.Run("GET /logs/tail streams SSE for service", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs/tail?service=web-api&tail=50", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
		assert.Equal(t, "50", mockCli.lastTail)

		body := rec.Body.String()
		assert.Contains(t, body, "data: 2026-09-16T05:00:00Z line 1\n\n")
		assert.Contains(t, body, "data: 2026-09-16T05:00:01Z line 2\n\n")
	})

	t.Run("GET /logs/tail returns 404 for unknown service", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs/tail?service=nonexistent", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("GET /logs/tail requires auth when auth-token set", func(t *testing.T) {
		authedHandler := setupHandler(collector, "secret-token", logServer)

		req := httptest.NewRequest(http.MethodGet, "/logs/tail?service=web-api", nil)
		rec := httptest.NewRecorder()
		authedHandler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		reqAuth := httptest.NewRequest(http.MethodGet, "/logs/tail?service=web-api", nil)
		reqAuth.Header.Set("Authorization", "Bearer secret-token")
		recAuth := httptest.NewRecorder()
		authedHandler.ServeHTTP(recAuth, reqAuth)
		assert.Equal(t, http.StatusOK, recAuth.Code)
	})
}
