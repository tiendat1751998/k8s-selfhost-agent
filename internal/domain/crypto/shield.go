package crypto

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

/*
ENGINEERING ANALYSIS NOTE: PHYSICAL HARDWARE BOTTLENECKS & REAL-WORLD FAILURE MODES
===================================================================================

1. Physical Hardware Bottlenecks under Enterprise Cryptographic Workloads:
   - PCIe PKCS#11 HSM Hardware Call Latency:
     Hardware Security Modules (HSMs, e.g. Thales Luna, AWS CloudHSM, Nitro Enclaves) interface
     over PCIe or dedicated local IP fabrics via PKCS#11 cryptoki C APIs. Every cryptographic
     operation (C_Sign, C_Decrypt) incurs user-to-kernel context switching, PCIe bus roundtrips,
     firmware command packetization (APDU/eCos commands), and hardware crypto accelerator ASIC/FPGA
     queue queuing. Typical single-operation latency is 2ms to 12ms. Unlike software CPU routines,
     HSM hardware execution cannot scale beyond fixed physical ASIC pipeline depth; attempting to
     pipeline thousands of concurrent signing requests causes driver command buffer overflow.
   - CA OCSP Responder HTTP Latency & Network Timeouts:
     Synchronous OCSP verification requires HTTP GET/POST queries over the public Internet to
     external Certificate Authority (CA) OCSP responder URLs parsed from X.509 AIA extensions.
     Public CA responders frequently experience cross-WAN latency (50ms - 500ms), BGP routing flapping,
     and periodic DDoS attacks. Performing inline OCSP lookups during TLS handshakes spikes connection
     tail latencies and degrades throughput by orders of magnitude.
   - RSA-4096 vs ECC P-384 Crypto Signing CPU Cycles:
     RSA-4096 signature generation relies on modular exponentiation with 4096-bit primes, requiring
     O(n^3) multi-precision arithmetic. A single RSA-4096 sign consumes ~15,000,000 CPU cycles (~5ms
     per single core on 3.0GHz Intel Xeon/AMD EPYC). In contrast, ECDSA P-384 point multiplication
     uses curve Weierstrass coordinates and projective Montgomery arithmetic requiring ~1,200,000
     CPU cycles (~0.4ms), yielding a 12x throughput improvement while providing 192-bit security strength.
   - Cloud KMS Network RPC Latency during DEK Decrypt:
     Envelope encryption requires calling Cloud KMS (GCP Cloud KMS, AWS KMS, HashiCorp Vault) to decrypt
     the Data Encryption Key (DEK). Every KMS Decrypt call involves gRPC/mTLS handshakes, OAuth2 token
     validation, regional KMS endpoint evaluation, and HSM backend decryption. Synchronous decrypt
     calls consume 15ms to 60ms of roundtrip RPC latency, creating an intolerable I/O bottleneck
     for high-throughput database read pipelines unless protected by in-memory stale-while-revalidate caching.

2. Production Failure Modes & Architectural Mitigations:
   - CA OCSP Server Blackout & Fail-Closed TLS Aborts:
     When enterprise edge proxies or payment gateways enforce strict OCSP checking with fail-closed
     semantics, a CA OCSP responder outage causes all incoming client TLS handshakes to instantly fail.
     This collapses critical banking and payment APIs.
     Mitigation: EnterpriseCryptoShield implements proactive OCSP prefetching (4 hours prior to NextUpdate)
     and an in-memory 24-hour grace retention window, ensuring continuous TLS termination even during
     prolonged CA network outages.
   - HSM Session Pool Saturation (Hard 256-Session Ceiling):
     Physical HSMs and PKCS#11 drivers enforce a hard limit on open concurrent sessions (typically 256
     or 512 sessions). Under sudden authentication traffic bursts, unconstrained session creation exhausts
     the HSM driver table, causing C_OpenSession to block or return CKR_DEVICE_MEMORY / CKR_SESSION_COUNT.
     Blocked goroutines accumulate exponentially, exhausting Go runtime memory and freezing all JWT signing.
     Mitigation: Bounded semaphore channel pool with a hard 256-session ceiling and 500ms fast-fail
     timeout (ErrHSMPoolSaturated), shedding load and preserving system responsiveness.
   - OpenSSL FIPS Mode Crash on Legacy MD5/SHA-1 Calls:
     When a container or host operating system is booted in strict FIPS 140-2/140-3 mode, the OpenSSL/BoringSSL
     FIPS canister intercepts calls to non-approved cryptographic primitives (MD5, SHA-1, DES). Invoking
     legacy hash functions triggers an uncatchable FIPS self-test panic or SIGABRT, crashing the entire pod.
     Mitigation: Static AST and source compliance scanner (ScanSourceForNonFIPSAlgorithms) to discover
     and reject banned algorithms before production deployment.
   - Cloud KMS Outages Evicting DEKs Rendering Databases Unreadable:
     Transient network partitions or IAM quota throttling to Cloud KMS causes on-demand DEK decryption
     to fail with HTTP 503 Service Unavailable. Naive caches evict keys upon TTL expiry, resulting in
     immediate database query failures.
     Mitigation: Envelope DEK Stale-While-Revalidate cache with a strict 4-hour grace window, permitting
     authenticated cryptographic workflows to proceed safely during temporary KMS disruptions.
*/

var (
	// ErrOCSPStapleFetchFailed indicates failure to fetch or validate OCSP staple from responder (HTTP 502, RFC 7807).
	ErrOCSPStapleFetchFailed = errors.New("ocsp staple fetch failed")

	// ErrHSMPoolSaturated indicates all HSM hardware sessions are in use and acquire timed out (HTTP 503).
	ErrHSMPoolSaturated = errors.New("hsm session pool saturated")

	// ErrFIPSNonCompliantHash indicates detection of non-FIPS approved cryptographic algorithms (HTTP 422).
	ErrFIPSNonCompliantHash = errors.New("fips non-compliant cryptographic algorithm detected")

	// ErrKMSDEKGraceExpired indicates the DEK stale grace period has lapsed without successful KMS refresh (HTTP 500).
	ErrKMSDEKGraceExpired = errors.New("kms dek grace period expired")
)

// PaddedCounter is a 64-byte cache-line aligned atomic counter.
// The 56-byte trailing array ensures that adjacent fields in memory
// reside on separate CPU cache lines, eliminating false sharing under MESI/MOESI cache coherence.
type PaddedCounter struct {
	value uint64
	_     [7]uint64 // 56 bytes padding (total: 64 bytes)
}

// Inc atomically increments the counter and returns the new value.
func (c *PaddedCounter) Inc() uint64 {
	return atomic.AddUint64(&c.value, 1)
}

// Add atomically adds delta to the counter and returns the new value.
func (c *PaddedCounter) Add(delta uint64) uint64 {
	return atomic.AddUint64(&c.value, delta)
}

// Load atomically loads and returns the counter value.
func (c *PaddedCounter) Load() uint64 {
	return atomic.LoadUint64(&c.value)
}

// Store atomically stores the given value.
func (c *PaddedCounter) Store(v uint64) {
	atomic.StoreUint64(&c.value, v)
}

// CryptoMetrics tracks cache-line isolated operational metrics across cryptographic subsystems.
type CryptoMetrics struct {
	OCSPPrefetches    PaddedCounter
	OCSPGraceHits     PaddedCounter
	OCSPFailures      PaddedCounter
	HSMAcquisitions   PaddedCounter
	HSMReleases       PaddedCounter
	HSMSaturations    PaddedCounter
	DEKCacheHits      PaddedCounter
	DEKKMSFetches     PaddedCounter
	DEKStaleHits      PaddedCounter
	DEKGraceExpired   PaddedCounter
	FIPSViolations    PaddedCounter
}

// OCSPStapleStatus captures the revocation and stapling state of a domain certificate.
type OCSPStapleStatus struct {
	Domain           string    `json:"domain"`
	HasStaple        bool      `json:"has_staple"`
	NextUpdate       time.Time `json:"next_update"`
	IsExpired        bool      `json:"is_expired"`
	ResponderURL     string    `json:"responder_url"`
	RevocationStatus string    `json:"revocation_status,omitempty"`
	ProducedAt       time.Time `json:"produced_at,omitempty"`
	GraceValidUntil  time.Time `json:"grace_valid_until,omitempty"`
	IsGraceUsed      bool      `json:"is_grace_used"`
}

// NeedsPrefetch returns true if the current time is within prefetchWindow before NextUpdate or expired.
func (s *OCSPStapleStatus) NeedsPrefetch(now time.Time, prefetchWindow time.Duration) bool {
	if !s.HasStaple || s.NextUpdate.IsZero() {
		return true
	}
	return now.After(s.NextUpdate.Add(-prefetchWindow))
}

// IsWithinGrace returns true if the cached staple is expired but within the allowable grace window past NextUpdate.
func (s *OCSPStapleStatus) IsWithinGrace(now time.Time, graceWindow time.Duration) bool {
	if !s.HasStaple || s.NextUpdate.IsZero() {
		return false
	}
	if now.Before(s.NextUpdate) {
		return true
	}
	return now.Sub(s.NextUpdate) <= graceWindow
}

// HSMSessionPoolStatus captures the real-time capacity and health of the hardware session pool.
type HSMSessionPoolStatus struct {
	MaxSessions       int           `json:"max_sessions"`
	ActiveSessions    int           `json:"active_sessions"`
	AvailableSessions int           `json:"available_sessions"`
	AcquireWaitTime   time.Duration `json:"acquire_wait_time_ns"`
	IsSaturated       bool          `json:"is_saturated"`
}

// Validate checks invariant constraints for HSM session pool metrics.
func (s *HSMSessionPoolStatus) Validate() error {
	if s.MaxSessions <= 0 {
		return fmt.Errorf("max_sessions must be greater than 0, got %d", s.MaxSessions)
	}
	if s.ActiveSessions < 0 || s.ActiveSessions > s.MaxSessions {
		return fmt.Errorf("active_sessions %d out of bounds [0, %d]", s.ActiveSessions, s.MaxSessions)
	}
	return nil
}

// EnvelopeCacheEntry represents a locally cached plaintext Data Encryption Key (DEK).
type EnvelopeCacheEntry struct {
	KeyID        string        `json:"key_id"`
	PlaintextKey []byte        `json:"-"`
	CachedAt     time.Time     `json:"cached_at"`
	TTL          time.Duration `json:"ttl_ns"`
	GracePeriod  time.Duration `json:"grace_period_ns"`
	IsStaleValid bool          `json:"is_stale_valid"`
}

// IsFresh returns true if the cached entry has not yet exceeded its primary TTL.
func (e *EnvelopeCacheEntry) IsFresh(now time.Time) bool {
	if e.CachedAt.IsZero() || len(e.PlaintextKey) == 0 {
		return false
	}
	return now.Sub(e.CachedAt) <= e.TTL
}

// IsWithinGrace returns true if the entry is within the allowable extended grace window (TTL + GracePeriod).
func (e *EnvelopeCacheEntry) IsWithinGrace(now time.Time) bool {
	if e.CachedAt.IsZero() || len(e.PlaintextKey) == 0 {
		return false
	}
	return now.Sub(e.CachedAt) <= (e.TTL + e.GracePeriod)
}

// EnterpriseCryptoShield defines the domain contract for enterprise PKI and hardware cryptography safeguarding.
type EnterpriseCryptoShield interface {
	// VerifyAndPrefetchOCSPStaple validates an X.509 certificate chain and prefetches signed OCSP staple responses.
	VerifyAndPrefetchOCSPStaple(ctx context.Context, certChainPEM []byte) (*OCSPStapleStatus, error)

	// AcquireHSMSession reserves a hardware session from the bounded pool with fast-fail timeout protection.
	AcquireHSMSession(ctx context.Context, timeout time.Duration) (sessionID uint64, release func(), err error)

	// GetOrExtendEnvelopeDEK retrieves a plaintext DEK from cache or fallback KMS with stale-while-revalidate grace.
	GetOrExtendEnvelopeDEK(ctx context.Context, keyID string, kmsFallbackFunc func() ([]byte, error)) ([]byte, error)

	// ScanSourceForNonFIPSAlgorithms audits source files for banned cryptographic algorithms.
	ScanSourceForNonFIPSAlgorithms(ctx context.Context, rootDir string) (violations []string, err error)
}
