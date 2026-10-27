package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	domaintenancy "github.com/datdt/k8sselfhost/internal/domain/tenancy"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	pkgtenancy "github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestTenancyExtendedRepo_Integration(t *testing.T) {
	pool := getAuthTestPool(t)
	if pool == nil {
		t.Skip("Database not available")
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ctx = pkgtenancy.WithUserRole(ctx, "platform_admin")

	repo := postgres.NewTenancyRepo(pool)
	ext, ok := repo.(interface {
		DeleteOrganization(ctx context.Context, id string) error
		CreateMember(ctx context.Context, member domaintenancy.Member) error
		UpdateMemberRole(ctx context.Context, id string, role string) error
		DeleteMember(ctx context.Context, id string) error
		UpdateRBAC(ctx context.Context, rbac map[string]map[string]bool) error
		UpdateQuota(ctx context.Context, orgID string, quota map[string]interface{}) error
		GetQuota(ctx context.Context, orgID string) (map[string]interface{}, error)
	})
	if !ok {
		t.Fatalf("TenancyRepo does not implement extendedTenancyRepo interface")
	}

	testOrgID := "test-org-" + uuid.NewString()[:8]

	// 1. Organization Lifecycle & default-tenant seeding
	orgs, err := repo.GetOrganizations(ctx)
	if err != nil {
		t.Fatalf("GetOrganizations failed: %v", err)
	}
	hasDefault := false
	for _, o := range orgs {
		if o.ID == "default-tenant" {
			hasDefault = true
			break
		}
	}
	if !hasDefault {
		t.Fatalf("expected default-tenant to be present in organizations")
	}

	// Create Organization
	newOrg := domaintenancy.Organization{
		ID:   testOrgID,
		Name: "Test Corp",
		Tier: "Enterprise",
	}
	if err := repo.CreateOrganization(ctx, newOrg); err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	// 2. Member CRUD
	memberID := uuid.NewString()
	member := domaintenancy.Member{
		ID:    memberID,
		OrgID: testOrgID,
		User:  "testuser@" + testOrgID + ".com",
		Role:  "developer",
		Scope: "project-wide",
	}
	if err := ext.CreateMember(ctx, member); err != nil {
		t.Fatalf("CreateMember failed: %v", err)
	}

	// Verify member in GetMembers
	members, err := repo.GetMembers(ctx)
	if err != nil {
		t.Fatalf("GetMembers failed: %v", err)
	}
	foundMember := false
	for _, m := range members {
		if m.ID == memberID {
			foundMember = true
			if m.Role != "developer" {
				t.Errorf("expected role 'developer', got %q", m.Role)
			}
			break
		}
	}
	if !foundMember {
		t.Errorf("created member %s not found in GetMembers", memberID)
	}

	// Update member role
	if err := ext.UpdateMemberRole(ctx, memberID, "admin"); err != nil {
		t.Fatalf("UpdateMemberRole failed: %v", err)
	}

	// Delete member
	if err := ext.DeleteMember(ctx, memberID); err != nil {
		t.Fatalf("DeleteMember failed: %v", err)
	}

	// 3. RBAC Matrix Persistence in platform_settings
	customRBAC := map[string]map[string]bool{
		"Platform Admin": {
			"pods:read": true,
			"pods:write": true,
			"custom:special_action": true,
		},
	}
	if err := ext.UpdateRBAC(ctx, customRBAC); err != nil {
		t.Fatalf("UpdateRBAC failed: %v", err)
	}

	// Read back RBAC from a fresh repo instance
	freshRepo := postgres.NewTenancyRepo(pool)
	loadedRBAC, err := freshRepo.GetRBAC(ctx)
	if err != nil {
		t.Fatalf("GetRBAC on freshRepo failed: %v", err)
	}
	if !loadedRBAC["Platform Admin"]["custom:special_action"] {
		t.Errorf("expected custom:special_action to be true in loaded RBAC")
	}

	// 4. Resource Quotas in platform_settings
	testQuota := map[string]interface{}{
		"cpu":       "16",
		"memory":    "32Gi",
		"max_pods":  float64(100),
	}
	if err := ext.UpdateQuota(ctx, testOrgID, testQuota); err != nil {
		t.Fatalf("UpdateQuota failed: %v", err)
	}
	loadedQuota, err := ext.GetQuota(ctx, testOrgID)
	if err != nil {
		t.Fatalf("GetQuota failed: %v", err)
	}
	if loadedQuota["cpu"] != "16" {
		t.Errorf("expected quota cpu='16', got %v", loadedQuota["cpu"])
	}

	// 5. Delete Organization cleanup
	if err := ext.DeleteOrganization(ctx, testOrgID); err != nil {
		t.Fatalf("DeleteOrganization failed: %v", err)
	}
}
