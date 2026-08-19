package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/scaffold"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
)

func cleanupScaffold(ctx context.Context, pool postgres.DBTX, tenantID string) {
	_, _ = pool.Exec(ctx, "DELETE FROM scaffold_templates WHERE tenant_id = $1", tenantID)
}

func TestScaffold_BuiltinsAvailable(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	repo := postgres.NewScaffoldRepo(pool)
	ctx := context.Background()

	// 1. GetByID on built-in Go API template
	goTmpl, err := repo.GetByID(ctx, scaffold.BuiltinIDGoAPI)
	if err != nil {
		t.Fatalf("failed to get built-in Go API template: %v", err)
	}
	if goTmpl == nil {
		t.Fatalf("expected built-in Go API template, got nil")
	}
	if goTmpl.Category != scaffold.CategoryAPI || goTmpl.Framework != scaffold.FrameworkGoChi {
		t.Errorf("unexpected category/framework: %s / %s", goTmpl.Category, goTmpl.Framework)
	}

	// 2. List templates should contain all 3 built-ins
	list, err := repo.List(ctx, "default-tenant", scaffold.ListFilter{})
	if err != nil {
		t.Fatalf("failed to list templates: %v", err)
	}
	if len(list) < 3 {
		t.Fatalf("expected at least 3 templates in list, got %d", len(list))
	}

	// 3. Category filter
	apiList, err := repo.List(ctx, "default-tenant", scaffold.ListFilter{Category: scaffold.CategoryAPI})
	if err != nil {
		t.Fatalf("failed to list api templates: %v", err)
	}
	for _, item := range apiList {
		if item.Category != scaffold.CategoryAPI {
			t.Errorf("expected category 'api', got %q", item.Category)
		}
	}
}

func TestScaffold_CreateGetUpdateDelete(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("tenant-scaffold-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewScaffoldRepo(pool)
	defer cleanupScaffold(ctx, pool, tenantID)

	custom := &scaffold.Template{
		Name:        "Python FastAPI Service",
		Description: "FastAPI REST API with Swagger and Pydantic",
		Category:    scaffold.CategoryAPI,
		Framework:   scaffold.FrameworkPythonFastAPI,
		ManifestYAML: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: {{.app_name}}",
		HelmValues:   "nameOverride: {{.app_name}}",
		DockerCompose: "version: '3.8'\nservices:\n  {{.app_name}}:\n    image: python:3.11",
		Variables: []scaffold.TemplateVariable{
			{Name: "app_name", Label: "Application Name", Type: "string", Default: "fastapi-app", Required: true},
			{Name: "port", Label: "Port", Type: "number", Default: "8000", Required: true},
		},
		Tags:     []string{"python", "fastapi", "rest"},
		TenantID: tenantID,
	}

	// Create
	if err := repo.Create(ctx, custom); err != nil {
		t.Fatalf("failed to create custom template: %v", err)
	}
	if custom.ID == "" {
		t.Errorf("expected template ID to be populated")
	}

	// GetByID
	fetched, err := repo.GetByID(ctx, custom.ID)
	if err != nil {
		t.Fatalf("failed to get template: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected template to be found, got nil")
	}
	if fetched.Name != "Python FastAPI Service" {
		t.Errorf("expected name 'Python FastAPI Service', got %q", fetched.Name)
	}
	if len(fetched.Variables) != 2 {
		t.Errorf("expected 2 variables, got %d", len(fetched.Variables))
	}
	if len(fetched.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(fetched.Tags))
	}

	// Update
	fetched.Description = "Updated FastAPI description"
	fetched.Variables = append(fetched.Variables, scaffold.TemplateVariable{
		Name: "workers", Label: "Uvicorn Workers", Type: "number", Default: "4",
	})
	if err := repo.Update(ctx, fetched); err != nil {
		t.Fatalf("failed to update template: %v", err)
	}

	updated, err := repo.GetByID(ctx, custom.ID)
	if err != nil || updated == nil {
		t.Fatalf("failed to fetch updated template: %v", err)
	}
	if updated.Description != "Updated FastAPI description" {
		t.Errorf("updated description mismatch: %q", updated.Description)
	}
	if len(updated.Variables) != 3 {
		t.Errorf("expected 3 variables after update, got %d", len(updated.Variables))
	}

	// Search filter
	searchList, err := repo.List(ctx, tenantID, scaffold.ListFilter{Search: "FastAPI"})
	if err != nil {
		t.Fatalf("failed to list with search: %v", err)
	}
	found := false
	for _, item := range searchList {
		if item.ID == custom.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected custom template to be found in search results")
	}

	// Delete
	if err := repo.Delete(ctx, custom.ID); err != nil {
		t.Fatalf("failed to delete template: %v", err)
	}

	deleted, err := repo.GetByID(ctx, custom.ID)
	if err != nil {
		t.Fatalf("unexpected error after delete: %v", err)
	}
	if deleted != nil {
		t.Errorf("expected template to be nil after deletion, got %+v", deleted)
	}
}

func TestScaffold_PreventBuiltinModification(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	repo := postgres.NewScaffoldRepo(pool)
	ctx := context.Background()

	// Attempt to update built-in
	goTmpl, err := repo.GetByID(ctx, scaffold.BuiltinIDGoAPI)
	if err != nil || goTmpl == nil {
		t.Fatalf("failed to get built-in template: %v", err)
	}

	goTmpl.Name = "Hacked Template"
	if err := repo.Update(ctx, goTmpl); err == nil {
		t.Errorf("expected error when attempting to update built-in template, got nil")
	}

	// Attempt to delete built-in
	if err := repo.Delete(ctx, scaffold.BuiltinIDGoAPI); err == nil {
		t.Errorf("expected error when attempting to delete built-in template, got nil")
	}
}
