package auth

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"

	"github.com/datdt/k8sselfhost/internal/domain/user"
)

// Usecase coordinates authentication logic.
type Usecase struct {
	repo user.Repository
}

// NewUsecase creates a new Auth Usecase.
func NewUsecase(repo user.Repository) *Usecase {
	return &Usecase{repo: repo}
}

// AuthResult holds details for a successfully authenticated user.
type AuthResult struct {
	UserID   string
	Role     string
	TenantID string
}

// Authenticate verifies user credentials and returns user details.
func (u *Usecase) Authenticate(ctx context.Context, email, password string) (*AuthResult, error) {
	usr, err := u.repo.GetByEmail(ctx, email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword([]byte(usr.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	role := usr.TenantRole
	if usr.PlatformRole == "platform_admin" {
		role = "platform_admin"
	}

	return &AuthResult{
		UserID:   usr.ID,
		Role:     role,
		TenantID: usr.TenantID,
	}, nil
}

// PasswordVerifier defines the interface for verifying a user's password.
type PasswordVerifier interface {
	VerifyPassword(ctx context.Context, userID, password string) error
}

// VerifyPassword verifies user credentials by user ID.
func (u *Usecase) VerifyPassword(ctx context.Context, userID, password string) error {
	usr, err := u.repo.GetByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	err = bcrypt.CompareHashAndPassword([]byte(usr.PasswordHash), []byte(password))
	if err != nil {
		return errors.New("invalid credentials")
	}

	return nil
}
