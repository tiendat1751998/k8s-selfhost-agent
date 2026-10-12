package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

func TestWebSocket_CheckOrigin(t *testing.T) {
	tests := []struct {
		name        string
		origin      string
		requestHost string
		envOrigins  string
		expectAllow bool
	}{
		{
			name:        "Empty origin (same-origin request)",
			origin:      "",
			requestHost: "localhost:8080",
			expectAllow: true,
		},
		{
			name:        "Same host",
			origin:      "http://api.k8sselfhost.local",
			requestHost: "api.k8sselfhost.local",
			expectAllow: true,
		},
		{
			name:        "Unconfigured 127.0.0.1 rejected when not same-host",
			origin:      "http://127.0.0.1:5173",
			requestHost: "localhost:8080",
			expectAllow: false,
		},
		{
			name:        "Unconfigured localhost rejected when not same-host",
			origin:      "http://localhost:5173",
			requestHost: "127.0.0.1:8080",
			expectAllow: false,
		},
		{
			name:        "Configured allowed origin via env",
			origin:      "https://portal.mycompany.com",
			requestHost: "api.mycompany.com",
			envOrigins:  "https://portal.mycompany.com,https://admin.mycompany.com",
			expectAllow: true,
		},
		{
			name:        "Configured localhost dev origin via env",
			origin:      "http://localhost:5173",
			requestHost: "api.mycompany.com",
			envOrigins:  "http://localhost:5173",
			expectAllow: true,
		},
		{
			name:        "Wildcard origin in env does not trust arbitrary origin",
			origin:      "https://anything.io",
			requestHost: "api.anything.io",
			envOrigins:  "*",
			expectAllow: false,
		},
		{
			name:        "Disallowed external origin",
			origin:      "http://malicious.com",
			requestHost: "localhost:8080",
			expectAllow: false,
		},
		{
			name:        "Spoofed localhost subdomain",
			origin:      "http://localhost.evil.com",
			requestHost: "localhost:8080",
			expectAllow: false,
		},
		{
			name:        "Spoofed 127.0.0.1 subdomain",
			origin:      "http://127.0.0.1.evil.com",
			requestHost: "localhost:8080",
			expectAllow: false,
		},
		{
			name:        "Malformed origin URL",
			origin:      "http://[::1:not-valid",
			requestHost: "localhost:8080",
			expectAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envOrigins != "" {
				t.Setenv("CORS_ALLOWED_ORIGINS", tt.envOrigins)
			} else {
				t.Setenv("CORS_ALLOWED_ORIGINS", "")
			}

			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			req.Host = tt.requestHost
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			got := upgrader.CheckOrigin(req)
			if got != tt.expectAllow {
				t.Errorf("CheckOrigin() = %v, expected %v (origin=%q, host=%q, env=%q)",
					got, tt.expectAllow, tt.origin, tt.requestHost, tt.envOrigins)
			}
		})
	}
}

func TestWSHub_GracefulShutdown(t *testing.T) {
	hub := NewWSHub()
	ctx, cancel := context.WithCancel(context.Background())

	runDone := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(runDone)
	}()

	server := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	const clientCount = 5
	clients := make([]*websocket.Conn, clientCount)
	for i := 0; i < clientCount; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("dial client %d failed: %v", i, err)
		}
		clients[i] = conn
		defer conn.Close()
	}

	// Ensure clients are registered
	deadline := time.Now().Add(2 * time.Second)
	for hub.ClientCount() < clientCount && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if hub.ClientCount() != clientCount {
		t.Fatalf("expected %d clients registered, got %d", clientCount, hub.ClientCount())
	}

	// Cancel context to trigger graceful shutdown
	cancel()

	// 1. Assert Run returns in < 1s
	select {
	case <-runDone:
		// Succeeded in returning < 1s
	case <-time.After(1 * time.Second):
		t.Fatal("hub.Run did not return within 1s after context cancellation")
	}

	// 2. Assert connected test clients receive close
	for i, conn := range clients {
		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, _, err := conn.ReadMessage()
		if err == nil {
			t.Errorf("client %d expected close error, got nil", i)
		} else if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) &&
			!strings.Contains(err.Error(), "closed network connection") &&
			!strings.Contains(err.Error(), "close 1000") {
			t.Logf("client %d received close condition: %v", i, err)
		}
	}

	if count := hub.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients remaining after shutdown, got %d", count)
	}
}

func TestWSHub_UnregisterDoesNotBlockAfterShutdown(t *testing.T) {
	hub := NewWSHub()
	ctx, cancel := context.WithCancel(context.Background())

	runDone := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(runDone)
	}()

	// Shutdown the hub
	cancel()
	select {
	case <-runDone:
	case <-time.After(1 * time.Second):
		t.Fatal("hub.Run did not exit within 1s")
	}

	// Attempt unregister after hub is dead
	c := &wsClient{
		hub:  hub,
		send: make(chan []byte, 1),
	}

	unregDone := make(chan struct{})
	go func() {
		select {
		case hub.unregister <- c:
		case <-hub.done:
		}
		close(unregDone)
	}()

	select {
	case <-unregDone:
		// success: did not block!
	case <-time.After(500 * time.Millisecond):
		t.Fatal("unregister blocked after hub shutdown")
	}
}

func TestWSHub_GoroutineLeak_ConnectDisconnectCycles(t *testing.T) {
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	hub := NewWSHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	server := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	const cycles = 100
	for i := 0; i < cycles; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("cycle %d dial failed: %v", i, err)
		}
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = conn.Close()
	}

	// Cancel hub context
	cancel()
	server.Close()

	// Wait for goroutines to return to baseline
	deadline := time.Now().Add(3 * time.Second)
	var finalGoroutines int
	for time.Now().Before(deadline) {
		runtime.GC()
		finalGoroutines = runtime.NumGoroutine()
		if finalGoroutines <= baseline+2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if finalGoroutines > baseline+2 {
		t.Fatalf("goroutine leak detected after 100 cycles: baseline=%d, final=%d (leaked %d)",
			baseline, finalGoroutines, finalGoroutines-baseline)
	}
}

func BenchmarkWSHub_Broadcast(b *testing.B) {
	logger.Init("error")
	hub := NewWSHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	const numClients = 100
	clients := make([]*wsClient, numClients)
	for i := 0; i < numClients; i++ {
		c := &wsClient{
			hub:  hub,
			send: make(chan []byte, 256),
		}
		clients[i] = c
		hub.register <- c
		go func(send chan []byte) {
			for range send {
			}
		}(c.send)
	}

	deadline := time.Now().Add(2 * time.Second)
	for hub.ClientCount() < numClients && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if hub.ClientCount() != numClients {
		b.Fatalf("expected %d clients, got %d", numClients, hub.ClientCount())
	}

	msg := WSMessage{
		Type: "event",
		Data: map[string]string{"key": "value", "status": "ok"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.Broadcast(msg)
	}
	b.StopTimer()

	for _, c := range clients {
		hub.unregister <- c
	}
}

func TestWSHub_TenantIsolation(t *testing.T) {
	hub := NewWSHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	clientA := &wsClient{
		hub:      hub,
		send:     make(chan []byte, 10),
		TenantID: "tenant-A",
	}
	clientB := &wsClient{
		hub:      hub,
		send:     make(chan []byte, 10),
		TenantID: "tenant-B",
	}

	hub.register <- clientA
	hub.register <- clientB

	deadline := time.Now().Add(1 * time.Second)
	for hub.ClientCount() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	// 1. Broadcast event for tenant-A
	hub.Broadcast(WSMessage{
		Type:     "incident",
		Data:     map[string]string{"msg": "incident in tenant-A"},
		TenantID: "tenant-A",
	})

	select {
	case msg := <-clientA.send:
		if !strings.Contains(string(msg), "incident in tenant-A") {
			t.Fatalf("clientA received unexpected payload: %s", string(msg))
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("clientA expected to receive tenant-A broadcast, but timed out")
	}

	select {
	case msg := <-clientB.send:
		t.Fatalf("cross-tenant leak! clientB received tenant-A message: %s", string(msg))
	case <-time.After(100 * time.Millisecond):
		// Expected: clientB receives nothing!
	}

	// 2. Broadcast event for tenant-B
	hub.Broadcast(WSMessage{
		Type:     "incident",
		Data:     map[string]string{"msg": "incident in tenant-B"},
		TenantID: "tenant-B",
	})

	select {
	case msg := <-clientB.send:
		if !strings.Contains(string(msg), "incident in tenant-B") {
			t.Fatalf("clientB received unexpected payload: %s", string(msg))
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("clientB expected to receive tenant-B broadcast, but timed out")
	}

	select {
	case msg := <-clientA.send:
		t.Fatalf("cross-tenant leak! clientA received tenant-B message: %s", string(msg))
	case <-time.After(100 * time.Millisecond):
		// Expected: clientA receives nothing!
	}
}




