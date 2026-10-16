package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
)

type mockRemoteStreamer struct {
	mu           sync.Mutex
	lastAgentURL string
	lastService  string
	called       bool
	linesToEmit  []string
	errToReturn  error
}

func (m *mockRemoteStreamer) StreamLogs(ctx context.Context, agentURL, service string, onLine func(line string)) error {
	m.mu.Lock()
	m.lastAgentURL = agentURL
	m.lastService = service
	m.called = true
	retErr := m.errToReturn
	lines := append([]string(nil), m.linesToEmit...)
	m.mu.Unlock()

	for _, line := range lines {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if onLine != nil {
			onLine(line)
		}
	}
	if retErr != nil {
		return retErr
	}
	<-ctx.Done()
	return ctx.Err()
}

func TestLogStreamHandler_RemoteAgentRelay(t *testing.T) {
	aggregator := logging.NewLogAggregator(100)
	streamer := &mockRemoteStreamer{
		linesToEmit: []string{
			"2026-09-16T05:00:00Z Remote log line from agent",
		},
	}

	hostRepo := newMockComputeHostRepo()
	_ = hostRepo.Create(context.Background(), &docker.ComputeHost{
		ID:       "remote-agent",
		Endpoint: "http://remote-agent:9100",
		Status:   "connected",
	})
	handler := NewLogStreamHandler(aggregator, streamer, hostRepo)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?service=payment-svc&node=http://remote-agent:9100"

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	// Read first message (could be handshake or the streamed entry)
	var foundEntry bool
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))

	for i := 0; i < 3; i++ {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}
		var entry logging.LogEntry
		if err := json.Unmarshal(msg, &entry); err == nil {
			if entry.Message == "Remote log line from agent" {
				foundEntry = true
				assert.Equal(t, "payment-svc", entry.Service)
				assert.Equal(t, "http://remote-agent:9100", entry.Node)
				break
			}
		}
	}

	assert.True(t, foundEntry, "expected to receive relayed remote log entry over websocket")

	streamer.mu.Lock()
	defer streamer.mu.Unlock()
	assert.True(t, streamer.called)
	assert.Equal(t, "http://remote-agent:9100", streamer.lastAgentURL)
	assert.Equal(t, "payment-svc", streamer.lastService)
}

func TestLogStreamHandler_ResolveAgentURL_NoBlindFallback(t *testing.T) {
	hostRepo := newMockComputeHostRepo()
	_ = hostRepo.Create(context.Background(), &docker.ComputeHost{
		ID:       "host-1",
		Name:     "worker-1",
		Endpoint: "http://192.168.1.50:9100",
		Status:   "connected",
	})

	handler := NewLogStreamHandler(nil, hostRepo)

	// If node is empty, resolveAgentURL MUST NOT return host-1 endpoint
	resolved := handler.resolveAgentURL(context.Background(), "", "payment-svc")
	assert.Empty(t, resolved, "must not blindly fall back to first remote host when node is empty")

	// If node is explicitly specified as host-1, resolveAgentURL MUST return host-1 endpoint
	resolvedWithNode := handler.resolveAgentURL(context.Background(), "host-1", "payment-svc")
	assert.Equal(t, "http://192.168.1.50:9100", resolvedWithNode)
}

func TestLogStreamHandler_RemoteAgentStreamErrorHandled(t *testing.T) {
	aggregator := logging.NewLogAggregator(100)
	streamer := &mockRemoteStreamer{
		errToReturn: fmt.Errorf("remote connection refused"),
	}

	hostRepo := newMockComputeHostRepo()
	_ = hostRepo.Create(context.Background(), &docker.ComputeHost{
		ID:       "remote-agent",
		Endpoint: "http://remote-agent:9100",
		Status:   "connected",
	})
	handler := NewLogStreamHandler(aggregator, streamer, hostRepo)
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?service=payment-svc&node=http://remote-agent:9100"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	// Even if remote stream fails, websocket connection remains alive and does not panic
	time.Sleep(50 * time.Millisecond)
	streamer.mu.Lock()
	defer streamer.mu.Unlock()
	assert.True(t, streamer.called)
}

func TestLogStreamHandler_UnregisteredNode_RejectedWithoutDial(t *testing.T) {
	var dialed int
	fakeAgent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dialed++
		w.WriteHeader(http.StatusOK)
	}))
	defer fakeAgent.Close()

	aggregator := logging.NewLogAggregator(100)
	hostRepo := newMockComputeHostRepo()
	// Register only host-1, not fakeAgent
	_ = hostRepo.Create(context.Background(), &docker.ComputeHost{
		ID:       "host-1",
		Name:     "worker-1",
		Endpoint: "http://192.168.1.50:9100",
		Status:   "connected",
	})

	streamer := &mockRemoteStreamer{}
	handler := NewLogStreamHandler(aggregator, streamer, hostRepo)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Try to stream logs with an unregistered node
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?service=payment-svc&node=" + fakeAgent.URL
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.Error(t, err, "expected dialing unregistered node to fail")
	require.NotNil(t, resp, "expected HTTP response")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "expected 400 Bad Request for unregistered node")

	// Ensure fakeAgent was never dialed
	assert.Equal(t, 0, dialed, "expected zero dials to unregistered node")
	assert.False(t, streamer.called, "expected streamer never to be called for unregistered node")
}

func TestLogStreamHandler_RemoteAgentRelay_ExtractsServiceTagWhenTargetServiceEmpty(t *testing.T) {
	aggregator := logging.NewLogAggregator(100)
	streamer := &mockRemoteStreamer{
		linesToEmit: []string{
			"2026-09-14T10:15:30.123456789Z [user@1000.service] [INFO] Started Session 1 of user datdt.",
		},
	}

	hostRepo := newMockComputeHostRepo()
	_ = hostRepo.Create(context.Background(), &docker.ComputeHost{
		ID:       "remote-agent",
		Endpoint: "http://remote-agent:9100",
		Status:   "connected",
	})
	handler := NewLogStreamHandler(aggregator, streamer, hostRepo)

	server := httptest.NewServer(handler)
	defer server.Close()

	// service is empty
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?node=http://remote-agent:9100"

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	var foundEntry bool
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))

	for i := 0; i < 3; i++ {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}
		var entry logging.LogEntry
		if err := json.Unmarshal(msg, &entry); err == nil {
			if strings.Contains(entry.Message, "Started Session 1 of user datdt.") {
				foundEntry = true
				assert.Equal(t, "user@1000.service", entry.Service)
				assert.Equal(t, "user@1000.service", entry.Pod)
				assert.Equal(t, "user@1000.service", entry.Container)
				assert.Equal(t, "INFO", entry.Level)
				assert.Equal(t, "http://remote-agent:9100", entry.Node)
				break
			}
		}
	}

	assert.True(t, foundEntry, "expected to receive relayed remote log entry with extracted service tag")
}
