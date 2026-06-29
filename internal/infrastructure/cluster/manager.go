package cluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/docker/docker/client"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/datdt/k8sselfhost/internal/domain/fleet"
)

// ClientManager manages dynamic client pools for both Kubernetes and Docker clusters.
type ClientManager struct {
	fleetRepo  fleet.Repository
	k8sPool    map[string]*kubernetes.Clientset
	dockerPool map[string]*client.Client
	mu         sync.RWMutex
}

// NewClientManager initializes a new ClientManager.
func NewClientManager(fleetRepo fleet.Repository) *ClientManager {
	return &ClientManager{
		fleetRepo:  fleetRepo,
		k8sPool:    make(map[string]*kubernetes.Clientset),
		dockerPool: make(map[string]*client.Client),
	}
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

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check
	if c, ok = m.k8sPool[clusterID]; ok {
		return c, nil
	}

	cluster, err := m.fleetRepo.GetCluster(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("retrieving cluster %s details: %w", clusterID, err)
	}
	if cluster == nil {
		return nil, fmt.Errorf("cluster %s not found in fleet repository", clusterID)
	}

	if cluster.Provider != "kubernetes" && cluster.Provider != "aws" && cluster.Provider != "gcp" {
		return nil, fmt.Errorf("cluster %s has provider %s, not kubernetes", clusterID, cluster.Provider)
	}

	if cluster.EncryptedToken == "" {
		return nil, fmt.Errorf("cluster %s has no token or kubeconfig data", clusterID)
	}

	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(cluster.EncryptedToken))
	if err != nil {
		return nil, fmt.Errorf("parsing kubeconfig for cluster %s: %w", clusterID, err)
	}

	config.QPS = 50
	config.Burst = 100
	config.Timeout = 30 * time.Second

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
	if host == "" {
		host = "tcp://10.10.10.133:2375"
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
	_, err := m.fleetRepo.GetCluster(ctx, "00000000-0000-0000-0000-000000000000")
	if err != nil && err.Error() != "sql: no rows in result set" && err.Error() != "pgx: no rows in result set" {
		return err
	}
	return nil
}
