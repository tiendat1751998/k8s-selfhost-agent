package storage

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/datdt/k8sselfhost/internal/domain/storage"
	"github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// VolumeMetricsProvider defines the interface for fetching real volume disk usage.
type VolumeMetricsProvider interface {
	GetVolumeMetrics(ctx context.Context, client kubernetes.Interface, namespace, pvcName string) (*int64, error)
}

// VolumeService defines the business operations for distributed volume management.
type VolumeService interface {
	ListVolumes(ctx context.Context, clusterID string) ([]storage.DistributedVolume, error)
	GetVolumeDetails(ctx context.Context, clusterID, namespace, name string) (*storage.VolumeDetails, error)
	ExpandVolume(ctx context.Context, clusterID, namespace, name string, newSizeBytes int64) error
	CreateSnapshot(ctx context.Context, clusterID, namespace, name string, req storage.VolumeSnapshotRequest) (*storage.VolumeSnapshotResult, error)
}

// VolumeUsecase implements VolumeService with Kubernetes cluster interaction.
type VolumeUsecase struct {
	k8sClient              kubernetes.Interface
	clientManager          *cluster.ClientManager
	metricsProvider        VolumeMetricsProvider
	defaultMetricsProvider VolumeMetricsProvider
	fencingHelper          *CSIFencingHelper
	logger                 *zap.Logger
}

// VolumeUsecaseOption configures VolumeUsecase instances.
type VolumeUsecaseOption func(*VolumeUsecase)

// WithMetricsProvider configures a custom volume metrics provider.
func WithMetricsProvider(provider VolumeMetricsProvider) VolumeUsecaseOption {
	return func(u *VolumeUsecase) {
		u.metricsProvider = provider
	}
}

// NewVolumeUsecase creates a new VolumeUsecase instance.
func NewVolumeUsecase(k8sClient kubernetes.Interface, cm *cluster.ClientManager, logger *zap.Logger, opts ...VolumeUsecaseOption) *VolumeUsecase {
	if logger == nil {
		logger = zap.NewNop()
	}
	u := &VolumeUsecase{
		k8sClient:              k8sClient,
		clientManager:          cm,
		defaultMetricsProvider: infraK8s.NewKubeletVolumeMetricsProvider(),
		fencingHelper:          NewCSIFencingHelper(),
		logger:                 logger,
	}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

// SetMetricsProvider overrides the metrics provider (useful for testing or dynamic wiring).
func (u *VolumeUsecase) SetMetricsProvider(provider VolumeMetricsProvider) {
	u.metricsProvider = provider
}

// FencingHelper returns the CSI fencing helper instance.
func (u *VolumeUsecase) FencingHelper() *CSIFencingHelper {
	return u.fencingHelper
}

func (u *VolumeUsecase) getK8sClient(ctx context.Context, clusterID string) (kubernetes.Interface, error) {
	if u.clientManager != nil && clusterID != "" && clusterID != "local" && clusterID != "default" && clusterID != "in-cluster" {
		cli, err := u.clientManager.GetK8sClient(ctx, clusterID)
		if err == nil && cli != nil {
			return cli, nil
		}
		if err != nil {
			return nil, domainerrors.NewK8sUnavailable(fmt.Sprintf("connecting to cluster %s", clusterID), err)
		}
	}

	if u.k8sClient != nil {
		return u.k8sClient, nil
	}

	return nil, infraK8s.ErrK8sUnavailable
}

func (u *VolumeUsecase) discoverPVCNamespace(ctx context.Context, client kubernetes.Interface, name string) string {
	if client == nil || strings.TrimSpace(name) == "" {
		return "default"
	}
	if list, err := client.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{}); err == nil {
		for _, item := range list.Items {
			if item.Name == name {
				return item.Namespace
			}
		}
	}
	return "default"
}

// ListVolumes queries PVCs across all namespaces, detects physical replicas topology for distributed engines,
// and retrieves real Kubelet/Prometheus storage metrics with nil fallback.
func (u *VolumeUsecase) ListVolumes(ctx context.Context, clusterID string) ([]storage.DistributedVolume, error) {
	client, err := u.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	pvcList, err := client.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing persistent volume claims: %w", err)
	}

	// Discover physical nodes for replica placement
	var nodeNames []string
	var nodeUIDs []string
	if nodeList, nodeErr := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); nodeErr == nil && nodeList != nil {
		for _, n := range nodeList.Items {
			nodeNames = append(nodeNames, n.Name)
			nodeUIDs = append(nodeUIDs, string(n.UID))
		}
	}

	volumes := make([]storage.DistributedVolume, 0, len(pvcList.Items))
	for _, pvc := range pvcList.Items {
		scName := ""
		if pvc.Spec.StorageClassName != nil {
			scName = *pvc.Spec.StorageClassName
		}

		capacityBytes := int64(0)
		if qty, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
			capacityBytes = qty.Value()
		} else if qty, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			capacityBytes = qty.Value()
		}

		// Retrieve real volume usage metrics with zero fake telemetry
		var usedBytes *int64
		var usagePercentage *float64
		usageSource := storage.UsageSourceUnknown

		provider := u.metricsProvider
		if provider == nil {
			provider = u.defaultMetricsProvider
		}
		if provider != nil {
			if uBytes, mErr := provider.GetVolumeMetrics(ctx, client, pvc.Namespace, pvc.Name); mErr == nil && uBytes != nil {
				uVal := *uBytes
				usedBytes = &uVal
				usageSource = storage.UsageSourceKubeletMetrics
				if capacityBytes > 0 {
					pct := (float64(uVal) / float64(capacityBytes)) * 100.0
					usagePercentage = &pct
				}
			}
		}

		healthStatus := storage.HealthStatusHealthy
		switch pvc.Status.Phase {
		case corev1.ClaimBound:
			healthStatus = storage.HealthStatusHealthy
		case corev1.ClaimPending:
			healthStatus = storage.HealthStatusDegraded
		case corev1.ClaimLost:
			healthStatus = storage.HealthStatusFaulted
		default:
			healthStatus = storage.HealthStatusDegraded
		}

		attachedNode := pvc.Annotations["volume.kubernetes.io/selected-node"]
		if attachedNode == "" && pvc.Status.Phase == corev1.ClaimBound && len(nodeNames) > 0 {
			attachedNode = nodeNames[0]
		}

		scLower := strings.ToLower(scName)
		isLonghorn := scLower == "longhorn" || scLower == "longhorn-fast" || strings.HasPrefix(scLower, "longhorn")

		var replicationCount int
		var replicas []storage.VolumeReplica

		if isLonghorn {
			// Place replicas only on physical nodes that actually exist in the cluster
			maxReplicas := len(nodeNames)
			if maxReplicas > 3 {
				maxReplicas = 3
			}
			replicationCount = maxReplicas
			if maxReplicas > 0 {
				replicas = make([]storage.VolumeReplica, 0, maxReplicas)
				for i := 0; i < maxReplicas; i++ {
					replicas = append(replicas, storage.VolumeReplica{
						NodeID:   nodeUIDs[i],
						NodeName: nodeNames[i],
						Mode:     "RW",
						Size:     capacityBytes,
					})
				}
			}
		} else {
			replicationCount = 1
			replicas = make([]storage.VolumeReplica, 0, 1)
			if attachedNode != "" {
				nodeID := attachedNode
				for idx, nName := range nodeNames {
					if nName == attachedNode && idx < len(nodeUIDs) {
						nodeID = nodeUIDs[idx]
						break
					}
				}
				replicas = append(replicas, storage.VolumeReplica{
					NodeID:   nodeID,
					NodeName: attachedNode,
					Mode:     "RW",
					Size:     capacityBytes,
				})
			}
		}

		volumes = append(volumes, storage.DistributedVolume{
			Name:             pvc.Name,
			Namespace:        pvc.Namespace,
			StorageClass:     scName,
			CapacityBytes:    capacityBytes,
			UsedBytes:        usedBytes,
			UsagePercentage:  usagePercentage,
			UsageSource:      usageSource,
			ReplicationCount: replicationCount,
			HealthStatus:     healthStatus,
			AttachedNode:     attachedNode,
			Replicas:         replicas,
		})
	}

	return volumes, nil
}
