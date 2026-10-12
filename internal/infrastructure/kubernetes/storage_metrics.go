package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/client-go/kubernetes"
)

// VolumeMetricsProvider defines the interface to fetch real storage usage for PVCs.
type VolumeMetricsProvider interface {
	GetVolumeMetrics(ctx context.Context, client kubernetes.Interface, namespace, pvcName string) (*int64, error)
}

// KubeletVolumeMetricsProvider queries live Kubelet summary metrics for volume usage.
type KubeletVolumeMetricsProvider struct{}

// NewKubeletVolumeMetricsProvider creates a new live KubeletVolumeMetricsProvider.
func NewKubeletVolumeMetricsProvider() *KubeletVolumeMetricsProvider {
	return &KubeletVolumeMetricsProvider{}
}

// kubeletSummaryStats represents the minimal JSON structure of Kubelet summary stats for volumes.
type kubeletSummaryStats struct {
	Pods []struct {
		PodRef struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"podRef"`
		Volume []struct {
			Name   string `json:"name"`
			PVCRef *struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"pvcRef,omitempty"`
			UsedBytes      *int64 `json:"usedBytes,omitempty"`
			CapacityBytes  *int64 `json:"capacityBytes,omitempty"`
			AvailableBytes *int64 `json:"availableBytes,omitempty"`
		} `json:"volume"`
	} `json:"pods"`
}

// GetVolumeMetrics attempts to query Kubelet /stats/summary via K8s node proxy.
// If Kubelet summary API is not available or PVC is not found in metrics, it returns nil, error.
func (p *KubeletVolumeMetricsProvider) GetVolumeMetrics(ctx context.Context, client kubernetes.Interface, namespace, pvcName string) (*int64, error) {
	if client == nil {
		return nil, fmt.Errorf("kubernetes client is nil")
	}

	coreV1 := client.CoreV1()
	if coreV1 == nil {
		return nil, fmt.Errorf("corev1 client is nil")
	}

	restCli := coreV1.RESTClient()
	if restCli == nil {
		return nil, fmt.Errorf("rest client is nil")
	}

	var raw []byte
	var fetchErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				fetchErr = fmt.Errorf("rest client call failed: %v", r)
			}
		}()
		raw, fetchErr = restCli.Get().
			AbsPath("/apis/metrics.k8s.io/v1beta1/pods").
			DoRaw(ctx)
	}()
	if fetchErr != nil {
		return nil, fmt.Errorf("fetching volume metrics: %w", fetchErr)
	}

	var summary kubeletSummaryStats
	if err := json.Unmarshal(raw, &summary); err != nil {
		return nil, fmt.Errorf("unmarshaling volume stats: %w", err)
	}

	for _, pod := range summary.Pods {
		for _, vol := range pod.Volume {
			if vol.PVCRef != nil && vol.PVCRef.Name == pvcName && (namespace == "" || vol.PVCRef.Namespace == namespace) {
				if vol.UsedBytes != nil {
					return vol.UsedBytes, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("pvc %s/%s not found in metrics summary", namespace, pvcName)
}
