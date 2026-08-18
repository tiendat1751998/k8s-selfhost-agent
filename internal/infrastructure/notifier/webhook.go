package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/datdt/k8sselfhost/internal/domain/alert"
)

type WebhookNotifier struct{}

func NewWebhookNotifier() *WebhookNotifier {
	return &WebhookNotifier{}
}

func (n *WebhookNotifier) Send(ctx context.Context, channel *alert.NotificationChannel, message string) error {
	url, ok := channel.Config["url"].(string)
	if !ok || url == "" {
		return fmt.Errorf("webhook url not configured")
	}

	payload := map[string]string{
		"message": message,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if headers, ok := channel.Config["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if vs, vok := v.(string); vok {
				req.Header.Set(k, vs)
			}
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-2xx status: %d", resp.StatusCode)
	}

	return nil
}
