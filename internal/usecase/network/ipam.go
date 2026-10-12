package network

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	domainnet "github.com/datdt/k8sselfhost/internal/domain/network"
)

// InMemoryIPAMBackend provides a thread-safe in-memory store for CNI IPAM leases.
type InMemoryIPAMBackend struct {
	mu     sync.RWMutex
	leases map[string]map[string][]domainnet.IPAMLease // clusterID -> nodeName -> []IPAMLease
}

// NewInMemoryIPAMBackend creates a new instance of InMemoryIPAMBackend.
func NewInMemoryIPAMBackend() *InMemoryIPAMBackend {
	return &InMemoryIPAMBackend{
		leases: make(map[string]map[string][]domainnet.IPAMLease),
	}
}

// GetAllocatedLeases returns all recorded leases for a given cluster and node.
func (b *InMemoryIPAMBackend) GetAllocatedLeases(_ context.Context, clusterID, nodeName string) ([]domainnet.IPAMLease, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	nodeMap, ok := b.leases[clusterID]
	if !ok {
		return nil, nil
	}
	existing := nodeMap[nodeName]
	result := make([]domainnet.IPAMLease, len(existing))
	copy(result, existing)
	return result, nil
}

// ReleaseLeases removes matching IP addresses from the node lease pool.
func (b *InMemoryIPAMBackend) ReleaseLeases(_ context.Context, clusterID, nodeName string, ips []string) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	nodeMap, ok := b.leases[clusterID]
	if !ok {
		return 0, nil
	}
	existing, ok := nodeMap[nodeName]
	if !ok {
		return 0, nil
	}

	ipSet := make(map[string]struct{}, len(ips))
	for _, ip := range ips {
		ipSet[ip] = struct{}{}
	}

	var remaining []domainnet.IPAMLease
	released := 0
	for _, l := range existing {
		if _, shouldRelease := ipSet[l.IP]; shouldRelease {
			released++
		} else {
			remaining = append(remaining, l)
		}
	}

	nodeMap[nodeName] = remaining
	return released, nil
}

// RegisterLease registers a new IP lease into the store.
func (b *InMemoryIPAMBackend) RegisterLease(_ context.Context, clusterID string, lease domainnet.IPAMLease) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.leases[clusterID]; !ok {
		b.leases[clusterID] = make(map[string][]domainnet.IPAMLease)
	}
	b.leases[clusterID][lease.NodeName] = append(b.leases[clusterID][lease.NodeName], lease)
	return nil
}

// RegisterLeases registers multiple IP leases into the store.
func (b *InMemoryIPAMBackend) RegisterLeases(ctx context.Context, clusterID string, leases []domainnet.IPAMLease) error {
	for _, l := range leases {
		if err := b.RegisterLease(ctx, clusterID, l); err != nil {
			return err
		}
	}
	return nil
}

// isPodActive checks if a pod is actively running, pending, or in container creation.
func isPodActive(pod *corev1.Pod) bool {
	if pod.DeletionTimestamp != nil {
		return false
	}
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return false
	}
	if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
		return true
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason == "ContainerCreating" {
			return true
		}
	}
	for _, ics := range pod.Status.InitContainerStatuses {
		if ics.State.Waiting != nil && ics.State.Waiting.Reason == "ContainerCreating" {
			return true
		}
	}
	return false
}

// calculateNodeUtilization analyzes IPAM utilization and leaked orphan IPs for a single node.
func calculateNodeUtilization(
	ctx context.Context,
	clusterID string,
	node *corev1.Node,
	pods []corev1.Pod,
	ipamBackend domainnet.IPAMBackend,
	logger *zap.Logger,
) domainnet.IPAMUtilization {
	cidr := node.Spec.PodCIDR
	if cidr == "" && len(node.Spec.PodCIDRs) > 0 {
		cidr = node.Spec.PodCIDRs[0]
	}

	totalAllocatable := domainnet.CalculateTotalAllocatable(cidr)
	if allocPods, ok := node.Status.Allocatable[corev1.ResourcePods]; ok && totalAllocatable <= 0 {
		totalAllocatable = int(allocPods.Value())
	}
	if totalAllocatable <= 0 {
		totalAllocatable = 254
	}

	activePodCount := 0
	for i := range pods {
		pod := &pods[i]
		if pod.Spec.NodeName == node.Name && isPodActive(pod) {
			activePodCount++
		}
	}

	allocatedIPs := activePodCount
	if ipamBackend != nil {
		leases, err := ipamBackend.GetAllocatedLeases(ctx, clusterID, node.Name)
		if err != nil {
			logger.Warn("Failed to fetch CNI leases from backend, defaulting to active pods",
				zap.String("cluster_id", clusterID),
				zap.String("node_name", node.Name),
				zap.Error(err),
			)
		} else if len(leases) > 0 {
			allocatedIPs = len(leases)
		}
	}

	util := domainnet.NewIPAMUtilization(node.Name, cidr, totalAllocatable, allocatedIPs, activePodCount)
	if util.Status == domainnet.StatusWarning {
		logger.Warn("CNI IPAM high utilization or lease leak warning detected",
			zap.String("cluster_id", clusterID),
			zap.String("node_name", node.Name),
			zap.Float64("usage_percent", util.UsagePercentage),
			zap.Int("leaked_orphans", util.LeakedOrphanIPs),
		)
	} else if util.Status == domainnet.StatusExhausted {
		logger.Error("CNI IPAM address pool exhausted",
			zap.String("cluster_id", clusterID),
			zap.String("node_name", node.Name),
			zap.Float64("usage_percent", util.UsagePercentage),
			zap.Int("allocated", util.AllocatedIPs),
			zap.Int("total", util.TotalAllocatable),
		)
	}

	return util
}

// performIPAMGarbageCollection reclaims orphaned IP handles on a node while preserving active pods.
func performIPAMGarbageCollection(
	ctx context.Context,
	clusterID, nodeName string,
	client kubernetes.Interface,
	ipamBackend domainnet.IPAMBackend,
	logger *zap.Logger,
) (int, error) {
	if nodeName == "" {
		return 0, fmt.Errorf("nodeName is required")
	}
	if ipamBackend == nil {
		return 0, nil
	}

	// 1. Verify node exists in cluster
	_, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("%w: %s", domainnet.ErrNodeNotFound, nodeName)
	}

	// 2. Fetch all pods on the node to identify active and protected pods
	podList, err := client.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("listing pods for IPAM garbage collection: %w", err)
	}

	protectedIPs := make(map[string]bool)
	activePodKeys := make(map[string]bool)

	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.Spec.NodeName != nodeName {
			continue
		}

		if isPodActive(pod) {
			activePodKeys[pod.Namespace+"/"+pod.Name] = true
			if pod.Status.PodIP != "" {
				protectedIPs[pod.Status.PodIP] = true
			}
			for _, podIP := range pod.Status.PodIPs {
				if podIP.IP != "" {
					protectedIPs[podIP.IP] = true
				}
			}
		}
	}

	// 3. Fetch allocated leases from CNI IPAM backend
	leases, err := ipamBackend.GetAllocatedLeases(ctx, clusterID, nodeName)
	if err != nil {
		return 0, fmt.Errorf("fetching allocated leases for node %s: %w", nodeName, err)
	}

	// 4. Find orphaned unreferenced leases
	var orphanIPs []string
	for _, l := range leases {
		podKey := l.Namespace + "/" + l.PodName
		if protectedIPs[l.IP] || activePodKeys[podKey] {
			// Pod is actively running or container is creating - DO NOT touch
			continue
		}
		orphanIPs = append(orphanIPs, l.IP)
	}

	if len(orphanIPs) == 0 {
		return 0, nil
	}

	// 5. Release orphaned IP blocks
	released, err := ipamBackend.ReleaseLeases(ctx, clusterID, nodeName, orphanIPs)
	if err != nil {
		return 0, fmt.Errorf("releasing orphaned IP leases on node %s: %w", nodeName, err)
	}

	logger.Info("CNI IPAM garbage collection completed successfully",
		zap.String("cluster_id", clusterID),
		zap.String("node_name", nodeName),
		zap.Int("released_ips", released),
		zap.Time("timestamp", time.Now().UTC()),
	)

	return released, nil
}
