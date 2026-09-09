package storage

import (
	"errors"
)

// Domain sentinel errors.
var (
	ErrEmptyVolumeName   = errors.New("volume name cannot be empty")
	ErrInvalidVolumeSize = errors.New("volume size must be greater than zero")
)

// Health status constants.
const (
	HealthStatusHealthy  = "Healthy"
	HealthStatusDegraded = "Degraded"
	HealthStatusFaulted  = "Faulted"
)

// Snapshot status constants.
const (
	SnapshotStatusReady    = "Ready"
	SnapshotStatusCreating = "Creating"
	SnapshotStatusFailed   = "Failed"
)

// VolumeReplica represents a physical replica of a distributed storage volume on a node.
type VolumeReplica struct {
	NodeID   string `json:"node_id"`
	NodeName string `json:"node_name"`
	Mode     string `json:"mode"`
	Size     int64  `json:"size_bytes"`
}

// DistributedVolume represents a distributed storage volume across nodes in a Kubernetes cluster.
type DistributedVolume struct {
	Name             string          `json:"name"`
	Namespace        string          `json:"namespace"`
	StorageClass     string          `json:"storage_class"`
	CapacityBytes    int64           `json:"capacity_bytes"`
	UsedBytes        int64           `json:"used_bytes"`
	ReplicationCount int             `json:"replication_count"`
	HealthStatus     string          `json:"health_status"` // "Healthy", "Degraded", "Faulted"
	AttachedNode     string          `json:"attached_node"`
	Replicas         []VolumeReplica `json:"replicas"`
}

// IsHealthy returns true if the volume health status is Healthy.
func (v *DistributedVolume) IsHealthy() bool {
	return v.HealthStatus == HealthStatusHealthy
}

// Validate guards invariants for DistributedVolume.
func (v *DistributedVolume) Validate() error {
	if v.Name == "" {
		return ErrEmptyVolumeName
	}
	return nil
}

// VolumeExpandRequest represents an online volume expansion request.
type VolumeExpandRequest struct {
	NewSizeBytes int64 `json:"new_size_bytes"`
}

// Validate checks whether the expand request contains valid arguments.
func (r *VolumeExpandRequest) Validate() error {
	if r.NewSizeBytes <= 0 {
		return ErrInvalidVolumeSize
	}
	return nil
}

// VolumeSnapshotRequest represents a volume snapshot creation request.
type VolumeSnapshotRequest struct {
	SnapshotName string            `json:"snapshot_name,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// VolumeSnapshotResult represents the result of a snapshot operation.
type VolumeSnapshotResult struct {
	SnapshotName string `json:"snapshot_name"`
	VolumeName   string `json:"volume_name"`
	Status       string `json:"status"` // "Ready", "Creating", "Failed"
	CreatedAt    string `json:"created_at"`
}
