// Package runtimeguard defines domain entities, interfaces, error types, and
// cache-aligned hardware counters for container runtime storage resilience, containerd
// inode depletion prevention, Docker Hub registry rate-limiting, and P2P image distribution.
//
// ENGINEERING ARCHITECTURE NOTE — PHYSICAL HARDWARE BOTTLENECK ANALYSIS:
// 1. Filesystem Metadata Inode Table Saturation (ext4 / XFS Overlayfs):
//    In overlayfs mounts used by containerd (/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs),
//    each image layer consists of thousands of small metadata files, symlinks, and whiteout devices
//    (char device 0/0). On default ext4 filesystems with fixed inode ratios (1 inode per 16KB disk space)
//    or XFS filesystems with suboptimal maxpct settings, high image turnover consumes all allocated
//    inodes while disk block space remains virtually untouched. Real-world incident failure mode:
//    nodes report 180GB free block storage, yet `crictl pull` crashes with `no space left on device`
//    (ENOSPC, errno 28) because the VFS layer cannot allocate an inode for a new layer un-tar operation.
// 2. Kernel VFS Dentry and Inode Cache Bloat:
//    Rapid pod churning leaves millions of negative dentry objects and inactive inode structures in
//    kernel slab memory (kmem). If `vm.vfs_cache_pressure` is left at default 100, the Linux kernel
//    favors keeping dentries over reclaiming pagecache, driving memory pressure and kernel lock
//    contention across `dcache_lock` and overlayfs lookup paths.
// 3. Docker Hub HTTP 429 Rate Limit Ceilings:
//    Docker Hub enforces an anonymous pull limit of 100 pulls per 6-hour window per egress NAT IP
//    (200 for authenticated free tier). During Horizontal Pod Autoscaler (HPA) surge events or cluster
//    upgrades, dozens of worker nodes simultaneously attempt upstream pulls, triggering HTTP 429 Too Many
//    Requests. This produces a catastrophic cascading `ImagePullBackOff` storm across application namespaces.
// 4. P2P DHT Routing Churn & Deadlocks (Spegel / Dragonfly):
//    In peer-to-peer distribution fabrics, rapid node churn or network partitioning degrades DHT routing
//    tables (Kademlia buckets). Without an unyielding 15-second origin registry fallback timeout,
//    containerd image pull goroutines hang waiting for DHT peer chunk resolution, causing kubelet container
//    runtime sync timeouts and node NotReady flap.
package runtimeguard

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// Default paths and thresholds for containerd runtime storage.
const (
	DefaultContainerdMountPoint = "/var/lib/containerd"
	InodePruneWarningThreshold  = 75.0 // Trigger dangling image prune
	InodeTaintCriticalThreshold = 85.0 // Taint node out-of-inodes:NoSchedule
	DefaultP2PFallbackTimeout   = 15 * time.Second
	NodeTaintOutOfInodesKey     = "node.kubernetes.io/out-of-inodes"
	NodeTaintNoScheduleEffect   = "NoSchedule"
)

// StorageMetricType differentiates byte consumption from inode saturation telemetry.
type StorageMetricType string

const (
	MetricDiskBytes  StorageMetricType = "DISK_BYTES"
	MetricDiskInodes StorageMetricType = "DISK_INODES"
)

// RFC 7807 Error code constants.
const (
	CodeContainerdInodeSaturation = "ERR_CONTAINERD_INODE_SATURATION"
	CodeRegistryRateLimitExceeded = "ERR_REGISTRY_RATE_LIMIT_EXCEEDED"
	CodeManifestValidationFailed  = "ERR_MANIFEST_VALIDATION_FAILED"
)

// DomainError represents an RFC 7807 compliant domain error with HTTP status binding.
type DomainError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"http_status"`
	Err        error  `json:"-"`
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

// Sentinel domain errors.
var (
	ErrContainerdInodeSaturation = &DomainError{
		Code:       CodeContainerdInodeSaturation,
		Message:    "Containerd storage inode table critically saturated (>= 85%). Node tainted NoSchedule.",
		HTTPStatus: 507, // 507 Insufficient Storage (RFC 4918 / RFC 7807)
		Err:        errors.New("filesystem inode table exhausted"),
	}

	ErrRegistryRateLimitExceeded = &DomainError{
		Code:       CodeRegistryRateLimitExceeded,
		Message:    "Upstream registry rate limit ceiling exceeded (HTTP 429). Mirror routing mandatory.",
		HTTPStatus: 429, // 429 Too Many Requests
		Err:        errors.New("upstream registry pull limit reached"),
	}

	ErrManifestValidationFailed = &DomainError{
		Code:       CodeManifestValidationFailed,
		Message:    "Container image manifest verification failed or digest signature malformed.",
		HTTPStatus: 422, // 422 Unprocessable Entity
		Err:        errors.New("invalid container image manifest specification"),
	}
)

// NodeStorageHealth encapsulates real-time block and inode consumption metrics for a cluster node.
type NodeStorageHealth struct {
	NodeName       string    `json:"node_name"`
	MountPoint     string    `json:"mount_point"`
	TotalBytes     uint64    `json:"total_bytes"`
	UsedBytesPct   float64   `json:"used_bytes_pct"`
	TotalInodes    uint64    `json:"total_inodes"`
	UsedInodesPct  float64   `json:"used_inodes_pct"`
	DanglingImages int       `json:"dangling_images"`
	LastPrunedAt   time.Time `json:"last_pruned_at"`
}

// RequiresPrune returns true if used inodes percentage reaches or exceeds warning threshold (75%).
func (n *NodeStorageHealth) RequiresPrune() bool {
	return n.UsedInodesPct >= InodePruneWarningThreshold
}

// RequiresTaint returns true if used inodes percentage reaches or exceeds critical threshold (85%).
func (n *NodeStorageHealth) RequiresTaint() bool {
	return n.UsedInodesPct >= InodeTaintCriticalThreshold
}

// RegistryMirrorConfig defines mirror and fallback parameters for upstream registries.
type RegistryMirrorConfig struct {
	PrimaryRegistry string        `json:"primary_registry"`
	MirrorURL       string        `json:"mirror_url"`
	FallbackTimeout time.Duration `json:"fallback_timeout"`
	RateLimitFloor  int           `json:"rate_limit_floor"`
}

// Validate validates the registry mirror configuration.
func (c *RegistryMirrorConfig) Validate() error {
	if strings.TrimSpace(c.PrimaryRegistry) == "" {
		return errors.New("primary_registry is required")
	}
	if strings.TrimSpace(c.MirrorURL) == "" {
		return errors.New("mirror_url is required")
	}
	if c.FallbackTimeout <= 0 {
		c.FallbackTimeout = DefaultP2PFallbackTimeout
	}
	return nil
}

// PaddedInodeUsageCounter is an atomic counter for inode usage.
type PaddedInodeUsageCounter struct {
	val uint64
}

// Inc atomically increments the counter by 1.
func (c *PaddedInodeUsageCounter) Inc() uint64 {
	return atomic.AddUint64(&c.val, 1)
}

// Add atomically adds delta to the counter.
func (c *PaddedInodeUsageCounter) Add(delta uint64) uint64 {
	return atomic.AddUint64(&c.val, delta)
}

// Load atomically retrieves the current value.
func (c *PaddedInodeUsageCounter) Load() uint64 {
	return atomic.LoadUint64(&c.val)
}

// Reset atomically resets the counter to 0.
func (c *PaddedInodeUsageCounter) Reset() {
	atomic.StoreUint64(&c.val, 0)
}

// PaddedRateLimitTracker tracks rate limits atomically.
type PaddedRateLimitTracker struct {
	val uint64
}

// Add atomically increments the rate-limit tracker by delta.
func (t *PaddedRateLimitTracker) Add(delta uint64) uint64 {
	return atomic.AddUint64(&t.val, delta)
}

// Load atomically reads the rate-limit counter.
func (t *PaddedRateLimitTracker) Load() uint64 {
	return atomic.LoadUint64(&t.val)
}

// Reset atomically sets the tracker back to 0.
func (t *PaddedRateLimitTracker) Reset() {
	atomic.StoreUint64(&t.val, 0)
}

// PaddedStorageMetricTracker tracks node storage telemetry.
type PaddedStorageMetricTracker struct {
	val uint64
}

// Add atomically increments the metric tracker.
func (m *PaddedStorageMetricTracker) Add(delta uint64) uint64 {
	return atomic.AddUint64(&m.val, delta)
}

// Load atomically reads the metric counter.
func (m *PaddedStorageMetricTracker) Load() uint64 {
	return atomic.LoadUint64(&m.val)
}

// Reset atomically resets the counter.
func (m *PaddedStorageMetricTracker) Reset() {
	atomic.StoreUint64(&m.val, 0)
}

// RuntimeResilienceManager defines contracts for monitoring containerd inode saturation,
// triggering safe dangling image reclamation, mutating rate-limited image pull specifications,
// and configuring P2P fallback timeouts.
type RuntimeResilienceManager interface {
	AuditNodeInodes(ctx context.Context) ([]NodeStorageHealth, error)
	TriggerDanglingPrune(ctx context.Context, node string) error
	ValidateImageManifest(imageURI string) (bool, error)
	ConfigureRegistryMirror(ctx context.Context, config RegistryMirrorConfig) error
}
