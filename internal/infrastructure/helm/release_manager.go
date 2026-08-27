package helm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"helm.sh/helm/v3/cmd/helm/search"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// InstallRequest represents parameters for installing a Helm chart.
type InstallRequest struct {
	ReleaseName     string                 `json:"releaseName"`
	Chart           string                 `json:"chart"`
	Repo            string                 `json:"repo"`
	Version         string                 `json:"version"`
	Namespace       string                 `json:"namespace"`
	Values          map[string]interface{} `json:"values"`
	CreateNamespace bool                   `json:"createNamespace"`
	Wait            bool                   `json:"wait"`
	Timeout         time.Duration          `json:"timeout"`
}

// UpgradeRequest represents parameters for upgrading a Helm release.
type UpgradeRequest struct {
	ReleaseName string                 `json:"releaseName"`
	Chart       string                 `json:"chart"`
	Repo        string                 `json:"repo"`
	Version     string                 `json:"version"`
	Namespace   string                 `json:"namespace"`
	Values      map[string]interface{} `json:"values"`
	ResetValues bool                   `json:"resetValues"`
	ReuseValues bool                   `json:"reuseValues"`
	Wait        bool                   `json:"wait"`
	Timeout     time.Duration          `json:"timeout"`
}

// ReleaseManager handles Helm operations across multiple Kubernetes clusters and manages Helm chart repositories.
type ReleaseManager struct {
	clientManager *cluster.ClientManager
	defaultConfig *rest.Config
	settings      *cli.EnvSettings
	repoFile      string
	repoCacheDir  string
	mu            sync.RWMutex
}

// NewReleaseManager creates a new ReleaseManager instance with client manager, default config, and optional base home directory.
func NewReleaseManager(clientManager *cluster.ClientManager, defaultConfig *rest.Config, homeDirs ...string) *ReleaseManager {
	var baseDir string
	if len(homeDirs) > 0 && homeDirs[0] != "" {
		baseDir = homeDirs[0]
	} else {
		baseDir = getHelmHomeDir()
	}

	repoDir := filepath.Join(baseDir, "repository")
	repoCacheDir := filepath.Join(baseDir, "cache")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		baseDir = filepath.Join(os.TempDir(), "helm-k8sselfhost")
		repoDir = filepath.Join(baseDir, "repository")
		repoCacheDir = filepath.Join(baseDir, "cache")
		_ = os.MkdirAll(repoDir, 0755)
		_ = os.MkdirAll(repoCacheDir, 0755)
	} else {
		_ = os.MkdirAll(repoCacheDir, 0755)
	}

	repoFile := filepath.Join(repoDir, "repositories.yaml")
	if !fileExists(repoFile) {
		rf := repo.NewFile()
		_ = rf.WriteFile(repoFile, 0644)
	}

	settings := cli.New()
	settings.RepositoryConfig = repoFile
	settings.RepositoryCache = repoCacheDir

	return &ReleaseManager{
		clientManager: clientManager,
		defaultConfig: defaultConfig,
		settings:      settings,
		repoFile:      repoFile,
		repoCacheDir:  repoCacheDir,
	}
}

func getHelmHomeDir() string {
	if h := os.Getenv("HELM_HOME"); h != "" {
		return h
	}
	if h := os.Getenv("HELM_CONFIG_HOME"); h != "" {
		return h
	}
	if userHome, err := os.UserHomeDir(); err == nil && userHome != "" {
		return filepath.Join(userHome, ".helm")
	}
	return filepath.Join(os.TempDir(), "helm-k8sselfhost")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// helmRESTClientGetter implements genericclioptions.RESTClientGetter for dynamic Helm action configurations.
type helmRESTClientGetter struct {
	config    *rest.Config
	namespace string
}

func (g *helmRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return g.config, nil
}

func (g *helmRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := g.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	dc, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	return memory.NewMemCacheClient(dc), nil
}

func (g *helmRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	dc, err := g.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(dc)
	expander := restmapper.NewShortcutExpander(mapper, dc, nil)
	return expander, nil
}

func (g *helmRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	if g.namespace != "" {
		configOverrides.Context.Namespace = g.namespace
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
}

func (m *ReleaseManager) getRestConfig(ctx context.Context, clusterID string) (*rest.Config, error) {
	if m.clientManager != nil && clusterID != "" && clusterID != "local" && clusterID != "default" && clusterID != "in-cluster" {
		cfg, err := m.clientManager.GetK8sRestConfig(ctx, clusterID)
		if err == nil && cfg != nil {
			return cfg, nil
		}
	}
	if m.defaultConfig != nil {
		return m.defaultConfig, nil
	}
	return nil, fmt.Errorf("kubernetes cluster %q is not connected or configured", clusterID)
}

func (m *ReleaseManager) getActionConfig(ctx context.Context, clusterID, namespace string) (*action.Configuration, error) {
	if namespace == "" {
		namespace = "default"
	}

	restConfig, err := m.getRestConfig(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	getter := &helmRESTClientGetter{
		config:    restConfig,
		namespace: namespace,
	}

	actionConfig := new(action.Configuration)
	driver := os.Getenv("HELM_DRIVER")
	if driver == "" {
		driver = "secret"
	}

	logFunc := func(format string, v ...interface{}) {
		logger.Get().Sugar().Debugf(format, v...)
	}

	if err := actionConfig.Init(getter, namespace, driver, logFunc); err != nil {
		return nil, fmt.Errorf("initializing helm action configuration: %w", err)
	}

	return actionConfig, nil
}

// ListReleases lists all Helm releases in a namespace (or all namespaces if empty or "all").
func (m *ReleaseManager) ListReleases(ctx context.Context, clusterID, namespace string) ([]*release.Release, error) {
	actionConfig, err := m.getActionConfig(ctx, clusterID, namespace)
	if err != nil {
		return nil, err
	}

	listAction := action.NewList(actionConfig)
	if namespace == "" || namespace == "all" || namespace == "_all" {
		listAction.AllNamespaces = true
	} else {
		listAction.AllNamespaces = false
	}
	listAction.StateMask = action.ListAll

	releases, err := listAction.Run()
	if err != nil {
		return nil, fmt.Errorf("listing releases: %w", err)
	}
	if releases == nil {
		releases = make([]*release.Release, 0)
	}
	return releases, nil
}

// GetRelease retrieves the current status and manifest of a named Helm release.
func (m *ReleaseManager) GetRelease(ctx context.Context, clusterID, name, namespace string) (*release.Release, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("release name is required")
	}

	actionConfig, err := m.getActionConfig(ctx, clusterID, namespace)
	if err != nil {
		return nil, err
	}

	getAction := action.NewGet(actionConfig)
	rel, err := getAction.Run(name)
	if err != nil {
		return nil, fmt.Errorf("getting release %s: %w", name, err)
	}
	return rel, nil
}

// InstallRelease installs a Helm chart into the specified cluster and namespace.
func (m *ReleaseManager) InstallRelease(ctx context.Context, clusterID string, req InstallRequest) (*release.Release, error) {
	if strings.TrimSpace(req.ReleaseName) == "" {
		return nil, fmt.Errorf("release name is required")
	}
	if strings.TrimSpace(req.Chart) == "" {
		return nil, fmt.Errorf("chart name is required")
	}
	ns := req.Namespace
	if strings.TrimSpace(ns) == "" {
		ns = "default"
	}

	actionConfig, err := m.getActionConfig(ctx, clusterID, ns)
	if err != nil {
		return nil, err
	}

	install := action.NewInstall(actionConfig)
	install.ReleaseName = req.ReleaseName
	install.Namespace = ns
	install.CreateNamespace = req.CreateNamespace
	install.Version = req.Version
	if req.Wait {
		install.Wait = true
	}
	if req.Timeout > 0 {
		install.Timeout = req.Timeout
	} else {
		install.Timeout = 5 * time.Minute
	}

	chartRef := req.Chart
	cpo := &install.ChartPathOptions
	cpo.Version = req.Version
	if req.Repo != "" {
		if strings.HasPrefix(req.Repo, "http://") || strings.HasPrefix(req.Repo, "https://") || strings.HasPrefix(req.Repo, "oci://") {
			cpo.RepoURL = req.Repo
		} else if !strings.Contains(req.Chart, "/") {
			chartRef = fmt.Sprintf("%s/%s", req.Repo, req.Chart)
		}
	}

	m.mu.RLock()
	chartPath, err := cpo.LocateChart(chartRef, m.settings)
	m.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("locating chart %s: %w", chartRef, err)
	}

	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("loading chart %s from %s: %w", chartRef, chartPath, err)
	}

	if chartRequested.Metadata.Type != "" && chartRequested.Metadata.Type != "application" {
		return nil, fmt.Errorf("%s charts are not installable", chartRequested.Metadata.Type)
	}

	values := req.Values
	if values == nil {
		values = make(map[string]interface{})
	}

	rel, err := install.RunWithContext(ctx, chartRequested, values)
	if err != nil {
		return nil, fmt.Errorf("installing release %s: %w", req.ReleaseName, err)
	}

	return rel, nil
}

// UpgradeRelease upgrades an existing Helm release with new values, chart, or version.
func (m *ReleaseManager) UpgradeRelease(ctx context.Context, clusterID string, req UpgradeRequest) (*release.Release, error) {
	if strings.TrimSpace(req.ReleaseName) == "" {
		return nil, fmt.Errorf("release name is required")
	}
	ns := req.Namespace
	if strings.TrimSpace(ns) == "" {
		ns = "default"
	}

	actionConfig, err := m.getActionConfig(ctx, clusterID, ns)
	if err != nil {
		return nil, err
	}

	upgrade := action.NewUpgrade(actionConfig)
	upgrade.Namespace = ns
	upgrade.Version = req.Version
	upgrade.ResetValues = req.ResetValues
	upgrade.ReuseValues = req.ReuseValues
	if req.Wait {
		upgrade.Wait = true
	}
	if req.Timeout > 0 {
		upgrade.Timeout = req.Timeout
	} else {
		upgrade.Timeout = 5 * time.Minute
	}

	var chartRequested *chart.Chart
	if req.Chart != "" {
		chartRef := req.Chart
		cpo := &upgrade.ChartPathOptions
		cpo.Version = req.Version
		if req.Repo != "" {
			if strings.HasPrefix(req.Repo, "http://") || strings.HasPrefix(req.Repo, "https://") || strings.HasPrefix(req.Repo, "oci://") {
				cpo.RepoURL = req.Repo
			} else if !strings.Contains(req.Chart, "/") {
				chartRef = fmt.Sprintf("%s/%s", req.Repo, req.Chart)
			}
		}

		m.mu.RLock()
		chartPath, err := cpo.LocateChart(chartRef, m.settings)
		m.mu.RUnlock()
		if err != nil {
			return nil, fmt.Errorf("locating chart %s: %w", chartRef, err)
		}

		chartRequested, err = loader.Load(chartPath)
		if err != nil {
			return nil, fmt.Errorf("loading chart %s from %s: %w", chartRef, chartPath, err)
		}
	} else {
		getAction := action.NewGet(actionConfig)
		existing, err := getAction.Run(req.ReleaseName)
		if err != nil {
			return nil, fmt.Errorf("getting existing release %s: %w", req.ReleaseName, err)
		}
		if existing.Chart == nil {
			return nil, fmt.Errorf("existing release %s has no chart", req.ReleaseName)
		}
		chartRequested = existing.Chart
	}

	values := req.Values
	if values == nil {
		values = make(map[string]interface{})
	}

	rel, err := upgrade.RunWithContext(ctx, req.ReleaseName, chartRequested, values)
	if err != nil {
		return nil, fmt.Errorf("upgrading release %s: %w", req.ReleaseName, err)
	}

	return rel, nil
}

// RollbackRelease rolls back a release to a previous revision.
func (m *ReleaseManager) RollbackRelease(ctx context.Context, clusterID, name, namespace string, revision int) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("release name is required")
	}

	actionConfig, err := m.getActionConfig(ctx, clusterID, namespace)
	if err != nil {
		return err
	}

	rollbackAction := action.NewRollback(actionConfig)
	rollbackAction.Version = revision
	rollbackAction.Wait = true
	rollbackAction.Timeout = 5 * time.Minute

	if err := rollbackAction.Run(name); err != nil {
		return fmt.Errorf("rolling back release %s to revision %d: %w", name, revision, err)
	}
	return nil
}

// UninstallRelease uninstalls a named release from the cluster.
func (m *ReleaseManager) UninstallRelease(ctx context.Context, clusterID, name, namespace string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("release name is required")
	}

	actionConfig, err := m.getActionConfig(ctx, clusterID, namespace)
	if err != nil {
		return err
	}

	uninstallAction := action.NewUninstall(actionConfig)
	uninstallAction.Wait = true
	uninstallAction.Timeout = 5 * time.Minute

	if _, err := uninstallAction.Run(name); err != nil {
		return fmt.Errorf("uninstalling release %s: %w", name, err)
	}
	return nil
}

// GetReleaseHistory retrieves the revision history for a release.
func (m *ReleaseManager) GetReleaseHistory(ctx context.Context, clusterID, name, namespace string) ([]*release.Release, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("release name is required")
	}

	actionConfig, err := m.getActionConfig(ctx, clusterID, namespace)
	if err != nil {
		return nil, err
	}

	historyAction := action.NewHistory(actionConfig)
	historyAction.Max = 256

	hist, err := historyAction.Run(name)
	if err != nil {
		return nil, fmt.Errorf("getting history for release %s: %w", name, err)
	}
	if hist == nil {
		hist = make([]*release.Release, 0)
	}
	return hist, nil
}

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
