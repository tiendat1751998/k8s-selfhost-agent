package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	var results []explorer.Resource
	kind = strings.ToLower(kind)

	if client == nil {
		if activeDockerRepo != nil {
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

	if client == nil {
		return nil, 0, fmt.Errorf("kubernetes client is not initialized")
	}

	// Default to all namespaces if empty
	if namespace == "" || namespace == "all" {
		namespace = metav1.NamespaceAll
	}

	switch kind {
	case "pod", "pods":
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, 0, fmt.Errorf("listing pods: %w", err)
		}
		for _, p := range pods.Items {
			if query != "" && !strings.Contains(p.Name, query) {
				continue
			}
			results = append(results, explorer.Resource{
				ID:        string(p.UID),
				Kind:      "Pod",
				Name:      p.Name,
				Namespace: p.Namespace,
				Cluster:   cluster,
				Status:    string(p.Status.Phase),
				Labels:    p.Labels,
				Age:       time.Since(p.CreationTimestamp.Time).String(),
			})
		}
	case "deployment", "deployments":
		deps, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, 0, fmt.Errorf("listing deployments: %w", err)
		}
		for _, d := range deps.Items {
			if query != "" && !strings.Contains(d.Name, query) {
				continue
			}
			status := "Ready"
			if d.Status.ReadyReplicas < *d.Spec.Replicas {
				status = "NotReady"
			}
			results = append(results, explorer.Resource{
				ID:        string(d.UID),
				Kind:      "Deployment",
				Name:      d.Name,
				Namespace: d.Namespace,
				Cluster:   cluster,
				Status:    status,
				Labels:    d.Labels,
				Age:       time.Since(d.CreationTimestamp.Time).String(),
			})
		}
	case "service", "services":
		svcs, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, 0, fmt.Errorf("listing services: %w", err)
		}
		for _, s := range svcs.Items {
			if query != "" && !strings.Contains(s.Name, query) {
				continue
			}
			results = append(results, explorer.Resource{
				ID:        string(s.UID),
				Kind:      "Service",
				Name:      s.Name,
				Namespace: s.Namespace,
				Cluster:   cluster,
				Status:    "Active",
				Labels:    s.Labels,
				Age:       time.Since(s.CreationTimestamp.Time).String(),
			})
		}
	case "node", "nodes":
		nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, 0, fmt.Errorf("listing nodes: %w", err)
		}
		for _, n := range nodes.Items {
			if query != "" && !strings.Contains(n.Name, query) {
				continue
			}
			status := "NotReady"
			for _, cond := range n.Status.Conditions {
				if cond.Type == "Ready" && cond.Status == "True" {
					status = "Ready"
					break
				}
			}
			results = append(results, explorer.Resource{
				ID:        string(n.UID),
				Kind:      "Node",
				Name:      n.Name,
				Namespace: "",
				Cluster:   cluster,
				Status:    status,
				Labels:    n.Labels,
				Age:       time.Since(n.CreationTimestamp.Time).String(),
			})
		}
	case "statefulset", "statefulsets":
		stsList, err := client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, 0, fmt.Errorf("listing statefulsets: %w", err)
		}
		for _, s := range stsList.Items {
			if query != "" && !strings.Contains(s.Name, query) {
				continue
			}
			status := "Ready"
			if s.Status.ReadyReplicas < s.Status.Replicas {
				status = "NotReady"
			}
			results = append(results, explorer.Resource{
				ID:        string(s.UID),
				Kind:      "StatefulSet",
				Name:      s.Name,
				Namespace: s.Namespace,
				Cluster:   cluster,
				Status:    status,
				Labels:    s.Labels,
				Age:       time.Since(s.CreationTimestamp.Time).String(),
			})
		}
	default:
		// Just a fallback to return empty list for unknown resources
		return []explorer.Resource{}, 0, nil
	}

	// Apply pagination (in memory since K8s API doesn't easily offset filter with search)
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

func (r *explorerRepo) GetByID(ctx context.Context, id string) (*explorer.Resource, error) {
	// Not easily implemented via live K8s API without knowing kind/namespace. 
	// In a real app we might store a cache in Postgres, or encode kind/ns in the ID.
	return nil, fmt.Errorf("GetByID not implemented in live K8s adapter")
}

func (r *explorerRepo) SyncResource(ctx context.Context, res *explorer.Resource) error {
	// No-op for live K8s adapter
	return nil
}
