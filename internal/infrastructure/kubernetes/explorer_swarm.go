package kubernetes

import (
	"context"
	"fmt"

	"github.com/datdt/k8sselfhost/internal/domain/explorer"
	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

// searchSwarm eliminates fake Kubernetes resource masquerading for Swarm providers.
// When K8s API server (:6443) is unreachable, the system must not fabricate Pod/Deployment objects from Docker Swarm tasks.
func (r *explorerRepo) searchSwarm(ctx context.Context, activeDockerRepo domainDocker.Repository, kind, cluster, query string, limit, offset int) ([]explorer.Resource, int, error) {
	return nil, 0, fmt.Errorf("kubernetes API server (:6443) is unreachable on registered control-plane nodes. Docker Swarm services/tasks cannot masquerade as Kubernetes objects")
}
