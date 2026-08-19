package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/plugin"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
)

func cleanupPlugin(ctx context.Context, pool postgres.DBTX, tenantID string) {
	_, _ = pool.Exec(ctx, "DELETE FROM plugins WHERE tenant_id = $1", tenantID)
}

func TestPlugin_CreateAndGetByID(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("tenant-plugin-rt-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewPluginRepo(pool)
	defer cleanupPlugin(ctx, pool, tenantID)

	p := &plugin.Plugin{
		Name:        "grafana-live-dashboards",
		Version:     "1.2.0",
		Description: "Embeds Grafana runtime panels in workload views",
		Author:      "DevOps Team",
		Enabled:     true,
		EntryPoint:  "https://cdn.example.com/plugins/grafana-live/bundle.js",
		Icon:        "📊",
		Category:    plugin.CategoryMonitoring,
		Permissions: []string{"metrics:read", "k8s:read"},
		Config: map[string]string{
			"grafana_url": "https://grafana.internal:3000",
			"refresh_sec": "15",
		},
		TenantID: tenantID,
	}

	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("failed to create plugin: %v", err)
	}

	if p.ID == "" {
		t.Errorf("expected plugin ID to be generated")
	}
	if p.CreatedAt.IsZero() {
		t.Errorf("expected plugin CreatedAt to be set")
	}
	if p.UpdatedAt.IsZero() {
		t.Errorf("expected plugin UpdatedAt to be set")
	}

	fetched, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("failed to get plugin by ID: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected plugin to be found, got nil")
	}

	if fetched.ID != p.ID {
		t.Errorf("ID mismatch: got %q, want %q", fetched.ID, p.ID)
	}
	if fetched.Name != p.Name {
		t.Errorf("Name mismatch: got %q, want %q", fetched.Name, p.Name)
	}
	if fetched.Version != "1.2.0" {
		t.Errorf("Version mismatch: got %q, want 1.2.0", fetched.Version)
	}
	if fetched.Category != plugin.CategoryMonitoring {
		t.Errorf("Category mismatch: got %q, want %q", fetched.Category, plugin.CategoryMonitoring)
	}
	if fetched.EntryPoint != p.EntryPoint {
		t.Errorf("EntryPoint mismatch: got %q, want %q", fetched.EntryPoint, p.EntryPoint)
	}
	if !fetched.Enabled {
		t.Errorf("Enabled mismatch: got false, want true")
	}
	if len(fetched.Permissions) != 2 || fetched.Permissions[0] != "metrics:read" {
		t.Errorf("Permissions mismatch: got %v", fetched.Permissions)
	}
	if fetched.Config["grafana_url"] != "https://grafana.internal:3000" {
		t.Errorf("Config mismatch: got %v", fetched.Config)
	}
}

func TestPlugin_List_EnabledFilter(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("tenant-plugin-list-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewPluginRepo(pool)
	defer cleanupPlugin(ctx, pool, tenantID)

	p1 := &plugin.Plugin{
		Name:     "tool-one",
		Category: plugin.CategoryDevtools,
		Enabled:  true,
		TenantID: tenantID,
	}
	p2 := &plugin.Plugin{
		Name:     "tool-two",
		Category: plugin.CategorySecurity,
		Enabled:  false,
		TenantID: tenantID,
	}

	if err := repo.Create(ctx, p1); err != nil {
		t.Fatalf("failed to create p1: %v", err)
	}
	if err := repo.Create(ctx, p2); err != nil {
		t.Fatalf("failed to create p2: %v", err)
	}

	all, err := repo.List(ctx, tenantID, false)
	if err != nil {
		t.Fatalf("failed to list all plugins: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(all))
	}

	enabledOnly, err := repo.List(ctx, tenantID, true)
	if err != nil {
		t.Fatalf("failed to list enabled plugins: %v", err)
	}
	if len(enabledOnly) != 1 {
		t.Errorf("expected 1 enabled plugin, got %d", len(enabledOnly))
	}
	if enabledOnly[0].Name != "tool-one" {
		t.Errorf("expected tool-one, got %q", enabledOnly[0].Name)
	}
}

func TestPlugin_Update(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("tenant-plugin-upd-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewPluginRepo(pool)
	defer cleanupPlugin(ctx, pool, tenantID)

	p := &plugin.Plugin{
		Name:        "trivy-scanner-ui",
		Version:     "1.0.0",
		Category:    plugin.CategorySecurity,
		Enabled:     true,
		Description: "Initial version",
		TenantID:    tenantID,
	}

	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("failed to create plugin: %v", err)
	}

	p.Version = "2.0.0"
	p.Description = "Upgraded to v2 with deep CVE inspections"
	p.Category = plugin.CategorySecurity
	p.Config = map[string]string{"timeout": "30s"}

	if err := repo.Update(ctx, p); err != nil {
		t.Fatalf("failed to update plugin: %v", err)
	}

	fetched, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("failed to get updated plugin: %v", err)
	}
	if fetched.Version != "2.0.0" {
		t.Errorf("expected version 2.0.0, got %q", fetched.Version)
	}
	if fetched.Description != "Upgraded to v2 with deep CVE inspections" {
		t.Errorf("expected updated description, got %q", fetched.Description)
	}
	if fetched.Config["timeout"] != "30s" {
		t.Errorf("expected config timeout 30s, got %q", fetched.Config["timeout"])
	}
}

func TestPlugin_Toggle(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("tenant-plugin-tgl-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewPluginRepo(pool)
	defer cleanupPlugin(ctx, pool, tenantID)

	p := &plugin.Plugin{
		Name:     "toggle-target",
		Category: plugin.CategoryDevtools,
		Enabled:  true,
		TenantID: tenantID,
	}

	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("failed to create plugin: %v", err)
	}

	// Disable
	if err := repo.Toggle(ctx, p.ID, false); err != nil {
		t.Fatalf("failed to toggle plugin off: %v", err)
	}

	fetched, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("failed to get plugin: %v", err)
	}
	if fetched.Enabled != false {
		t.Errorf("expected Enabled to be false, got true")
	}

	// Enable
	if err := repo.Toggle(ctx, p.ID, true); err != nil {
		t.Fatalf("failed to toggle plugin on: %v", err)
	}

	fetched, err = repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("failed to get plugin: %v", err)
	}
	if fetched.Enabled != true {
		t.Errorf("expected Enabled to be true, got false")
	}
}

func TestPlugin_Delete(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("tenant-plugin-del-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewPluginRepo(pool)
	defer cleanupPlugin(ctx, pool, tenantID)

	p := &plugin.Plugin{
		Name:     "delete-target",
		Category: plugin.CategoryDevtools,
		TenantID: tenantID,
	}

	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("failed to create plugin: %v", err)
	}

	if err := repo.Delete(ctx, p.ID); err != nil {
		t.Fatalf("failed to delete plugin: %v", err)
	}

	fetched, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("unexpected error getting deleted plugin: %v", err)
	}
	if fetched != nil {
		t.Errorf("expected plugin to be nil after deletion, got %+v", fetched)
	}
}

func TestPlugin_DuplicateName(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("tenant-plugin-dup-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewPluginRepo(pool)
	defer cleanupPlugin(ctx, pool, tenantID)

	p1 := &plugin.Plugin{
		Name:     "unique-plugin",
		Category: plugin.CategoryDevtools,
		TenantID: tenantID,
	}
	if err := repo.Create(ctx, p1); err != nil {
		t.Fatalf("failed to create initial plugin: %v", err)
	}

	p2 := &plugin.Plugin{
		Name:     "unique-plugin",
		Category: plugin.CategoryMonitoring,
		TenantID: tenantID,
	}
	err := repo.Create(ctx, p2)
	if err == nil {
		t.Fatalf("expected error creating duplicate plugin name, got nil")
	}
	if err != plugin.ErrDuplicateName {
		t.Errorf("expected ErrDuplicateName, got %v", err)
	}
}

func TestPlugin_Stats(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("tenant-plugin-stats-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewPluginRepo(pool)
	defer cleanupPlugin(ctx, pool, tenantID)

	_ = repo.Create(ctx, &plugin.Plugin{Name: "p1", Category: plugin.CategoryMonitoring, Enabled: true, TenantID: tenantID})
	_ = repo.Create(ctx, &plugin.Plugin{Name: "p2", Category: plugin.CategoryMonitoring, Enabled: false, TenantID: tenantID})
	_ = repo.Create(ctx, &plugin.Plugin{Name: "p3", Category: plugin.CategorySecurity, Enabled: true, TenantID: tenantID})
	_ = repo.Create(ctx, &plugin.Plugin{Name: "p4", Category: plugin.CategoryIntegration, Enabled: true, TenantID: tenantID})

	stats, err := repo.GetStats(ctx, tenantID)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if stats.Total != 4 {
		t.Errorf("expected total 4, got %d", stats.Total)
	}
	if stats.Enabled != 3 {
		t.Errorf("expected enabled 3, got %d", stats.Enabled)
	}
	if stats.Disabled != 1 {
		t.Errorf("expected disabled 1, got %d", stats.Disabled)
	}
	if stats.ByCategory[plugin.CategoryMonitoring] != 2 {
		t.Errorf("expected 2 monitoring plugins, got %d", stats.ByCategory[plugin.CategoryMonitoring])
	}
}

func TestPlugin_TenantIsolation(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantA := fmt.Sprintf("tenant-plugin-a-%d", time.Now().UnixNano())
	tenantB := fmt.Sprintf("tenant-plugin-b-%d", time.Now().UnixNano())
	ctxA := tenantContext(tenantA)
	ctxB := tenantContext(tenantB)
	repo := postgres.NewPluginRepo(pool)
	defer cleanupPlugin(ctxA, pool, tenantA)
	defer cleanupPlugin(ctxB, pool, tenantB)

	pA := &plugin.Plugin{Name: "plugin-isolated", Category: plugin.CategoryDevtools, TenantID: tenantA}
	if err := repo.Create(ctxA, pA); err != nil {
		t.Fatalf("failed to create plugin in tenant A: %v", err)
	}

	// Try to get from tenant B context
	fetched, err := repo.GetByID(ctxB, pA.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fetched != nil {
		t.Errorf("expected tenant B cannot access tenant A plugin, but got: %+v", fetched)
	}
}
