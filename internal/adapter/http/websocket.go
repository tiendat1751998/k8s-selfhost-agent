// Package http provides WebSocket hub for real-time event broadcasting.
package http

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func isSameHost(uHost, rHost string) bool {
	if uHost == rHost {
		return true
	}
	uName := uHost
	if h, _, err := net.SplitHostPort(uHost); err == nil {
		uName = h
	}
	rName := rHost
	if h, _, err := net.SplitHostPort(rHost); err == nil {
		rName = h
	}
	return strings.EqualFold(uName, rName)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	Subprotocols:    []string{"bearer"},
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Same-origin requests don't send Origin
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		if isSameHost(u.Host, r.Host) {
			return true
		}
		return middleware.IsOriginAllowed(origin)
	},
}

// WSMessage is the standard WebSocket message envelope.
type WSMessage struct {
	Type     string      `json:"type"`
	Data     interface{} `json:"data"`
	TenantID string      `json:"tenant_id,omitempty"`
}

type wsBroadcastItem struct {
	tenantID string
	data     []byte
}

// WSHub manages WebSocket connections and broadcasts messages.
type WSHub struct {
	clients        map[*wsClient]struct{}
	broadcast      chan wsBroadcastItem
	register       chan *wsClient
	unregister     chan *wsClient
	welcomeMessage []byte
	done           chan struct{}
	mu             sync.RWMutex
	dropCount      atomic.Uint64
}

type wsClient struct {
	hub      *WSHub
	conn     *websocket.Conn
	send     chan []byte
	TenantID string
	Role     string
}

// NewWSHub creates a new WebSocket hub.
func NewWSHub() *WSHub {
	return &WSHub{
		clients:    make(map[*wsClient]struct{}),
		broadcast:  make(chan wsBroadcastItem, 1024),
		register:   make(chan *wsClient, 256),
		unregister: make(chan *wsClient, 256),
		done:       make(chan struct{}),
	}
}

// Run starts the hub's event loop. Call in a goroutine.
func (h *WSHub) Run(ctx context.Context) {
	defer close(h.done)
	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			for client := range h.clients {
				if client.conn != nil {
					_ = client.conn.WriteControl(
						websocket.CloseMessage,
						websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server shutting down"),
						time.Now().Add(250*time.Millisecond),
					)
				}
				close(client.send)
				delete(h.clients, client)
			}
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			welcome := h.welcomeMessage
			h.mu.Unlock()
			logger.Get().Info("WebSocket client connected",
				zap.Int("total_clients", len(h.clients)),
			)
			// Send welcome message (config data) to new client
			if len(welcome) > 0 {
				select {
				case client.send <- welcome:
				default:
				}
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			logger.Get().Info("WebSocket client disconnected",
				zap.Int("total_clients", len(h.clients)),
			)

		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				// Filter broadcast events strictly by client.TenantID and Role.
				if message.tenantID != "" {
					if client.TenantID != message.tenantID && client.Role != "platform_admin" {
						continue
					}
				} else {
					// Empty tenant: deliver only to default tenant, untyped, or platform_admin
					if client.TenantID != "" && client.TenantID != "default" && client.Role != "platform_admin" {
						continue
					}
				}
				select {
				case client.send <- message.data:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Broadcast sends a message to all connected WebSocket clients matching the event tenant.
func (h *WSHub) Broadcast(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		logger.Get().Error("failed to marshal WebSocket message", zap.Error(err))
		return
	}
	item := wsBroadcastItem{
		tenantID: msg.TenantID,
		data:     data,
	}
	select {
	case h.broadcast <- item:
	default:
		h.dropCount.Add(1)
	}
}

// DroppedMessages returns the total count of dropped broadcast messages.
func (h *WSHub) DroppedMessages() uint64 {
	return h.dropCount.Load()
}

// ClientCount returns the number of connected clients.
func (h *WSHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// SetWelcomeMessage sets a message that is automatically sent to every new client on connect.
func (h *WSHub) SetWelcomeMessage(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		logger.Get().Error("failed to marshal welcome message", zap.Error(err))
		return
	}
	h.mu.Lock()
	h.welcomeMessage = data
	h.mu.Unlock()
}

// ServeWS handles WebSocket upgrade requests.
func (h *WSHub) ServeWS(w http.ResponseWriter, r *http.Request) {
	select {
	case <-h.done:
		http.Error(w, "server shutting down", http.StatusServiceUnavailable)
		return
	default:
	}

	w.Header().Del("Sec-WebSocket-Protocol")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Get().Error("WebSocket upgrade failed", zap.Error(err))
		return
	}

	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = r.URL.Query().Get("tenant_id")
	}
	if tenantID == "" {
		tenantID = r.Header.Get("X-Tenant-ID")
	}

	role := tenancy.UserRoleFromContext(r.Context())
	if role == "" {
		role = r.URL.Query().Get("role")
	}

	client := &wsClient{
		hub:      h,
		conn:     conn,
		send:     make(chan []byte, 256),
		TenantID: tenantID,
		Role:     role,
	}

	select {
	case h.register <- client:
	case <-h.done:
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server shutting down"),
			time.Now().Add(250*time.Millisecond),
		)
		conn.Close()
		return
	}

	go client.writePump()
	go client.readPump()
}

func (c *wsClient) readPump() {
	defer func() {
		select {
		case c.hub.unregister <- c:
		case <-c.hub.done:
		}
		c.conn.Close()
	}()

	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *wsClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// WSBridge bridges WSHub to other services needing to broadcast messages.
type WSBridge struct {
	Hub *WSHub
}

// NewWSBridge creates a new WSBridge.
func NewWSBridge(hub *WSHub) *WSBridge {
	return &WSBridge{Hub: hub}
}

// extractTenantID attempts to retrieve the tenant ID from the broadcast payload.
func extractTenantID(data interface{}) string {
	if data == nil {
		return ""
	}
	switch v := data.(type) {
	case *incident.Incident:
		if v != nil {
			return v.TenantID
		}
	case incident.Incident:
		return v.TenantID
	case interface{ GetTenantID() string }:
		return v.GetTenantID()
	case map[string]interface{}:
		if tid, ok := v["tenant_id"].(string); ok {
			return tid
		}
		if tid, ok := v["TenantID"].(string); ok {
			return tid
		}
	case map[string]string:
		if tid, ok := v["tenant_id"]; ok {
			return tid
		}
		if tid, ok := v["TenantID"]; ok {
			return tid
		}
	}
	return ""
}

// Broadcast sends a message to the hub with tenant isolation.
func (b *WSBridge) Broadcast(msgType string, data interface{}) {
	tenantID := extractTenantID(data)
	b.Hub.Broadcast(WSMessage{Type: msgType, Data: data, TenantID: tenantID})
}
