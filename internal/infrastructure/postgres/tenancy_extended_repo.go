package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/tenancy"
)

// DeleteOrganization purges an organization along with associated members, projects, and quotas.
func (r *tenancyRepo) DeleteOrganization(ctx context.Context, id string) error {
	db := r.getDB(ctx)

	// Clean up related quotas in platform_settings
	quotaKey := "tenancy.quotas." + id
	_, _ = db.Exec(ctx, "DELETE FROM platform_settings WHERE category = 'tenancy' AND (key = $1 OR tenant_id = $2)", quotaKey, id)

	// Clean up members and projects
	_, _ = db.Exec(ctx, "DELETE FROM tenant_members WHERE org_id = $1 OR tenant_id = $1", id)
	_, _ = db.Exec(ctx, "DELETE FROM projects WHERE org_id = $1 OR tenant_id = $1", id)

	_, err := db.Exec(ctx, "DELETE FROM organizations WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting organization %q: %w", id, err)
	}
	return nil
}

// CreateMember inserts or updates a member in the tenant_members table.
func (r *tenancyRepo) CreateMember(ctx context.Context, member tenancy.Member) error {
	db := r.getDB(ctx)
	memberID := member.ID
	if memberID == "" {
		memberID = uuid.NewString()
	}
	orgID := member.OrgID
	if orgID == "" {
		orgID = "default-tenant"
	}
	scope := member.Scope
	if scope == "" {
		scope = "project-wide"
	}

	query := `
		INSERT INTO tenant_members (id, org_id, username, role, scope, tenant_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			org_id = EXCLUDED.org_id,
			username = EXCLUDED.username,
			role = EXCLUDED.role,
			scope = EXCLUDED.scope,
			tenant_id = EXCLUDED.tenant_id
	`
	_, err := db.Exec(ctx, query, memberID, orgID, member.User, member.Role, scope, orgID)
	if err != nil {
		return fmt.Errorf("creating tenant member %q: %w", member.User, err)
	}
	return nil
}

// UpdateMemberRole updates the role assigned to a member.
func (r *tenancyRepo) UpdateMemberRole(ctx context.Context, id string, role string) error {
	db := r.getDB(ctx)
	tag, err := db.Exec(ctx, "UPDATE tenant_members SET role = $2 WHERE id = $1", id, role)
	if err != nil {
		return fmt.Errorf("updating member %s role: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("member %s not found", id)
	}
	return nil
}

// DeleteMember removes a member by ID.
func (r *tenancyRepo) DeleteMember(ctx context.Context, id string) error {
	db := r.getDB(ctx)
	_, err := db.Exec(ctx, "DELETE FROM tenant_members WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting tenant member %s: %w", id, err)
	}
	return nil
}

// UpdateRBAC persists the role-permission matrix into platform_settings under category 'tenancy'.
func (r *tenancyRepo) UpdateRBAC(ctx context.Context, rbac map[string]map[string]bool) error {
	jsonBytes, err := json.Marshal(rbac)
	if err != nil {
		return fmt.Errorf("marshaling rbac matrix: %w", err)
	}

	db := r.getDB(ctx)
	query := `
		INSERT INTO platform_settings (id, category, key, value, tenant_id, updated_by, updated_at)
		VALUES ($1, 'tenancy', 'tenancy.rbac_roles', $2, 'default-tenant', 'system', NOW())
		ON CONFLICT (category, key, tenant_id)
		DO UPDATE SET
			value = EXCLUDED.value,
			updated_at = NOW()
	`
	newID := uuid.NewString()
	_, err = db.Exec(ctx, query, newID, string(jsonBytes))
	if err != nil {
		return fmt.Errorf("persisting rbac matrix in platform_settings: %w", err)
	}
	return nil
}

// UpdateQuota stores organization resource limits into platform_settings.
func (r *tenancyRepo) UpdateQuota(ctx context.Context, orgID string, quota map[string]interface{}) error {
	jsonBytes, err := json.Marshal(quota)
	if err != nil {
		return fmt.Errorf("marshaling quota: %w", err)
	}

	key := "tenancy.quotas." + orgID
	db := r.getDB(ctx)
	query := `
		INSERT INTO platform_settings (id, category, key, value, tenant_id, updated_by, updated_at)
		VALUES ($1, 'tenancy', $2, $3, $4, 'system', NOW())
		ON CONFLICT (category, key, tenant_id)
		DO UPDATE SET
			value = EXCLUDED.value,
			updated_at = NOW()
	`
	newID := uuid.NewString()
	_, err = db.Exec(ctx, query, newID, key, string(jsonBytes), orgID)
	if err != nil {
		return fmt.Errorf("updating quota for org %s: %w", orgID, err)
	}
	return nil
}

// GetQuota retrieves organization resource limits from platform_settings.
func (r *tenancyRepo) GetQuota(ctx context.Context, orgID string) (map[string]interface{}, error) {
	key := "tenancy.quotas." + orgID
	db := r.getDB(ctx)

	var rawValue string
	err := db.QueryRow(ctx, "SELECT value FROM platform_settings WHERE category = 'tenancy' AND key = $1 LIMIT 1", key).Scan(&rawValue)
	if err != nil {
		if err == pgx.ErrNoRows {
			return make(map[string]interface{}), nil
		}
		return nil, fmt.Errorf("querying quota for org %s: %w", orgID, err)
	}

	var quota map[string]interface{}
	if err := json.Unmarshal([]byte(rawValue), &quota); err != nil {
		return make(map[string]interface{}), nil
	}
	return quota, nil
}
