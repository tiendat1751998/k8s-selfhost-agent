package helm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"helm.sh/helm/v3/cmd/helm/search"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/datdt/k8sselfhost/internal/pkg/netguard"
)

var repoFileMu sync.Mutex

// ListRepos returns all configured Helm repositories.
func (m *ReleaseManager) ListRepos() ([]*repo.Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !fileExists(m.repoFile) {
		return make([]*repo.Entry, 0), nil
	}

	repoFile, err := repo.LoadFile(m.repoFile)
	if err != nil {
		return nil, fmt.Errorf("loading repo file: %w", err)
	}

	if repoFile.Repositories == nil {
		return make([]*repo.Entry, 0), nil
	}

	return repoFile.Repositories, nil
}

// AddRepo adds or updates a Helm repository and downloads its index.
func (m *ReleaseManager) AddRepo(name, url string) error {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if name == "" {
		return fmt.Errorf("repo name cannot be empty")
	}
	if url == "" {
		return fmt.Errorf("repo url cannot be empty")
	}

	// SEC-15: Scheme check (http/https only)
	if _, err := netguard.ValidateURL(url, "http", "https"); err != nil {
		return fmt.Errorf("invalid helm repository url scheme: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entry := &repo.Entry{
		Name: name,
		URL:  url,
	}

	// SEC-15: Guarded transport (public by default, allowlist for private via EGRESS_ALLOW_PRIVATE_CIDRS)
	guard := netguard.New(netguard.WithFromEnv())
	guardedGetters := getter.All(m.settings, getter.WithTransport(guard.NewTransport()))

	chartRepo, err := repo.NewChartRepository(entry, guardedGetters)
	if err != nil {
		return fmt.Errorf("initializing chart repository %s: %w", name, err)
	}
	chartRepo.CachePath = m.repoCacheDir
	if _, err := chartRepo.DownloadIndexFile(); err != nil {
		return fmt.Errorf("downloading repository index for %s (%s): %w", name, url, err)
	}

	repoFileMu.Lock()
	var repoFile repo.File
	if fileExists(m.repoFile) {
		rf, err := repo.LoadFile(m.repoFile)
		if err != nil {
			repoFileMu.Unlock()
			return fmt.Errorf("loading repo file: %w", err)
		}
		repoFile = *rf
	} else {
		repoFile = *repo.NewFile()
	}

	if repoFile.Has(name) {
		repoFile.Update(entry)
	} else {
		repoFile.Add(entry)
	}

	if err := repoFile.WriteFile(m.repoFile, 0644); err != nil {
		repoFileMu.Unlock()
		return fmt.Errorf("saving repo file: %w", err)
	}
	repoFileMu.Unlock()

	m.cachedIndex = nil
	m.cachedIndexTime = time.Time{}

	return nil
}

// UpdateRepos updates the cached index for all configured Helm repositories.
func (m *ReleaseManager) UpdateRepos() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cachedIndex = nil
	m.cachedIndexTime = time.Time{}

	if !fileExists(m.repoFile) {
		return nil
	}

	repoFile, err := repo.LoadFile(m.repoFile)
	if err != nil {
		return fmt.Errorf("loading repo file: %w", err)
	}

	guard := netguard.New(netguard.WithFromEnv())
	guardedGetters := getter.All(m.settings, getter.WithTransport(guard.NewTransport()))

	var updateErrors []string
	for _, entry := range repoFile.Repositories {
		if _, err := netguard.ValidateURL(entry.URL, "http", "https"); err != nil {
			updateErrors = append(updateErrors, fmt.Sprintf("%s: invalid repo url: %v", entry.Name, err))
			continue
		}
		chartRepo, err := repo.NewChartRepository(entry, guardedGetters)
		if err != nil {
			updateErrors = append(updateErrors, fmt.Sprintf("%s: %v", entry.Name, err))
			continue
		}
		chartRepo.CachePath = m.repoCacheDir
		if _, err := chartRepo.DownloadIndexFile(); err != nil {
			updateErrors = append(updateErrors, fmt.Sprintf("%s: %v", entry.Name, err))
		}
	}

	if len(updateErrors) > 0 {
		return fmt.Errorf("errors updating repositories: %s", strings.Join(updateErrors, "; "))
	}

	return nil
}

// RemoveRepo removes a configured Helm repository and its cached index file.
func (m *ReleaseManager) RemoveRepo(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("repo name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	repoFileMu.Lock()
	if !fileExists(m.repoFile) {
		repoFileMu.Unlock()
		return fmt.Errorf("no repo named %q found", name)
	}

	repoFile, err := repo.LoadFile(m.repoFile)
	if err != nil {
		repoFileMu.Unlock()
		return fmt.Errorf("loading repo file: %w", err)
	}

	if !repoFile.Remove(name) {
		repoFileMu.Unlock()
		return fmt.Errorf("no repo named %q found", name)
	}

	if err := repoFile.WriteFile(m.repoFile, 0644); err != nil {
		repoFileMu.Unlock()
		return fmt.Errorf("saving repo file: %w", err)
	}
	repoFileMu.Unlock()

	indexPath := filepath.Join(m.repoCacheDir, fmt.Sprintf("%s-index.yaml", name))
	_ = os.Remove(indexPath)

	m.cachedIndex = nil
	m.cachedIndexTime = time.Time{}

	return nil
}

// buildSearchIndexLocked builds a fresh search index across all configured repositories.
// Caller must hold m.mu Lock.
func (m *ReleaseManager) buildSearchIndexLocked() (*search.Index, error) {
	index := search.NewIndex()
	if !fileExists(m.repoFile) {
		return index, nil
	}

	repoFile, err := repo.LoadFile(m.repoFile)
	if err != nil {
		return nil, fmt.Errorf("loading repo file: %w", err)
	}

	for _, entry := range repoFile.Repositories {
		indexPath := filepath.Join(m.repoCacheDir, fmt.Sprintf("%s-index.yaml", entry.Name))
		if !fileExists(indexPath) {
			continue
		}
		indexFile, err := repo.LoadIndexFile(indexPath)
		if err != nil {
			continue
		}

		// Deduplicate: filter out deprecated versions if non-deprecated exists,
		// and pass all=false to index.AddRepo to index only the latest version.
		filteredEntries := make(map[string]repo.ChartVersions, len(indexFile.Entries))
		for name, versions := range indexFile.Entries {
			var activeVersions []*repo.ChartVersion
			for _, v := range versions {
				if v != nil && !v.Deprecated {
					activeVersions = append(activeVersions, v)
				}
			}
			if len(activeVersions) > 0 {
				filteredEntries[name] = activeVersions
			} else if len(versions) > 0 {
				filteredEntries[name] = versions[:1]
			}
		}
		indexFile.Entries = filteredEntries

		index.AddRepo(entry.Name, indexFile, false)
	}

	return index, nil
}

const maxSearchResults = 120

// SearchCharts searches for charts across all configured repositories matching a keyword.
func (m *ReleaseManager) SearchCharts(keyword string) ([]*search.Result, error) {
	m.mu.RLock()
	if m.cachedIndex == nil {
		m.mu.RUnlock()
		m.mu.Lock()
		if m.cachedIndex == nil {
			idx, err := m.buildSearchIndexLocked()
			if err != nil {
				m.mu.Unlock()
				return nil, err
			}
			m.cachedIndex = idx
			m.cachedIndexTime = time.Now()
		}
		m.mu.Unlock()
		m.mu.RLock()
	}
	defer m.mu.RUnlock()

	if m.cachedIndex == nil {
		return make([]*search.Result, 0), nil
	}

	results, err := m.cachedIndex.Search(keyword, maxSearchResults, false)
	if err != nil {
		return nil, fmt.Errorf("searching charts: %w", err)
	}
	search.SortScore(results)

	// Explicit deduplication by (repo/chartName) ensuring 1 latest entry per chart
	deduped := make([]*search.Result, 0, len(results))
	seen := make(map[string]bool, len(results))
	for _, res := range results {
		if res == nil || seen[res.Name] {
			continue
		}
		if res.Chart != nil && res.Chart.Deprecated {
			continue
		}
		seen[res.Name] = true
		deduped = append(deduped, res)
		if len(deduped) >= maxSearchResults {
			break
		}
	}

	return deduped, nil
}

// GetChartValues retrieves default values.yaml for a given chart from a repository.
func (m *ReleaseManager) GetChartValues(repoName, chartName, version string) (map[string]interface{}, error) {
	if strings.TrimSpace(chartName) == "" {
		return nil, fmt.Errorf("chart name is required")
	}

	chartRef := chartName
	cpo := &action.ChartPathOptions{
		Version: version,
	}

	if repoName != "" {
		if strings.HasPrefix(repoName, "http://") || strings.HasPrefix(repoName, "https://") || strings.HasPrefix(repoName, "oci://") {
			cpo.RepoURL = repoName
		} else if !strings.Contains(chartName, "/") {
			chartRef = fmt.Sprintf("%s/%s", repoName, chartName)
		}
	}

	m.mu.RLock()
	chartPath, err := cpo.LocateChart(chartRef, m.settings)
	m.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("locating chart %s: %w", chartRef, err)
	}

	ch, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("loading chart %s from %s: %w", chartRef, chartPath, err)
	}

	if ch.Values == nil {
		return make(map[string]interface{}), nil
	}

	return ch.Values, nil
}
