package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/ecosystem"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
)

func cleanupEcosystem(ctx context.Context, pool postgres.DBTX, tenantID string) {
	_, _ = pool.Exec(ctx, "DELETE FROM ecosystem_tools WHERE tenant_id = $1", tenantID)
}

func TestEcosystem_CreateAndGetByID(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	repo := postgres.NewEcosystemRepo(pool)
	tenantID := fmt.Sprintf("tenant-eco-crud-%d", time.Now().UnixNano())
	defer cleanupEcosystem(ctx, pool, tenantID)

	tool := &ecosystem.DetectedTool{
		Name:        "ArgoCD",
		Category:    ecosystem.CategoryGitOps,
		Status:      ecosystem.StatusDetected,
		Version:     "v2.10.2",
		Endpoint:    "https://argocd.example.com",
		Source:      ecosystem.SourceSettings,
		Health:      ecosystem.HealthHealthy,
		LastChecked: time.Now().UTC(),
		Metadata: map[string]string{
			"cluster": "prod-cluster",
		},
		TenantID: tenantID,
	}

	if err := repo.Create(ctx, tool); err != nil {
		t.Fatalf("failed to create ecosystem tool: %v", err)
	}

	if tool.ID == "" {
		t.Errorf("expected tool ID to be populated")
	}

	fetched, err := repo.GetByID(ctx, tenantID, tool.ID)
	if err != nil {
		t.Fatalf("failed to get tool by ID: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected tool to be found, got nil")
	}

	if fetched.Name != "ArgoCD" {
		t.Errorf("Name mismatch: got %q, want ArgoCD", fetched.Name)
	}
	if fetched.Category != ecosystem.CategoryGitOps {
		t.Errorf("Category mismatch: got %q, want %q", fetched.Category, ecosystem.CategoryGitOps)
	}
	if fetched.Version != "v2.10.2" {
		t.Errorf("Version mismatch: got %q, want v2.10.2", fetched.Version)
	}
	if fetched.Health != ecosystem.HealthHealthy {
		t.Errorf("Health mismatch: got %q, want %q", fetched.Health, ecosystem.HealthHealthy)
	}
	if fetched.Metadata["cluster"] != "prod-cluster" {
		t.Errorf("Metadata cluster mismatch: got %q, want prod-cluster", fetched.Metadata["cluster"])
	}

	// Non-existent ID returns nil, nil
	missing, err := repo.GetByID(ctx, tenantID, "non-existent-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil for non-existent ID, got %+v", missing)
	}
}

func TestEcosystem_BulkUpsertAndGetAll(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	repo := postgres.NewEcosystemRepo(pool)
	tenantID := fmt.Sprintf("tenant-eco-bulk-%d", time.Now().UnixNano())
	defer cleanupEcosystem(ctx, pool, tenantID)

	tools := []ecosystem.DetectedTool{
		{
			Name:        "Trivy",
			Category:    ecosystem.CategorySecurity,
			Status:      ecosystem.StatusDetected,
			Version:     "v0.49.0",
			Endpoint:    "http://trivy.security:4954",
			Source:      ecosystem.SourceSettings,
			Health:      ecosystem.HealthHealthy,
			LastChecked: time.Now().UTC(),
			TenantID:    tenantID,
		},
		{
			Name:        "Vault",
			Category:    ecosystem.CategorySecrets,
			Status:      ecosystem.StatusDetected,
			Version:     "1.15.2",
			Endpoint:    "https://vault.corp:8200",
			Source:      ecosystem.SourceSettings,
			Health:      ecosystem.HealthHealthy,
			LastChecked: time.Now().UTC(),
			TenantID:    tenantID,
		},
	}

	if err := repo.BulkUpsert(ctx, tools); err != nil {
		t.Fatalf("BulkUpsert failed: %v", err)
	}

	all, err := repo.GetAll(ctx, tenantID)
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(all))
	}

	// Update existing tool via BulkUpsert
	tools[0].Health = ecosystem.HealthDegraded
	tools[0].Status = ecosystem.StatusUnreachable
	if err := repo.BulkUpsert(ctx, tools); err != nil {
		t.Fatalf("second BulkUpsert failed: %v", err)
	}

	allAfter, err := repo.GetAll(ctx, tenantID)
	if err != nil {
		t.Fatalf("GetAll after update failed: %v", err)
	}
	if len(allAfter) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(allAfter))
	}

	var trivy *ecosystem.DetectedTool
	for _, tool := range allAfter {
		if tool.Name == "Trivy" {
			trivy = &tool
			break
		}
	}
	if trivy == nil {
		t.Fatalf("Trivy not found")
	}
	if trivy.Health != ecosystem.HealthDegraded {
		t.Errorf("expected Trivy health degraded, got %q", trivy.Health)
	}
}

func TestEcosystem_Delete(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	repo := postgres.NewEcosystemRepo(pool)
	tenantID := fmt.Sprintf("tenant-eco-del-%d", time.Now().UnixNano())
	defer cleanupEcosystem(ctx, pool, tenantID)

	tool := &ecosystem.DetectedTool{
		Name:     "Grafana",
		Category: ecosystem.CategoryMonitoring,
		Status:   ecosystem.StatusDetected,
		TenantID: tenantID,
	}

	if err := repo.Create(ctx, tool); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := repo.Delete(ctx, tenantID, tool.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	fetched, err := repo.GetByID(ctx, tenantID, tool.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if fetched != nil {
		t.Fatalf("expected tool to be deleted, got %+v", fetched)
	}

	// Delete non-existent returns error
	if err := repo.Delete(ctx, tenantID, tool.ID); err == nil {
		t.Fatalf("expected error deleting non-existent tool, got nil")
	}
}

func TestEcosystem_GetSummary(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	repo := postgres.NewEcosystemRepo(pool)
	tenantID := fmt.Sprintf("tenant-eco-sum-%d", time.Now().UnixNano())
	defer cleanupEcosystem(ctx, pool, tenantID)

	tools := []ecosystem.DetectedTool{
		{Name: "ArgoCD", Category: ecosystem.CategoryGitOps, Health: ecosystem.HealthHealthy, Status: ecosystem.StatusDetected, TenantID: tenantID},
		{Name: "Trivy", Category: ecosystem.CategorySecurity, Health: ecosystem.HealthHealthy, Status: ecosystem.StatusDetected, TenantID: tenantID},
		{Name: "Vault", Category: ecosystem.CategorySecrets, Health: ecosystem.HealthDegraded, Status: ecosystem.StatusDetected, TenantID: tenantID},
		{Name: "Prometheus", Category: ecosystem.CategoryMonitoring, Health: ecosystem.HealthHealthy, Status: ecosystem.StatusDetected, TenantID: tenantID},
	}

	if err := repo.BulkUpsert(ctx, tools); err != nil {
		t.Fatalf("BulkUpsert failed: %v", err)
	}

	summary, err := repo.GetSummary(ctx, tenantID)
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}

	if summary.Total != 4 {
		t.Errorf("expected Total 4, got %d", summary.Total)
	}
	if summary.Healthy != 3 {
		t.Errorf("expected Healthy 3, got %d", summary.Healthy)
	}
	if summary.Degraded != 1 {
		t.Errorf("expected Degraded 1, got %d", summary.Degraded)
	}
	if summary.ByCategory[ecosystem.CategoryGitOps] != 1 {
		t.Errorf("expected CategoryGitOps 1, got %d", summary.ByCategory[ecosystem.CategoryGitOps])
	}
	if summary.ByCategory[ecosystem.CategoryMonitoring] != 1 {
		t.Errorf("expected CategoryMonitoring 1, got %d", summary.ByCategory[ecosystem.CategoryMonitoring])
	}
}

func TestEcosystem_TenantIsolation(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	repo := postgres.NewEcosystemRepo(pool)
	tenantA := fmt.Sprintf("tenant-eco-a-%d", time.Now().UnixNano())
	tenantB := fmt.Sprintf("tenant-eco-b-%d", time.Now().UnixNano())
	defer cleanupEcosystem(ctx, pool, tenantA)
	defer cleanupEcosystem(ctx, pool, tenantB)

	toolA := &ecosystem.DetectedTool{
		Name:     "ArgoCD",
		Category: ecosystem.CategoryGitOps,
		TenantID: tenantA,
	}
	toolB := &ecosystem.DetectedTool{
		Name:     "ArgoCD",
		Category: ecosystem.CategoryGitOps,
		TenantID: tenantB,
	}

	if err := repo.Create(ctx, toolA); err != nil {
		t.Fatalf("Create toolA failed: %v", err)
	}
	if err := repo.Create(ctx, toolB); err != nil {
		t.Fatalf("Create toolB failed: %v", err)
	}

	// Tool from tenantA cannot be retrieved by tenantB
	fetched, err := repo.GetByID(ctx, tenantB, toolA.ID)
	if err != nil {
		t.Fatalf("GetByID unexpected error: %v", err)
	}
	if fetched != nil {
		t.Errorf("tenantB should not be able to fetch tenantA's tool: %+v", fetched)
	}

	// GetAll isolation
	allA, err := repo.GetAll(ctx, tenantA)
	if err != nil {
		t.Fatalf("GetAll tenantA failed: %v", err)
	}
	if len(allA) != 1 || allA[0].ID != toolA.ID {
		t.Errorf("expected only toolA for tenantA, got %+v", allA)
	}
}
