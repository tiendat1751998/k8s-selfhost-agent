package alert

import "context"

type Notifier interface {
	Send(ctx context.Context, channel *NotificationChannel, message string) error
}
