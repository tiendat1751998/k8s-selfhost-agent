package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

type LogStreamHandler struct {
	aggregator *logging.LogAggregator
}

func NewLogStreamHandler(aggregator *logging.LogAggregator) *LogStreamHandler {
	return &LogStreamHandler{
		aggregator: aggregator,
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
	filter := logging.LogFilter{
		Namespace: query.Get("namespace"),
		Pod:       query.Get("pod"),
		Container: query.Get("container"),
		Level:     query.Get("level"),
		Keyword:   query.Get("keyword"),
	}

	subID := fmt.Sprintf("ws-%s", uuid.New().String()[:8])
	sub, history := h.aggregator.Subscribe(subID, filter, 1000)
	defer h.aggregator.Unsubscribe(subID)

	// Send historical logs first
	for _, entry := range history {
		data, _ := json.Marshal(entry)
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
