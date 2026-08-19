package user

import (
	"context"
	"time"
)

// User represents a user entity in the domain layer.
type User struct {
	ID               string     `json:"id"`
	Email            string     `json:"email"`
	PasswordHash     string     `json:"-"`
	PlatformRole     string     `json:"platform_role"`
	TenantID         string     `json:"tenant_id"`
	TenantRole       string     `json:"tenant_role"`
	MFASecret        string     `json:"-"`              // encrypted TOTP secret
	MFAEnabled       bool       `json:"mfa_enabled"`
	MFARecoveryCodes string     `json:"-"`              // encrypted JSON array
	MFAVerifiedAt    *time.Time `json:"mfa_verified_at,omitempty"`
}

// Repository defines data access ports for user entity.
type Repository interface {
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
	UpdateMFA(ctx context.Context, userID, encryptedSecret string, enabled bool) error
	SetRecoveryCodes(ctx context.Context, userID, encryptedCodes string) error
	GetMFASecret(ctx context.Context, userID string) (string, error)
}
