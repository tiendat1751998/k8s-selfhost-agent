// Package changes provides domain entities for change management.
package changes

import "time"

// ChangeStatus represents approval status.
type ChangeStatus string

const (
	StatusPending  ChangeStatus = "pending"
	StatusApproved ChangeStatus = "approved"
	StatusRejected ChangeStatus = "rejected"
	StatusDeployed ChangeStatus = "deployed"
)

// ChangeType represents the type of change.
type ChangeType string

const (
	TypeStandard  ChangeType = "standard"
	TypeEmergency ChangeType = "emergency"
)

// ChangeRequest represents a deployment change request.
type ChangeRequest struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Type        ChangeType   `json:"type"`
	Status      ChangeStatus `json:"status"`
	Requester   string       `json:"requester"`
	Approver    string       `json:"approver,omitempty"`
	Cluster     string       `json:"cluster"`
	Namespace   string       `json:"namespace"`
	Resource    string       `json:"resource"`
	ScheduledAt *time.Time   `json:"scheduled_at,omitempty"`
	ApprovedAt  *time.Time   `json:"approved_at,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// MaintenanceWindow represents a scheduled maintenance window.
type MaintenanceWindow struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Cluster   string    `json:"cluster"`
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}
