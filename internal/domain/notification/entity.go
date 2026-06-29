// Package notification provides domain entities for the notification system.
package notification

import "time"

// ChannelType represents a notification channel type.
type ChannelType string

const (
	ChannelSlack    ChannelType = "slack"
	ChannelEmail    ChannelType = "email"
	ChannelTeams    ChannelType = "teams"
	ChannelTelegram ChannelType = "telegram"
	ChannelWebhook  ChannelType = "webhook"
)

// DeliveryStatus represents the delivery status of a notification.
type DeliveryStatus string

const (
	StatusPending   DeliveryStatus = "pending"
	StatusDelivered DeliveryStatus = "delivered"
	StatusFailed    DeliveryStatus = "failed"
	StatusRetried   DeliveryStatus = "retried"
)

// Severity represents notification severity.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Channel represents a notification channel configuration.
type Channel struct {
	ID         string            `json:"id"`
	Type       ChannelType       `json:"type"`
	Name       string            `json:"name"`
	WebhookURL string            `json:"webhook_url,omitempty"`
	Config     map[string]string `json:"config,omitempty"`
	Enabled    bool              `json:"enabled"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// Notification represents a notification record.
type Notification struct {
	ID        string         `json:"id"`
	ChannelID string         `json:"channel_id,omitempty"`
	Title     string         `json:"title"`
	Severity  Severity       `json:"severity"`
	Message   string         `json:"message"`
	Status    DeliveryStatus `json:"status"`
	Error     string         `json:"error_detail,omitempty"`
	Read      bool           `json:"read"`
	CreatedAt time.Time      `json:"created_at"`
}
