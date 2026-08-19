package catalog_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/catalog"
)

func TestServiceType_Constants(t *testing.T) {
	tests := []struct {
		constant string
		expected string
	}{
		{catalog.TypeService, "service"},
		{catalog.TypeAPI, "api"},
		{catalog.TypeLibrary, "library"},
		{catalog.TypeDatabase, "database"},
		{catalog.TypeFrontend, "frontend"},
		{catalog.TypeWorker, "worker"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("expected constant %q, got %q", tt.expected, tt.constant)
		}
	}
}

func TestLifecycle_Constants(t *testing.T) {
	tests := []struct {
		constant string
		expected string
	}{
		{catalog.LifecycleDevelopment, "development"},
		{catalog.LifecycleStaging, "staging"},
		{catalog.LifecycleProduction, "production"},
		{catalog.LifecycleDeprecated, "deprecated"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("expected constant %q, got %q", tt.expected, tt.constant)
		}
	}
}

func TestServiceEntry_JSONSerialization(t *testing.T) {
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	entry := &catalog.ServiceEntry{
		ID:          "svc-123",
		Name:        "auth-service",
		Description: "Authentication and authorization service",
		Type:        catalog.TypeService,
		Lifecycle:   catalog.LifecycleProduction,
		OwnerTeam:   "security-team",
		OwnerEmail:  "security@example.com",
		RepoURL:     "https://github.com/org/auth-service",
		DocsURL:     "https://docs.example.com/auth-service",
		Tags:        []string{"auth", "jwt", "oauth2"},
		Annotations: map[string]string{
			"k8s.io/namespace":  "auth",
			"k8s.io/deployment": "auth-api",
		},
		TenantID:  "tenant-abc",
		CreatedAt: now,
		UpdatedAt: now,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal ServiceEntry: %v", err)
	}

	var unmarshaled catalog.ServiceEntry
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal ServiceEntry: %v", err)
	}

	if unmarshaled.ID != entry.ID {
		t.Errorf("ID mismatch: got %q, want %q", unmarshaled.ID, entry.ID)
	}
	if unmarshaled.Name != entry.Name {
		t.Errorf("Name mismatch: got %q, want %q", unmarshaled.Name, entry.Name)
	}
	if unmarshaled.Description != entry.Description {
		t.Errorf("Description mismatch: got %q, want %q", unmarshaled.Description, entry.Description)
	}
	if unmarshaled.Type != entry.Type {
		t.Errorf("Type mismatch: got %q, want %q", unmarshaled.Type, entry.Type)
	}
	if unmarshaled.Lifecycle != entry.Lifecycle {
		t.Errorf("Lifecycle mismatch: got %q, want %q", unmarshaled.Lifecycle, entry.Lifecycle)
	}
	if unmarshaled.OwnerTeam != entry.OwnerTeam {
		t.Errorf("OwnerTeam mismatch: got %q, want %q", unmarshaled.OwnerTeam, entry.OwnerTeam)
	}
	if unmarshaled.OwnerEmail != entry.OwnerEmail {
		t.Errorf("OwnerEmail mismatch: got %q, want %q", unmarshaled.OwnerEmail, entry.OwnerEmail)
	}
	if unmarshaled.RepoURL != entry.RepoURL {
		t.Errorf("RepoURL mismatch: got %q, want %q", unmarshaled.RepoURL, entry.RepoURL)
	}
	if unmarshaled.DocsURL != entry.DocsURL {
		t.Errorf("DocsURL mismatch: got %q, want %q", unmarshaled.DocsURL, entry.DocsURL)
	}
	if len(unmarshaled.Tags) != 3 || unmarshaled.Tags[0] != "auth" || unmarshaled.Tags[1] != "jwt" || unmarshaled.Tags[2] != "oauth2" {
		t.Errorf("Tags mismatch: got %v, want %v", unmarshaled.Tags, entry.Tags)
	}
	if len(unmarshaled.Annotations) != 2 || unmarshaled.Annotations["k8s.io/namespace"] != "auth" {
		t.Errorf("Annotations mismatch: got %v, want %v", unmarshaled.Annotations, entry.Annotations)
	}
	if unmarshaled.TenantID != entry.TenantID {
		t.Errorf("TenantID mismatch: got %q, want %q", unmarshaled.TenantID, entry.TenantID)
	}
	if !unmarshaled.CreatedAt.Equal(entry.CreatedAt) {
		t.Errorf("CreatedAt mismatch: got %v, want %v", unmarshaled.CreatedAt, entry.CreatedAt)
	}
	if !unmarshaled.UpdatedAt.Equal(entry.UpdatedAt) {
		t.Errorf("UpdatedAt mismatch: got %v, want %v", unmarshaled.UpdatedAt, entry.UpdatedAt)
	}
}

func TestCatalogStats_JSONSerialization(t *testing.T) {
	stats := &catalog.CatalogStats{
		Total: 10,
		ByType: map[string]int{
			catalog.TypeService:  5,
			catalog.TypeFrontend: 3,
			catalog.TypeWorker:   2,
		},
		ByLifecycle: map[string]int{
			catalog.LifecycleProduction:  7,
			catalog.LifecycleDevelopment: 3,
		},
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("failed to marshal CatalogStats: %v", err)
	}

	var unmarshaled catalog.CatalogStats
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal CatalogStats: %v", err)
	}

	if unmarshaled.Total != stats.Total {
		t.Errorf("Total mismatch: got %d, want %d", unmarshaled.Total, stats.Total)
	}
	if unmarshaled.ByType[catalog.TypeService] != 5 {
		t.Errorf("ByType service mismatch: got %d, want 5", unmarshaled.ByType[catalog.TypeService])
	}
	if unmarshaled.ByLifecycle[catalog.LifecycleProduction] != 7 {
		t.Errorf("ByLifecycle production mismatch: got %d, want 7", unmarshaled.ByLifecycle[catalog.LifecycleProduction])
	}
}
