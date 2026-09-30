package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/agent"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// RemoteLogStreamer defines an interface for streaming remote container logs.
type RemoteLogStreamer interface {
	StreamLogs(ctx context.Context, agentURL, service string, onLine func(line string)) error
}

type LogStreamHandler struct {
	aggregator *logging.LogAggregator
	streamer   RemoteLogStreamer
	hostRepo   docker.ComputeHostRepository
}

func NewLogStreamHandler(aggregator *logging.LogAggregator, args ...any) *LogStreamHandler {
	h := &LogStreamHandler{
		aggregator: aggregator,
		streamer:   agent.NewAgentLogClient(),
	}
	for _, arg := range args {
		switch v := arg.(type) {
		case RemoteLogStreamer:
			h.streamer = v
		case docker.ComputeHostRepository:
			h.hostRepo = v
		}
	}
	return h
}

func (h *LogStreamHandler) resolveAgentURL(ctx context.Context, node, service string) string {
	node = strings.TrimSpace(node)
	if node != "" {
		if isLocalNode(node) {
			return ""
		}
		if strings.HasPrefix(node, "http://") || strings.HasPrefix(node, "https://") || strings.Contains(node, ":9100") {
			return node
		}
		if h.hostRepo != nil {
			hosts, err := h.hostRepo.ListAll(ctx)
			if err == nil {
				for _, host := range hosts {
					if strings.EqualFold(host.ID, node) || strings.EqualFold(host.Name, node) || strings.EqualFold(host.Endpoint, node) {
						if host.Endpoint != "" && !isLocalNode(host.Endpoint) {
							return host.Endpoint
						}
					}
				}
			}
		}
		if envAgent := os.Getenv("AGENT_URL"); envAgent != "" {
			return envAgent
		}
	}
	return ""
}

func isLocalNode(target string) bool {
	t := strings.ToLower(strings.TrimSpace(target))
	t = strings.TrimPrefix(t, "http://")
	t = strings.TrimPrefix(t, "https://")
	if idx := strings.IndexByte(t, ':'); idx != -1 {
		t = t[:idx]
	}
	return t == "standalone-host" || t == "local" || t == "localhost" || t == "127.0.0.1" || t == ""
}

func parseStreamLogLine(line string) (time.Time, string) {
	if idx := strings.IndexByte(line, ' '); idx >= 19 && idx <= 35 {
		rawTS := line[:idx]
		if t, err := time.Parse(time.RFC3339Nano, rawTS); err == nil {
			return t.UTC(), strings.TrimSpace(line[idx+1:])
		}
		if t, err := time.Parse(time.RFC3339, rawTS); err == nil {
			return t.UTC(), strings.TrimSpace(line[idx+1:])
		}
	}
	return time.Now().UTC(), line
}

func detectStreamLogLevel(msg string) string {
	u := strings.ToUpper(msg)
	switch {
	case strings.Contains(u, "ERROR") || strings.Contains(u, "FATAL") || strings.Contains(u, "PANIC"):
		return "ERROR"
	case strings.Contains(u, "WARN"):
		return "WARN"
	case strings.Contains(u, "DEBUG") || strings.Contains(u, "TRACE"):
		return "DEBUG"
	default:
		return "INFO"
	}
}

func (h *LogStreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.aggregator == nil {
		http.Error(w, "log aggregator unavailable", http.StatusServiceUnavailable)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Get().Error("failed to upgrade websocket for log streaming", zap.Error(err))
		return
	}
	defer conn.Close()

	query := r.URL.Query()
	node := query.Get("node")
	service := query.Get("service")
	container := query.Get("container")
	namespace := query.Get("namespace")
	pod := query.Get("pod")
	level := query.Get("level")
	keyword := query.Get("keyword")

	targetService := service
	if targetService == "" {
		targetService = container
	}
	if targetService == "" {
		targetService = pod
	}

	filter := logging.LogFilter{
		Namespace: namespace,
		Pod:       pod,
		Container: container,
		Node:      node,
		Service:   service,
		Level:     level,
		Keyword:   keyword,
	}

	replayLines := 1000
	if s := query.Get("replay_lines"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			replayLines = n
			if replayLines > 10000 {
				replayLines = 10000
			}
		}
	}

	subID := fmt.Sprintf("ws-%s", uuid.New().String()[:8])
	sub, history := h.aggregator.Subscribe(subID, filter, replayLines)
	defer h.aggregator.Unsubscribe(subID)

	batchMode := query.Get("batch") == "true" || query.Get("microbatch") == "true" || query.Get("replay_lines") != "" || query.Get("format") == "batch"
	if query.Get("batch") == "false" {
		batchMode = false
	}

	// If the target container is on a remote agent, spawn a goroutine using StreamLogs
	agentURL := h.resolveAgentURL(r.Context(), node, targetService)
	if agentURL != "" && h.streamer != nil {
		streamCtx, cancelStream := context.WithCancel(r.Context())
		defer cancelStream()

		go func() {
			if err := h.streamer.StreamLogs(streamCtx, agentURL, targetService, func(line string) {
				entryTS, msg := parseStreamLogLine(line)
				h.aggregator.Ingest(logging.LogEntry{
					Timestamp: entryTS,
					Namespace: namespace,
					Pod:       targetService,
					Container: targetService,
					Service:   targetService,
					Node:      node,
					Stream:    "stdout",
					Level:     detectStreamLogLevel(msg),
					Message:   msg,
				})
			}); err != nil && streamCtx.Err() == nil {
				logger.Get().Warn("remote agent log stream failed",
					zap.String("agent_url", agentURL),
					zap.String("service", targetService),
					zap.Error(err),
				)
			}
		}()
	}

	// Send historical logs first
	if batchMode {
		for i := 0; i < len(history); i += 50 {
			end := i + 50
			if end > len(history) {
				end = len(history)
			}
			chunk := history[i:end]
			data, _ := json.Marshal(chunk)
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	} else {
		for _, entry := range history {
			data, _ := json.Marshal(entry)
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}

	// Emit initial handshake status if targeting specific node/service with no historical records
	if len(history) == 0 && (node != "" || service != "") {
		handshakeEntry := logging.LogEntry{
			Timestamp: time.Now().UTC(),
			Namespace: namespace,
			Pod:       pod,
			Container: container,
			Node:      node,
			Service:   service,
			Stream:    "stdout",
			Level:     "INFO",
			Message:   fmt.Sprintf("[INFO] Real-time log stream opened for target (node: %s, service: %s). Waiting for live log events...", node, service),
		}
		var data []byte
		if batchMode {
			data, _ = json.Marshal([]logging.LogEntry{handshakeEntry})
		} else {
			data, _ = json.Marshal(handshakeEntry)
		}
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return
		}
	}

	// Read pump to handle close
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	if batchMode {
		h.streamBatched(conn, sub, done)
	} else {
		h.streamLegacy(conn, sub, done)
	}
}

func (h *LogStreamHandler) streamBatched(conn *websocket.Conn, sub *logging.Subscriber, done <-chan struct{}) {
	batch := make([]logging.LogEntry, 0, 50)
	flushTicker := time.NewTicker(50 * time.Millisecond)
	defer flushTicker.Stop()

	telemetryTicker := time.NewTicker(1 * time.Second)
	defer telemetryTicker.Stop()

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	var (
		droppedCount      int64
		entriesThisWindow int64
	)

	flushBatch := func() error {
		if len(batch) == 0 {
			return nil
		}
		data, err := json.Marshal(batch)
		batch = batch[:0]
		if err != nil {
			return nil
		}
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		return conn.WriteMessage(websocket.TextMessage, data)
	}

	for {
		select {
		case <-done:
			return
		case <-pingTicker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-flushTicker.C:
			if err := flushBatch(); err != nil {
				return
			}
		case <-telemetryTicker.C:
			droppedCount = sub.Dropped.Load()
			currentRate := float64(entriesThisWindow)
			entriesThisWindow = 0
			if droppedCount > 0 {
				telemetry := map[string]any{
					"type":          "stream_telemetry",
					"dropped":       droppedCount,
					"dropped_count": droppedCount,
					"rate":          currentRate,
				}
				tBytes, err := json.Marshal(telemetry)
				if err == nil {
					_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
					if err := conn.WriteMessage(websocket.TextMessage, tBytes); err != nil {
						return
					}
				}
			}
		case entry, ok := <-sub.Ch:
			if !ok {
				_ = flushBatch()
				return
			}
			entriesThisWindow++
			batch = append(batch, entry)
			if len(batch) >= 50 {
				if err := flushBatch(); err != nil {
					return
				}
			}
		}
	}
}

func (h *LogStreamHandler) streamLegacy(conn *websocket.Conn, sub *logging.Subscriber, done <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case entry, ok := <-sub.Ch:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			data, err := json.Marshal(entry)
			if err != nil {
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}
}
