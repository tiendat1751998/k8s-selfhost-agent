// Package changes provides domain entities for change management.
package changes

import (
	"fmt"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

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

// NewChangeRequest creates a new ChangeRequest domain entity with validation.
func NewChangeRequest(title, description string, changeType ChangeType, requester, cluster, namespace, resource string) (*ChangeRequest, error) {
	if title == "" {
		return nil, errors.NewValidation("title", "must not be empty")
	}
	if requester == "" {
		return nil, errors.NewValidation("requester", "must not be empty")
	}
	if cluster == "" {
		return nil, errors.NewValidation("cluster", "must not be empty")
	}
	if namespace == "" {
		return nil, errors.NewValidation("namespace", "must not be empty")
	}
	if resource == "" {
		return nil, errors.NewValidation("resource", "must not be empty")
	}
	if changeType == "" {
		changeType = TypeStandard
	}

	now := time.Now().UTC()
	return &ChangeRequest{
		Title:       title,
		Description: description,
		Type:        changeType,
		Status:      StatusPending,
		Requester:   requester,
		Cluster:     cluster,
		Namespace:   namespace,
		Resource:    resource,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// CanApprove returns true if the change request is in pending state.
func (r *ChangeRequest) CanApprove() bool {
	return r.Status == StatusPending
}

// CanReject returns true if the change request is in pending state.
func (r *ChangeRequest) CanReject() bool {
	return r.Status == StatusPending
}

// CanDeploy returns true if the change request is in approved state.
func (r *ChangeRequest) CanDeploy() bool {
	return r.Status == StatusApproved
}

// Approve transitions the change request to approved status.
func (r *ChangeRequest) Approve(approver string) error {
	if !r.CanApprove() {
		return errors.NewConflict("change_request", fmt.Sprintf("cannot approve change request in status %s", r.Status))
	}
	now := time.Now().UTC()
	r.Status = StatusApproved
	r.Approver = approver
	r.ApprovedAt = &now
	r.UpdatedAt = now
	return nil
}

// Reject transitions the change request to rejected status.
func (r *ChangeRequest) Reject(approver string) error {
	if !r.CanReject() {
		return errors.NewConflict("change_request", fmt.Sprintf("cannot reject change request in status %s", r.Status))
	}
	now := time.Now().UTC()
	r.Status = StatusRejected
	r.Approver = approver
	r.UpdatedAt = now
	return nil
}

// MarkDeployed transitions the change request to deployed status.
func (r *ChangeRequest) MarkDeployed() error {
	if !r.CanDeploy() {
		return errors.NewConflict("change_request", fmt.Sprintf("cannot mark change request as deployed in status %s", r.Status))
	}
	now := time.Now().UTC()
	r.Status = StatusDeployed
	r.UpdatedAt = now
	return nil
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
