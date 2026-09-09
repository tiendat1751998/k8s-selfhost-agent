package kubernetes

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/datdt/k8sselfhost/internal/domain/deployment"
	"github.com/datdt/k8sselfhost/internal/domain/fleet"
	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	infraCluster "github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	infraDocker "github.com/datdt/k8sselfhost/internal/infrastructure/provider/docker"
)

type deploymentRepo struct {
	defaultK8sClient  kubernetes.Interface
	defaultDockerRepo domainDocker.Repository
	fleetRepo         fleet.Repository
	clientManager     *infraCluster.ClientManager
}

// NewDeploymentRepo creates a new live deployment repository.
func NewDeploymentRepo(
	defaultK8sClient *kubernetes.Clientset,
	defaultDockerRepo domainDocker.Repository,
	fleetRepo fleet.Repository,
	clientManager *infraCluster.ClientManager,
) deployment.Repository {
	return &deploymentRepo{
		defaultK8sClient:  defaultK8sClient,
		defaultDockerRepo: defaultDockerRepo,
		fleetRepo:         fleetRepo,
		clientManager:     clientManager,
	}
}

// NewDeploymentRepoWithInterface creates a deployment repository with any kubernetes.Interface.
func NewDeploymentRepoWithInterface(
	defaultK8sClient kubernetes.Interface,
	defaultDockerRepo domainDocker.Repository,
	fleetRepo fleet.Repository,
	clientManager *infraCluster.ClientManager,
) deployment.Repository {
	return &deploymentRepo{
		defaultK8sClient:  defaultK8sClient,
		defaultDockerRepo: defaultDockerRepo,
		fleetRepo:         fleetRepo,
		clientManager:     clientManager,
	}
}

func isNilK8sClient(client kubernetes.Interface) bool {
	if client == nil {
		return true
	}
	if cs, ok := client.(*kubernetes.Clientset); ok && cs == nil {
		return true
	}
	return false
}

func (r *deploymentRepo) getK8sClient(ctx context.Context, clusterName string) (kubernetes.Interface, error) {
	if r.clientManager != nil && clusterName != "" {
		clusters, err := r.fleetRepo.ListClusters(ctx)
		if err == nil {
			for _, c := range clusters {
				if c.Name == clusterName {
					cli, err := r.clientManager.GetK8sClient(ctx, c.ID)
					if err == nil && cli != nil {
						return cli, nil
					}
				}
			}
		}
	}
	if !isNilK8sClient(r.defaultK8sClient) {
		return r.defaultK8sClient, nil
	}
	return nil, fmt.Errorf("kubernetes client not found for cluster %s", clusterName)
}

func (r *deploymentRepo) getDockerClient(ctx context.Context, clusterName string) (domainDocker.Repository, error) {
	if r.clientManager != nil && clusterName != "" {
		clusters, err := r.fleetRepo.ListClusters(ctx)
		if err == nil {
			for _, c := range clusters {
				if c.Name == clusterName {
					cli, err := r.clientManager.GetDockerClient(ctx, c.ID)
					if err == nil && cli != nil {
						return infraDocker.NewDockerRepoWithClient(cli), nil
					}
				}
			}
		}
	}
	if r.defaultDockerRepo != nil {
		return r.defaultDockerRepo, nil
	}
	return nil, fmt.Errorf("docker repository not found for cluster %s", clusterName)
}

func (r *deploymentRepo) List(ctx context.Context) ([]deployment.Application, error) {
	var apps []deployment.Application

	var clusters []fleet.Cluster
	var err error
	if r.fleetRepo != nil {
		clusters, err = r.fleetRepo.ListClusters(ctx)
	}

	if err == nil && len(clusters) > 0 {
		for _, c := range clusters {
			if c.Provider == "docker" {
				dCli, err := r.clientManager.GetDockerClient(ctx, c.ID)
				if err == nil && dCli != nil {
					repo := infraDocker.NewDockerRepoWithClient(dCli)
					svcs, err := repo.ListServices(ctx)
					if err == nil {
						for _, s := range svcs {
							apps = append(apps, r.mapSwarmService(s, c.Name))
						}
					}
				}
			} else {
				kCli, err := r.clientManager.GetK8sClient(ctx, c.ID)
				if err == nil && kCli != nil {
					deps, err := kCli.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
					if err == nil {
						for _, d := range deps.Items {
							apps = append(apps, r.mapK8sDeployment(d, c.Name))
						}
					}
				}
			}
		}
	}

	if len(apps) == 0 {
		// Fallback to local default clients if available
		if !isNilK8sClient(r.defaultK8sClient) {
			deps, err := r.defaultK8sClient.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
			if err == nil {
				for _, d := range deps.Items {
					apps = append(apps, r.mapK8sDeployment(d, "prod-us-east"))
				}
			}
		}
		if r.defaultDockerRepo != nil {
			svcs, err := r.defaultDockerRepo.ListServices(ctx)
			if err == nil {
				for _, s := range svcs {
					apps = append(apps, r.mapSwarmService(s, "swarm-cluster"))
				}
			}
		}
	}

	return apps, nil
}

func (r *deploymentRepo) Scale(ctx context.Context, targetType, targetCluster, namespace, name string, replicas int) error {
	if targetType == "kubernetes" {
		return r.scaleK8sDeployment(ctx, targetCluster, namespace, name, replicas)
	} else if targetType == "swarm" || targetType == "docker" {
		return r.scaleSwarmService(ctx, targetCluster, name, replicas)
	}
	return fmt.Errorf("unsupported target type: %s", targetType)
}

func (r *deploymentRepo) Restart(ctx context.Context, targetType, targetCluster, namespace, name string) error {
	if targetType == "kubernetes" {
		return r.restartK8sDeployment(ctx, targetCluster, namespace, name)
	} else if targetType == "swarm" || targetType == "docker" {
		return r.restartSwarmService(ctx, targetCluster, name)
	}
	return fmt.Errorf("unsupported target type: %s", targetType)
}

func (r *deploymentRepo) Delete(ctx context.Context, targetType, targetCluster, namespace, name string) error {
	if targetType == "kubernetes" {
		return r.deleteK8sDeployment(ctx, targetCluster, namespace, name)
	} else if targetType == "swarm" || targetType == "docker" {
		return r.deleteSwarmService(ctx, targetCluster, name)
	}
	return fmt.Errorf("unsupported target type: %s", targetType)
}

func (r *deploymentRepo) Create(ctx context.Context, app deployment.Application) error {
	if app.Type == "kubernetes" {
		return r.createK8sDeployment(ctx, app)
	} else if app.Type == "swarm" || app.Type == "docker" {
		return r.createSwarmService(ctx, app)
	}
	return fmt.Errorf("unsupported target type: %s", app.Type)
}

func (r *deploymentRepo) UpdateResources(ctx context.Context, targetType, targetCluster, namespace, name string, memoryLimitBytes, memoryReservBytes, nanoCPUs int64, replicas int) error {
	if targetType == "kubernetes" || targetType == "" {
		return r.updateK8sResources(ctx, targetCluster, namespace, name, memoryLimitBytes, memoryReservBytes, nanoCPUs, replicas)
	} else if targetType == "swarm" || targetType == "docker" {
		return r.updateSwarmResources(ctx, targetCluster, name, memoryLimitBytes, memoryReservBytes, nanoCPUs, replicas)
	}
	return fmt.Errorf("unsupported target type: %s", targetType)
}
