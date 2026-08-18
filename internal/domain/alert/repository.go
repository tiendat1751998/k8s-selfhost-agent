package alert

import "context"

type Repository interface {
	CreateChannel(ctx context.Context, channel *NotificationChannel) error
	ListChannels(ctx context.Context, tenantID string) ([]*NotificationChannel, error)

	CreateRule(ctx context.Context, rule *AlertRule) error
	ListRules(ctx context.Context, tenantID string) ([]*AlertRule, error)
	UpdateRule(ctx context.Context, rule *AlertRule) error
	DeleteRule(ctx context.Context, id, tenantID string) error

	CreateHistory(ctx context.Context, history *AlertHistory) error
	ListHistory(ctx context.Context, tenantID string) ([]*AlertHistory, error)
	AcknowledgeAlert(ctx context.Context, id, tenantID, userID string) error
}
