package user

import "context"

// User represents a user entity in the domain layer.
type User struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
	PlatformRole string `json:"platform_role"`
	TenantID     string `json:"tenant_id"`
	TenantRole   string `json:"tenant_role"`
}

// Repository defines data access ports for user entity.
type Repository interface {
	GetByEmail(ctx context.Context, email string) (*User, error)
}
