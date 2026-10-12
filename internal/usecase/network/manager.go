package network

import (
	"context"
	"fmt"
	"reflect"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	domainnet "github.com/datdt/k8sselfhost/internal/domain/network"
	"github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
)

var _ domainnet.NetworkManager = (*NetworkUsecase)(nil)

// NetworkUsecase orchestrates CNI IPAM tracking, lease GC, and CoreDNS thundering herd defense.
type NetworkUsecase struct {
	k8sClient     kubernetes.Interface
	clientManager *cluster.ClientManager
	ipamBackend   domainnet.IPAMBackend
	dnsMetrics    domainnet.DNSMetricsProvider
	logger        *zap.Logger
}

// isNilK8sClient safely inspects whether a kubernetes.Interface contains a nil pointer or interface.
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

// NewNetworkUsecase initializes a production-grade NetworkUsecase instance.
func NewNetworkUsecase(
	k8sClient kubernetes.Interface,
	clientManager *cluster.ClientManager,
	ipamBackend domainnet.IPAMBackend,
	dnsMetrics domainnet.DNSMetricsProvider,
	logger *zap.Logger,
) *NetworkUsecase {
	if logger == nil {
		logger = zap.NewNop()
	}
	if isNilK8sClient(k8sClient) {
		k8sClient = nil
	}
	if ipamBackend == nil {
		ipamBackend = NewInMemoryIPAMBackend()
	}
	if dnsMetrics == nil {
		dnsMetrics = NewDefaultDNSMetricsProvider()
	}

	return &NetworkUsecase{
		k8sClient:     k8sClient,
		clientManager: clientManager,
		ipamBackend:   ipamBackend,
		dnsMetrics:    dnsMetrics,
		logger:        logger,
	}
}

// getK8sClient dynamically resolves the Kubernetes client for the given clusterID.
func (u *NetworkUsecase) getK8sClient(ctx context.Context, clusterID string) (kubernetes.Interface, error) {
	if clusterID == "default" || clusterID == "" || clusterID == "local" || clusterID == "in-cluster" {
		if u.k8sClient != nil && !isNilK8sClient(u.k8sClient) {
			return u.k8sClient, nil
		}
		if u.clientManager != nil {
			targetID := clusterID
			if targetID != "" {
				if cli, err := u.clientManager.GetK8sClient(ctx, targetID); err == nil && !isNilK8sClient(cli) {
					return cli, nil
				}
			}
			for _, fallbackID := range []string{"default", "local"} {
				if fallbackID != targetID {
					if cli, err := u.clientManager.GetK8sClient(ctx, fallbackID); err == nil && !isNilK8sClient(cli) {
						return cli, nil
					}
				}
			}
		}
	} else {
		if u.clientManager != nil {
			if cli, err := u.clientManager.GetK8sClient(ctx, clusterID); err == nil && !isNilK8sClient(cli) {
				return cli, nil
			}
		}
		if u.k8sClient != nil && !isNilK8sClient(u.k8sClient) {
			return u.k8sClient, nil
		}
	}

	return nil, fmt.Errorf("%w for cluster '%s'", domainnet.ErrClusterUnavailable, clusterID)
}

// GetIPAMUtilization calculates the CNI IP pool allocation and leaked leases across all cluster nodes.
func (u *NetworkUsecase) GetIPAMUtilization(ctx context.Context, clusterID string) ([]domainnet.IPAMUtilization, error) {
	client, err := u.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("resolving cluster %s: %w", clusterID, err)
	}

	nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing cluster nodes: %w", err)
	}

	podList, err := client.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing cluster pods: %w", err)
	}

	result := make([]domainnet.IPAMUtilization, 0, len(nodeList.Items))
	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		util := calculateNodeUtilization(ctx, clusterID, node, podList.Items, u.ipamBackend, u.logger)
		result = append(result, util)
	}

	return result, nil
}

// TriggerIPAMGarbageCollection purges unreferenced orphaned IP leases on a node while protecting active pods.
func (u *NetworkUsecase) TriggerIPAMGarbageCollection(ctx context.Context, clusterID, nodeName string) (int, error) {
	client, err := u.getK8sClient(ctx, clusterID)
	if err != nil {
		return 0, fmt.Errorf("resolving cluster %s: %w", clusterID, err)
	}

	return performIPAMGarbageCollection(ctx, clusterID, nodeName, client, u.ipamBackend, u.logger)
}

// CheckDNSHealth checks CoreDNS availability, replica count, NodeLocal status, and latency SLA.
func (u *NetworkUsecase) CheckDNSHealth(ctx context.Context, clusterID string) (*domainnet.DNSHealthReport, error) {
	client, err := u.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("resolving cluster %s: %w", clusterID, err)
	}

	return performDNSHealthCheck(ctx, clusterID, client, u.dnsMetrics, u.logger)
}

// InjectDNSConfigOptimization updates Deployments to inject ndots:2 and single-request-reopen options.
func (u *NetworkUsecase) InjectDNSConfigOptimization(ctx context.Context, clusterID, namespace string) error {
	client, err := u.getK8sClient(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("resolving cluster %s: %w", clusterID, err)
	}

	return performInjectDNSConfigOptimization(ctx, clusterID, namespace, client, u.logger)
}
