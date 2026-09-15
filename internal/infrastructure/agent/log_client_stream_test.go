package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentLogClient_StreamLogs(t *testing.T) {
	t.Run("streams SSE lines and invokes callback", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/logs/tail", r.URL.Path)
			assert.Equal(t, "redis-svc", r.URL.Query().Get("service"))

			w.Header().Set("Content-Type", "text/event-stream")
			flusher, ok := w.(http.Flusher)
			require.True(t, ok)

			lines := []string{
				"2026-09-16T05:00:00Z Redis ready",
				"2026-09-16T05:00:01Z Client connected 10.0.0.1",
			}
			for _, line := range lines {
				fmt.Fprintf(w, "data: %s\n\n", line)
				flusher.Flush()
			}
		}))
		defer server.Close()

		client := NewAgentLogClient()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var (
			mu       sync.Mutex
			received []string
		)
		done := make(chan struct{})

		go func() {
			defer close(done)
			_ = client.StreamLogs(ctx, server.URL, "redis-svc", func(line string) {
				mu.Lock()
				received = append(received, line)
				if len(received) == 2 {
					cancel()
				}
				mu.Unlock()
			})
		}()

		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for stream to finish")
		}

		mu.Lock()
		defer mu.Unlock()
		require.Len(t, received, 2)
		assert.Equal(t, "2026-09-16T05:00:00Z Redis ready", received[0])
		assert.Equal(t, "2026-09-16T05:00:01Z Client connected 10.0.0.1", received[1])
	})

	t.Run("returns error on HTTP failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "service not found", http.StatusNotFound)
		}))
		defer server.Close()

		client := NewAgentLogClient()
		err := client.StreamLogs(context.Background(), server.URL, "missing-svc", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP 404")
	})

	t.Run("returns error on empty url", func(t *testing.T) {
		client := NewAgentLogClient()
		err := client.StreamLogs(context.Background(), "", "svc", nil)
		require.Error(t, err)
	})
}
