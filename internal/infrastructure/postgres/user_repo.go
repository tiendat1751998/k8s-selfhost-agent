package postgres

import (
	"context"

	"github.com/datdt/k8sselfhost/internal/domain/user"
)

type userRepo struct {
	db DBTX
}

// NewUserRepo creates a new Postgres-backed User repository.
func NewUserRepo(db DBTX) user.Repository {
	return &userRepo{db: db}
}

func (r *userRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.db)
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	query := `
		SELECT 
			u.id::text, 
			u.email,
			u.password_hash, 
			COALESCE(r_platform.name, '') as platform_role,
			COALESCE(tb.tenant_id, 'default-tenant') as tenant_id,
			COALESCE(r_tenant.name, 'viewer') as tenant_role,
			COALESCE(u.mfa_secret, '') as mfa_secret,
			COALESCE(u.mfa_enabled, false) as mfa_enabled,
			COALESCE(u.mfa_recovery_codes, '') as mfa_recovery_codes,
			u.mfa_verified_at
		FROM users u
		LEFT JOIN user_roles ur ON u.id = ur.user_id
		LEFT JOIN roles r_platform ON ur.role_id = r_platform.id AND r_platform.name = 'platform_admin'
		LEFT JOIN tenant_bindings tb ON u.id = tb.user_id
		LEFT JOIN roles r_tenant ON tb.role_id = r_tenant.id
		WHERE u.email = $1
		LIMIT 1
	`
	var u user.User
	err := r.getDB(ctx).QueryRow(ctx, query, email).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.PlatformRole,
		&u.TenantID,
		&u.TenantRole,
		&u.MFASecret,
		&u.MFAEnabled,
		&u.MFARecoveryCodes,
		&u.MFAVerifiedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepo) GetByID(ctx context.Context, id string) (*user.User, error) {
	query := `
		SELECT 
			u.id::text, 
			u.email,
			u.password_hash, 
			COALESCE(r_platform.name, '') as platform_role,
			COALESCE(tb.tenant_id, 'default-tenant') as tenant_id,
			COALESCE(r_tenant.name, 'viewer') as tenant_role,
			COALESCE(u.mfa_secret, '') as mfa_secret,
			COALESCE(u.mfa_enabled, false) as mfa_enabled,
			COALESCE(u.mfa_recovery_codes, '') as mfa_recovery_codes,
			u.mfa_verified_at
		FROM users u
		LEFT JOIN user_roles ur ON u.id = ur.user_id
		LEFT JOIN roles r_platform ON ur.role_id = r_platform.id AND r_platform.name = 'platform_admin'
		LEFT JOIN tenant_bindings tb ON u.id = tb.user_id
		LEFT JOIN roles r_tenant ON tb.role_id = r_tenant.id
		WHERE u.id::text = $1
		LIMIT 1
	`
	var u user.User
	err := r.getDB(ctx).QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.PlatformRole,
		&u.TenantID,
		&u.TenantRole,
		&u.MFASecret,
		&u.MFAEnabled,
		&u.MFARecoveryCodes,
		&u.MFAVerifiedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepo) UpdateMFA(ctx context.Context, userID, encryptedSecret string, enabled bool) error {
	if enabled {
		if encryptedSecret != "" {
			query := `UPDATE users SET mfa_secret = $2, mfa_enabled = true, mfa_verified_at = NOW(), updated_at = NOW() WHERE id::text = $1`
			_, err := r.getDB(ctx).Exec(ctx, query, userID, encryptedSecret)
			return err
		}
		query := `UPDATE users SET mfa_enabled = true, mfa_verified_at = NOW(), updated_at = NOW() WHERE id::text = $1`
		_, err := r.getDB(ctx).Exec(ctx, query, userID)
		return err
	}

	if encryptedSecret != "" {
		query := `UPDATE users SET mfa_secret = $2, mfa_enabled = false, updated_at = NOW() WHERE id::text = $1`
		_, err := r.getDB(ctx).Exec(ctx, query, userID, encryptedSecret)
		return err
	}

	query := `UPDATE users SET mfa_secret = NULL, mfa_enabled = false, mfa_recovery_codes = NULL, mfa_verified_at = NULL, updated_at = NOW() WHERE id::text = $1`
	_, err := r.getDB(ctx).Exec(ctx, query, userID)
	return err
}

func (r *userRepo) SetRecoveryCodes(ctx context.Context, userID, encryptedCodes string) error {
	query := `UPDATE users SET mfa_recovery_codes = $2, updated_at = NOW() WHERE id::text = $1`
	_, err := r.getDB(ctx).Exec(ctx, query, userID, encryptedCodes)
	return err
}

func (r *userRepo) GetMFASecret(ctx context.Context, userID string) (string, error) {
	query := `SELECT COALESCE(mfa_secret, '') FROM users WHERE id::text = $1`
	var encryptedSecret string
	err := r.getDB(ctx).QueryRow(ctx, query, userID).Scan(&encryptedSecret)
	if err != nil {
		return "", err
	}
	return encryptedSecret, nil
}
