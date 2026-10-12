package network

import (
	"context"
	"errors"
	"math"
	"net"
	"time"
)

// IPAM status constants.
const (
	StatusHealthy   = "HEALTHY"
	StatusWarning   = "WARNING"
	StatusExhausted = "EXHAUSTED"
)

// Sentinel domain errors.
var (
	ErrCNIIPAMExhausted   = errors.New("CNI IPAM exhausted: pod scheduling blocked")
	ErrCoreDNSHighLatency = errors.New("CoreDNS cluster P99 latency exceeded critical threshold")
	ErrNodeNotFound       = errors.New("node not found")
	ErrClusterUnavailable = errors.New("cluster connection unavailable")
)

// IPAMUtilization captures the CNI address pool allocation and lease leakage on a node.
type IPAMUtilization struct {
	NodeName         string    `json:"node_name"`
	PodCIDR          string    `json:"pod_cidr"`
	TotalAllocatable int       `json:"total_allocatable"` // e.g. /24 = 254 IPs
	AllocatedIPs     int       `json:"allocated_ips"`
	ActivePodCount   int       `json:"active_pod_count"`
	LeakedOrphanIPs  int       `json:"leaked_orphan_ips"` // AllocatedIPs - ActivePodCount
	UsagePercentage  float64   `json:"usage_percentage"`
	Status           string    `json:"status"`            // HEALTHY, WARNING, EXHAUSTED
	UpdatedAt        time.Time `json:"updated_at"`
}

// DNSHealthReport summarizes CoreDNS operational health and latency metrics.
type DNSHealthReport struct {
	ClusterID       string        `json:"cluster_id"`
	CoreDNSReplicas int           `json:"coredns_replicas"`
	P99Latency      time.Duration `json:"p99_latency"`
	ErrorRate       float64       `json:"error_rate"`
	NodeLocalActive bool          `json:"node_local_active"`
	AlertTriggered  bool          `json:"alert_triggered"`
}

// IPAMLease represents an IP lease allocated to a pod or sandbox.
type IPAMLease struct {
	IP        string    `json:"ip"`
	NodeName  string    `json:"node_name"`
	PodName   string    `json:"pod_name"`
	Namespace string    `json:"namespace"`
	CreatedAt time.Time `json:"created_at"`
}

// IPAMBackend provides an abstraction for interacting with CNI IPAM drivers.
type IPAMBackend interface {
	GetAllocatedLeases(ctx context.Context, clusterID, nodeName string) ([]IPAMLease, error)
	ReleaseLeases(ctx context.Context, clusterID, nodeName string, ips []string) (int, error)
	RegisterLease(ctx context.Context, clusterID string, lease IPAMLease) error
}

// DNSMetricsProvider provides CoreDNS metric observations.
type DNSMetricsProvider interface {
	GetDNSMetrics(ctx context.Context, clusterID string) (p99Latency time.Duration, errorRate float64, err error)
}

// NetworkManager defines the primary usecase port for network and DNS lifecycle management.
type NetworkManager interface {
	GetIPAMUtilization(ctx context.Context, clusterID string) ([]IPAMUtilization, error)
	TriggerIPAMGarbageCollection(ctx context.Context, clusterID, nodeName string) (int, error)
	CheckDNSHealth(ctx context.Context, clusterID string) (*DNSHealthReport, error)
	InjectDNSConfigOptimization(ctx context.Context, clusterID, namespace string) error
}

// CalculateTotalAllocatable determines the total usable addresses from a CIDR notation string.
func CalculateTotalAllocatable(podCIDR string) int {
	if podCIDR == "" {
		return 254
	}
	_, ipnet, err := net.ParseCIDR(podCIDR)
	if err != nil {
		return 254
	}
	ones, bits := ipnet.Mask.Size()
	if bits == 0 {
		return 254
	}
	hostBits := bits - ones
	if hostBits <= 0 {
		return 1
	}
	// For IPv4: 2^(hostBits) - 2 (network & broadcast address excluded)
	total := int(math.Pow(2, float64(hostBits)))
	if bits == 32 && total > 2 {
		return total - 2
	}
	return total
}

// NewIPAMUtilization computes utilization metrics and status for a worker node.
func NewIPAMUtilization(nodeName, podCIDR string, totalAllocatable, allocatedIPs, activePods int) IPAMUtilization {
	if totalAllocatable <= 0 {
		totalAllocatable = CalculateTotalAllocatable(podCIDR)
	}
	if totalAllocatable <= 0 {
		totalAllocatable = 254
	}

	leaked := allocatedIPs - activePods
	if leaked < 0 {
		leaked = 0
	}

	usage := (float64(allocatedIPs) / float64(totalAllocatable)) * 100.0
	// Round to two decimal places for stable representation
	usage = math.Round(usage*100) / 100

	status := StatusHealthy
	if usage >= 95.0 {
		status = StatusExhausted
	} else if usage > 80.0 || leaked > 20 {
		status = StatusWarning
	}

	return IPAMUtilization{
		NodeName:         nodeName,
		PodCIDR:          podCIDR,
		TotalAllocatable: totalAllocatable,
		AllocatedIPs:     allocatedIPs,
		ActivePodCount:   activePods,
		LeakedOrphanIPs:  leaked,
		UsagePercentage:  usage,
		Status:           status,
		UpdatedAt:        time.Now().UTC(),
	}
}
