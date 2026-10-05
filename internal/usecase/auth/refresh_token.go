package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	domainAuth "github.com/datdt/k8sselfhost/internal/domain/auth"
	"github.com/datdt/k8sselfhost/internal/domain/user"
)

// TokenIssuer defines token generation and hashing operations.
type TokenIssuer interface {
	GenerateAccessToken(userID, role, tenantID string) (string, error)
	GenerateRefreshToken() (string, error)
	HashToken(token string) string
}

type defaultTokenIssuer struct{}

func (defaultTokenIssuer) GenerateAccessToken(userID, role, tenantID string) (string, error) {
	return "access-token-" + userID, nil
}

func (defaultTokenIssuer) GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (defaultTokenIssuer) HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

type noopTxManager struct{}

func (noopTxManager) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// RotateTokenInput contains parameters for rotating a refresh token.
type RotateTokenInput struct {
	RefreshToken string
	UserAgent    string
	IPAddress    string
}

// RotateTokenResult contains the output of a successful token rotation.
type RotateTokenResult struct {
	AccessToken     string
	RawRefreshToken string
	UserID          string
}

// RotateRefreshToken performs atomic refresh token rotation using row-level locking.
func (u *Usecase) RotateRefreshToken(ctx context.Context, input RotateTokenInput) (*RotateTokenResult, error) {
	if strings.TrimSpace(input.RefreshToken) == "" {
		return nil, domainAuth.ErrInvalidToken
	}
	if u.tokenRepo == nil {
		return nil, errors.New("token repository not configured")
	}

	tokenHash := u.tokenIssuer.HashToken(input.RefreshToken)
	var result *RotateTokenResult

	err := u.txManager.RunInTx(ctx, func(txCtx context.Context) error {
		// 1. Fetch token with row-level lock (SELECT ... FOR UPDATE)
		rt, err := u.getByHashForUpdate(txCtx, tokenHash)
		if err != nil || rt == nil {
			return domainAuth.ErrInvalidToken
		}

		// 2. Reuse detection: if token is already revoked, reject immediately
		if rt.RevokedAt != nil {
			return domainAuth.ErrTokenRevoked
		}

		// 3. Expiration check
		if time.Now().After(rt.ExpiresAt) {
			return domainAuth.ErrTokenExpired
		}

		// 4. Invalidate/revoke presented token
		if err := u.tokenRepo.Revoke(txCtx, rt.ID); err != nil {
			return fmt.Errorf("revoking old token: %w", err)
		}

		// 5. Lookup user
		usr, err := u.repo.GetByID(txCtx, rt.UserID)
		if err != nil || usr == nil {
			return domainAuth.ErrUserNotFound
		}

		role := usr.TenantRole
		if usr.PlatformRole == "platform_admin" {
			role = "platform_admin"
		}

		// 6. Generate new token pair
		accessToken, err := u.tokenIssuer.GenerateAccessToken(usr.ID, role, usr.TenantID)
		if err != nil {
			return fmt.Errorf("generating access token: %w", err)
		}

		newRawRefreshToken, err := u.tokenIssuer.GenerateRefreshToken()
		if err != nil {
			return fmt.Errorf("generating refresh token: %w", err)
		}

		duration := u.refreshTokenDuration
		if duration <= 0 {
			duration = 7 * 24 * time.Hour
		}

		newRT := &user.RefreshToken{
			UserID:    usr.ID,
			TokenHash: u.tokenIssuer.HashToken(newRawRefreshToken),
			ExpiresAt: time.Now().Add(duration),
			UserAgent: input.UserAgent,
			IPAddress: input.IPAddress,
		}

		// 7. Persist newly issued token inside the transaction
		if err := u.tokenRepo.Create(txCtx, newRT); err != nil {
			return fmt.Errorf("persisting new refresh token: %w", err)
		}

		result = &RotateTokenResult{
			AccessToken:     accessToken,
			RawRefreshToken: newRawRefreshToken,
			UserID:          usr.ID,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

func (u *Usecase) getByHashForUpdate(ctx context.Context, hash string) (*user.RefreshToken, error) {
	if tr, ok := u.tokenRepo.(domainAuth.TokenRepository); ok {
		return tr.GetByHashForUpdate(ctx, hash)
	}
	return u.tokenRepo.GetByHash(ctx, hash)
}

// RevokeRefreshToken revokes a refresh token (used during logout).
func (u *Usecase) RevokeRefreshToken(ctx context.Context, rawToken string) error {
	if strings.TrimSpace(rawToken) == "" || u.tokenRepo == nil {
		return nil
	}
	tokenHash := u.tokenIssuer.HashToken(rawToken)
	rt, err := u.tokenRepo.GetByHash(ctx, tokenHash)
	if err != nil || rt == nil {
		return nil
	}
	return u.tokenRepo.Revoke(ctx, rt.ID)
}
