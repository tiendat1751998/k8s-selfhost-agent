package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/docker/docker/client"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/datdt/k8sselfhost/internal/domain/fleet"
	infraConfig "github.com/datdt/k8sselfhost/internal/infrastructure/config"
	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
)

// ClientManager manages dynamic client pools for both Kubernetes and Docker clusters.
type ClientManager struct {
	fleetRepo     fleet.Repository
	k8sPool       map[string]*kubernetes.Clientset
	k8sConfigPool map[string]*rest.Config
	dockerPool    map[string]*client.Client
	mu            sync.RWMutex
}

// NewClientManager initializes a new ClientManager.
func NewClientManager(fleetRepo fleet.Repository) *ClientManager {
	return &ClientManager{
		fleetRepo:     fleetRepo,
		k8sPool:       make(map[string]*kubernetes.Clientset),
		k8sConfigPool: make(map[string]*rest.Config),
		dockerPool:    make(map[string]*client.Client),
	}
}

// GetK8sRestConfig retrieves or creates a cached Kubernetes rest.Config for a cluster.
func (m *ClientManager) GetK8sRestConfig(ctx context.Context, clusterID string) (*rest.Config, error) {
	if clusterID == "" {
		return nil, fmt.Errorf("clusterID cannot be empty")
	}

	m.mu.RLock()
	cfg, ok := m.k8sConfigPool[clusterID]
	m.mu.RUnlock()
	if ok {
		return cfg, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check
	if cfg, ok = m.k8sConfigPool[clusterID]; ok {
		return cfg, nil
	}

	if m.fleetRepo == nil {
		return nil, fmt.Errorf("fleet repository not initialized")
	}

	cluster, err := m.fleetRepo.GetCluster(ctx, clusterID)
	if (err != nil || cluster == nil) && m.fleetRepo != nil {
		if clusters, listErr := m.fleetRepo.ListClusters(ctx); listErr == nil {
			for _, c := range clusters {
				if c.ID == clusterID || c.Name == clusterID {
					target := c
					cluster = &target
					err = nil
					break
				}
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("retrieving cluster %s details: %w", clusterID, err)
	}
	if cluster == nil {
		return nil, fmt.Errorf("cluster %s not found in fleet repository", clusterID)
	}

	if cluster.Provider == "docker" && cluster.ImportMethod != "kubeconfig" {
		return nil, fmt.Errorf("cluster %s has provider %s, not kubernetes", clusterID, cluster.Provider)
	}

	if cluster.EncryptedToken == "" {
		return nil, fmt.Errorf("cluster %s has no token or kubeconfig data", clusterID)
	}

	kubeconfigData, err := crypto.Decrypt(cluster.EncryptedToken)
	if err != nil {
		return nil, fmt.Errorf("decrypting cluster %s credentials: %w", clusterID, err)
	}

	trimmed := strings.TrimSpace(kubeconfigData)
	for strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"") {
		var unquoted string
		if err := json.Unmarshal([]byte(trimmed), &unquoted); err == nil {
			trimmed = strings.TrimSpace(unquoted)
		} else {
			trimmed = strings.Trim(trimmed, "\"")
			break
		}
	}
	if strings.Contains(trimmed, "\\n") {
		trimmed = strings.ReplaceAll(trimmed, "\\r\\n", "\n")
		trimmed = strings.ReplaceAll(trimmed, "\\n", "\n")
		trimmed = strings.ReplaceAll(trimmed, "\\t", "\t")
		trimmed = strings.ReplaceAll(trimmed, "\\\"", "\"")
	}
	trimmed = strings.Trim(trimmed, "\"")
	trimmed = strings.Trim(trimmed, "'")
	trimmed = strings.TrimSpace(trimmed)
	kubeconfigData = trimmed

	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfigData))
	if err != nil {
		return nil, fmt.Errorf("parsing kubeconfig for cluster %s: %w", clusterID, err)
	}

	k8sCfg := infraConfig.GetK8sClientConfig()
	config.QPS = k8sCfg.QPS
	config.Burst = k8sCfg.Burst
	config.Timeout = k8sCfg.Timeout
	if len(config.TLSClientConfig.CAData) == 0 && config.TLSClientConfig.CAFile == "" {
		return nil, fmt.Errorf("insecure TLS rejected: cluster %s is missing CA certificate", clusterID)
	}
	if config.TLSClientConfig.Insecure {
		return nil, fmt.Errorf("insecure TLS rejected: cluster %s has insecure TLS enabled", clusterID)
	}

	m.k8sConfigPool[clusterID] = config
	return config, nil
}

// GetK8sClient retrieves or creates a cached Kubernetes clientset for a cluster.
func (m *ClientManager) GetK8sClient(ctx context.Context, clusterID string) (*kubernetes.Clientset, error) {
	if clusterID == "" {
		return nil, fmt.Errorf("clusterID cannot be empty")
	}

	m.mu.RLock()
	c, ok := m.k8sPool[clusterID]
	m.mu.RUnlock()
	if ok {
		return c, nil
	}

	config, err := m.GetK8sRestConfig(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check
	if c, ok = m.k8sPool[clusterID]; ok {
		return c, nil
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("building clientset config for cluster %s: %w", clusterID, err)
	}

	m.k8sPool[clusterID] = clientset
	return clientset, nil
}

// GetDockerClient retrieves or creates a cached Docker Engine/Swarm client.
func (m *ClientManager) GetDockerClient(ctx context.Context, clusterID string) (*client.Client, error) {
	if clusterID == "" {
		return nil, fmt.Errorf("clusterID cannot be empty")
	}

	m.mu.RLock()
	c, ok := m.dockerPool[clusterID]
	m.mu.RUnlock()
	if ok {
		return c, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check
	if c, ok = m.dockerPool[clusterID]; ok {
		return c, nil
	}

	if m.fleetRepo == nil {
		return nil, fmt.Errorf("fleet repository not initialized")
	}

	cluster, err := m.fleetRepo.GetCluster(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("retrieving cluster %s details: %w", clusterID, err)
	}
	if cluster == nil {
		return nil, fmt.Errorf("cluster %s not found in fleet repository", clusterID)
	}

	if cluster.Provider != "docker" {
		return nil, fmt.Errorf("cluster %s has provider %s, not docker", clusterID, cluster.Provider)
	}

	host := cluster.EncryptedToken
	if decrypted, err := crypto.Decrypt(cluster.EncryptedToken); err == nil && decrypted != "" {
		host = decrypted
	}
	if host == "" {
		if envHost := os.Getenv("DOCKER_HOST"); envHost != "" {
			host = envHost
		} else if envHost := os.Getenv("K8S_DOCKER_HOST"); envHost != "" {
			host = envHost
		} else {
			host = "unix:///var/run/docker.sock"
		}
	}

	cli, err := client.NewClientWithOpts(
		client.WithHost(host),
		client.WithVersion("1.41"),
	)
	if err != nil {
		return nil, fmt.Errorf("building docker client for host %s: %w", host, err)
	}

	m.dockerPool[clusterID] = cli
	return cli, nil
}

// PingDB verifies connectivity to the PostgreSQL database pool.
func (m *ClientManager) PingDB(ctx context.Context) error {
	if m.fleetRepo == nil {
		return nil
	}
	_, err := m.fleetRepo.GetCluster(ctx, "00000000-0000-0000-0000-000000000000")
	if err != nil && err.Error() != "sql: no rows in result set" && err.Error() != "pgx: no rows in result set" {
		return err
	}
	return nil
}

// GetClusterClient resolves a kubernetes.Interface client for clusterID, satisfying clean architecture ports.
func (m *ClientManager) GetClusterClient(ctx context.Context, clusterID string) (kubernetes.Interface, error) {
	if m == nil {
		return nil, fmt.Errorf("client manager is not configured")
	}
	return m.GetK8sClient(ctx, clusterID)
}

// ClientManagerAdapter adapts ClientManager to clean architecture usecase client resolver ports.
type ClientManagerAdapter struct {
	mgr *ClientManager
}

// NewClientManagerAdapter wraps ClientManager as a generic cluster resolver.
func NewClientManagerAdapter(mgr *ClientManager) *ClientManagerAdapter {
	return &ClientManagerAdapter{mgr: mgr}
}

// GetClusterClient delegates to ClientManager.GetClusterClient.
func (a *ClientManagerAdapter) GetClusterClient(ctx context.Context, clusterID string) (kubernetes.Interface, error) {
	if a == nil || a.mgr == nil {
		return nil, fmt.Errorf("client manager is not configured")
	}
	return a.mgr.GetClusterClient(ctx, clusterID)
}

// GetK8sClient delegates to ClientManager.GetK8sClient returning kubernetes.Interface.
func (a *ClientManagerAdapter) GetK8sClient(ctx context.Context, clusterID string) (kubernetes.Interface, error) {
	if a == nil || a.mgr == nil {
		return nil, fmt.Errorf("client manager is not configured")
	}
	return a.mgr.GetK8sClient(ctx, clusterID)
}

// GetK8sRestConfig delegates to ClientManager.GetK8sRestConfig.
func (a *ClientManagerAdapter) GetK8sRestConfig(ctx context.Context, clusterID string) (*rest.Config, error) {
	if a == nil || a.mgr == nil {
		return nil, fmt.Errorf("client manager is not configured")
	}
	return a.mgr.GetK8sRestConfig(ctx, clusterID)
}

// AsClusterResolver returns a ClientManagerAdapter wrapping this ClientManager.
func (m *ClientManager) AsClusterResolver() *ClientManagerAdapter {
	return NewClientManagerAdapter(m)
}

