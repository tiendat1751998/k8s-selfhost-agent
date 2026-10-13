// Package spiffeguard defines domain contracts, entities, and hardware-aligned metrics
// for SPIFFE/SPIRE workload identity rotation, projected service account token safeguards,
// and clock skew validation in zero-trust Kubernetes architectures.
//
// ==============================================================================================
// PHYSICAL HARDWARE BOTTLENECK & REAL-WORLD FAILURE MODE ANALYSIS:
//
// 1. Unix Domain Socket Backlog Queue Saturation (/tmp/spire-agent/public/api.sock):
//    - The SPIRE Agent exposes the Workload API over a Unix Domain Socket (UDS). Under Linux,
//      UDS connection queues are governed by the kernel parameter net.core.somaxconn (often
//      defaulting to 128 or 4096) and the listener backlog passed to listen(2).
//    - When a Kubernetes node recovers or during mass daemonset/workload rollout, hundreds of pods
//      simultaneously open gRPC/UDS streams to /tmp/spire-agent/public/api.sock.
//    - Once the socket listen backlog overflows, the Linux kernel drops incoming SYN/connection packets
//      or returns ECONNREFUSED/ETIMEDOUT. Without intelligent client-side jitter and exponential backoff,
//      workloads enter a thundering herd cycle, exhausting local file descriptors and causing cluster-wide
//      mTLS handshake rejections ("Bad Certificate" / connection reset).
//
// 2. SPIRE Agent CPU Exhaustion During Mass CSR Generation:
//    - Workload SVID rotation requires generating cryptographic keypairs and Certificate Signing
//      Requests (CSRs). While modern curves (Ed25519, ECDSA P-256) are optimized via AVX-512 and crypto
//      accelerators (~50-100 microseconds per keygen), legacy RSA-2048 operations require 2-5 milliseconds
//      of intensive prime generation per certificate.
//    - When hundreds of workloads trigger synchronous SVID renewal at identical 50% TTL boundaries,
//      the single SPIRE Agent container on the node consumes 100% CPU quota, triggering CFS throttling
//      and delaying all downstream SVID issuance.
//
// 3. Kubelet Projected Service Account Token Disk Sync I/O Stalls:
//    - Kubernetes mounts projected service account tokens into containers via tmpfs or memory-backed
//      emptyDir volumes. However, the Kubelet atomic token renewal loop writes tokens to a temporary
//      symlink tree (..data_tmp -> ..data) and calls fsync.
//    - By default, Kubelet's token sync period is approximately 1 minute. If node disk I/O or VFS
//      buffer cache contention stalls directory inode updates, or if inotify events are dropped due to
//      fs.inotify.max_user_watches limits, the application process reads stale token data from memory,
//      resulting in 401 Unauthorized API rejections when interacting with cloud OIDC providers.
//
// 4. NTP Clock Drift Between Nodes and OIDC Providers:
//    - Zero-Trust OIDC tokens enforce strict Issued At ("iat") and Expiration ("exp") claims.
//    - In physical multi-cloud or hybrid edge clusters, nodes utilizing slewed NTP corrections can drift
//      by 500ms to several seconds relative to cloud Identity Providers (AWS IAM, GCP Workload Identity,
//      Keycloak). If a token's "iat" timestamp is evaluated on a node whose clock is behind, the token
//      appears to originate in the future, triggering immediate clock skew validation failures and login lockouts.
// ==============================================================================================
package spiffeguard

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

// Standard Domain Errors for SPIFFE/SPIRE Identity Safeguard.
var (
	// ErrSVIDBacklogSaturated is returned when the SPIRE agent Unix domain socket backlog is exhausted (HTTP 503, RFC 7807).
	ErrSVIDBacklogSaturated = errors.New("SPIRE agent unix domain socket backlog saturated (HTTP 503)")

	// ErrTokenClockSkewExceeded is returned when the token 'iat' timestamp drifts beyond allowed leeway (HTTP 401).
	ErrTokenClockSkewExceeded = errors.New("token clock skew exceeded allowable leeway (HTTP 401)")

	// ErrTokenStaleExpired is returned when a projected service account token is stale or expired (HTTP 401).
	ErrTokenStaleExpired = errors.New("projected service account token is stale or expired (HTTP 401)")
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

// Load atomically loads the counter value.
func (c *PaddedCounter) Load() uint64 {
	return atomic.LoadUint64(&c.value)
}

// Reset atomically resets the counter value to zero.
func (c *PaddedCounter) Reset() {
	atomic.StoreUint64(&c.value, 0)
}

// IdentityMetrics groups hardware cache-line aligned metrics for high-throughput SPIFFE & token lifecycle.
type IdentityMetrics struct {
	SVIDRenewalsTotal           PaddedCounter
	SVIDBacklogDropsTotal       PaddedCounter
	TokenReloadsTotal           PaddedCounter
	TokenEmergencyRequestsTotal PaddedCounter
	ClockSkewViolationsTotal    PaddedCounter
	ActiveTokensWatched         PaddedCounter
}

// NewIdentityMetrics initializes a new IdentityMetrics instance.
func NewIdentityMetrics() *IdentityMetrics {
	return &IdentityMetrics{}
}

// SVIDStatus captures the health and rotation status of a workload SPIFFE SVID.
type SVIDStatus struct {
	SPIFFEID     string        `json:"spiffe_id"`
	ExpiresAt    time.Time     `json:"expires_at"`
	TimeToLive   time.Duration `json:"time_to_live"`
	IsNearExpiry bool          `json:"is_near_expiry"`
	CertSerial   string        `json:"cert_serial"`
}

// NewSVIDStatus constructs an SVIDStatus with evaluated IsNearExpiry flag.
// A certificate is considered near expiry if remaining TTL is less than or equal to 50% of total lifetime.
func NewSVIDStatus(spiffeID string, expiresAt time.Time, totalLifetime time.Duration, certSerial string) *SVIDStatus {
	ttl := time.Until(expiresAt)
	isNearExpiry := false

	if totalLifetime > 0 {
		threshold := totalLifetime / 2
		isNearExpiry = ttl <= threshold
	} else {
		// Fallback if total lifetime not provided: assume near expiry if TTL <= 30 minutes or expired
		isNearExpiry = ttl <= 30*time.Minute
	}

	return &SVIDStatus{
		SPIFFEID:     spiffeID,
		ExpiresAt:    expiresAt,
		TimeToLive:   ttl,
		IsNearExpiry: isNearExpiry,
		CertSerial:   certSerial,
	}
}

// TokenWatchStatus captures the status of an inotify/fs-monitored projected service account token.
type TokenWatchStatus struct {
	TokenPath     string    `json:"token_path"`
	LastRefreshed time.Time `json:"last_refreshed"`
	ExpiresAt     time.Time `json:"expires_at"`
	ClockSkewSecs float64   `json:"clock_skew_secs"`
	IsStale       bool      `json:"is_stale"`
}

// NewTokenWatchStatus constructs a TokenWatchStatus with freshness evaluation.
func NewTokenWatchStatus(tokenPath string, lastRefreshed, expiresAt time.Time, clockSkewSecs float64) *TokenWatchStatus {
	now := time.Now()
	isStale := now.After(expiresAt)

	return &TokenWatchStatus{
		TokenPath:     tokenPath,
		LastRefreshed: lastRefreshed,
		ExpiresAt:     expiresAt,
		ClockSkewSecs: clockSkewSecs,
		IsStale:       isStale,
	}
}

// WorkloadIdentityManager defines domain operations for SPIFFE SVID rotation and projected token safeguards.
type WorkloadIdentityManager interface {
	// InspectSVID inspects the current workload SVID status and expiration via SPIRE Workload API socket.
	InspectSVID(ctx context.Context, socketPath string) (*SVIDStatus, error)

	// RenewSVIDWithJitter triggers SVID renewal applying randomized jitter when TTL <= 50% to prevent thundering herds.
	RenewSVIDWithJitter(ctx context.Context, socketPath string, maxJitter time.Duration) (*SVIDStatus, error)

	// WatchProjectedToken monitors projected token staleness and triggers fallback scoped token requests if required.
	WatchProjectedToken(ctx context.Context, tokenPath string, leeway time.Duration) (*TokenWatchStatus, error)

	// ValidateTokenClockSkew validates token 'iat' timestamp against current host time within allowed skew bounds.
	ValidateTokenClockSkew(ctx context.Context, tokenIAT time.Time, allowedSkew time.Duration) error
}
