// Package promotion provides domain entities for deployment promotion.
package promotion

import "time"

// Environment represents a deployment environment.
type Environment string

const (
	EnvDev        Environment = "dev"
	EnvQA         Environment = "qa"
	EnvStaging    Environment = "staging"
	EnvProduction Environment = "production"
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
