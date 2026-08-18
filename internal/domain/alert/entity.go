package alert

import "time"

type NotificationChannel struct {
	ID        string
	TenantID  string
	Name      string
	Type      string
	Config    map[string]interface{}
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type AlertRule struct {
	ID              string
	TenantID        string
	Name            string
	Description     string
	MetricName      string
	Condition       string
	Threshold       float64
	DurationSeconds int
	Severity        string
	ChannelIDs      []string
	Enabled         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type AlertHistory struct {
	ID             string
	TenantID       string
	RuleID         string
	Status         string
	Value          float64
	Message        string
	AcknowledgedBy string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
