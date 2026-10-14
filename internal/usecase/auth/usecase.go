package auth

import (
	"context"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/datdt/k8sselfhost/internal/domain/ports"
	"github.com/datdt/k8sselfhost/internal/domain/user"
)

// dummyBcryptHash is a precomputed valid bcrypt hash used to mitigate timing side-channel attacks when a user is not found.
const dummyBcryptHash = "$2a$10$w095Hk4W4r1k6XGjS9j72.g98Z/JpT10wI7y8YkPZ3X9eU56q1iGi"

// Usecase coordinates authentication logic.
type Usecase struct {
	repo                 user.Repository
	rateLimiter          *totpRateLimiter
	replayTracker        *totpReplayTracker
	tokenRepo            user.RefreshTokenRepository
	txManager            ports.TransactionManager
	tokenIssuer          TokenIssuer
	refreshTokenDuration time.Duration
}

// Option configures Usecase.
type Option func(*Usecase)

// WithRefreshTokenRepo sets the token repository on Usecase.
func WithRefreshTokenRepo(repo user.RefreshTokenRepository) Option {
	return func(u *Usecase) {
		u.tokenRepo = repo
	}
}

// WithTxManager sets the transaction manager on Usecase.
func WithTxManager(tx ports.TransactionManager) Option {
	return func(u *Usecase) {
		u.txManager = tx
	}
}

// WithTokenIssuer sets the token issuer on Usecase.
func WithTokenIssuer(issuer TokenIssuer) Option {
	return func(u *Usecase) {
		u.tokenIssuer = issuer
	}
}

// WithRefreshTokenDuration sets the refresh token validity duration.
func WithRefreshTokenDuration(d time.Duration) Option {
	return func(u *Usecase) {
		u.refreshTokenDuration = d
	}
}

// SetRefreshTokenRepo configures the token repository.
func (u *Usecase) SetRefreshTokenRepo(repo user.RefreshTokenRepository) {
	u.tokenRepo = repo
}

// SetTokenIssuer configures the token issuer.
func (u *Usecase) SetTokenIssuer(issuer TokenIssuer) {
	u.tokenIssuer = issuer
}

// SetTxManager configures the transaction manager.
func (u *Usecase) SetTxManager(tx ports.TransactionManager) {
	u.txManager = tx
}

// NewUsecase creates a new Auth Usecase.
func NewUsecase(repo user.Repository, opts ...Option) *Usecase {
	u := &Usecase{
		repo:                 repo,
		rateLimiter:          newTOTPRateLimiter(5, 15*time.Minute),
		replayTracker:        newTOTPReplayTracker(),
		txManager:            noopTxManager{},
		refreshTokenDuration: 7 * 24 * time.Hour,
	}
	for _, opt := range opts {
		opt(u)
	}
	if u.tokenIssuer == nil {
		u.tokenIssuer = &defaultTokenIssuer{}
	}
	return u
}

// AuthResult holds details for a successfully authenticated user.
type AuthResult struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	TenantID    string `json:"tenant_id"`
	MFARequired bool   `json:"mfa_required"`
}

// Authenticate verifies user credentials and returns user details.
func (u *Usecase) Authenticate(ctx context.Context, email, password string) (*AuthResult, error) {
	usr, err := u.repo.GetByEmail(ctx, email)
	if err != nil || usr == nil {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyBcryptHash), []byte(password))
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
		UserID:      usr.ID,
		Email:       usr.Email,
		Role:        role,
		TenantID:    usr.TenantID,
		MFARequired: usr.MFAEnabled,
	}, nil
}

// PasswordVerifier defines the interface for verifying a user's password.
type PasswordVerifier interface {
	VerifyPassword(ctx context.Context, userID, password string) error
}

// VerifyPassword verifies user credentials by user ID.
func (u *Usecase) VerifyPassword(ctx context.Context, userID, password string) error {
	usr, err := u.repo.GetByID(ctx, userID)
	if err != nil || usr == nil {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyBcryptHash), []byte(password))
		return errors.New("user not found")
	}

	err = bcrypt.CompareHashAndPassword([]byte(usr.PasswordHash), []byte(password))
	if err != nil {
		return errors.New("invalid credentials")
	}

	return nil
}
