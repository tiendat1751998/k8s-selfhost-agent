package auth

import (
	"context"

	"github.com/datdt/k8sselfhost/internal/domain/user"
)

// TokenRepository defines repository operations required for atomic refresh token rotation.
type TokenRepository interface {
	user.RefreshTokenRepository
	GetByHashForUpdate(ctx context.Context, hash string) (*user.RefreshToken, error)
}
