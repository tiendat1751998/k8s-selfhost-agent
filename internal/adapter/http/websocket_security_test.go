package http

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
)

func TestWSHub_EmptyTenantFiltering(t *testing.T) {
	hub := NewWSHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	clientSpecificTenant := &wsClient{
		hub:      hub,
		send:     make(chan []byte, 10),
		TenantID: "tenant-isolated",
		Role:     "developer",
	}
	clientDefaultTenant := &wsClient{
		hub:      hub,
		send:     make(chan []byte, 10),
		TenantID: "default",
		Role:     "viewer",
	}
	clientPlatformAdmin := &wsClient{
		hub:      hub,
		send:     make(chan []byte, 10),
		TenantID: "tenant-isolated",
		Role:     "platform_admin",
	}
	clientUntyped := &wsClient{
		hub:      hub,
		send:     make(chan []byte, 10),
		TenantID: "",
		Role:     "viewer",
	}

	hub.register <- clientSpecificTenant
	hub.register <- clientDefaultTenant
	hub.register <- clientPlatformAdmin
	hub.register <- clientUntyped

	deadline := time.Now().Add(1 * time.Second)
	for hub.ClientCount() < 4 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	// Broadcast message with empty TenantID
	hub.Broadcast(WSMessage{
		Type:     "system_alert",
		Data:     map[string]string{"msg": "global notification"},
		TenantID: "",
	})

	// clientSpecificTenant should NOT receive the empty tenant event
	select {
	case msg := <-clientSpecificTenant.send:
		t.Fatalf("unexpected message delivered to non-default tenant client: %s", string(msg))
	case <-time.After(100 * time.Millisecond):
		// Expected
	}

	// clientDefaultTenant SHOULD receive it
	select {
	case msg := <-clientDefaultTenant.send:
		if !strings.Contains(string(msg), "global notification") {
			t.Fatalf("clientDefaultTenant got unexpected payload: %s", string(msg))
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("clientDefaultTenant timed out waiting for empty tenant message")
	}

	// clientPlatformAdmin SHOULD receive it
	select {
	case msg := <-clientPlatformAdmin.send:
		if !strings.Contains(string(msg), "global notification") {
			t.Fatalf("clientPlatformAdmin got unexpected payload: %s", string(msg))
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("clientPlatformAdmin timed out waiting for empty tenant message")
	}

	// clientUntyped SHOULD receive it
	select {
	case msg := <-clientUntyped.send:
		if !strings.Contains(string(msg), "global notification") {
			t.Fatalf("clientUntyped got unexpected payload: %s", string(msg))
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("clientUntyped timed out waiting for empty tenant message")
	}
}

func TestWSHub_DropCountAtomic(t *testing.T) {
	hub := NewWSHub()
	// Fill broadcast channel (capacity 1024)
	for i := 0; i < 1024; i++ {
		hub.broadcast <- wsBroadcastItem{tenantID: "default", data: []byte("{}")}
	}

	initialDropped := hub.DroppedMessages()
	// Next broadcast should drop and increment atomic counter
	hub.Broadcast(WSMessage{Type: "overflow", Data: "test"})

	if hub.DroppedMessages() != initialDropped+1 {
		t.Fatalf("expected drop count %d, got %d", initialDropped+1, hub.DroppedMessages())
	}
}

func TestWSBridge_ExtractIncidentTenant(t *testing.T) {
	hub := NewWSHub()
	bridge := NewWSBridge(hub)

	inc := &incident.Incident{
		ID:       "inc-42",
		TenantID: "tenant-target",
		Message:  "pod crashed",
	}

	bridge.Broadcast("incident", inc)

	select {
	case item := <-hub.broadcast:
		if item.tenantID != "tenant-target" {
			t.Fatalf("expected broadcast tenantID 'tenant-target', got %q", item.tenantID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for bridge broadcast item")
	}
}
