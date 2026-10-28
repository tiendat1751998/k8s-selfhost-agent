package helm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/repo"
)

func TestSearchCharts_Deduplication(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewReleaseManager(nil, nil, tempDir)

	repoDir := filepath.Join(tempDir, "repository")
	cacheDir := filepath.Join(tempDir, "cache")
	_ = os.MkdirAll(repoDir, 0755)
	_ = os.MkdirAll(cacheDir, 0755)

	repoFile := filepath.Join(repoDir, "repositories.yaml")
	rf := repo.NewFile()
	rf.Add(&repo.Entry{
		Name: "bitnami",
		URL:  "https://charts.bitnami.com/bitnami",
	})
	if err := rf.WriteFile(repoFile, 0644); err != nil {
		t.Fatalf("failed to write repo file: %v", err)
	}

	// Index YAML with multiple historical versions of airflow (25.0.2, 25.0.1, 25.0.0)
	// plus other distinct charts
	indexYAML := `apiVersion: v1
entries:
  airflow:
    - name: airflow
      version: 25.0.2
      description: Apache Airflow latest
      appVersion: 2.7.1
    - name: airflow
      version: 25.0.1
      description: Apache Airflow older
      appVersion: 2.7.0
    - name: airflow
      version: 25.0.0
      description: Apache Airflow oldest
      appVersion: 2.6.3
  apache:
    - name: apache
      version: 1.2.0
      description: Apache HTTP Server
  argo-cd:
    - name: argo-cd
      version: 5.4.1
      description: Argo CD
  clickhouse:
    - name: clickhouse
      version: 4.1.0
      description: ClickHouse DB
  grafana:
    - name: grafana
      version: 7.0.0
      description: Grafana visualization
  kafka:
    - name: kafka
      version: 26.0.0
      description: Apache Kafka
  nginx:
    - name: nginx
      version: 15.0.0
      description: NGINX Ingress
  postgresql:
    - name: postgresql
      version: 12.1.0
      description: PostgreSQL DB
  redis:
    - name: redis
      version: 18.0.0
      description: Redis Cache
`
	indexPath := filepath.Join(cacheDir, "bitnami-index.yaml")
	if err := os.WriteFile(indexPath, []byte(indexYAML), 0644); err != nil {
		t.Fatalf("failed to write mock index yaml: %v", err)
	}

	results, err := rm.SearchCharts("")
	if err != nil {
		t.Fatalf("SearchCharts failed: %v", err)
	}

	// Should return 9 distinct charts (airflow, apache, argo-cd, clickhouse, grafana, kafka, nginx, postgresql, redis)
	if len(results) != 9 {
		t.Fatalf("expected 9 distinct charts, got %d", len(results))
	}

	// Airflow must appear exactly once, with the highest semver (25.0.2)
	airflowCount := 0
	var airflowVersion string
	for _, res := range results {
		if strings.HasSuffix(res.Name, "airflow") {
			airflowCount++
			if res.Chart != nil {
				airflowVersion = res.Chart.Version
			}
		}
	}

	if airflowCount != 1 {
		t.Errorf("expected airflow to appear exactly once, appeared %d times", airflowCount)
	}
	if airflowVersion != "25.0.2" {
		t.Errorf("expected airflow version 25.0.2, got %s", airflowVersion)
	}

	// Ensure all results are distinct by name
	seen := make(map[string]bool)
	for _, res := range results {
		if seen[res.Name] {
			t.Errorf("duplicate chart found in search results: %s", res.Name)
		}
		seen[res.Name] = true
	}
}

func TestSearchCharts_DeprecatedVersionSkipped(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewReleaseManager(nil, nil, tempDir)

	repoDir := filepath.Join(tempDir, "repository")
	cacheDir := filepath.Join(tempDir, "cache")
	_ = os.MkdirAll(repoDir, 0755)
	_ = os.MkdirAll(cacheDir, 0755)

	repoFile := filepath.Join(repoDir, "repositories.yaml")
	rf := repo.NewFile()
	rf.Add(&repo.Entry{Name: "myrepo", URL: "https://example.com/charts"})
	_ = rf.WriteFile(repoFile, 0644)

	// Chart with newest version deprecated, older version active
	indexYAML := `apiVersion: v1
entries:
  mychart:
    - name: mychart
      version: 2.0.0
      deprecated: true
      description: Deprecated version
    - name: mychart
      version: 1.9.0
      deprecated: false
      description: Active stable version
`
	indexPath := filepath.Join(cacheDir, "myrepo-index.yaml")
	_ = os.WriteFile(indexPath, []byte(indexYAML), 0644)

	results, err := rm.SearchCharts("")
	if err != nil {
		t.Fatalf("SearchCharts failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Chart == nil || results[0].Chart.Version != "1.9.0" {
		t.Errorf("expected active non-deprecated version 1.9.0, got %+v", results[0].Chart)
	}
}
