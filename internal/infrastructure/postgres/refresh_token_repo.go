package postgres

import (
	"context"

	"github.com/datdt/k8sselfhost/internal/domain/user"
)

type refreshTokenRepo struct {
	db DBTX
}

// NewRefreshTokenRepo creates a new Postgres-backed RefreshToken repository.
func NewRefreshTokenRepo(db DBTX) user.RefreshTokenRepository {
	return &refreshTokenRepo{db: db}
}

func (r *refreshTokenRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.db)
}

func (r *refreshTokenRepo) Create(ctx context.Context, rt *user.RefreshToken) error {
	if rt.ID != "" {
		query := `
			INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, user_agent, ip_address)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING created_at
		`
		return r.getDB(ctx).QueryRow(ctx, query, rt.ID, rt.UserID, rt.TokenHash, rt.ExpiresAt, rt.UserAgent, rt.IPAddress).Scan(&rt.CreatedAt)
	}

	query := `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at, user_agent, ip_address)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text, created_at
	`
	return r.getDB(ctx).QueryRow(ctx, query, rt.UserID, rt.TokenHash, rt.ExpiresAt, rt.UserAgent, rt.IPAddress).Scan(&rt.ID, &rt.CreatedAt)
}

func (r *refreshTokenRepo) GetByHash(ctx context.Context, hash string) (*user.RefreshToken, error) {
	query := `
		SELECT id::text, user_id::text, token_hash, expires_at, created_at, revoked_at, COALESCE(user_agent, ''), COALESCE(ip_address, '')
		FROM refresh_tokens
		WHERE token_hash = $1
		LIMIT 1
	`
	var rt user.RefreshToken
	err := r.getDB(ctx).QueryRow(ctx, query, hash).Scan(
		&rt.ID,
		&rt.UserID,
		&rt.TokenHash,
		&rt.ExpiresAt,
		&rt.CreatedAt,
		&rt.RevokedAt,
		&rt.UserAgent,
		&rt.IPAddress,
	)
	if err != nil {
		return nil, err
	}
	return &rt, nil
}

func (r *refreshTokenRepo) Revoke(ctx context.Context, id string) error {
	query := `UPDATE refresh_tokens SET revoked_at = NOW() WHERE id::text = $1 AND revoked_at IS NULL`
	_, err := r.getDB(ctx).Exec(ctx, query, id)
	return err
}

func (r *refreshTokenRepo) RevokeAllForUser(ctx context.Context, userID string) error {
	query := `UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id::text = $1 AND revoked_at IS NULL`
	_, err := r.getDB(ctx).Exec(ctx, query, userID)
	return err
}

func (r *refreshTokenRepo) DeleteExpired(ctx context.Context) (int64, error) {
	query := `DELETE FROM refresh_tokens WHERE expires_at < NOW()`
	tag, err := r.getDB(ctx).Exec(ctx, query)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
