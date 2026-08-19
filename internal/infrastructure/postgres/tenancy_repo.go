package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/datdt/k8sselfhost/internal/domain/tenancy"
)

type tenancyRepo struct {
	pool DBTX
}

func NewTenancyRepo(pool DBTX) tenancy.Repository {
	return &tenancyRepo{pool: pool}
}

func (r *tenancyRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

func (r *tenancyRepo) GetOrganizations(ctx context.Context) ([]tenancy.Organization, error) {
	rows, err := r.getDB(ctx).Query(ctx, "SELECT id, name, tier FROM organizations ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("querying organizations: %w", err)
	}
	defer rows.Close()

	var result []tenancy.Organization
	for rows.Next() {
		var o tenancy.Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.Tier); err != nil {
			return nil, fmt.Errorf("scanning organization: %w", err)
		}
		result = append(result, o)
	}
	return result, nil
}

func (r *tenancyRepo) GetProjects(ctx context.Context) ([]tenancy.Project, error) {
	rows, err := r.getDB(ctx).Query(ctx, "SELECT id, org_id, name, envs, workloads FROM projects ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("querying projects: %w", err)
	}
	defer rows.Close()

	var result []tenancy.Project
	for rows.Next() {
		var p tenancy.Project
		var envsJSON []byte
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Name, &envsJSON, &p.Workloads); err != nil {
			return nil, fmt.Errorf("scanning project: %w", err)
		}
		if err := json.Unmarshal(envsJSON, &p.Envs); err != nil {
			p.Envs = []string{}
		}
		result = append(result, p)
	}
	return result, nil
}

func (r *tenancyRepo) GetMembers(ctx context.Context) ([]tenancy.Member, error) {
	rows, err := r.getDB(ctx).Query(ctx, "SELECT id, org_id, username, role, scope FROM tenant_members ORDER BY username")
	if err != nil {
		return nil, fmt.Errorf("querying tenant members: %w", err)
	}
	defer rows.Close()

	var result []tenancy.Member
	for rows.Next() {
		var m tenancy.Member
		if err := rows.Scan(&m.ID, &m.OrgID, &m.User, &m.Role, &m.Scope); err != nil {
			return nil, fmt.Errorf("scanning member: %w", err)
		}
		result = append(result, m)
	}
	return result, nil
}

func (r *tenancyRepo) GetRBAC(ctx context.Context) (map[string]map[string]bool, error) {
	rows, err := r.getDB(ctx).Query(ctx, "SELECT permission, roles FROM rbac_matrix")
	if err != nil {
		return nil, fmt.Errorf("querying rbac matrix: %w", err)
	}
	defer rows.Close()

	roleMatrix := make(map[string]map[string]bool)

	for rows.Next() {
		var rowKey string
		var jsonBytes []byte
		if err := rows.Scan(&rowKey, &jsonBytes); err != nil {
			return nil, fmt.Errorf("scanning rbac row: %w", err)
		}
		var valMap map[string]bool
		if err := json.Unmarshal(jsonBytes, &valMap); err != nil {
			valMap = make(map[string]bool)
		}

		// Check if rowKey is a role name
		isRole := strings.Contains(rowKey, "Admin") || strings.Contains(rowKey, "DevOps") ||
			strings.Contains(rowKey, "Developer") || strings.Contains(rowKey, "Viewer") ||
			strings.Contains(rowKey, "Auditor")

		if isRole {
			if roleMatrix[rowKey] == nil {
				roleMatrix[rowKey] = make(map[string]bool)
			}
			for perm, allowed := range valMap {
				roleMatrix[rowKey][perm] = allowed
			}
		} else {
			// rowKey is a permission name (e.g. "pods:read", "Clusters Read")
			for role, allowed := range valMap {
				if roleMatrix[role] == nil {
					roleMatrix[role] = make(map[string]bool)
				}
				roleMatrix[role][rowKey] = allowed
			}
		}
	}

	// If empty, return default enterprise RBAC matrix
	if len(roleMatrix) == 0 {
		roleMatrix = map[string]map[string]bool{
			"Platform Admin": {
				"pods:read": true, "pods:write": true, "deployments:scale": true,
				"secrets:manage": true, "backups:execute": true, "nodes:drain": true,
				"ai:configure": true, "changes:approve": true, "audit:view": true,
			},
			"DevOps Team": {
				"pods:read": true, "pods:write": true, "deployments:scale": true,
				"secrets:manage": true, "backups:execute": true, "nodes:drain": false,
				"ai:configure": true, "changes:approve": true, "audit:view": true,
			},
			"Developer": {
				"pods:read": true, "pods:write": false, "deployments:scale": false,
				"secrets:manage": false, "backups:execute": false, "nodes:drain": false,
				"ai:configure": false, "changes:approve": false, "audit:view": true,
			},
			"Viewer": {
				"pods:read": true, "pods:write": false, "deployments:scale": false,
				"secrets:manage": false, "backups:execute": false, "nodes:drain": false,
				"ai:configure": false, "changes:approve": false, "audit:view": false,
			},
			"Security Auditor": {
				"pods:read": true, "pods:write": false, "deployments:scale": false,
				"secrets:manage": false, "backups:execute": false, "nodes:drain": false,
				"ai:configure": false, "changes:approve": false, "audit:view": true,
			},
		}
	}

	return roleMatrix, nil
}

func (r *tenancyRepo) CreateOrganization(ctx context.Context, org tenancy.Organization) error {
	_, err := r.getDB(ctx).Exec(ctx, "INSERT INTO organizations (id, name, tier) VALUES ($1, $2, $3) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, tier = EXCLUDED.tier",
		org.ID, org.Name, org.Tier)
	if err != nil {
		return fmt.Errorf("inserting organization: %w", err)
	}
	return nil
}

func (r *tenancyRepo) CreateProject(ctx context.Context, proj tenancy.Project) error {
	envsJSON, err := json.Marshal(proj.Envs)
	if err != nil {
		return fmt.Errorf("marshaling envs: %w", err)
	}
	_, err = r.getDB(ctx).Exec(ctx, "INSERT INTO projects (id, org_id, name, envs, workloads) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, envs = EXCLUDED.envs, workloads = EXCLUDED.workloads",
		proj.ID, proj.OrgID, proj.Name, envsJSON, proj.Workloads)
	if err != nil {
		return fmt.Errorf("inserting project: %w", err)
	}
	return nil
}
