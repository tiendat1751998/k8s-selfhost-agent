package storage

import (
	"errors"
	"fmt"
	"time"

	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// VolumeUsageSource describes the provenance of storage capacity telemetry.
type VolumeUsageSource string

const (
	UsageSourceKubeletMetrics VolumeUsageSource = "kubelet_metrics"
	UsageSourceCSIDriver      VolumeUsageSource = "csi_driver"
	UsageSourceUnknown        VolumeUsageSource = "unknown"
)

// RFC 7807 and Domain error codes for persistent storage.
const (
	ErrCodeStorageShrinkUnsupported = "ERR_STORAGE_SHRINK_UNSUPPORTED"
	ErrCodeStorageClassNoExpansion  = "ERR_STORAGE_CLASS_NO_EXPANSION"
	ErrCodeVolumeAttachmentLocked   = "ERR_VOLUME_ATTACHMENT_LOCKED"
)

// Domain sentinel errors.
var (
	ErrEmptyVolumeName          = errors.New("volume name cannot be empty")
	ErrInvalidVolumeSize        = errors.New("volume size must be greater than zero")
	ErrStorageShrinkUnsupported = errors.New("Kubernetes does not support shrinking persistent volume claims")
	ErrStorageClassNoExpansion  = errors.New("storage class does not allow volume expansion")
	ErrVolumeAttachmentLocked   = errors.New("volume is locked by offline node; storage fencing required")
)

// NewErrStorageShrinkUnsupported returns a domain error when attempting to shrink a PVC.
func NewErrStorageShrinkUnsupported() *domainerrors.DomainError {
	return &domainerrors.DomainError{
		Code:    ErrCodeStorageShrinkUnsupported,
		Message: "Kubernetes does not support shrinking persistent volume claims.",
		Err:     ErrStorageShrinkUnsupported,
	}
}

// NewErrStorageClassNoExpansion returns a domain error when StorageClass forbids PVC expansion.
func NewErrStorageClassNoExpansion(scName string) *domainerrors.DomainError {
	return &domainerrors.DomainError{
		Code:    ErrCodeStorageClassNoExpansion,
		Message: fmt.Sprintf("StorageClass '%s' has allowVolumeExpansion=false. Expansion rejected.", scName),
		Err:     ErrStorageClassNoExpansion,
	}
}

// NewErrVolumeAttachmentLocked returns a domain error when a volume remains attached to an offline node.
func NewErrVolumeAttachmentLocked(volName, nodeName string) *domainerrors.DomainError {
	return &domainerrors.DomainError{
		Code:    ErrCodeVolumeAttachmentLocked,
		Message: fmt.Sprintf("Volume '%s' is locked by offline node '%s'. Storage fencing required.", volName, nodeName),
		Err:     ErrVolumeAttachmentLocked,
	}
}

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
	Name             string            `json:"name"`
	Namespace        string            `json:"namespace"`
	StorageClass     string            `json:"storage_class"`
	CapacityBytes    int64             `json:"capacity_bytes"`
	UsedBytes        *int64            `json:"used_bytes"`
	UsagePercentage  *float64          `json:"usage_percentage"`
	UsageSource      VolumeUsageSource `json:"usage_source"`
	ReplicationCount int               `json:"replication_count"`
	HealthStatus     string            `json:"health_status"` // "Healthy", "Degraded", "Faulted"
	AttachedNode     string            `json:"attached_node"`
	Replicas         []VolumeReplica   `json:"replicas"`
}

// VolumeDetails represents detailed metadata and real telemetry for a persistent volume claim.
type VolumeDetails struct {
	VolumeID         string            `json:"volume_id"`
	PersistentVolume string            `json:"persistent_volume"`
	ClaimName        string            `json:"claim_name"`
	Namespace        string            `json:"namespace"`
	StorageClassName string            `json:"storage_class_name"`
	CapacityBytes    int64             `json:"capacity_bytes"`
	UsedBytes        *int64            `json:"used_bytes"`       // nil if metrics unavailable
	UsagePercentage  *float64          `json:"usage_percentage"` // nil if UsedBytes == nil
	UsageSource      VolumeUsageSource `json:"usage_source"`
	AllowExpansion   bool              `json:"allow_expansion"`
	VolumeMode       string            `json:"volume_mode"`
	AccessModes      []string          `json:"access_modes"`
	Phase            string            `json:"phase"`
	AttachedNode     string            `json:"attached_node,omitempty"`
	AttachmentStatus string            `json:"attachment_status,omitempty"`
	UpdatedAt        time.Time         `json:"updated_at"`
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
