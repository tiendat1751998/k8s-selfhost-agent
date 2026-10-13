// Package secretguard provides domain models and interfaces for enterprise Secret Store CSI,
// External Secrets Operator governance, and runtime secret drift reconciliation.
//
// =================================================================================================
// ARCHITECTURAL ENGINEERING NOTE: PHYSICAL HARDWARE BOTTLENECKS & REAL-WORLD FAILURE MODES
// =================================================================================================
// In high-scale Kubernetes deployments with hundreds of tenants and tens of thousands of pods,
// naive secret management and automated rotation trigger severe cascade failures:
//
// 1. PHYSICAL HARDWARE & INFRASTRUCTURE BOTTLENECKS:
//    - Vault / Cloud KMS API Rate Limits: Cloud KMS providers (AWS KMS, Google Cloud KMS, HashiCorp
//      Vault) impose strict rate limits (e.g., AWS KMS baseline of 10,000 req/sec per region, Vault
//      Raft leader commit serialization). Uncoordinated polling loops from thousands of ExternalSecret
//      CRDs exhaust client quotas, triggering HTTP 429 Too Many Requests and locking down operational workloads.
//    - etcd Key-Value Watch Overhead: Every secret update writes a new revision to etcd, generating watch
//      events across all controller-manager and kubelet watchers. Rapid changes to thousands of Secret objects
//      cause raft log bloat, disk fsync latency spikes (>100ms), and etcd leader heartbeats to time out.
//    - Container Filesystem cgroup I/O Spikes: Triggering mass rolling restarts flushes container page caches,
//      pulls container images, and floods local NVMe / EBS storage with overlay2 mount/unmount operations,
//      choking cgroup v2 I/O queues and starving co-located production workloads.
//    - TLS Handshake CPU Overhead: Synchronized pod restarts initiate simultaneous TLS handshakes (ECDHE/RSA)
//      against Vault and KMS endpoints, saturating cluster node CPU cores with cryptographic verification.
//
// 2. REAL-WORLD FAILURE MODES:
//    - ExternalSecrets Polling Storm DDoS: Developers configuring 'refreshInterval: 10s' create a synchronized
//      polling stampede against Vault, exhausting HTTP connection pools and taking down the central Vault cluster.
//    - In-Memory Container Secret Staleness: Kubernetes Secret updates are mounted into pods asynchronously
//      or projected via CSI. Long-lived applications (e.g. JVM JDBC pools, Go pgx connection pools) cache credentials
//      in heap memory. When the upstream database password rotates, old connection pool sessions fail en masse,
//      causing instant cascade outages unless a graceful rolling rollout is sequenced.
//    - Disaster Recovery Master Key Loss: SealedSecrets and SOPS rely on an asymmetric master private key or
//      Cloud KMS key ring. If GitOps manifests are continuously applied without verifying that disaster recovery
//      backups of the decryption key exist off-cluster, any cluster rebuild results in permanent, unrecoverable
//      data loss of all encrypted secrets.
// =================================================================================================
package secretguard

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Constants defining safe operational thresholds for secret reconciliation.
const (
	// MinSafetyInterval defines the absolute minimum refresh interval floor for static external secrets.
	MinSafetyInterval = 1 * time.Hour
	// MaxJitter defines the maximum randomized jitter added to refresh intervals to prevent polling stampedes.
	MaxJitter = 10 * time.Minute
	// MaxTolerableErrorRate defines the safety guardrail threshold (5%) above which rolling restarts are halted.
	MaxTolerableErrorRate = 0.05
)

// Domain sentinel errors.
var (
	// ErrSecretPollingRateTooHigh is returned when an ExternalSecret specifies a refresh rate below 1h (HTTP 400).
	ErrSecretPollingRateTooHigh = errors.New("external secret polling rate too high: minimum allowed interval is 1h")
	// ErrRolloutDegraded is returned when workload error rates exceed 5% during rolling restart reconciliation.
	ErrRolloutDegraded = errors.New("rollout degraded: error rate exceeded 5% safety threshold")
	// ErrMasterKeyBackupMissing is returned when a SealedSecrets/KMS master key lacks a verified disaster recovery backup.
	ErrMasterKeyBackupMissing = errors.New("disaster recovery master key backup missing")
)

// SecretProviderType identifies upstream external secret engines.
type SecretProviderType string

const (
	// ProviderVault represents HashiCorp Vault.
	ProviderVault SecretProviderType = "vault"
	// ProviderAWS represents AWS Secrets Manager / Parameter Store.
	ProviderAWS SecretProviderType = "aws"
	// ProviderGCP represents Google Secret Manager.
	ProviderGCP SecretProviderType = "gcp"
	// ProviderInCluster represents in-cluster SealedSecrets or CSI Secret Store.
	ProviderInCluster SecretProviderType = "in-cluster"
)

// ExternalSecretPolicy defines the operational governance policy for an ExternalSecret CRD.
type ExternalSecretPolicy struct {
	Namespace           string             `json:"namespace"`
	Name                string             `json:"name"`
	Provider            SecretProviderType `json:"provider"`
	ConfiguredInterval  time.Duration      `json:"configured_interval"`
	EnforcedInterval    time.Duration      `json:"enforced_interval"`
	ReloaderAutoRestart bool               `json:"reloader_auto_restart"`
	SecretDataHash      string             `json:"secret_data_hash"`
	LastSyncedAt        time.Time          `json:"last_synced_at"`
}

// IsViolation checks whether the configured polling interval violates the minimum safety floor.
func (p *ExternalSecretPolicy) IsViolation() bool {
	return p.ConfiguredInterval < MinSafetyInterval
}

// Validate checks invariant constraints on the ExternalSecretPolicy entity.
func (p *ExternalSecretPolicy) Validate() error {
	if strings.TrimSpace(p.Namespace) == "" {
		return fmt.Errorf("namespace is required: %w", ErrSecretPollingRateTooHigh)
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("name is required: %w", ErrSecretPollingRateTooHigh)
	}
	switch p.Provider {
	case ProviderVault, ProviderAWS, ProviderGCP, ProviderInCluster:
	default:
		return fmt.Errorf("invalid secret provider '%s'", p.Provider)
	}
	return nil
}

// ReconcilerCounters tracks high-frequency telemetry counters during reconciliation.
// To prevent CPU false sharing on x86/ARM64 64-byte cache lines under concurrent multi-tenant loads,
// each 8-byte uint64 counter is followed by 56 bytes of explicit cache-line padding.
type ReconcilerCounters struct {
	AuditRuns      uint64
	_              [56]byte // 64-byte cache-line alignment padding
	MutationsDone  uint64
	_              [56]byte // 64-byte cache-line alignment padding
	RolloutsDone   uint64
	_              [56]byte // 64-byte cache-line alignment padding
	RolloutsFailed uint64
	_              [56]byte // 64-byte cache-line alignment padding
	DriftDetected  uint64
	_              [56]byte // 64-byte cache-line alignment padding
}

// DriftDetector defines the domain ports for external secret inspection, mutation,
// workload restart orchestration, and master key disaster recovery verification.
type DriftDetector interface {
	// AuditExternalSecrets inspects all external secret policies within a given namespace.
	AuditExternalSecrets(ctx context.Context, namespace string) ([]ExternalSecretPolicy, error)
	// MutateRefreshInterval updates the CRD refresh interval to an enforced floor.
	MutateRefreshInterval(ctx context.Context, ns, name string, minInterval time.Duration) error
	// TriggerGracefulRollout executes a canary or rolling restart on the designated deployment.
	TriggerGracefulRollout(ctx context.Context, targetDeployment string) error
	// VerifySealedKeyBackup verifies that an off-cluster DR backup exists for the given master key URI.
	VerifySealedKeyBackup(ctx context.Context, kmsKeyURI string) (bool, error)
}
