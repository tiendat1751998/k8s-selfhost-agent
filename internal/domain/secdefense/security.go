// Package secdefense defines domain contracts, entities, and hardware-aligned metrics
// for DevSecOps supply chain firewalls, SSRF / IMDSv2 protection, and native sidecar barriers.
//
// ==============================================================================================
// PHYSICAL HARDWARE BOTTLENECK & REAL-WORLD FAILURE MODE ANALYSIS:
//
// 1. DNS Socket Resolver Round-Trip Latency & TCP Handshake Overhead (Double Resolution):
//    - Re-validating hostnames introduces an additional round-trip time (RTT) to external DNS resolvers
//      (typically 15-45ms across WAN, or 1-3ms via CoreDNS in-cluster).
//    - Performing validation at both pre-flight and socket dial time can double DNS latency if not
//      cached or piped directly through a custom net.Dialer with IP pinning.
//    - To mitigate connection latency, socket dial validation must intercept raw IP addresses directly
//      inside the syscall/Control layer after OS resolution, avoiding duplicate recursive DNS lookups.
//
// 2. Linux Network Namespace Veth Pair Latency:
//    - Pod network namespaces communicate with host veth pairs and CNI bridges (e.g. Calico, Cilium).
//    - Ingress/egress packet inspection through naive user-space filters introduces kernel-to-user context
//      switches (overhead: ~3-5 microseconds per packet). Dial-time IP inspection keeps validation within
//      the Go runtime socket creation lifecycle with negligible (< 100ns) overhead.
//
// 3. Envoy / Sidecar Memory Footprint & Init Ordering:
//    - Envoy and service mesh sidecars typically consume 50-150MB RSS baseline upon bootstrapping.
//    - When sidecars are declared as standard application containers, Kubernetes launches them concurrently
//      with application pods. A race condition occurs: application containers bootstrap within 200-500ms
//      and emit unencrypted plaintext network traffic before Envoy initializes its listener socket and mTLS keys.
//    - Enforcing K8s 1.28+ Native Sidecars (initContainers with restartPolicy: Always) guarantees K8s will
//      block subsequent containers until the sidecar's startupProbe / readiness probe passes.
//
// 4. Multi-Core Cache Contention (False Sharing & MESI/MOESI Protocol):
//    - On modern multi-socket NUMA / multi-core architectures, high-frequency security inspection counters
//      updated concurrently by multiple goroutines across CPU cores invalidate each other's L1/L2 cache lines.
//    - To avoid bus locking and false sharing latency spikes, high-frequency metric counters are padded
//      to 64 bytes (the standard L1/L2 cache line width).
//
// 5. Real-World Attack Vectors & Failure Modes:
//    - Dependency Confusion: Build pipelines configured with public registries alongside private artifactory
//      can be tricked into pulling malicious packages uploaded to public npm/PyPI with inflated versions (e.g. 99.0.0).
//    - Cloud Metadata SSRF via IPv6: Attackers bypass naive IPv4 filters (169.254.169.254) by requesting
//      AWS/Azure link-local IPv6 metadata endpoints (e.g. [fd00:ec2::254], [fe80::...]).
//    - DNS Rebinding: An attacker controls a malicious DNS server returning a safe public IP on initial check (TTL=0)
//      and returning 169.254.169.254 or 127.0.0.1 on the actual HTTP dial.
// ==============================================================================================
package secdefense

import (
	"context"
	"errors"
	"sync/atomic"
)

// Standard Domain Errors for DevSecOps Security Defense.
var (
	// ErrSecSSRFMetadataAttempt is returned when an SSRF request targets cloud metadata or forbidden ranges (RFC 7807 403 Forbidden).
	ErrSecSSRFMetadataAttempt = errors.New("SSRF cloud metadata access blocked (HTTP 403 forbidden)")

	// ErrSecDependencyConfusion is returned when an internal package resolves to an untrusted registry (HTTP 422 Unprocessable).
	ErrSecDependencyConfusion = errors.New("dependency confusion detected: private internal namespace resolved to untrusted public registry (HTTP 422)")

	// ErrInvalidSidecarOrder is returned when sidecar containers are not properly ordered as native initContainers.
	ErrInvalidSidecarOrder = errors.New("invalid sidecar startup ordering: sidecar must be declared as native initContainer with restartPolicy: Always")

	// ErrIMDSv2EnforcementFailed is returned when an IMDSv2 enforcement call fails.
	ErrIMDSv2EnforcementFailed = errors.New("failed to enforce IMDSv2 token configuration")
)

// PaddedCounter is a 64-byte aligned atomic counter designed to prevent false sharing
// across CPU cores under the MESI/MOESI cache coherence protocols.
// Standard x86_64 / arm64 cache line width is 64 bytes.
type PaddedCounter struct {
	value uint64
	_     [56]byte // 8 bytes uint64 + 56 bytes padding = 64 bytes
}

// Inc atomically increments the counter by 1.
func (c *PaddedCounter) Inc() uint64 {
	return atomic.AddUint64(&c.value, 1)
}

// Add atomically adds delta to the counter.
func (c *PaddedCounter) Add(delta uint64) uint64 {
	return atomic.AddUint64(&c.value, delta)
}

// Load atomically loads the counter's current value.
func (c *PaddedCounter) Load() uint64 {
	return atomic.LoadUint64(&c.value)
}

// Reset atomically resets the counter value to zero.
func (c *PaddedCounter) Reset() {
	atomic.StoreUint64(&c.value, 0)
}

// DefenseMetrics groups hardware cache-line aligned metrics for high-throughput security defense.
type DefenseMetrics struct {
	SSRFValidationsTotal PaddedCounter
	SSRFBlockedTotal     PaddedCounter
	MetadataBlockedTotal PaddedCounter
	AuditScansTotal      PaddedCounter
	ConfusionAlertsTotal PaddedCounter
	SidecarValidations   PaddedCounter
}

// NewDefenseMetrics initializes a new DefenseMetrics structure.
func NewDefenseMetrics() *DefenseMetrics {
	return &DefenseMetrics{}
}

// SuspiciousPackage describes a package flagged during supply chain dependency confusion auditing.
type SuspiciousPackage struct {
	Name             string `json:"name"`
	ResolvedVersion  string `json:"resolved_version"`
	ResolvedRegistry string `json:"resolved_registry"`
	InternalPrefix   string `json:"internal_prefix"`
	Reason           string `json:"reason"`
}

// DependencyAuditReport captures the result of a supply chain dependency confusion audit.
type DependencyAuditReport struct {
	TotalPackages      int                 `json:"total_packages"`
	InternalNamespaces []string            `json:"internal_namespaces"`
	SuspiciousPackages []SuspiciousPackage `json:"suspicious_packages"`
	IsClean            bool                `json:"is_clean"`
}

// Evaluate updates IsClean based on whether any suspicious packages were detected.
func (r *DependencyAuditReport) Evaluate() {
	r.IsClean = len(r.SuspiciousPackages) == 0
}

// SSRFValidationResult details whether a requested URL was blocked and the underlying reasoning.
type SSRFValidationResult struct {
	TargetURL       string `json:"target_url"`
	ResolvedIP      string `json:"resolved_ip"`
	IsBlocked       bool   `json:"is_blocked"`
	BlockReason     string `json:"block_reason,omitempty"`
	IsCloudMetadata bool   `json:"is_cloud_metadata"`
}

// IMDSv2EnforceRequest contains instance configuration for IMDSv2 token mandate.
type IMDSv2EnforceRequest struct {
	InstanceID     string `json:"instance_id"`
	HopLimit       int    `json:"hop_limit"`
	RequireTokens  bool   `json:"require_tokens"`
	BlockInsecureV1 bool   `json:"block_insecure_v1"`
}

// SidecarValidationResult details the compliance of pod sidecars with native K8s 1.28+ ordering.
type SidecarValidationResult struct {
	Compliant       bool     `json:"compliant"`
	DetectedSidecars []string `json:"detected_sidecars,omitempty"`
	Violations      []string `json:"violations,omitempty"`
}

// SecurityDefenseEngine defines domain operations for zero-trust supply chain and network defense.
type SecurityDefenseEngine interface {
	// AuditDependencies inspects a dependency lockfile for dependency confusion vulnerabilities.
	AuditDependencies(ctx context.Context, lockfilePath string, privatePrefixes []string) (*DependencyAuditReport, error)

	// ValidateURLAgainstSSRF validates a target URL against IPv4/IPv6 private ranges and metadata endpoints.
	ValidateURLAgainstSSRF(ctx context.Context, rawURL string) (*SSRFValidationResult, error)

	// EnforceIMDSv2 enforces IMDSv2 token requirements and hop limit policies for a node/instance.
	EnforceIMDSv2(ctx context.Context, instanceID string) error

	// ValidateSidecarStartupOrdering verifies that sidecar containers adhere to native initContainer ordering.
	ValidateSidecarStartupOrdering(ctx context.Context, podSpecYAML []byte) (bool, error)
}
