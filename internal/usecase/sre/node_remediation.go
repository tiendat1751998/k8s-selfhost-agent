package sre

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
)

// RemediationResult contains details of the node fast-failover remediation operation.
type RemediationResult struct {
	NodeName    string   `json:"node_name"`
	EvictedPods []string `json:"evicted_pods"`
	Timestamp   string   `json:"timestamp"`
	DurationMs  int64    `json:"duration_ms"`
}

// RemediationController handles automated node remediation (<30s fast failover) for offline/crashing nodes.
type RemediationController struct {
	k8sClient     kubernetes.Interface
	clientManager *cluster.ClientManager
	logger        *zap.Logger
}

// isNilK8sClient checks if a kubernetes.Interface is nil or holds a typed nil value.
func isNilK8sClient(client kubernetes.Interface) bool {
	if client == nil {
		return true
	}
	val := reflect.ValueOf(client)
	switch val.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return val.IsNil()
	default:
		return false
	}
}

// NewRemediationController constructs a new RemediationController instance.
func NewRemediationController(k8sClient kubernetes.Interface, clientManager *cluster.ClientManager, logger *zap.Logger) *RemediationController {
	if logger == nil {
		logger = zap.NewNop()
	}
	if isNilK8sClient(k8sClient) {
		k8sClient = nil
	}
	return &RemediationController{
		k8sClient:     k8sClient,
		clientManager: clientManager,
		logger:        logger,
	}
}

// getK8sClient resolves the kubernetes.Interface for the given clusterID.
func (c *RemediationController) getK8sClient(ctx context.Context, clusterID string) (kubernetes.Interface, error) {
	if clusterID == "default" || clusterID == "" || clusterID == "local" {
		if c.k8sClient != nil && !isNilK8sClient(c.k8sClient) {
			return c.k8sClient, nil
		}
		if c.clientManager != nil {
			targetID := clusterID
			if targetID != "" {
				if cli, err := c.clientManager.GetK8sClient(ctx, targetID); err == nil && !isNilK8sClient(cli) {
					return cli, nil
				}
			}
			for _, fallbackID := range []string{"default", "local"} {
				if fallbackID != targetID {
					if cli, err := c.clientManager.GetK8sClient(ctx, fallbackID); err == nil && !isNilK8sClient(cli) {
						return cli, nil
					}
				}
			}
		}
	} else {
		if c.clientManager != nil {
			if cli, err := c.clientManager.GetK8sClient(ctx, clusterID); err == nil && !isNilK8sClient(cli) {
				return cli, nil
			}
		}
		if c.k8sClient != nil && !isNilK8sClient(c.k8sClient) {
			return c.k8sClient, nil
		}
	}

	return nil, fmt.Errorf("kubernetes client unavailable for cluster '%s': no active cluster connection", clusterID)
}

// HandleNodeOffline performs fast-failover remediation on an offline or crashing node:
// 1. Cordons the node (spec.unschedulable = true)
// 2. Lists pods scheduled on nodeName
// 3. Filters pods that are stuck in Terminating or whose node is not ready
// 4. Force deletes them (grace period 0) to trigger immediate re-scheduling by controllers
// 5. Returns RemediationResult
func (c *RemediationController) HandleNodeOffline(ctx context.Context, clusterID, nodeName string) (*RemediationResult, error) {
	startTime := time.Now()

	if nodeName == "" {
		return nil, fmt.Errorf("nodeName is required")
	}

	client, err := c.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("resolving k8s client for cluster %s: %w", clusterID, err)
	}
	if client == nil || isNilK8sClient(client) {
		return nil, fmt.Errorf("kubernetes client unavailable for cluster '%s'", clusterID)
	}

	// 1. Fetch node and cordon it (patch spec.unschedulable = true)
	node, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting node %s: %w", nodeName, err)
	}

	// Strategic merge patch for cordoning
	patchBytes := []byte(`{"spec":{"unschedulable":true}}`)
	if _, patchErr := client.CoreV1().Nodes().Patch(ctx, nodeName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}); patchErr != nil {
		c.logger.Debug("cordon strategic merge patch non-critical error", zap.String("node", nodeName), zap.Error(patchErr))
	}

	// Ensure node struct reflects unschedulable = true for fake and live clients
	if !node.Spec.Unschedulable {
		node.Spec.Unschedulable = true
		if _, err := client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{}); err != nil {
			return nil, fmt.Errorf("cordoning node %s: %w", nodeName, err)
		}
	}

	// Determine if node is not ready
	nodeNotReady := true
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			nodeNotReady = (cond.Status != corev1.ConditionTrue)
			break
		}
	}

	// 2. List pods scheduled on nodeName
	podList, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods on node %s: %w", nodeName, err)
	}

	evictedPods := make([]string, 0)
	deleteOpts := metav1.DeleteOptions{
		GracePeriodSeconds: new(int64), // 0 seconds for force deletion
	}

	// 3. Filter pods that are stuck in Terminating or whose node is not ready
	for _, pod := range podList.Items {
		// Filter by spec.nodeName (fieldSelector fallback for fake client)
		if pod.Spec.NodeName != "" && pod.Spec.NodeName != nodeName {
			continue
		}

		// Skip mirror pods
		if _, isMirror := pod.Annotations[corev1.MirrorPodAnnotationKey]; isMirror {
			continue
		}

		isTerminating := pod.DeletionTimestamp != nil
		if !isTerminating && !nodeNotReady {
			continue
		}

		// Skip pods already completed or failed unless terminating
		if (pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed) && !isTerminating {
			continue
		}

		// 4. Force delete pod
		ns := pod.Namespace
		if ns == "" {
			ns = "default"
		}

		if err := client.CoreV1().Pods(ns).Delete(ctx, pod.Name, deleteOpts); err != nil && !k8serrors.IsNotFound(err) {
			c.logger.Warn("failed to force delete pod during remediation",
				zap.String("cluster", clusterID),
				zap.String("node", nodeName),
				zap.String("pod", pod.Name),
				zap.Error(err),
			)
			continue
		}

		evictedPods = append(evictedPods, pod.Name)
	}

	c.logger.Info("node remediation completed",
		zap.String("cluster", clusterID),
		zap.String("node", nodeName),
		zap.Int("evicted_count", len(evictedPods)),
	)

	// 5. Return RemediationResult
	return &RemediationResult{
		NodeName:    nodeName,
		EvictedPods: evictedPods,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		DurationMs:  time.Since(startTime).Milliseconds(),
	}, nil
}