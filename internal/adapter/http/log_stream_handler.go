package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/infrastructure/agent"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// RemoteLogStreamer defines an interface for streaming remote container logs.
type RemoteLogStreamer interface {
	StreamLogs(ctx context.Context, agentURL, service string, onLine func(line string)) error
}

type LogStreamHandler struct {
	aggregator *logging.LogAggregator
	streamer   RemoteLogStreamer
	hostRepo   docker.ComputeHostRepository
	redactor   *logengine.InStreamRedactor
}

func NewLogStreamHandler(aggregator *logging.LogAggregator, args ...any) *LogStreamHandler {
	h := &LogStreamHandler{
		aggregator: aggregator,
		streamer:   agent.NewAgentLogClient(),
		redactor:   logengine.NewInStreamRedactor(),
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

// resolveRegisteredNode validates that node resolves to a registered compute host or local node.
// It returns the remote agent URL (if remote and registered) and whether the node is authorized.
func (h *LogStreamHandler) resolveRegisteredNode(ctx context.Context, node, tenantID string) (string, bool) {
	node = strings.TrimSpace(node)
	if node == "" || isLocalNode(node) {
		return "", true
	}

	// If node contains a URL scheme or host:port, it is an endpoint.
	isEndpoint := strings.Contains(node, "://") || strings.Contains(node, ":")

	if h.hostRepo != nil {
		var hosts []docker.ComputeHost
		var err error
		if tenantID != "" && tenancy.UserRoleFromContext(ctx) != "platform_admin" {
			hosts, err = h.hostRepo.List(ctx, tenantID)
		} else {
			hosts, err = h.hostRepo.ListAll(ctx)
		}
		if err == nil {
			for _, host := range hosts {
				if strings.EqualFold(host.ID, node) || strings.EqualFold(host.Name, node) || strings.EqualFold(host.Endpoint, node) {
					if host.Endpoint != "" && !isLocalNode(host.Endpoint) {
						return host.Endpoint, true
					}
					return "", true
				}
			}
		}
		// hostRepo is present, but node was not found among registered compute hosts
		return "", false
	}

	// hostRepo is nil: endpoints cannot be dialed without verification
	if isEndpoint {
		return "", false
	}

	// Plain node label when hostRepo is nil (e.g., local aggregator unit testing)
	return "", true
}

func (h *LogStreamHandler) resolveAgentURL(ctx context.Context, node, service string) string {
	endpoint, _ := h.resolveRegisteredNode(ctx, node, "")
	return endpoint
}

// resolveOnlineStreamHosts returns active, reachable remote compute hosts for multi-node streaming fan-out:
// 1. For platform_admin: ListAll online hosts.
// 2. For non-platform_admin:
//    - If tenantID != "": List online hosts for this tenant ONLY. Never fall back to ListAll!
//    - If tenantID == "": Return empty slice (zero tenant laundering).
// 3. Online hosts have status in {"connected", "online", "healthy"}, non-empty endpoint, and are not local nodes.
// 4. Returns at most 8 hosts to bound fan-out concurrency.
func (h *LogStreamHandler) resolveOnlineStreamHosts(ctx context.Context, tenantID string) []docker.ComputeHost {
	if h.hostRepo == nil {
		return nil
	}

	isAdmin := isPlatformAdmin(ctx)
	var (
		hosts []docker.ComputeHost
		err   error
	)

	if isAdmin {
		hosts, err = h.hostRepo.ListAll(ctx)
	} else if tenantID != "" {
		hosts, err = h.hostRepo.List(ctx, tenantID)
	} else {
		return nil
	}

	if err != nil || len(hosts) == 0 {
		return nil
	}

	online := make([]docker.ComputeHost, 0, len(hosts))
	for _, host := range hosts {
		endpoint := strings.TrimSpace(host.Endpoint)
		if endpoint == "" || isLocalNode(endpoint) {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(host.Status))
		if status == "connected" || status == "online" || status == "healthy" {
			online = append(online, host)
		}
	}

	if len(online) > 8 {
		online = online[:8]
	}
	return online
}

func isLocalNode(target string) bool {
	t := strings.ToLower(strings.TrimSpace(target))
	if strings.Contains(t, "://") || strings.Contains(t, ":") {
		return false
	}
	return t == "standalone-host" || t == "local" || t == "localhost" || t == "127.0.0.1" || t == ""
}

func (h *LogStreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.aggregator == nil {
		http.Error(w, "log aggregator unavailable", http.StatusServiceUnavailable)
		return
	}

	query := r.URL.Query()
	node := query.Get("node")
	service := query.Get("service")
	container := query.Get("container")
	namespace := query.Get("namespace")
	pod := query.Get("pod")
	level := query.Get("level")
	keyword := firstNonEmpty(query.Get("keyword"), query.Get("query"), query.Get("search_text"), query.Get("q"))

	targetService := service
	if targetService == "" {
		targetService = container
	}
	if targetService == "" {
		targetService = pod
	}

	if code, msg := CheckLogAccessRBAC(r.Context(), namespace, targetService); code != 0 {
		http.Error(w, msg, code)
		return
	}

	tenantID := resolveTenant(r.Context(), r)
	if tenancy.UserRoleFromContext(r.Context()) == "platform_admin" {
		reqTenant := firstNonEmpty(r.Header.Get("X-Tenant-ID"), query.Get("tenant_id"), query.Get("tenant"))
		tenantID = strings.TrimSpace(reqTenant)
	}

	// SEC-07: Validate node against registered compute hosts before dialing or upgrading
	var agentURL string
	if node != "" && !isLocalNode(node) {
		if h.streamer != nil || strings.Contains(node, "://") || strings.Contains(node, ":") {
			targetURL, ok := h.resolveRegisteredNode(r.Context(), node, tenantID)
			if !ok {
				http.Error(w, "unknown node: not a registered compute host", http.StatusBadRequest)
				return
			}
			agentURL = targetURL
		}
	}

	w.Header().Del("Sec-WebSocket-Protocol")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Get().Error("failed to upgrade websocket for log streaming", zap.Error(err))
		return
	}
	defer conn.Close()

	filter := logging.LogFilter{
		TenantID:  tenantID,
		Namespace: namespace,
		Pod:       pod,
		Container: container,
		Node:      node,
		Service:   service,
		Level:     level,
		Keyword:   keyword,
	}
	if domainLogging.IsJournalctlAlias(targetService) {
		filter.Service = "journalctl"
		filter.Container = ""
		filter.Pod = ""
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

	type streamTarget struct {
		endpoint  string
		nodeLabel string
	}
	var streamTargets []streamTarget

	if agentURL != "" {
		streamTargets = append(streamTargets, streamTarget{endpoint: agentURL, nodeLabel: node})
	} else if node == "" && h.streamer != nil && h.hostRepo != nil {
		onlineHosts := h.resolveOnlineStreamHosts(r.Context(), tenantID)
		for _, host := range onlineHosts {
			nodeLabel := strings.TrimSpace(host.Name)
			if nodeLabel == "" {
				nodeLabel = strings.TrimSpace(host.IPAddress)
			}
			if nodeLabel == "" {
				nodeLabel = strings.TrimSpace(host.Endpoint)
			}
			streamTargets = append(streamTargets, streamTarget{endpoint: host.Endpoint, nodeLabel: nodeLabel})
		}
	}

	// If remote agent(s) should be streamed, spawn streamCtx-governed goroutines
	if len(streamTargets) > 0 && h.streamer != nil {
		streamCtx, cancelStream := context.WithCancel(r.Context())
		defer cancelStream()

		remoteTarget := targetService
		if domainLogging.IsJournalctlAlias(targetService) {
			remoteTarget = ""
		}
		isAdmin := isPlatformAdmin(r.Context())

		for _, st := range streamTargets {
			t := st
			go func() {
				if err := h.streamer.StreamLogs(streamCtx, t.endpoint, remoteTarget, func(line string) {
					if entry := processRemoteStreamLine(line, targetService, namespace, t.nodeLabel, tenantID, isAdmin, h.maskLogMessage); entry != nil {
						h.aggregator.Ingest(*entry)
					}
				}); err != nil && streamCtx.Err() == nil {
					logger.Get().Warn("remote agent log stream failed",
						zap.String("agent_url", t.endpoint),
						zap.String("service", targetService),
						zap.Error(err),
					)
				}
			}()
		}
	}

	// Redact historical logs before sending
	for i := range history {
		history[i].Message = h.maskLogMessage(history[i].Message)
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
			TenantID:  tenantID,
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
			entry.Message = h.maskLogMessage(entry.Message)
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
			entry.Message = h.maskLogMessage(entry.Message)
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

func (h *LogStreamHandler) maskLogMessage(msg string) string {
	if h.redactor != nil {
		return h.redactor.MaskString(msg)
	}
	return msg
}

