package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/settings"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
)

func cleanupSettings(ctx context.Context, pool postgres.DBTX, tenantID string) {
	_, _ = pool.Exec(ctx, "DELETE FROM platform_settings WHERE tenant_id = $1", tenantID)
}

func TestSettings_UpsertAndGet(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	repo := postgres.NewSettingsRepo(pool)
	tenantID := fmt.Sprintf("tenant-upsert-get-%d", time.Now().UnixNano())
	defer cleanupSettings(ctx, pool, tenantID)

	s := &settings.Setting{
		Category:  settings.CategoryPlatform,
		Key:       "timezone",
		Value:     "America/New_York",
		TenantID:  tenantID,
		UpdatedBy: "admin@example.com",
	}

	if err := repo.Upsert(ctx, s); err != nil {
		t.Fatalf("failed to upsert setting: %v", err)
	}

	if s.ID == "" {
		t.Errorf("expected setting ID to be populated after upsert")
	}
	if s.UpdatedAt.IsZero() {
		t.Errorf("expected setting UpdatedAt to be populated after upsert")
	}

	fetched, err := repo.GetByKey(ctx, tenantID, settings.CategoryPlatform, "timezone")
	if err != nil {
		t.Fatalf("failed to get setting by key: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected setting to be found, got nil")
	}

	if fetched.ID != s.ID {
		t.Errorf("ID mismatch: got %q, want %q", fetched.ID, s.ID)
	}
	if fetched.Category != settings.CategoryPlatform {
		t.Errorf("Category mismatch: got %q, want %q", fetched.Category, settings.CategoryPlatform)
	}
	if fetched.Key != "timezone" {
		t.Errorf("Key mismatch: got %q, want timezone", fetched.Key)
	}
	if fetched.Value != "America/New_York" {
		t.Errorf("Value mismatch: got %q, want America/New_York", fetched.Value)
	}
	if fetched.TenantID != tenantID {
		t.Errorf("TenantID mismatch: got %q, want %q", fetched.TenantID, tenantID)
	}
	if fetched.UpdatedBy != "admin@example.com" {
		t.Errorf("UpdatedBy mismatch: got %q, want admin@example.com", fetched.UpdatedBy)
	}

	// Non-existent key should return nil, nil
	missing, err := repo.GetByKey(ctx, tenantID, settings.CategoryPlatform, "non_existent_key")
	if err != nil {
		t.Fatalf("unexpected error querying non-existent key: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil for non-existent key, got %+v", missing)
	}
}

func TestSettings_GetByCategory(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	repo := postgres.NewSettingsRepo(pool)
	tenantID := fmt.Sprintf("tenant-by-cat-%d", time.Now().UnixNano())
	defer cleanupSettings(ctx, pool, tenantID)

	items := []settings.Setting{
		{Category: settings.CategoryPlatform, Key: "name", Value: "My Cluster", TenantID: tenantID},
		{Category: settings.CategoryPlatform, Key: "language", Value: "vi", TenantID: tenantID},
		{Category: settings.CategorySecurity, Key: "require_2fa", Value: "true", TenantID: tenantID},
	}

	for i := range items {
		if err := repo.Upsert(ctx, &items[i]); err != nil {
			t.Fatalf("failed to upsert item %d: %v", i, err)
		}
	}

	platformSettings, err := repo.GetByCategory(ctx, tenantID, settings.CategoryPlatform)
	if err != nil {
		t.Fatalf("failed to get platform settings: %v", err)
	}

	if len(platformSettings) != 2 {
		t.Fatalf("expected 2 platform settings, got %d", len(platformSettings))
	}

	keys := make(map[string]string)
	for _, s := range platformSettings {
		keys[s.Key] = s.Value
	}
	if keys["name"] != "My Cluster" {
		t.Errorf("expected name 'My Cluster', got %q", keys["name"])
	}
	if keys["language"] != "vi" {
		t.Errorf("expected language 'vi', got %q", keys["language"])
	}

	secSettings, err := repo.GetByCategory(ctx, tenantID, settings.CategorySecurity)
	if err != nil {
		t.Fatalf("failed to get security settings: %v", err)
	}
	if len(secSettings) != 1 {
		t.Fatalf("expected 1 security setting, got %d", len(secSettings))
	}
	if secSettings[0].Key != "require_2fa" || secSettings[0].Value != "true" {
		t.Errorf("unexpected security setting: %+v", secSettings[0])
	}
}

func TestSettings_BulkUpsert(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	repo := postgres.NewSettingsRepo(pool)
	tenantID := fmt.Sprintf("tenant-bulk-%d", time.Now().UnixNano())
	defer cleanupSettings(ctx, pool, tenantID)

	bulkItems := []settings.Setting{
		{Category: settings.CategoryNotifications, Key: "smtp_enabled", Value: "true", TenantID: tenantID, UpdatedBy: "user1"},
		{Category: settings.CategoryNotifications, Key: "smtp_host", Value: "smtp.mail.com", TenantID: tenantID, UpdatedBy: "user1"},
		{Category: settings.CategoryIntegrations, Key: "argocd_url", Value: "https://argocd.example.com", TenantID: tenantID, UpdatedBy: "user1"},
	}

	if err := repo.BulkUpsert(ctx, bulkItems); err != nil {
		t.Fatalf("failed to bulk upsert: %v", err)
	}

	all, err := repo.GetAll(ctx, tenantID)
	if err != nil {
		t.Fatalf("failed to get all settings: %v", err)
	}

	if len(all) != 3 {
		t.Fatalf("expected 3 settings, got %d", len(all))
	}

	// Verify each key was saved
	smtpHost, err := repo.GetByKey(ctx, tenantID, settings.CategoryNotifications, "smtp_host")
	if err != nil {
		t.Fatalf("failed to get smtp_host: %v", err)
	}
	if smtpHost == nil || smtpHost.Value != "smtp.mail.com" {
		t.Errorf("smtp_host mismatch: %+v", smtpHost)
	}

	// Bulk upsert with empty slice should be a no-op and succeed
	if err := repo.BulkUpsert(ctx, []settings.Setting{}); err != nil {
		t.Fatalf("bulk upsert of empty slice failed: %v", err)
	}
}

func TestSettings_TenantIsolation(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	repo := postgres.NewSettingsRepo(pool)
	tenantA := fmt.Sprintf("tenant-iso-a-%d", time.Now().UnixNano())
	tenantB := fmt.Sprintf("tenant-iso-b-%d", time.Now().UnixNano())
	defer cleanupSettings(ctx, pool, tenantA)
	defer cleanupSettings(ctx, pool, tenantB)

	sA := &settings.Setting{
		Category: settings.CategoryPlatform,
		Key:      "name",
		Value:    "Tenant A Cluster",
		TenantID: tenantA,
	}
	sB := &settings.Setting{
		Category: settings.CategoryPlatform,
		Key:      "name",
		Value:    "Tenant B Cluster",
		TenantID: tenantB,
	}

	if err := repo.Upsert(ctx, sA); err != nil {
		t.Fatalf("failed to upsert sA: %v", err)
	}
	if err := repo.Upsert(ctx, sB); err != nil {
		t.Fatalf("failed to upsert sB: %v", err)
	}

	// Tenant A gets Tenant A's value
	fetchedA, err := repo.GetByKey(ctx, tenantA, settings.CategoryPlatform, "name")
	if err != nil {
		t.Fatalf("failed to get sA: %v", err)
	}
	if fetchedA == nil || fetchedA.Value != "Tenant A Cluster" {
		t.Errorf("expected Tenant A Cluster, got %+v", fetchedA)
	}

	// Tenant B gets Tenant B's value
	fetchedB, err := repo.GetByKey(ctx, tenantB, settings.CategoryPlatform, "name")
	if err != nil {
		t.Fatalf("failed to get sB: %v", err)
	}
	if fetchedB == nil || fetchedB.Value != "Tenant B Cluster" {
		t.Errorf("expected Tenant B Cluster, got %+v", fetchedB)
	}

	// Tenant A listing all should only return 1 item
	allA, err := repo.GetAll(ctx, tenantA)
	if err != nil {
		t.Fatalf("failed to list all for Tenant A: %v", err)
	}
	if len(allA) != 1 || allA[0].Value != "Tenant A Cluster" {
		t.Errorf("unexpected list for tenant A: %+v", allA)
	}

	// Tenant C (non-existent) should get empty list and nil on key query
	allC, err := repo.GetAll(ctx, "non-existent-tenant")
	if err != nil {
		t.Fatalf("failed to get all for non-existent tenant: %v", err)
	}
	if len(allC) != 0 {
		t.Errorf("expected 0 settings for non-existent tenant, got %d", len(allC))
	}
}

func TestSettings_UpdateExisting(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	repo := postgres.NewSettingsRepo(pool)
	tenantID := fmt.Sprintf("tenant-update-%d", time.Now().UnixNano())
	defer cleanupSettings(ctx, pool, tenantID)

	s1 := &settings.Setting{
		Category:  settings.CategorySecurity,
		Key:       "session_timeout_minutes",
		Value:     "30",
		TenantID:  tenantID,
		UpdatedBy: "initial_admin",
	}

	if err := repo.Upsert(ctx, s1); err != nil {
		t.Fatalf("failed initial upsert: %v", err)
	}
	initialID := s1.ID

	// Sleep slightly to ensure timestamp increment
	time.Sleep(10 * time.Millisecond)

	// Update the existing setting
	s2 := &settings.Setting{
		Category:  settings.CategorySecurity,
		Key:       "session_timeout_minutes",
		Value:     "120",
		TenantID:  tenantID,
		UpdatedBy: "super_admin",
	}

	if err := repo.Upsert(ctx, s2); err != nil {
		t.Fatalf("failed second upsert: %v", err)
	}

	fetched, err := repo.GetByKey(ctx, tenantID, settings.CategorySecurity, "session_timeout_minutes")
	if err != nil {
		t.Fatalf("failed to get updated setting: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected updated setting to be found")
	}

	if fetched.Value != "120" {
		t.Errorf("Value mismatch: got %q, want 120", fetched.Value)
	}
	if fetched.UpdatedBy != "super_admin" {
		t.Errorf("UpdatedBy mismatch: got %q, want super_admin", fetched.UpdatedBy)
	}
	if fetched.ID != initialID {
		t.Errorf("expected ID to remain stable on upsert: got %q, want %q", fetched.ID, initialID)
	}
}
