package notifier_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/alert"
	"github.com/datdt/k8sselfhost/internal/infrastructure/notifier"
)

func TestSlackNotifier_RequiresHTTPS(t *testing.T) {
	n := notifier.NewSlackNotifier()

	channel := &alert.NotificationChannel{
		ID:   "chan-1",
		Type: "slack",
		Config: map[string]interface{}{
			"webhook_url": "http://hooks.slack.com/services/T00/B00/XXXX",
		},
	}

	err := n.Send(context.Background(), channel, "test alert message")
	require.Error(t, err, "expected error when slack webhook URL uses http instead of https")
	assert.Contains(t, err.Error(), "scheme")
}

func TestSlackNotifier_BlocksPrivateAndLoopback(t *testing.T) {
	n := notifier.NewSlackNotifier()

	blockedURLs := []string{
		"https://127.0.0.1:8443/webhook",
		"https://localhost:8443/webhook",
		"https://169.254.169.254/webhook",
		"https://10.0.0.1/webhook",
		"https://192.168.1.1/webhook",
	}

	for _, u := range blockedURLs {
		channel := &alert.NotificationChannel{
			ID:   "chan-blocked",
			Type: "slack",
			Config: map[string]interface{}{
				"webhook_url": u,
			},
		}

		err := n.Send(context.Background(), channel, "test alert message")
		require.Error(t, err, "expected error when dialing private/loopback URL %s", u)
	}
}
