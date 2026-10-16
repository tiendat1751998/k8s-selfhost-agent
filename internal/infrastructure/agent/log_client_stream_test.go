package agent

import (
	"context"
	"encoding/json"
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

	t.Run("sends Authorization header when AGENT_AUTH_TOKEN is set", func(t *testing.T) {
		t.Setenv("AGENT_AUTH_TOKEN", "env-secret-token")
		var receivedAuth string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "text/event-stream")
			if f, ok := w.(http.Flusher); ok {
				fmt.Fprintf(w, "data: ready\n\n")
				f.Flush()
			}
		}))
		defer server.Close()

		client := NewAgentLogClient()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_ = client.StreamLogs(ctx, server.URL, "redis-svc", func(line string) {
			cancel()
		})

		assert.Equal(t, "Bearer env-secret-token", receivedAuth)
	})

	t.Run("sends Authorization header when configured via WithAuthToken", func(t *testing.T) {
		var receivedAuth string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "text/event-stream")
			if f, ok := w.(http.Flusher); ok {
				fmt.Fprintf(w, "data: ready\n\n")
				f.Flush()
			}
		}))
		defer server.Close()

		client := NewAgentLogClient(WithAuthToken("client-token-456"))
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_ = client.StreamLogs(ctx, server.URL, "redis-svc", func(line string) {
			cancel()
		})

		assert.Equal(t, "Bearer client-token-456", receivedAuth)
	})

	t.Run("falls back to /logs/search when /logs/tail returns 404 or 503 or service is empty", func(t *testing.T) {
		ts, _ := time.Parse(time.RFC3339Nano, "2026-09-14T10:15:30Z")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/logs/tail" {
				http.Error(w, "docker unavailable or service empty", http.StatusServiceUnavailable)
				return
			}
			if r.URL.Path == "/logs/search" {
				w.Header().Set("Content-Type", "application/json")
				results := []LogSearchResult{
					{
						Timestamp: ts,
						Service:   "user@1000.service",
						Level:     "info",
						Message:   "Started Session 1",
					},
				}
				_ = json.NewEncoder(w).Encode(results)
				return
			}
			http.NotFound(w, r)
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
			_ = client.StreamLogs(ctx, server.URL, "", func(line string) {
				mu.Lock()
				received = append(received, line)
				cancel()
				mu.Unlock()
			})
		}()

		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for fallback stream")
		}

		mu.Lock()
		defer mu.Unlock()
		require.NotEmpty(t, received)
		assert.Contains(t, received[0], "user@1000.service")
		assert.Contains(t, received[0], "Started Session 1")
	})
}
