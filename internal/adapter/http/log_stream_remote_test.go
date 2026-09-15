package http

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
)

type mockRemoteStreamer struct {
	mu           sync.Mutex
	lastAgentURL string
	lastService  string
	called       bool
	linesToEmit  []string
}

func (m *mockRemoteStreamer) StreamLogs(ctx context.Context, agentURL, service string, onLine func(line string)) error {
	m.mu.Lock()
	m.lastAgentURL = agentURL
	m.lastService = service
	m.called = true
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

	handler := NewLogStreamHandler(aggregator, streamer)

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
