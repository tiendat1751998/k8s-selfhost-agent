package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/datdt/k8sselfhost/internal/domain/capacity"
	infraCluster "github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
)

type capacityRepo struct {
	client        *kubernetes.Clientset
	clientManager *infraCluster.ClientManager
}

// NewCapacityRepo creates a new live Kubernetes-backed Capacity repository.
func NewCapacityRepo(client *kubernetes.Clientset, clientManager *infraCluster.ClientManager) capacity.Repository {
	return &capacityRepo{
		client:        client,
		clientManager: clientManager,
	}
}

func (r *capacityRepo) List(ctx context.Context, cluster string) ([]capacity.Forecast, error) {
	if cluster == "" {
		cluster = "all"
	}

	var client *kubernetes.Clientset
	if r.clientManager != nil && cluster != "" && cluster != "all" {
		// Check if it's a Docker provider first
		dCli, err := r.clientManager.GetDockerClient(ctx, cluster)
		if err == nil && dCli != nil {
			return nil, fmt.Errorf("ErrMetricsUnavailable: real metrics source unavailable for docker provider")
		}

		kCli, err := r.clientManager.GetK8sClient(ctx, cluster)
		if err == nil && kCli != nil {
			client = kCli
		}
	}

	if client == nil {
		var err error
		client, err = getClient(ctx, r.client)
		if err != nil {
			return nil, err
		}
	}

	if client == nil {
		return nil, fmt.Errorf("kubernetes client not initialized and cluster is unreachable")
	}

	// 1. Get all nodes to calculate total allocatable CPU and Memory
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	var totalAllocatableCPU int64
	var totalAllocatableMem int64
	for _, n := range nodes.Items {
		totalAllocatableCPU += n.Status.Allocatable.Cpu().MilliValue()
		totalAllocatableMem += n.Status.Allocatable.Memory().Value()
	}

	// 2. Fetch metrics
	rawMetrics, err := client.RESTClient().Get().AbsPath("/apis/metrics.k8s.io/v1beta1/nodes").DoRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch raw node metrics: %w", err)
	}

	var metricsResp struct {
		Items []struct {
			Usage struct {
				CPU    string `json:"cpu"`
				Memory string `json:"memory"`
			} `json:"usage"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rawMetrics, &metricsResp); err != nil {
		return nil, fmt.Errorf("failed to parse node metrics JSON: %w", err)
	}

	var totalUsedCPU int64
	var totalUsedMem int64
	for _, item := range metricsResp.Items {
		if cpuQ, err := resource.ParseQuantity(item.Usage.CPU); err == nil {
			totalUsedCPU += cpuQ.MilliValue()
		}
		if memQ, err := resource.ParseQuantity(item.Usage.Memory); err == nil {
			totalUsedMem += memQ.Value()
		}
	}

	if len(metricsResp.Items) == 0 || totalAllocatableCPU == 0 || totalAllocatableMem == 0 {
		return nil, fmt.Errorf("no nodes or capacity metrics available for cluster")
	}

	cpuPercent := (float64(totalUsedCPU) / float64(totalAllocatableCPU)) * 100.0
	memPercent := (float64(totalUsedMem) / float64(totalAllocatableMem)) * 100.0

	return buildLiveForecasts(cluster, cpuPercent, memPercent, time.Now().UTC()), nil
}

// buildLiveForecasts constructs capacity forecasts from live cluster CPU and memory usage percentages.
// When historical time-series data is not available, projections and exhaustion dates are omitted
// and marked as "insufficient_data" rather than fabricating synthetic numbers.
func buildLiveForecasts(cluster string, cpuPercent, memPercent float64, now time.Time) []capacity.Forecast {
	return []capacity.Forecast{
		{
			ID:           "cap-cpu-live",
			Cluster:      cluster,
			ResourceType: capacity.ResourceCPU,
			CurrentUsage: cpuPercent,
			Forecast7d:   0,
			Forecast30d:  0,
			Forecast90d:  0,
			ExhaustionAt: nil,
			Status:       "insufficient_data",
			RecordedAt:   now,
		},
		{
			ID:           "cap-mem-live",
			Cluster:      cluster,
			ResourceType: capacity.ResourceMemory,
			CurrentUsage: memPercent,
			Forecast7d:   0,
			Forecast30d:  0,
			Forecast90d:  0,
			ExhaustionAt: nil,
			Status:       "insufficient_data",
			RecordedAt:   now,
		},
	}
}

func (r *capacityRepo) Record(ctx context.Context, f *capacity.Forecast) error {
	return fmt.Errorf("Record not implemented: missing persistence layer")
}
