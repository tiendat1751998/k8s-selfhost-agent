package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/explorer"
	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

func (r *explorerRepo) searchSwarm(ctx context.Context, activeDockerRepo domainDocker.Repository, kind, cluster, query string, limit, offset int) ([]explorer.Resource, int, error) {
	var results []explorer.Resource

	switch kind {
	case "node", "nodes":
		nodes, err := activeDockerRepo.ListNodes(ctx)
		if err == nil {
			for _, n := range nodes {
				roleLabel := "node-role.kubernetes.io/worker"
				if n.Role == "manager" {
					roleLabel = "node-role.kubernetes.io/control-plane"
				}
				status := "NotReady"
				if n.Status == "ready" {
					status = "Ready"
				}
				results = append(results, explorer.Resource{
					ID:      n.ID,
					Kind:    "Node",
					Name:    n.Name,
					Cluster: cluster,
					Status:  status,
					Labels: map[string]string{
						"kubernetes.io/hostname": n.Name,
						roleLabel:                "",
						"beta.kubernetes.io/arch": "amd64",
					},
					Age:     fmt.Sprintf("%dh", int(time.Since(n.UpdatedAt).Hours())),
				})
			}
		}
	case "pod", "pods":
		containers, err := activeDockerRepo.ListContainers(ctx)
		if err == nil {
			for _, c := range containers {
				status := "Running"
				if c.State != "running" {
					status = "Failed"
				}
				appName := c.Name
				if strings.HasPrefix(appName, "/") {
					appName = appName[1:]
				}
				results = append(results, explorer.Resource{
					ID:        c.ID,
					Kind:      "Pod",
					Name:      appName,
					Namespace: "default",
					Cluster:   cluster,
					Status:    status,
					Labels:    map[string]string{"app": appName},
					Age:       fmt.Sprintf("%dh", int(time.Since(c.Created).Hours())),
				})
			}
		}
	case "service", "services":
		services, err := activeDockerRepo.ListServices(ctx)
		if err == nil {
			for _, s := range services {
				results = append(results, explorer.Resource{
					ID:        s.ID,
					Kind:      "Service",
					Name:      s.Name,
					Namespace: "default",
					Cluster:   cluster,
					Status:    "Active",
					Age:       fmt.Sprintf("%dh", int(time.Since(s.UpdatedAt).Hours())),
				})
			}
		}
	case "deployment", "deployments":
		services, err := activeDockerRepo.ListServices(ctx)
		if err == nil {
			for _, s := range services {
				results = append(results, explorer.Resource{
					ID:        s.ID,
					Kind:      "Deployment",
					Name:      s.Name,
					Namespace: "default",
					Cluster:   cluster,
					Status:    "Ready",
					Age:       fmt.Sprintf("%dh", int(time.Since(s.UpdatedAt).Hours())),
				})
			}
		}
	}

	if len(results) == 0 {
		return nil, 0, fmt.Errorf("no live resources found on Docker Swarm provider or connection is down")
	}

	// Filter by query if present
	if query != "" {
		var filtered []explorer.Resource
		for _, res := range results {
			if strings.Contains(strings.ToLower(res.Name), strings.ToLower(query)) {
				filtered = append(filtered, res)
			}
		}
		results = filtered
	}

	total := len(results)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return results[start:end], total, nil
}
