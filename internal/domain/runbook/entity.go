// Package runbook provides domain entities for operational runbooks.
package runbook

import "time"

// Runbook represents an operational runbook.
type Runbook struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Category   string    `json:"category"`
	Content    string    `json:"content"`
	Tags       []string  `json:"tags"`
	Author     string    `json:"author"`
	StepsCount int       `json:"steps_count"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	TenantID   string    `json:"tenant_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
