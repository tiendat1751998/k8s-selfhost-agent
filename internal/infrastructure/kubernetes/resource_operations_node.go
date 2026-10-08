package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CordonNode sets node.Spec.Unschedulable to true.
func (r *ResourceRepo) CordonNode(ctx context.Context, clusterID, name string) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	node, err := client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting node %s: %w", name, err)
	}

	node.Spec.Unschedulable = true
	_, err = client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("cordoning node %s: %w", name, err)
	}
	return nil
}

// UncordonNode sets node.Spec.Unschedulable to false.
func (r *ResourceRepo) UncordonNode(ctx context.Context, clusterID, name string) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	node, err := client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting node %s: %w", name, err)
	}

	node.Spec.Unschedulable = false
	_, err = client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("uncordoning node %s: %w", name, err)
	}
	return nil
}

func isEvictablePod(p *corev1.Pod, nodeName string, ignoreDaemonSets bool) bool {
	if p.Spec.NodeName != "" && p.Spec.NodeName != nodeName {
		return false
	}
	// Skip mirror pods
	if _, isMirror := p.Annotations[corev1.MirrorPodAnnotationKey]; isMirror {
		return false
	}
	// Skip DaemonSet pods when ignoreDaemonSets is true
	if ignoreDaemonSets {
		for _, ref := range p.OwnerReferences {
			if strings.EqualFold(ref.Kind, "DaemonSet") {
				return false
			}
		}
	}
	return true
}

// DrainNode cordons the node, evicts pods via EvictV1 with 429 backoff,
// and polls until no evictable pods remain on the node or ctx deadline.
func (r *ResourceRepo) DrainNode(ctx context.Context, clusterID, name string, gracePeriodSeconds int64, ignoreDaemonSets bool) error {
	pollCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		timeout := time.Duration(gracePeriodSeconds)*time.Second + 30*time.Second
		if timeout < 60*time.Second {
			timeout = 60 * time.Second
		}
		var cancel context.CancelFunc
		pollCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	client, err := r.getK8sClient(pollCtx, clusterID)
	if err != nil {
		return err
	}

	// 1. Cordon the node
	if err := r.CordonNode(pollCtx, clusterID, name); err != nil {
		return fmt.Errorf("cordoning node during drain: %w", err)
	}

	// 2. List pods running on the node
	podList, err := client.CoreV1().Pods("").List(pollCtx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + name,
	})
	if err != nil {
		return fmt.Errorf("listing pods on node %s: %w", name, err)
	}

	// 3. Evict pods via EvictV1 with 429 retry
	for _, p := range podList.Items {
		if !isEvictablePod(&p, name, ignoreDaemonSets) {
			continue
		}

		eviction := &policyv1.Eviction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      p.Name,
				Namespace: p.Namespace,
			},
		}
		if gracePeriodSeconds >= 0 {
			eviction.DeleteOptions = &metav1.DeleteOptions{
				GracePeriodSeconds: &gracePeriodSeconds,
			}
		}

		b := backoff.NewExponentialBackOff()
		b.InitialInterval = 50 * time.Millisecond
		b.MaxInterval = 2 * time.Second

		operation := func() (struct{}, error) {
			evictErr := client.CoreV1().Pods(p.Namespace).EvictV1(pollCtx, eviction)
			if evictErr == nil || k8serrors.IsNotFound(evictErr) {
				return struct{}{}, nil
			}
			if k8serrors.IsTooManyRequests(evictErr) {
				return struct{}{}, evictErr
			}
			return struct{}{}, backoff.Permanent(evictErr)
		}

		if _, err := backoff.Retry(pollCtx, operation, backoff.WithBackOff(b)); err != nil {
			return fmt.Errorf("evicting pod %s/%s on node %s: %w", p.Namespace, p.Name, name, err)
		}
	}

	// 4. Poll until no evictable pods remain on the node or ctx deadline
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		remainingPods, err := client.CoreV1().Pods("").List(pollCtx, metav1.ListOptions{
			FieldSelector: "spec.nodeName=" + name,
		})
		if err != nil {
			return fmt.Errorf("listing remaining pods on node %s: %w", name, err)
		}

		evictableRemaining := 0
		for i := range remainingPods.Items {
			if isEvictablePod(&remainingPods.Items[i], name, ignoreDaemonSets) {
				evictableRemaining++
			}
		}

		if evictableRemaining == 0 {
			return nil
		}

		select {
		case <-pollCtx.Done():
			return fmt.Errorf("timed out waiting for %d evictable pods to terminate on node %s: %w", evictableRemaining, name, pollCtx.Err())
		case <-ticker.C:
		}
	}
}

// UpdateNodeTaints updates taints on the specified node.
func (r *ResourceRepo) UpdateNodeTaints(ctx context.Context, clusterID, name string, taints []corev1.Taint) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	node, err := client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting node %s: %w", name, err)
	}

	node.Spec.Taints = taints
	_, err = client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating taints for node %s: %w", name, err)
	}
	return nil
}

// UpdateNodeLabels updates labels on the specified node.
func (r *ResourceRepo) UpdateNodeLabels(ctx context.Context, clusterID, name string, labels map[string]string) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	node, err := client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting node %s: %w", name, err)
	}

	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}
	for k, v := range labels {
		node.Labels[k] = v
	}

	_, err = client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating labels for node %s: %w", name, err)
	}
	return nil
}

func (r *ResourceRepo) listNodeOrCluster(ctx context.Context, client kubernetes.Interface, kind, namespace string) ([]map[string]interface{}, bool, error) {
	var results []map[string]interface{}
	switch kind {
	case "serviceaccounts":
		list, err := client.CoreV1().ServiceAccounts(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing serviceaccounts: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "ServiceAccount")
			if err != nil {
				return nil, true, err
			}
			results = append(results, m)
		}
		return results, true, nil

	case "nodes":
		list, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing nodes: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "Node")
			if err != nil {
				return nil, true, err
			}
			results = append(results, m)
		}
		return results, true, nil

	case "events":
		list, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing events: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "Event")
			if err != nil {
				return nil, true, err
			}
			results = append(results, m)
		}
		return results, true, nil
	}

	return nil, false, nil
}

func (r *ResourceRepo) getNodeOrCluster(ctx context.Context, client kubernetes.Interface, kind, namespace, name string) (map[string]interface{}, bool, error) {
	switch kind {
	case "serviceaccounts":
		obj, err := client.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "v1", "ServiceAccount")
		return m, true, err

	case "nodes":
		obj, err := client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "v1", "Node")
		return m, true, err

	case "events":
		obj, err := client.CoreV1().Events(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "v1", "Event")
		return m, true, err
	}

	return nil, false, nil
}

func (r *ResourceRepo) createNodeOrCluster(ctx context.Context, client kubernetes.Interface, kind, namespace string, manifest map[string]interface{}) (map[string]interface{}, bool, error) {
	switch kind {
	case "serviceaccounts":
		var obj corev1.ServiceAccount
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding serviceaccount manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.CoreV1().ServiceAccounts(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("creating serviceaccount: %w", err)
		}
		m, err := toMap(created, "v1", "ServiceAccount")
		return m, true, err

	case "nodes":
		var obj corev1.Node
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding node manifest: %w", err)
		}
		created, err := client.CoreV1().Nodes().Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("creating node: %w", err)
		}
		m, err := toMap(created, "v1", "Node")
		return m, true, err
	}

	return nil, false, nil
}

func (r *ResourceRepo) updateNodeOrCluster(ctx context.Context, client kubernetes.Interface, kind, namespace, name string, manifest map[string]interface{}) (map[string]interface{}, bool, error) {
	switch kind {
	case "serviceaccounts":
		var obj corev1.ServiceAccount
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding serviceaccount manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.CoreV1().ServiceAccounts(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("updating serviceaccount %s: %w", name, err)
		}
		m, err := toMap(updated, "v1", "ServiceAccount")
		return m, true, err

	case "nodes":
		var obj corev1.Node
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding node manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		updated, err := client.CoreV1().Nodes().Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("updating node %s: %w", name, err)
		}
		m, err := toMap(updated, "v1", "Node")
		return m, true, err
	}

	return nil, false, nil
}

func (r *ResourceRepo) deleteNodeOrCluster(ctx context.Context, client kubernetes.Interface, kind, namespace, name string) (bool, error) {
	switch kind {
	case "serviceaccounts":
		return true, client.CoreV1().ServiceAccounts(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "nodes":
		return true, client.CoreV1().Nodes().Delete(ctx, name, metav1.DeleteOptions{})
	case "events":
		return true, client.CoreV1().Events(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}

	return false, nil
}
