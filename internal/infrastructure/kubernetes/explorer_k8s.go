package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/datdt/k8sselfhost/internal/domain/explorer"
)

func (r *explorerRepo) searchK8s(ctx context.Context, client *kubernetes.Clientset, kind, cluster, namespace, query string, limit, offset int) ([]explorer.Resource, int, error) {
	var results []explorer.Resource

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

func (r *explorerRepo) getK8sByID(ctx context.Context, client *kubernetes.Clientset, id, clusterName, kind, namespace, name string) (*explorer.Resource, error) {
	switch strings.ToLower(kind) {
	case "pod", "pods":
		p, err := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &explorer.Resource{
			ID:        id,
			Kind:      "Pod",
			Name:      p.Name,
			Namespace: p.Namespace,
			Cluster:   clusterName,
			Status:    string(p.Status.Phase),
			Labels:    p.Labels,
			Age:       time.Since(p.CreationTimestamp.Time).String(),
		}, nil
	case "deployment", "deployments":
		d, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		status := "Ready"
		if d.Status.ReadyReplicas < *d.Spec.Replicas {
			status = "NotReady"
		}
		return &explorer.Resource{
			ID:        id,
			Kind:      "Deployment",
			Name:      d.Name,
			Namespace: d.Namespace,
			Cluster:   clusterName,
			Status:    status,
			Labels:    d.Labels,
			Age:       time.Since(d.CreationTimestamp.Time).String(),
		}, nil
	case "service", "services":
		s, err := client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &explorer.Resource{
			ID:        id,
			Kind:      "Service",
			Name:      s.Name,
			Namespace: s.Namespace,
			Cluster:   clusterName,
			Status:    "Active",
			Labels:    s.Labels,
			Age:       time.Since(s.CreationTimestamp.Time).String(),
		}, nil
	case "node", "nodes":
		n, err := client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		status := "NotReady"
		for _, cond := range n.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				status = "Ready"
				break
			}
		}
		return &explorer.Resource{
			ID:        id,
			Kind:      "Node",
			Name:      n.Name,
			Namespace: "",
			Cluster:   clusterName,
			Status:    status,
			Labels:    n.Labels,
			Age:       time.Since(n.CreationTimestamp.Time).String(),
		}, nil
	case "statefulset", "statefulsets":
		s, err := client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		status := "Ready"
		if s.Status.ReadyReplicas < s.Status.Replicas {
			status = "NotReady"
		}
		return &explorer.Resource{
			ID:        id,
			Kind:      "StatefulSet",
			Name:      s.Name,
			Namespace: s.Namespace,
			Cluster:   clusterName,
			Status:    status,
			Labels:    s.Labels,
			Age:       time.Since(s.CreationTimestamp.Time).String(),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported resource kind for GetByID: %s", kind)
	}
}

func (r *explorerRepo) syncK8sResource(ctx context.Context, client *kubernetes.Clientset, res *explorer.Resource) error {
	namespace := res.Namespace
	if namespace == "" {
		namespace = "default"
	}

	switch strings.ToLower(res.Kind) {
	case "pod", "pods":
		p, err := client.CoreV1().Pods(namespace).Get(ctx, res.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		p.Labels = res.Labels
		_, err = client.CoreV1().Pods(namespace).Update(ctx, p, metav1.UpdateOptions{})
		return err
	case "deployment", "deployments":
		d, err := client.AppsV1().Deployments(namespace).Get(ctx, res.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		d.Labels = res.Labels
		_, err = client.AppsV1().Deployments(namespace).Update(ctx, d, metav1.UpdateOptions{})
		return err
	case "service", "services":
		s, err := client.CoreV1().Services(namespace).Get(ctx, res.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		s.Labels = res.Labels
		_, err = client.CoreV1().Services(namespace).Update(ctx, s, metav1.UpdateOptions{})
		return err
	case "node", "nodes":
		n, err := client.CoreV1().Nodes().Get(ctx, res.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		n.Labels = res.Labels
		_, err = client.CoreV1().Nodes().Update(ctx, n, metav1.UpdateOptions{})
		return err
	case "statefulset", "statefulsets":
		s, err := client.AppsV1().StatefulSets(namespace).Get(ctx, res.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		s.Labels = res.Labels
		_, err = client.AppsV1().StatefulSets(namespace).Update(ctx, s, metav1.UpdateOptions{})
		return err
	default:
		return fmt.Errorf("unsupported resource kind for SyncResource: %s", res.Kind)
	}
}
