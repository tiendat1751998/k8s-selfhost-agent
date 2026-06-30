package deployment

import (
	"context"
	"fmt"

	"github.com/datdt/k8sselfhost/internal/domain/deployment"
)

// Usecase handles application-level orchestration logic for deployments.
type Usecase struct {
	repo deployment.Repository
}

// NewUsecase creates a new Usecase instance.
func NewUsecase(repo deployment.Repository) *Usecase {
	return &Usecase{repo: repo}
}

// ListDeployments retrieves all applications across registered clusters.
func (u *Usecase) ListDeployments(ctx context.Context) ([]deployment.Application, error) {
	return u.repo.List(ctx)
}

// ScaleDeployment modifies replica count of a deployment or swarm service.
func (u *Usecase) ScaleDeployment(ctx context.Context, targetType, targetCluster, namespace, name string, replicas int) error {
	if name == "" {
		return fmt.Errorf("application name cannot be empty")
	}
	if replicas < 0 || replicas > 100 {
		return fmt.Errorf("invalid replica count: must be between 0 and 100")
	}
	return u.repo.Scale(ctx, targetType, targetCluster, namespace, name, replicas)
}

// RestartDeployment triggers a rolling restart for a deployment or swarm service.
func (u *Usecase) RestartDeployment(ctx context.Context, targetType, targetCluster, namespace, name string) error {
	if name == "" {
		return fmt.Errorf("application name cannot be empty")
	}
	return u.repo.Restart(ctx, targetType, targetCluster, namespace, name)
}

// DeleteDeployment deletes a deployment or swarm service.
func (u *Usecase) DeleteDeployment(ctx context.Context, targetType, targetCluster, namespace, name string) error {
	if name == "" {
		return fmt.Errorf("application name cannot be empty")
	}
	return u.repo.Delete(ctx, targetType, targetCluster, namespace, name)
}

// CreateDeployment creates a deployment or swarm service.
func (u *Usecase) CreateDeployment(ctx context.Context, app deployment.Application) error {
	if app.Name == "" {
		return fmt.Errorf("application name cannot be empty")
	}
	if app.Image == "" {
		return fmt.Errorf("container image cannot be empty")
	}
	if app.Replicas < 0 || app.Replicas > 100 {
		return fmt.Errorf("invalid replica count: must be between 0 and 100")
	}
	if app.CPU == "" {
		app.CPU = "100m"
	}
	if app.Memory == "" {
		app.Memory = "128Mi"
	}
	if app.Port <= 0 {
		app.Port = 80
	}
	if app.NetType == "" {
		app.NetType = "ClusterIP"
	}
	if app.Team == "" {
		app.Team = "SRE"
	}
	if app.Env == "" {
		app.Env = "production"
	}
	return u.repo.Create(ctx, app)
}
