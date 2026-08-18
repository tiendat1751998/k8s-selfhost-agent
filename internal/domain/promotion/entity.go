// Package promotion provides domain entities for deployment promotion.
package promotion

import (
	"fmt"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// Environment represents a deployment environment.
type Environment string

const (
	EnvDev        Environment = "dev"
	EnvQA         Environment = "qa"
	EnvStaging    Environment = "staging"
	EnvProduction Environment = "production"
)

// Status constants for Promotion lifecycle.
const (
	StatusPending   = "pending"
	StatusApproved  = "approved"
	StatusPromoting = "promoting"
	StatusCompleted = "completed"
	StatusRejected  = "rejected"
	StatusFailed    = "failed"
)

// Promotion represents a deployment promotion record.
type Promotion struct {
	ID          string      `json:"id"`
	Service     string      `json:"service"`
	Version     string      `json:"version"`
	FromEnv     Environment `json:"from_env"`
	ToEnv       Environment `json:"to_env"`
	Status      string      `json:"status"` // pending | approved | promoting | completed | failed
	Requester   string      `json:"requester"`
	Approver    string      `json:"approver,omitempty"`
	ApprovedAt  *time.Time  `json:"approved_at,omitempty"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
}

// NewPromotion creates a new Promotion domain entity with validation.
func NewPromotion(service, version string, fromEnv, toEnv Environment, requester string) (*Promotion, error) {
	if service == "" {
		return nil, errors.NewValidation("service", "must not be empty")
	}
	if version == "" {
		return nil, errors.NewValidation("version", "must not be empty")
	}
	if fromEnv == "" {
		return nil, errors.NewValidation("from_env", "must not be empty")
	}
	if toEnv == "" {
		return nil, errors.NewValidation("to_env", "must not be empty")
	}
	if fromEnv == toEnv {
		return nil, errors.NewValidation("to_env", "source and target environments must be different")
	}
	if requester == "" {
		return nil, errors.NewValidation("requester", "must not be empty")
	}

	now := time.Now().UTC()
	return &Promotion{
		Service:   service,
		Version:   version,
		FromEnv:   fromEnv,
		ToEnv:     toEnv,
		Status:    StatusPending,
		Requester: requester,
		CreatedAt: now,
	}, nil
}

// CanApprove returns true if the promotion is in pending state.
func (p *Promotion) CanApprove() bool {
	return p.Status == StatusPending
}

// CanReject returns true if the promotion is in pending state.
func (p *Promotion) CanReject() bool {
	return p.Status == StatusPending
}

// CanComplete returns true if the promotion is in approved or promoting state.
func (p *Promotion) CanComplete() bool {
	return p.Status == StatusApproved || p.Status == StatusPromoting
}

// Approve transitions the promotion to approved status.
func (p *Promotion) Approve(approver string) error {
	if !p.CanApprove() {
		return errors.NewConflict("promotion", fmt.Sprintf("cannot approve promotion in status %s", p.Status))
	}
	now := time.Now().UTC()
	p.Status = StatusApproved
	p.Approver = approver
	p.ApprovedAt = &now
	return nil
}

// Reject transitions the promotion to rejected status.
func (p *Promotion) Reject(rejecter string) error {
	if !p.CanReject() {
		return errors.NewConflict("promotion", fmt.Sprintf("cannot reject promotion in status %s", p.Status))
	}
	p.Status = StatusRejected
	p.Approver = rejecter
	return nil
}

// Complete transitions the promotion to completed status.
func (p *Promotion) Complete() error {
	if !p.CanComplete() {
		return errors.NewConflict("promotion", fmt.Sprintf("cannot complete promotion in status %s", p.Status))
	}
	now := time.Now().UTC()
	p.Status = StatusCompleted
	p.CompletedAt = &now
	return nil
}

// MarkPromoting transitions the promotion to promoting status.
func (p *Promotion) MarkPromoting() error {
	if p.Status != StatusApproved {
		return errors.NewConflict("promotion", fmt.Sprintf("cannot start promoting promotion in status %s", p.Status))
	}
	p.Status = StatusPromoting
	return nil
}

// Fail transitions the promotion to failed status.
func (p *Promotion) Fail() {
	p.Status = StatusFailed
}
