package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/alert"
	"github.com/datdt/k8sselfhost/internal/pkg/netguard"
)

type SlackNotifier struct {
	client *http.Client
}

// NewSlackNotifier creates a new SlackNotifier configured with a guarded HTTP client.
// Slack webhooks are public endpoints, so private/loopback/link-local ranges are blocked.
func NewSlackNotifier(client ...*http.Client) *SlackNotifier {
	var c *http.Client
	if len(client) > 0 && client[0] != nil {
		c = client[0]
	} else {
		c = netguard.New().NewClient(10 * time.Second)
	}
	return &SlackNotifier{client: c}
}

func (n *SlackNotifier) Send(ctx context.Context, channel *alert.NotificationChannel, message string) error {
	webhookURL, ok := channel.Config["webhook_url"].(string)
	if !ok || webhookURL == "" {
		return fmt.Errorf("slack webhook_url not configured")
	}

	// Require https scheme
	if _, err := netguard.New().ValidateURL(webhookURL, "https"); err != nil {
		return fmt.Errorf("invalid slack webhook url: %w", err)
	}

	payload := map[string]string{
		"text": message,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := n.client
	if client == nil {
		client = netguard.New().NewClient(10 * time.Second)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack returned non-2xx status: %d", resp.StatusCode)
	}

	return nil
}
