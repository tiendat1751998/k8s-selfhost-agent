package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/client-go/kubernetes"

	"github.com/datdt/k8sselfhost/internal/domain/explorer"
	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	infraDocker "github.com/datdt/k8sselfhost/internal/infrastructure/provider/docker"
)

type explorerRepo struct {
	client        *kubernetes.Clientset
	dockerRepo    domainDocker.Repository
	clientManager *cluster.ClientManager
}

// NewExplorerRepo creates a new live Kubernetes-backed Explorer repository.
func NewExplorerRepo(client *kubernetes.Clientset, dockerRepo domainDocker.Repository, clientManager *cluster.ClientManager) explorer.Repository {
	return &explorerRepo{
		client:        client,
		dockerRepo:    dockerRepo,
		clientManager: clientManager,
	}
}

func (r *explorerRepo) Search(ctx context.Context, kind, cluster, namespace, query string, limit, offset int) ([]explorer.Resource, int, error) {
	var client *kubernetes.Clientset
	var activeDockerRepo domainDocker.Repository = r.dockerRepo

	if r.clientManager != nil && cluster != "" {
		// 1. Try to get dynamic Docker client first to see if it's a Docker Swarm provider
		dCli, err := r.clientManager.GetDockerClient(ctx, cluster)
		if err == nil && dCli != nil {
			activeDockerRepo = infraDocker.NewDockerRepoWithClient(dCli)
		} else {
			// 2. Try to get dynamic Kubernetes clientset
			kCli, err := r.clientManager.GetK8sClient(ctx, cluster)
			if err == nil && kCli != nil {
				client = kCli
			}
		}
	}

	if client == nil {
		var err error
		client, err = getClient(ctx, r.client)
		if err != nil {
			return nil, 0, err
		}
	}

	kind = strings.ToLower(kind)

	if client == nil {
		if activeDockerRepo != nil {
			return r.searchSwarm(ctx, activeDockerRepo, kind, cluster, query, limit, offset)
		}
		return nil, 0, fmt.Errorf("kubernetes client is not initialized")
	}

	return r.searchK8s(ctx, client, kind, cluster, namespace, query, limit, offset)
}

func (r *explorerRepo) GetByID(ctx context.Context, id string) (*explorer.Resource, error) {
	// Parse ID format: [cluster/]kind/namespace/name or kind/namespace/name
	var clusterName, kind, namespace, name string
	parts := strings.Split(id, "/")
	if len(parts) >= 3 {
		if len(parts) == 4 {
			clusterName = parts[0]
			kind = parts[1]
			namespace = parts[2]
			name = parts[3]
		} else {
			kind = parts[0]
			namespace = parts[1]
			name = parts[2]
		}
	} else if len(parts) == 2 {
		kind = parts[0]
		name = parts[1]
	} else {
		return nil, fmt.Errorf("invalid resource ID format: %s", id)
	}

	var client *kubernetes.Clientset
	if r.clientManager != nil && clusterName != "" {
		kCli, err := r.clientManager.GetK8sClient(ctx, clusterName)
		if err == nil && kCli != nil {
			client = kCli
		}
	}
	if client == nil {
		var err error
		client, err = getClient(ctx, r.client)
		if err != nil {
			return nil, err
		}
	}
	if client == nil {
		return nil, fmt.Errorf("kubernetes client is not initialized")
	}

	return r.getK8sByID(ctx, client, id, clusterName, kind, namespace, name)
}

func (r *explorerRepo) SyncResource(ctx context.Context, res *explorer.Resource) error {
	var client *kubernetes.Clientset
	if r.clientManager != nil && res.Cluster != "" {
		kCli, err := r.clientManager.GetK8sClient(ctx, res.Cluster)
		if err == nil && kCli != nil {
			client = kCli
		}
	}
	if client == nil {
		var err error
		client, err = getClient(ctx, r.client)
		if err != nil {
			return err
		}
	}
	if client == nil {
		return fmt.Errorf("kubernetes client is not initialized")
	}

	return r.syncK8sResource(ctx, client, res)
}
