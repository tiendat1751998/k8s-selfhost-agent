package helm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
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
	if namespace == "all" || namespace == "_all" {
		namespace = ""
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

	if namespace == "" || namespace == "all" || namespace == "_all" {
		// Auto-discover namespace of the release across all cluster releases
		allRels, err := m.ListReleases(ctx, clusterID, "")
		if err == nil {
			for _, r := range allRels {
				if r != nil && r.Name == name {
					namespace = r.Namespace
					break
				}
			}
		}
		if namespace == "" || namespace == "all" || namespace == "_all" {
			namespace = "default"
		}
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


// GetReleaseHistory retrieves the revision history for a release.
func (m *ReleaseManager) GetReleaseHistory(ctx context.Context, clusterID, name, namespace string) ([]*release.Release, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("release name is required")
	}

	if namespace == "" || namespace == "all" || namespace == "_all" {
		// Auto-discover namespace of the release across all cluster releases
		allRels, err := m.ListReleases(ctx, clusterID, "")
		if err == nil {
			for _, r := range allRels {
				if r != nil && r.Name == name {
					namespace = r.Namespace
					break
				}
			}
		}
		if namespace == "" || namespace == "all" || namespace == "_all" {
			namespace = "default"
		}
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
