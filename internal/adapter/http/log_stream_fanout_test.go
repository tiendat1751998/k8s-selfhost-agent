package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestLogStreamHandler_AllNodes_RemoteFanOut_PlatformAdmin(t *testing.T) {
	aggregator := logging.NewLogAggregator(100)
	streamer := &mockRemoteStreamer{
		linesPerAgent: map[string][]string{
			"http://10.10.10.133:9100": {
				"2026-09-16T05:00:00Z [db] [INFO] node 133 log line",
			},
			"http://10.10.10.80:9100": {
				"2026-09-16T05:00:01Z [my-nginx] [INFO] node 80 log line",
			},
		},
	}

	hostRepo := newMockComputeHostRepo()
	_ = hostRepo.Create(context.Background(), &docker.ComputeHost{
		ID:       "node-1",
		Name:     "k8smater133",
		Endpoint: "http://10.10.10.133:9100",
		Status:   "connected",
	})
	_ = hostRepo.Create(context.Background(), &docker.ComputeHost{
		ID:       "node-2",
		Name:     "k8sworker",
		Endpoint: "http://10.10.10.80:9100",
		Status:   "connected",
	})
	_ = hostRepo.Create(context.Background(), &docker.ComputeHost{
		ID:       "node-3",
		Name:     "k8smasterdeb",
		Endpoint: "http://10.10.10.60:9100",
		Status:   "offline",
	})

	handler := NewLogStreamHandler(aggregator, streamer, hostRepo)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), tenancy.UserRoleKey, "platform_admin")
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
	server := httptest.NewServer(testHandler)
	defer server.Close()

	// node is empty ("All Nodes")
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?node="

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	var receivedEntries []logging.LogEntry
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))

	for i := 0; i < 5; i++ {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}
		var entry logging.LogEntry
		if err := json.Unmarshal(msg, &entry); err == nil {
			if strings.HasPrefix(entry.Message, "[INFO] Real-time log stream opened") {
				continue
			}
			receivedEntries = append(receivedEntries, entry)
		}
	}

	streamer.mu.Lock()
	urls := streamer.allAgentURLs
	streamer.mu.Unlock()

	// Should dial both online hosts (133 and 80), but not offline (60)
	assert.Contains(t, urls, "http://10.10.10.133:9100")
	assert.Contains(t, urls, "http://10.10.10.80:9100")
	assert.NotContains(t, urls, "http://10.10.10.60:9100")

	assert.Len(t, receivedEntries, 2, "expected logs from both online nodes to be streamed")
}

func TestLogStreamHandler_AllNodes_RemoteFanOut_TenantIsolation(t *testing.T) {
	aggregator := logging.NewLogAggregator(100)
	streamer := &mockRemoteStreamer{
		linesPerAgent: map[string][]string{
			"http://10.10.10.133:9100": {
				"2026-09-16T05:00:00Z [tenant-a-app] [INFO] tenant a log",
			},
			"http://10.10.10.80:9100": {
				"2026-09-16T05:00:01Z [tenant-b-app] [INFO] tenant b log",
			},
		},
	}

	hostRepo := newMockComputeHostRepo()
	_ = hostRepo.Create(context.Background(), &docker.ComputeHost{
		ID:       "node-1",
		Name:     "k8smater133",
		Endpoint: "http://10.10.10.133:9100",
		TenantID: "tenant-a",
		Status:   "connected",
	})
	_ = hostRepo.Create(context.Background(), &docker.ComputeHost{
		ID:       "node-2",
		Name:     "k8sworker",
		Endpoint: "http://10.10.10.80:9100",
		TenantID: "tenant-b",
		Status:   "connected",
	})

	handler := NewLogStreamHandler(aggregator, streamer, hostRepo)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := tenancy.WithTenantID(r.Context(), "tenant-a")
		ctx = tenancy.WithUserRole(ctx, "developer")
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
	server := httptest.NewServer(testHandler)
	defer server.Close()

	// node is empty ("All Nodes") for non-admin tenant-a user
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?node="

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	var receivedEntries []logging.LogEntry
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))

	for i := 0; i < 5; i++ {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}
		var entry logging.LogEntry
		if err := json.Unmarshal(msg, &entry); err == nil {
			if strings.HasPrefix(entry.Message, "[INFO] Real-time log stream opened") {
				continue
			}
			receivedEntries = append(receivedEntries, entry)
		}
	}

	streamer.mu.Lock()
	urls := streamer.allAgentURLs
	streamer.mu.Unlock()

	// Must ONLY dial tenant-a host, NEVER tenant-b host!
	assert.Contains(t, urls, "http://10.10.10.133:9100")
	assert.NotContains(t, urls, "http://10.10.10.80:9100")

	assert.Len(t, receivedEntries, 1, "expected only tenant-a log entry")
}
