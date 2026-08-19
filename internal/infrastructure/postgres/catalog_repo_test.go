package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/catalog"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
)

func cleanupCatalog(ctx context.Context, pool postgres.DBTX, tenantID string) {
	_, _ = pool.Exec(ctx, "DELETE FROM service_catalog WHERE tenant_id = $1", tenantID)
}

func TestCatalog_CreateAndGetByID(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("tenant-catalog-rt-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewCatalogRepo(pool)
	defer cleanupCatalog(ctx, pool, tenantID)

	entry := &catalog.ServiceEntry{
		Name:        "payment-gateway",
		Description: "Core payment processing service",
		Type:        catalog.TypeService,
		Lifecycle:   catalog.LifecycleProduction,
		OwnerTeam:   "payments-team",
		OwnerEmail:  "payments@example.com",
		RepoURL:     "https://github.com/org/payment-gateway",
		DocsURL:     "https://docs.example.com/payment-gateway",
		Tags:        []string{"payments", "stripe", "pci-dss"},
		Annotations: map[string]string{
			"k8s.io/namespace":  "payments",
			"k8s.io/deployment": "payment-api",
		},
		TenantID: tenantID,
	}

	if err := repo.Create(ctx, entry); err != nil {
		t.Fatalf("failed to create service entry: %v", err)
	}

	if entry.ID == "" {
		t.Errorf("expected entry ID to be generated")
	}
	if entry.CreatedAt.IsZero() {
		t.Errorf("expected entry CreatedAt to be set")
	}
	if entry.UpdatedAt.IsZero() {
		t.Errorf("expected entry UpdatedAt to be set")
	}

	fetched, err := repo.GetByID(ctx, entry.ID)
	if err != nil {
		t.Fatalf("failed to get service entry by ID: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected service entry to be found, got nil")
	}

	if fetched.ID != entry.ID {
		t.Errorf("ID mismatch: got %q, want %q", fetched.ID, entry.ID)
	}
	if fetched.Name != entry.Name {
		t.Errorf("Name mismatch: got %q, want %q", fetched.Name, entry.Name)
	}
	if fetched.Description != entry.Description {
		t.Errorf("Description mismatch: got %q, want %q", fetched.Description, entry.Description)
	}
	if fetched.Type != catalog.TypeService {
		t.Errorf("Type mismatch: got %q, want %q", fetched.Type, catalog.TypeService)
	}
	if fetched.Lifecycle != catalog.LifecycleProduction {
		t.Errorf("Lifecycle mismatch: got %q, want %q", fetched.Lifecycle, catalog.LifecycleProduction)
	}
	if fetched.OwnerTeam != entry.OwnerTeam {
		t.Errorf("OwnerTeam mismatch: got %q, want %q", fetched.OwnerTeam, entry.OwnerTeam)
	}
	if fetched.OwnerEmail != entry.OwnerEmail {
		t.Errorf("OwnerEmail mismatch: got %q, want %q", fetched.OwnerEmail, entry.OwnerEmail)
	}
	if fetched.RepoURL != entry.RepoURL {
		t.Errorf("RepoURL mismatch: got %q, want %q", fetched.RepoURL, entry.RepoURL)
	}
	if fetched.DocsURL != entry.DocsURL {
		t.Errorf("DocsURL mismatch: got %q, want %q", fetched.DocsURL, entry.DocsURL)
	}
	if len(fetched.Tags) != 3 || fetched.Tags[0] != "payments" || fetched.Tags[1] != "stripe" || fetched.Tags[2] != "pci-dss" {
		t.Errorf("Tags mismatch: got %v, want %v", fetched.Tags, entry.Tags)
	}
	if len(fetched.Annotations) != 2 || fetched.Annotations["k8s.io/namespace"] != "payments" || fetched.Annotations["k8s.io/deployment"] != "payment-api" {
		t.Errorf("Annotations mismatch: got %v, want %v", fetched.Annotations, entry.Annotations)
	}
	if fetched.TenantID != tenantID {
		t.Errorf("TenantID mismatch: got %q, want %q", fetched.TenantID, tenantID)
	}

	// Non-existent ID returns nil, nil
	missing, err := repo.GetByID(ctx, "non-existent-uuid-1234")
	if err != nil {
		t.Fatalf("unexpected error for non-existent ID: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil for non-existent ID, got %+v", missing)
	}
}

func TestCatalog_List_FilterByType(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("tenant-catalog-type-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewCatalogRepo(pool)
	defer cleanupCatalog(ctx, pool, tenantID)

	entries := []*catalog.ServiceEntry{
		{Name: "api-gw", Type: catalog.TypeAPI, Lifecycle: catalog.LifecycleProduction, TenantID: tenantID},
		{Name: "user-svc", Type: catalog.TypeService, Lifecycle: catalog.LifecycleProduction, TenantID: tenantID},
		{Name: "order-svc", Type: catalog.TypeService, Lifecycle: catalog.LifecycleStaging, TenantID: tenantID},
		{Name: "postgres-main", Type: catalog.TypeDatabase, Lifecycle: catalog.LifecycleProduction, TenantID: tenantID},
	}

	for _, e := range entries {
		if err := repo.Create(ctx, e); err != nil {
			t.Fatalf("failed to create entry %s: %v", e.Name, err)
		}
	}

	// Filter by TypeService
	services, err := repo.List(ctx, tenantID, catalog.ListFilter{Type: catalog.TypeService})
	if err != nil {
		t.Fatalf("failed to list services by type: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services of type 'service', got %d", len(services))
	}

	// Filter by TypeDatabase
	databases, err := repo.List(ctx, tenantID, catalog.ListFilter{Type: catalog.TypeDatabase})
	if err != nil {
		t.Fatalf("failed to list databases: %v", err)
	}
	if len(databases) != 1 || databases[0].Name != "postgres-main" {
		t.Fatalf("expected 1 database postgres-main, got %+v", databases)
	}
}

func TestCatalog_List_FilterByLifecycle(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("tenant-catalog-lifecycle-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewCatalogRepo(pool)
	defer cleanupCatalog(ctx, pool, tenantID)

	entries := []*catalog.ServiceEntry{
		{Name: "legacy-auth", Type: catalog.TypeService, Lifecycle: catalog.LifecycleDeprecated, TenantID: tenantID},
		{Name: "new-auth", Type: catalog.TypeService, Lifecycle: catalog.LifecycleProduction, TenantID: tenantID},
		{Name: "experimental-ai", Type: catalog.TypeWorker, Lifecycle: catalog.LifecycleDevelopment, TenantID: tenantID},
	}

	for _, e := range entries {
		if err := repo.Create(ctx, e); err != nil {
			t.Fatalf("failed to create entry %s: %v", e.Name, err)
		}
	}

	deprecated, err := repo.List(ctx, tenantID, catalog.ListFilter{Lifecycle: catalog.LifecycleDeprecated})
	if err != nil {
		t.Fatalf("failed to list deprecated services: %v", err)
	}
	if len(deprecated) != 1 || deprecated[0].Name != "legacy-auth" {
		t.Fatalf("expected 1 deprecated service legacy-auth, got %+v", deprecated)
	}

	dev, err := repo.List(ctx, tenantID, catalog.ListFilter{Lifecycle: catalog.LifecycleDevelopment})
	if err != nil {
		t.Fatalf("failed to list dev services: %v", err)
	}
	if len(dev) != 1 || dev[0].Name != "experimental-ai" {
		t.Fatalf("expected 1 development service experimental-ai, got %+v", dev)
	}
}

func TestCatalog_List_SearchByName(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("tenant-catalog-search-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewCatalogRepo(pool)
	defer cleanupCatalog(ctx, pool, tenantID)

	entries := []*catalog.ServiceEntry{
		{Name: "billing-invoice-api", Description: "Generates monthly customer invoices", Type: catalog.TypeAPI, TenantID: tenantID},
		{Name: "notification-hub", Description: "Sends push and email notifications for billing events", Type: catalog.TypeService, TenantID: tenantID},
		{Name: "analytics-collector", Description: "Collects clickstream telemetry", Type: catalog.TypeWorker, TenantID: tenantID},
	}

	for _, e := range entries {
		if err := repo.Create(ctx, e); err != nil {
			t.Fatalf("failed to create entry %s: %v", e.Name, err)
		}
	}

	// Search for "billing" should match billing-invoice-api (by name) and notification-hub (by description)
	results, err := repo.List(ctx, tenantID, catalog.ListFilter{Search: "billing"})
	if err != nil {
		t.Fatalf("failed to search services: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results matching 'billing', got %d", len(results))
	}

	// Search case-insensitively "TELEMETRY"
	resultsTelemetry, err := repo.List(ctx, tenantID, catalog.ListFilter{Search: "TELEMETRY"})
	if err != nil {
		t.Fatalf("failed to search services case-insensitively: %v", err)
	}
	if len(resultsTelemetry) != 1 || resultsTelemetry[0].Name != "analytics-collector" {
		t.Fatalf("expected analytics-collector, got %+v", resultsTelemetry)
	}
}

func TestCatalog_Update(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("tenant-catalog-upd-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewCatalogRepo(pool)
	defer cleanupCatalog(ctx, pool, tenantID)

	entry := &catalog.ServiceEntry{
		Name:        "order-processor",
		Description: "Initial description",
		Type:        catalog.TypeWorker,
		Lifecycle:   catalog.LifecycleDevelopment,
		OwnerTeam:   "dev-team",
		OwnerEmail:  "dev@example.com",
		RepoURL:     "https://github.com/org/order-processor-v1",
		DocsURL:     "https://docs.example.com/v1",
		Tags:        []string{"v1"},
		Annotations: map[string]string{"env": "dev"},
		TenantID:    tenantID,
	}

	if err := repo.Create(ctx, entry); err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	entry.Description = "Updated order processing worker with batching"
	entry.Lifecycle = catalog.LifecycleProduction
	entry.OwnerTeam = "orders-sre"
	entry.OwnerEmail = "orders-sre@example.com"
	entry.RepoURL = "https://github.com/org/order-processor-v2"
	entry.DocsURL = "https://docs.example.com/v2"
	entry.Tags = []string{"v2", "kafka", "batch"}
	entry.Annotations = map[string]string{"env": "prod", "k8s.io/namespace": "orders"}

	if err := repo.Update(ctx, entry); err != nil {
		t.Fatalf("failed to update entry: %v", err)
	}

	fetched, err := repo.GetByID(ctx, entry.ID)
	if err != nil {
		t.Fatalf("failed to get updated entry: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected updated entry to be found")
	}

	if fetched.Description != "Updated order processing worker with batching" {
		t.Errorf("Description mismatch: got %q", fetched.Description)
	}
	if fetched.Lifecycle != catalog.LifecycleProduction {
		t.Errorf("Lifecycle mismatch: got %q", fetched.Lifecycle)
	}
	if fetched.OwnerTeam != "orders-sre" {
		t.Errorf("OwnerTeam mismatch: got %q", fetched.OwnerTeam)
	}
	if fetched.RepoURL != "https://github.com/org/order-processor-v2" {
		t.Errorf("RepoURL mismatch: got %q", fetched.RepoURL)
	}
	if len(fetched.Tags) != 3 || fetched.Tags[1] != "kafka" {
		t.Errorf("Tags mismatch: got %v", fetched.Tags)
	}
	if fetched.Annotations["env"] != "prod" || fetched.Annotations["k8s.io/namespace"] != "orders" {
		t.Errorf("Annotations mismatch: got %v", fetched.Annotations)
	}
}

func TestCatalog_Delete(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("tenant-catalog-del-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewCatalogRepo(pool)
	defer cleanupCatalog(ctx, pool, tenantID)

	entry := &catalog.ServiceEntry{
		Name:      "temp-svc",
		Type:      catalog.TypeService,
		Lifecycle: catalog.LifecycleDevelopment,
		TenantID:  tenantID,
	}

	if err := repo.Create(ctx, entry); err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	if err := repo.Delete(ctx, entry.ID); err != nil {
		t.Fatalf("failed to delete entry: %v", err)
	}

	fetched, err := repo.GetByID(ctx, entry.ID)
	if err != nil {
		t.Fatalf("unexpected error after delete: %v", err)
	}
	if fetched != nil {
		t.Fatalf("expected nil after delete, got %+v", fetched)
	}

	// Deleting again should return error
	if err := repo.Delete(ctx, entry.ID); err == nil {
		t.Fatalf("expected error deleting already deleted entry, got nil")
	}
}

func TestCatalog_Stats(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("tenant-catalog-stats-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewCatalogRepo(pool)
	defer cleanupCatalog(ctx, pool, tenantID)

	entries := []*catalog.ServiceEntry{
		{Name: "svc-1", Type: catalog.TypeService, Lifecycle: catalog.LifecycleProduction, TenantID: tenantID},
		{Name: "svc-2", Type: catalog.TypeService, Lifecycle: catalog.LifecycleStaging, TenantID: tenantID},
		{Name: "api-1", Type: catalog.TypeAPI, Lifecycle: catalog.LifecycleProduction, TenantID: tenantID},
		{Name: "db-1", Type: catalog.TypeDatabase, Lifecycle: catalog.LifecycleProduction, TenantID: tenantID},
		{Name: "worker-1", Type: catalog.TypeWorker, Lifecycle: catalog.LifecycleDevelopment, TenantID: tenantID},
	}

	for _, e := range entries {
		if err := repo.Create(ctx, e); err != nil {
			t.Fatalf("failed to create entry %s: %v", e.Name, err)
		}
	}

	stats, err := repo.Stats(ctx, tenantID)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}
	if stats == nil {
		t.Fatalf("expected non-nil stats")
	}

	if stats.Total != 5 {
		t.Errorf("Total mismatch: got %d, want 5", stats.Total)
	}
	if stats.ByType[catalog.TypeService] != 2 {
		t.Errorf("ByType service mismatch: got %d, want 2", stats.ByType[catalog.TypeService])
	}
	if stats.ByType[catalog.TypeAPI] != 1 {
		t.Errorf("ByType api mismatch: got %d, want 1", stats.ByType[catalog.TypeAPI])
	}
	if stats.ByType[catalog.TypeDatabase] != 1 {
		t.Errorf("ByType database mismatch: got %d, want 1", stats.ByType[catalog.TypeDatabase])
	}
	if stats.ByType[catalog.TypeWorker] != 1 {
		t.Errorf("ByType worker mismatch: got %d, want 1", stats.ByType[catalog.TypeWorker])
	}
	if stats.ByLifecycle[catalog.LifecycleProduction] != 3 {
		t.Errorf("ByLifecycle production mismatch: got %d, want 3", stats.ByLifecycle[catalog.LifecycleProduction])
	}
	if stats.ByLifecycle[catalog.LifecycleStaging] != 1 {
		t.Errorf("ByLifecycle staging mismatch: got %d, want 1", stats.ByLifecycle[catalog.LifecycleStaging])
	}
	if stats.ByLifecycle[catalog.LifecycleDevelopment] != 1 {
		t.Errorf("ByLifecycle dev mismatch: got %d, want 1", stats.ByLifecycle[catalog.LifecycleDevelopment])
	}
}

func TestCatalog_TenantIsolation(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantA := fmt.Sprintf("tenant-catalog-a-%d", time.Now().UnixNano())
	tenantB := fmt.Sprintf("tenant-catalog-b-%d", time.Now().UnixNano())

	ctxA := tenantContext(tenantA)
	ctxB := tenantContext(tenantB)

	repo := postgres.NewCatalogRepo(pool)
	defer cleanupCatalog(ctxA, pool, tenantA)
	defer cleanupCatalog(ctxB, pool, tenantB)

	entryA := &catalog.ServiceEntry{
		Name:      "service-alpha",
		Type:      catalog.TypeService,
		Lifecycle: catalog.LifecycleProduction,
		TenantID:  tenantA,
	}
	entryB := &catalog.ServiceEntry{
		Name:      "service-beta",
		Type:      catalog.TypeService,
		Lifecycle: catalog.LifecycleProduction,
		TenantID:  tenantB,
	}

	if err := repo.Create(ctxA, entryA); err != nil {
		t.Fatalf("failed to create entryA: %v", err)
	}
	if err := repo.Create(ctxB, entryB); err != nil {
		t.Fatalf("failed to create entryB: %v", err)
	}

	// List for Tenant A should only return entryA
	listA, err := repo.List(ctxA, tenantA, catalog.ListFilter{})
	if err != nil {
		t.Fatalf("failed to list tenant A: %v", err)
	}
	if len(listA) != 1 || listA[0].Name != "service-alpha" {
		t.Errorf("unexpected list for tenant A: %+v", listA)
	}

	// List for Tenant B should only return entryB
	listB, err := repo.List(ctxB, tenantB, catalog.ListFilter{})
	if err != nil {
		t.Fatalf("failed to list tenant B: %v", err)
	}
	if len(listB) != 1 || listB[0].Name != "service-beta" {
		t.Errorf("unexpected list for tenant B: %+v", listB)
	}

	// Tenant B cannot Get Tenant A's entry
	fetchedByB, err := repo.GetByID(ctxB, entryA.ID)
	if err != nil {
		t.Fatalf("unexpected error fetching cross-tenant: %v", err)
	}
	if fetchedByB != nil {
		t.Fatalf("Tenant B was able to fetch Tenant A's service: %+v", fetchedByB)
	}
}

func TestCatalog_DuplicateName(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("tenant-catalog-dup-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewCatalogRepo(pool)
	defer cleanupCatalog(ctx, pool, tenantID)

	e1 := &catalog.ServiceEntry{
		Name:      "unique-service-name",
		Type:      catalog.TypeService,
		Lifecycle: catalog.LifecycleProduction,
		TenantID:  tenantID,
	}

	if err := repo.Create(ctx, e1); err != nil {
		t.Fatalf("failed to create first entry: %v", err)
	}

	e2 := &catalog.ServiceEntry{
		Name:      "unique-service-name",
		Type:      catalog.TypeAPI,
		Lifecycle: catalog.LifecycleDevelopment,
		TenantID:  tenantID,
	}

	if err := repo.Create(ctx, e2); err == nil {
		t.Fatalf("expected error on duplicate service name within the same tenant, got nil")
	}
}
