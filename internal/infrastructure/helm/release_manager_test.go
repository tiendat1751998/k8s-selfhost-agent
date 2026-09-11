package helm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"helm.sh/helm/v3/pkg/repo"
)

func TestNewReleaseManager(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewReleaseManager(nil, nil, tempDir)
	if rm == nil {
		t.Fatal("expected non-nil ReleaseManager")
	}

	repos, err := rm.ListRepos()
	if err != nil {
		t.Fatalf("expected no error listing repos, got: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("expected 0 repos, got: %d", len(repos))
	}
}

func TestRepoManagement(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewReleaseManager(nil, nil, tempDir)

	// Test adding empty repo name/url
	if err := rm.AddRepo("", "https://example.com"); err == nil {
		t.Error("expected error adding empty repo name")
	}
	if err := rm.AddRepo("test", ""); err == nil {
		t.Error("expected error adding empty repo url")
	}

	// Create a mock repo index file in cache to test search
	repoFile := filepath.Join(tempDir, "repository", "repositories.yaml")
	rf := repo.NewFile()
	rf.Add(&repo.Entry{
		Name: "bitnami",
		URL:  "https://charts.bitnami.com/bitnami",
	})
	if err := rf.WriteFile(repoFile, 0644); err != nil {
		t.Fatalf("failed to write mock repo file: %v", err)
	}

	mockIndexYAML := `apiVersion: v1
entries:
  nginx:
  - apiVersion: v2
    appVersion: 1.25.3
    description: Bitnami NGINX Chart
    name: nginx
    version: 15.4.4
  redis:
  - apiVersion: v2
    appVersion: 7.2.3
    description: Bitnami Redis Chart
    name: redis
    version: 18.6.1
`
	indexPath := filepath.Join(tempDir, "cache", "bitnami-index.yaml")
	if err := os.WriteFile(indexPath, []byte(mockIndexYAML), 0644); err != nil {
		t.Fatalf("failed to write mock index yaml: %v", err)
	}

	// Test ListRepos
	repos, err := rm.ListRepos()
	if err != nil {
		t.Fatalf("failed to list repos: %v", err)
	}
	if len(repos) != 1 || repos[0].Name != "bitnami" {
		t.Fatalf("unexpected repos list: %+v", repos)
	}

	// Test SearchCharts
	results, err := rm.SearchCharts("nginx")
	if err != nil {
		t.Fatalf("failed to search charts: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 search result, got: %d", len(results))
	}
	if results[0].Name != "bitnami/nginx" {
		t.Fatalf("expected chart name bitnami/nginx, got: %s", results[0].Name)
	}

	// Search non-existing chart
	results, err = rm.SearchCharts("postgres")
	if err != nil {
		t.Fatalf("failed to search charts: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got: %d", len(results))
	}
}

func TestReleaseManager_ClusterErrors(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewReleaseManager(nil, nil, tempDir)
	ctx := context.Background()

	// ListReleases on non-connected cluster
	_, err := rm.ListReleases(ctx, "non-existent-cluster", "default")
	if err == nil {
		t.Error("expected error for non-existent cluster")
	}

	// GetRelease on non-connected cluster
	_, err = rm.GetRelease(ctx, "non-existent-cluster", "my-release", "default")
	if err == nil {
		t.Error("expected error for non-existent cluster")
	}

	// Empty release name
	_, err = rm.GetRelease(ctx, "non-existent-cluster", "", "default")
	if err == nil {
		t.Error("expected error for empty release name")
	}

	// InstallRelease validations
	_, err = rm.InstallRelease(ctx, "c1", InstallRequest{Chart: "nginx"})
	if err == nil {
		t.Error("expected error for empty release name")
	}
	_, err = rm.InstallRelease(ctx, "c1", InstallRequest{ReleaseName: "rel"})
	if err == nil {
		t.Error("expected error for empty chart name")
	}

	// UpgradeRelease validations
	_, err = rm.UpgradeRelease(ctx, "c1", UpgradeRequest{})
	if err == nil {
		t.Error("expected error for empty release name")
	}

	// RollbackRelease validations
	err = rm.RollbackRelease(ctx, "c1", "", "default", 1)
	if err == nil {
		t.Error("expected error for empty release name")
	}

	// UninstallRelease validations
	err = rm.UninstallRelease(ctx, "c1", "", "default")
	if err == nil {
		t.Error("expected error for empty release name")
	}

	// GetReleaseHistory validations
	_, err = rm.GetReleaseHistory(ctx, "c1", "", "default")
	if err == nil {
		t.Error("expected error for empty release name")
	}
}

func TestReleaseManager_GetChartValuesErrors(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewReleaseManager(nil, nil, tempDir)

	// Empty chart name
	_, err := rm.GetChartValues("bitnami", "", "1.0.0")
	if err == nil {
		t.Error("expected error for empty chart name")
	}

	// Non-existent chart
	_, err = rm.GetChartValues("bitnami", "non-existent-chart", "1.0.0")
	if err == nil {
		t.Error("expected error for non-existent chart")
	}
}

func TestReleaseManager_Concurrency(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewReleaseManager(nil, nil, tempDir)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, _ = rm.ListRepos()
			_, _ = rm.SearchCharts("test")
		}(i)
	}
	wg.Wait()
}


func TestSearchCharts_Caching(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewReleaseManager(nil, nil, tempDir)

	repoFile := filepath.Join(tempDir, "repository", "repositories.yaml")
	rf := repo.NewFile()
	rf.Add(&repo.Entry{
		Name: "bitnami",
		URL:  "https://charts.bitnami.com/bitnami",
	})
	if err := rf.WriteFile(repoFile, 0644); err != nil {
		t.Fatalf("failed to write mock repo file: %v", err)
	}

	mockIndexYAML := `apiVersion: v1
entries:
  nginx:
  - apiVersion: v2
    appVersion: 1.25.3
    description: Bitnami NGINX Chart
    name: nginx
    version: 15.4.4
`
	indexPath := filepath.Join(tempDir, "cache", "bitnami-index.yaml")
	if err := os.WriteFile(indexPath, []byte(mockIndexYAML), 0644); err != nil {
		t.Fatalf("failed to write mock index yaml: %v", err)
	}

	// Cache must be initially nil
	rm.mu.RLock()
	if rm.cachedIndex != nil {
		t.Fatal("expected cachedIndex to be initially nil")
	}
	rm.mu.RUnlock()

	// First search populates cache
	results, err := rm.SearchCharts("nginx")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	rm.mu.RLock()
	if rm.cachedIndex == nil {
		t.Fatal("expected cachedIndex to be populated after search")
	}
	if rm.cachedIndexTime.IsZero() {
		t.Fatal("expected cachedIndexTime to be set")
	}
	rm.mu.RUnlock()

	// Delete index file from disk: in-memory cache should still answer
	if err := os.Remove(indexPath); err != nil {
		t.Fatalf("failed to remove index file: %v", err)
	}

	results2, err := rm.SearchCharts("nginx")
	if err != nil {
		t.Fatalf("cached search failed after file removal: %v", err)
	}
	if len(results2) != 1 || results2[0].Name != "bitnami/nginx" {
		t.Fatalf("expected cached search result bitnami/nginx, got %+v", results2)
	}
}

func TestSearchCharts_CacheInvalidation(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewReleaseManager(nil, nil, tempDir)

	repoFile := filepath.Join(tempDir, "repository", "repositories.yaml")
	rf := repo.NewFile()
	rf.Add(&repo.Entry{
		Name: "testrepo",
		URL:  "https://example.com/charts",
	})
	if err := rf.WriteFile(repoFile, 0644); err != nil {
		t.Fatalf("failed to write repo file: %v", err)
	}

	mockIndexYAML := `apiVersion: v1
entries:
  app:
  - apiVersion: v2
    name: app
    version: 1.0.0
`
	indexPath := filepath.Join(tempDir, "cache", "testrepo-index.yaml")
	if err := os.WriteFile(indexPath, []byte(mockIndexYAML), 0644); err != nil {
		t.Fatalf("failed to write mock index yaml: %v", err)
	}

	// Populate cache
	_, err := rm.SearchCharts("app")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	rm.mu.RLock()
	if rm.cachedIndex == nil {
		t.Fatal("expected cachedIndex to be set")
	}
	rm.mu.RUnlock()

	// UpdateRepos invalidates cache
	_ = rm.UpdateRepos()
	rm.mu.RLock()
	if rm.cachedIndex != nil {
		t.Fatal("expected cachedIndex to be nil after UpdateRepos")
	}
	rm.mu.RUnlock()

	// Re-populate cache
	_, _ = rm.SearchCharts("app")

	// RemoveRepo invalidates cache
	if err := rm.RemoveRepo("testrepo"); err != nil {
		t.Fatalf("failed to remove repo: %v", err)
	}

	rm.mu.RLock()
	if rm.cachedIndex != nil {
		t.Fatal("expected cachedIndex to be nil after RemoveRepo")
	}
	rm.mu.RUnlock()

	repos, err := rm.ListRepos()
	if err != nil {
		t.Fatalf("failed to list repos: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("expected 0 repos after RemoveRepo, got %d", len(repos))
	}
}

func TestSearchCharts_ResultLimiting(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewReleaseManager(nil, nil, tempDir)

	repoFile := filepath.Join(tempDir, "repository", "repositories.yaml")
	rf := repo.NewFile()
	rf.Add(&repo.Entry{
		Name: "megarepo",
		URL:  "https://example.com/megarepo",
	})
	if err := rf.WriteFile(repoFile, 0644); err != nil {
		t.Fatalf("failed to write repo file: %v", err)
	}

	// Generate 150 charts in index YAML
	var sb strings.Builder
	sb.WriteString("apiVersion: v1\nentries:\n")
	for i := 1; i <= 150; i++ {
		sb.WriteString(fmt.Sprintf("  chart-%03d:\n  - apiVersion: v2\n    name: chart-%03d\n    version: 1.0.0\n    description: Description %d\n", i, i, i))
	}

	indexPath := filepath.Join(tempDir, "cache", "megarepo-index.yaml")
	if err := os.WriteFile(indexPath, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write mock index yaml: %v", err)
	}

	// Empty keyword (browsing) must return at most 120 charts
	results, err := rm.SearchCharts("")
	if err != nil {
		t.Fatalf("search with empty keyword failed: %v", err)
	}
	if len(results) != 120 {
		t.Fatalf("expected exactly 120 results on empty keyword, got %d", len(results))
	}

	// Keyword matching all 150 charts must also be capped at 120
	resultsMatch, err := rm.SearchCharts("chart")
	if err != nil {
		t.Fatalf("search with keyword 'chart' failed: %v", err)
	}
	if len(resultsMatch) != 120 {
		t.Fatalf("expected exactly 120 results on keyword matching 150 charts, got %d", len(resultsMatch))
	}
}
