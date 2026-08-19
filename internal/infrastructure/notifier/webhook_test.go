package notifier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/alert"
)

func TestWebhookNotifier_SSRFBlocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := NewWebhookNotifier()
	channel := &alert.NotificationChannel{
		Config: map[string]interface{}{
			"url": server.URL,
		},
	}

	err := n.Send(context.Background(), channel, "test message")
	if err == nil {
		t.Fatal("expected error due to SSRF protection blocking local address, got nil")
	}
}

func TestWebhookNotifier_Success(t *testing.T) {
	received := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := &WebhookNotifier{
		client: server.Client(),
	}
	channel := &alert.NotificationChannel{
		Config: map[string]interface{}{
			"url": server.URL,
		},
	}

	err := n.Send(context.Background(), channel, "test message")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !received {
		t.Error("expected webhook request to be received")
	}
}
