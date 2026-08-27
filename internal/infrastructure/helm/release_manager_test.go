package helm

import (
	"context"
	"os"
	"path/filepath"
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
