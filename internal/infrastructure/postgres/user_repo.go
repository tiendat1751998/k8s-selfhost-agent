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
			u.password_hash, 
			COALESCE(r_platform.name, '') as platform_role,
			COALESCE(tb.tenant_id, 'default-tenant') as tenant_id,
			COALESCE(r_tenant.name, 'viewer') as tenant_role
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
		&u.PasswordHash,
		&u.PlatformRole,
		&u.TenantID,
		&u.TenantRole,
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
			u.password_hash, 
			COALESCE(r_platform.name, '') as platform_role,
			COALESCE(tb.tenant_id, 'default-tenant') as tenant_id,
			COALESCE(r_tenant.name, 'viewer') as tenant_role
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
		&u.PasswordHash,
		&u.PlatformRole,
		&u.TenantID,
		&u.TenantRole,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
