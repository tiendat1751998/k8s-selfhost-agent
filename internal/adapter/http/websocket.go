// Package http provides WebSocket hub for real-time event broadcasting.
package http

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Same-origin requests don't send Origin
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return u.Host == r.Host
	},
}

// WSMessage is the standard WebSocket message envelope.
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// WSHub manages WebSocket connections and broadcasts messages.
type WSHub struct {
	clients        map[*wsClient]struct{}
	broadcast      chan []byte
	register       chan *wsClient
	unregister     chan *wsClient
	welcomeMessage []byte
	mu             sync.RWMutex
}

type wsClient struct {
	hub  *WSHub
	conn *websocket.Conn
	send chan []byte
}

// NewWSHub creates a new WebSocket hub.
func NewWSHub() *WSHub {
	return &WSHub{
		clients:    make(map[*wsClient]struct{}),
		broadcast:  make(chan []byte, 1024),
		register:   make(chan *wsClient, 256),
		unregister: make(chan *wsClient, 256),
	}
}

// Run starts the hub's event loop. Call in a goroutine.
func (h *WSHub) Run() {
	for {
		select {
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
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Broadcast sends a message to all connected WebSocket clients.
func (h *WSHub) Broadcast(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		logger.Get().Error("failed to marshal WebSocket message", zap.Error(err))
		return
	}
	select {
	case h.broadcast <- data:
	default:
		logger.Get().Warn("WebSocket broadcast channel full, dropping message")
	}
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
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Get().Error("WebSocket upgrade failed", zap.Error(err))
		return
	}

	client := &wsClient{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *wsClient) readPump() {
	defer func() {
		c.hub.unregister <- c
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
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
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

// Broadcast sends a message to the hub.
func (b *WSBridge) Broadcast(msgType string, data interface{}) {
	b.Hub.Broadcast(WSMessage{Type: msgType, Data: data})
}
