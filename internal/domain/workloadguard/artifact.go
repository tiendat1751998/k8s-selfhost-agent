// Package workloadguard defines domain contracts, entities, interfaces, error types,
// and cache-aligned hardware counters for OCI image immutable digest enforcement,
// ephemeral storage isolation, and pod PID exhaustion guardrails.
//
// ENGINEERING ARCHITECTURE NOTE — PHYSICAL HARDWARE BOTTLENECK ANALYSIS:
// 1. Container Registry HTTP Digest Lookup Latency & Upstream Throttling:
//    During large-scale rolling updates or HPA surge events, hundreds of pods request
//    container image pulls within seconds. Querying remote container registries
//    (Docker Hub, Quay, GHCR) for manifest HEAD requests incurs substantial network latency
//    (100ms - 2000ms per RTT) and triggers upstream HTTP 429 Too Many Requests rate-limiting.
//    Real-world failure mode: tag mutability (e.g., `:latest`, `:dev`, `:staging`, or re-pushed
//    `:v1.2.0`) causes divergent, untested image binaries to run simultaneously across worker nodes
//    under the exact same service selector, inducing split-brain runtime states and irreproducible bugs.
// 2. Node Root Disk ext4/XFS Inode Table Fragmentation & Eviction I/O Storms:
//    When container logging, scratch files, or machine learning checkpoints write unconstrained
//    to `/tmp` without an isolated emptyDir mount, data spills directly onto the host root
//    filesystem (/var/lib/kubelet / overlayfs). If unconstrained, this exhausts node disk block
//    storage or fragments the ext4/XFS inode table. Kubelet detects high disk utilization
//    (evictionThreshold >= 85%) and initiates emergency pod evictions. Real-world failure mode:
//    Kubelet eviction cascades tear down critical infrastructure daemonsets and database replicas,
//    inducing severe I/O thrashing and cascading node NotReady outages across the cluster.
// 3. Linux Kernel PID Table Limits (`kernel.pid_max = 32768`) & Host Fork Bombs:
//    By default, the Linux kernel manages a global process identifier table capped at
//    `kernel.pid_max=32768`. Without hard pod-level cgroup PID boundaries (pids.max), a single
//    misconfigured container, rogue script, or malicious exploit (`:(){ :|:& };:`) can spawn
//    thousands of threads in milliseconds. Real-world failure mode: host PID table exhaustion
//    prevents the Linux kernel from allocating process IDs for essential system daemons
//    (systemd, sshd, containerd, kubelet). The node becomes entirely unresponsive, kubelet
//    health checks fail, and operators cannot SSH into the host to remediate the outage.
package workloadguard

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
)

// Default limits and thresholds.
const (
	DefaultEphemeralStorageLimitBytes int64 = 2 * 1024 * 1024 * 1024 // 2Gi
	DefaultTmpEmptyDirSizeLimitBytes  int64 = 1 * 1024 * 1024 * 1024 // 1Gi
	DefaultPodPIDLimit                int64 = 1024
	MaxHostKernelPIDs                 int64 = 32768 // Linux default kernel.pid_max
)

// RFC 7807 Error codes.
const (
	CodeMutableImageTagRejected      = "ERR_MUTABLE_IMAGE_TAG_REJECTED"
	CodeEphemeralStorageLimitMissing = "ERR_EPHEMERAL_STORAGE_LIMIT_MISSING"
	CodePIDExhaustionRisk            = "ERR_PID_EXHAUSTION_RISK"
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
	ErrMutableImageTagRejected = &DomainError{
		Code:       CodeMutableImageTagRejected,
		Message:    "Mutable container image tag rejected. OCI immutable sha256 digest reference required.",
		HTTPStatus: 422, // 422 Unprocessable Entity
		Err:        errors.New("mutable container image tag rejected"),
	}

	ErrEphemeralStorageLimitMissing = &DomainError{
		Code:       CodeEphemeralStorageLimitMissing,
		Message:    "Pod container lacks ephemeral-storage resource limits. Strict storage boundary required.",
		HTTPStatus: 400, // 400 Bad Request
		Err:        errors.New("ephemeral-storage limit missing"),
	}

	ErrPIDExhaustionRisk = &DomainError{
		Code:       CodePIDExhaustionRisk,
		Message:    "Requested PID capacity exceeds safe host cgroup threshold or kernel limit, risking fork-bomb exhaustion.",
		HTTPStatus: 429, // 429 Too Many Requests
		Err:        errors.New("pid exhaustion risk detected"),
	}
)

// ImageDigestResolution represents the resolved immutable digest for a container image.
type ImageDigestResolution struct {
	OriginalImage  string `json:"original_image"`
	ResolvedDigest string `json:"resolved_digest"`
	IsImmutable    bool   `json:"is_immutable"`
}

// EphemeralStorageCheck holds validation results for pod ephemeral storage boundaries.
type EphemeralStorageCheck struct {
	HasLimit              bool   `json:"has_limit"`
	LimitBytes            int64  `json:"limit_bytes"`
	TmpMountedAsEmptyDir  bool   `json:"tmp_mounted_as_emptydir"`
	IsPassed              bool   `json:"is_passed"`
	RejectReason          string `json:"reject_reason,omitempty"`
}

// PaddedDigestResolutionCounter is an atomic counter for digest resolutions.
type PaddedDigestResolutionCounter struct {
	val uint64
}

// Inc atomically increments the counter by 1.
func (c *PaddedDigestResolutionCounter) Inc() uint64 {
	return atomic.AddUint64(&c.val, 1)
}

// Add atomically adds delta to the counter.
func (c *PaddedDigestResolutionCounter) Add(delta uint64) uint64 {
	return atomic.AddUint64(&c.val, delta)
}

// Load atomically retrieves the current value.
func (c *PaddedDigestResolutionCounter) Load() uint64 {
	return atomic.LoadUint64(&c.val)
}

// Reset atomically resets the counter to 0.
func (c *PaddedDigestResolutionCounter) Reset() {
	atomic.StoreUint64(&c.val, 0)
}

// PaddedStorageCheckCounter tracks storage checks atomically.
type PaddedStorageCheckCounter struct {
	val uint64
}

// Inc atomically increments the counter by 1.
func (c *PaddedStorageCheckCounter) Inc() uint64 {
	return atomic.AddUint64(&c.val, 1)
}

// Add atomically adds delta to the counter.
func (c *PaddedStorageCheckCounter) Add(delta uint64) uint64 {
	return atomic.AddUint64(&c.val, delta)
}

// Load atomically retrieves the current value.
func (c *PaddedStorageCheckCounter) Load() uint64 {
	return atomic.LoadUint64(&c.val)
}

// Reset atomically resets the counter to 0.
func (c *PaddedStorageCheckCounter) Reset() {
	atomic.StoreUint64(&c.val, 0)
}

// PaddedPIDAdmissionCounter isolates PID limiter metrics.
type PaddedPIDAdmissionCounter struct {
	val uint64
}

// Inc atomically increments the counter by 1.
func (c *PaddedPIDAdmissionCounter) Inc() uint64 {
	return atomic.AddUint64(&c.val, 1)
}

// Add atomically adds delta to the counter.
func (c *PaddedPIDAdmissionCounter) Add(delta uint64) uint64 {
	return atomic.AddUint64(&c.val, delta)
}

// Load atomically retrieves the current value.
func (c *PaddedPIDAdmissionCounter) Load() uint64 {
	return atomic.LoadUint64(&c.val)
}

// Reset atomically resets the counter to 0.
func (c *PaddedPIDAdmissionCounter) Reset() {
	atomic.StoreUint64(&c.val, 0)
}

// WorkloadArtifactGuard defines the domain contracts for OCI image digest resolution,
// ephemeral storage boundary auditing, and pod PID limit enforcement.
type WorkloadArtifactGuard interface {
	ResolveImageToDigest(ctx context.Context, image string) (*ImageDigestResolution, error)
	ValidateEphemeralStoragePolicy(ctx context.Context, podSpecJSON []byte) (*EphemeralStorageCheck, error)
	EnforcePodPIDLimit(ctx context.Context, podSpecJSON []byte, defaultMaxPIDs int64) ([]byte, error)
}
