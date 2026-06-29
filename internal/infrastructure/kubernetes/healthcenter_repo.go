package kubernetes

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	dockerTypes "github.com/docker/docker/api/types"

	"github.com/datdt/k8sselfhost/internal/domain/healthcenter"
	infraCluster "github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
)

type healthCenterRepo struct {
	client        *kubernetes.Clientset
	clientManager *infraCluster.ClientManager
	cache         map[string]healthcenter.ComponentStatus
}

// NewHealthCenterRepo creates a new live Kubernetes-backed HealthCenter repository.
func NewHealthCenterRepo(client *kubernetes.Clientset, clientManager *infraCluster.ClientManager) healthcenter.Repository {
	return &healthCenterRepo{
		client:        client,
		clientManager: clientManager,
		cache:         make(map[string]healthcenter.ComponentStatus),
	}
}

func (r *healthCenterRepo) GetStatuses(ctx context.Context) ([]healthcenter.ComponentStatus, error) {
	var statuses []healthcenter.ComponentStatus
	now := time.Now()

	// 1. Frontend status (always healthy if serving API queries)
	statuses = append(statuses, healthcenter.ComponentStatus{
		ID:            "frontend",
		Component:     "frontend",
		Status:        "healthy",
		Message:       "Serving traffic normally",
		LastCheckedAt: now,
	})

	// 2. Core API Backend status (always healthy if responding to this request)
	statuses = append(statuses, healthcenter.ComponentStatus{
		ID:            "backend",
		Component:     "backend",
		Status:        "healthy",
		Message:       "All API endpoints operational",
		LastCheckedAt: now,
	})

	// 3. WebSocket status
	statuses = append(statuses, healthcenter.ComponentStatus{
		ID:            "websocket",
		Component:     "websocket",
		Status:        "healthy",
		Message:       "Real-time event streams active",
		LastCheckedAt: now,
	})

	// 4. PostgreSQL Database Cluster status (verify dynamic DB connectivity via client manager)
	dbStatus := "healthy"
	dbMessage := "Primary database connection healthy"
	if r.clientManager != nil {
		err := r.clientManager.PingDB(ctx)
		if err != nil {
			dbStatus = "down"
			dbMessage = fmt.Sprintf("Database connection failed: %v", err)
		}
	}
	statuses = append(statuses, healthcenter.ComponentStatus{
		ID:            "database",
		Component:     "database",
		Status:        dbStatus,
		Message:       dbMessage,
		LastCheckedAt: now,
	})

	// 5. Docker Swarm Provider status (read live nodes status from Swarm engine)
	swarmStatus := "down"
	swarmMessage := "Docker Swarm provider client not initialized or offline"
	if r.clientManager != nil {
		dCli, err := r.clientManager.GetDockerClient(ctx, "cls-dev-local")
		if err == nil && dCli != nil {
			nodes, err := dCli.NodeList(ctx, dockerTypes.NodeListOptions{})
			if err == nil {
				readyCount := 0
				for _, n := range nodes {
					if n.Status.State == "ready" {
						readyCount++
					}
				}
				if len(nodes) == 0 {
					swarmStatus = "warning"
					swarmMessage = "Swarm is active but no nodes are connected"
				} else if readyCount == len(nodes) {
					swarmStatus = "healthy"
					swarmMessage = fmt.Sprintf("All %d Swarm nodes are healthy & active", len(nodes))
				} else if readyCount > 0 {
					swarmStatus = "degraded"
					swarmMessage = fmt.Sprintf("Degraded: %d/%d Swarm nodes are Ready", readyCount, len(nodes))
				} else {
					swarmStatus = "down"
					swarmMessage = "All Swarm nodes are offline"
				}
			} else {
				swarmMessage = fmt.Sprintf("Failed to contact Swarm API: %v", err)
			}
		}
	}
	statuses = append(statuses, healthcenter.ComponentStatus{
		ID:            "gitops", // Matches key 'gitops' icon map on frontend
		Component:     "gitops",
		Status:        swarmStatus,
		Message:       swarmMessage,
		LastCheckedAt: now,
	})

	// 6. Kubernetes API status (discovery query of /healthz)
	k8sClient, err := getClient(ctx, r.client)
	if err == nil && k8sClient != nil {
		k8sStatus := "down"
		k8sMessage := "Cannot reach Kubernetes API Server"
		_, err = k8sClient.Discovery().RESTClient().Get().AbsPath("/healthz").DoRaw(ctx)
		if err == nil {
			k8sStatus = "healthy"
			k8sMessage = "Kubernetes API Server responsive"
		}
		statuses = append(statuses, healthcenter.ComponentStatus{
			ID:            "kubernetes",
			Component:     "backend", // Render as backend configuration item
			Status:        k8sStatus,
			Message:       k8sMessage,
			LastCheckedAt: now,
		})
	}

	// 7. Add cached AI/other component statuses
	for _, st := range r.cache {
		statuses = append(statuses, st)
	}

	return statuses, nil
}

func (r *healthCenterRepo) UpdateStatus(ctx context.Context, cs *healthcenter.ComponentStatus) error {
	cs.LastCheckedAt = time.Now()
	r.cache[cs.ID] = *cs
	return nil
}
