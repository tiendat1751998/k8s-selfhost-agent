package helm

import (
	"fmt"
	"path/filepath"
	"strings"

	"helm.sh/helm/v3/cmd/helm/search"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

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

	m.mu.Lock()
	defer m.mu.Unlock()

	var repoFile repo.File
	if fileExists(m.repoFile) {
		rf, err := repo.LoadFile(m.repoFile)
		if err != nil {
			return fmt.Errorf("loading repo file: %w", err)
		}
		repoFile = *rf
	} else {
		repoFile = *repo.NewFile()
	}

	entry := &repo.Entry{
		Name: name,
		URL:  url,
	}

	chartRepo, err := repo.NewChartRepository(entry, getter.All(m.settings))
	if err != nil {
		return fmt.Errorf("initializing chart repository %s: %w", name, err)
	}
	chartRepo.CachePath = m.repoCacheDir
	if _, err := chartRepo.DownloadIndexFile(); err != nil {
		return fmt.Errorf("downloading repository index for %s (%s): %w", name, url, err)
	}

	if repoFile.Has(name) {
		repoFile.Update(entry)
	} else {
		repoFile.Add(entry)
	}

	if err := repoFile.WriteFile(m.repoFile, 0644); err != nil {
		return fmt.Errorf("saving repo file: %w", err)
	}

	return nil
}

// UpdateRepos updates the cached index for all configured Helm repositories.
func (m *ReleaseManager) UpdateRepos() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !fileExists(m.repoFile) {
		return nil
	}

	repoFile, err := repo.LoadFile(m.repoFile)
	if err != nil {
		return fmt.Errorf("loading repo file: %w", err)
	}

	var updateErrors []string
	for _, entry := range repoFile.Repositories {
		chartRepo, err := repo.NewChartRepository(entry, getter.All(m.settings))
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

// SearchCharts searches for charts across all configured repositories matching a keyword.
func (m *ReleaseManager) SearchCharts(keyword string) ([]*search.Result, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !fileExists(m.repoFile) {
		return make([]*search.Result, 0), nil
	}

	repoFile, err := repo.LoadFile(m.repoFile)
	if err != nil {
		return nil, fmt.Errorf("loading repo file: %w", err)
	}

	index := search.NewIndex()
	for _, entry := range repoFile.Repositories {
		indexPath := filepath.Join(m.repoCacheDir, fmt.Sprintf("%s-index.yaml", entry.Name))
		if !fileExists(indexPath) {
			continue
		}
		indexFile, err := repo.LoadIndexFile(indexPath)
		if err != nil {
			continue
		}
		index.AddRepo(entry.Name, indexFile, true)
	}

	results, err := index.Search(keyword, 100, false)
	if err != nil {
		return nil, fmt.Errorf("searching charts: %w", err)
	}
	search.SortScore(results)
	if results == nil {
		results = make([]*search.Result, 0)
	}
	return results, nil
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
