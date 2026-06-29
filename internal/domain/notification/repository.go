package notification

import "context"

// Repository defines the data access interface for notifications.
type Repository interface {
	// Channels
	ListChannels(ctx context.Context) ([]Channel, error)
	CreateChannel(ctx context.Context, ch *Channel) error
	UpdateChannel(ctx context.Context, ch *Channel) error
	DeleteChannel(ctx context.Context, id string) error

	// Notifications
	ListNotifications(ctx context.Context, limit, offset int) ([]Notification, int, error)
	MarkRead(ctx context.Context, id string) error
	MarkAllRead(ctx context.Context) error

	// History
	ListHistory(ctx context.Context, limit, offset int) ([]Notification, int, error)
	CreateNotification(ctx context.Context, n *Notification) error
}
