// Package secretguard provides application usecase workflows for External Secrets governance,
// polling storm mitigation, cryptographic secret drift reconciliation, and DR key validation.
package secretguard

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/secretguard"
)

// ReconcileResult contains the aggregated metrics from a polling storm audit and mitigation run.
type ReconcileResult struct {
	AuditedCount       int                          `json:"audited_count"`
	ViolationsFound    int                          `json:"violations_found"`
	MutatedCount       int                          `json:"mutated_count"`
	RolloutsTriggered  int                          `json:"rollouts_triggered"`
	RolloutsHalted     int                          `json:"rollouts_halted"`
	DriftDetectedCount int                          `json:"drift_detected_count"`
	Policies           []domain.ExternalSecretPolicy `json:"policies"`
}

// WorkloadHealthChecker abstracts error-rate introspection on running workloads.
type WorkloadHealthChecker interface {
	GetWorkloadErrorRate(ctx context.Context, deploymentName string) (float64, error)
}

// TelemetryState provides lock-free, cache-line padded operational metrics.
// Each metric counter block has 56 bytes of padding so it occupies exactly 64 bytes
// (one x86-64 / ARM64 cache line) to prevent CPU false sharing under high-throughput reconciliation.
type TelemetryState struct {
	AuditRuns      uint64
	_              [56]byte
	MutationsDone  uint64
	_              [56]byte
	RolloutsDone   uint64
	_              [56]byte
	RolloutsHalted uint64
	_              [56]byte
	DriftDetected  uint64
	_              [56]byte
}

// DriftReconciler coordinates secret drift detection, polling rate enforcement, and graceful rollouts.
type DriftReconciler struct {
	detector      domain.DriftDetector
	healthChecker WorkloadHealthChecker

	// Memory cache tracking known baseline hashes by "namespace/name"
	mu            sync.RWMutex
	trackedHashes map[string]string

	// Padded high-frequency state machine metrics
	telemetry TelemetryState
}

// NewDriftReconciler instantiates a new DriftReconciler.
func NewDriftReconciler(detector domain.DriftDetector, healthChecker WorkloadHealthChecker) *DriftReconciler {
	return &DriftReconciler{
		detector:      detector,
		healthChecker: healthChecker,
		trackedHashes: make(map[string]string),
	}
}

// TrackSecretBaseline records the initial or updated baseline hash for a secret.
func (r *DriftReconciler) TrackSecretBaseline(policy domain.ExternalSecretPolicy) error {
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("validating policy: %w", err)
	}
	key := policy.Namespace + "/" + policy.Name
	r.mu.Lock()
	r.trackedHashes[key] = policy.SecretDataHash
	r.mu.Unlock()
	return nil
}

// AuditAndMitigatePollingStorms queries external secret CRDs across namespaces.
// If any CRD specifies a refreshInterval < 1h, it automatically flags the violation and
// applies a safe interval (1h + randomized jitter up to 10m) to defeat synchronized Vault DDoS spikes.
func (r *DriftReconciler) AuditAndMitigatePollingStorms(ctx context.Context, namespace string) (*ReconcileResult, error) {
	atomic.AddUint64(&r.telemetry.AuditRuns, 1)

	if r.detector == nil {
		return &ReconcileResult{
			AuditedCount: 0,
			Policies:     make([]domain.ExternalSecretPolicy, 0),
		}, nil
	}

	policies, err := r.detector.AuditExternalSecrets(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("auditing external secrets: %w", err)
	}

	result := &ReconcileResult{
		AuditedCount: len(policies),
		Policies:     make([]domain.ExternalSecretPolicy, 0, len(policies)),
	}

	for _, policy := range policies {
		p := policy
		if p.IsViolation() {
			result.ViolationsFound++

			// Calculate randomized jitter: 3600s + rand(600s)
			jitteredInterval, jitterErr := calculateJitterInterval()
			if jitterErr != nil {
				return nil, fmt.Errorf("generating interval jitter: %w", jitterErr)
			}

			if err := r.detector.MutateRefreshInterval(ctx, p.Namespace, p.Name, jitteredInterval); err != nil {
				return nil, fmt.Errorf("mutating refresh interval for %s/%s: %w", p.Namespace, p.Name, err)
			}

			p.EnforcedInterval = jitteredInterval
			result.MutatedCount++
			atomic.AddUint64(&r.telemetry.MutationsDone, 1)
		} else {
			p.EnforcedInterval = p.ConfiguredInterval
		}
		result.Policies = append(result.Policies, p)
	}

	return result, nil
}

// ReconcileSecretDataHash evaluates whether a secret's cryptographic hash has drifted.
// When drift is identified:
// 1. If the workload supports dynamic in-memory reload (ReloaderAutoRestart=true), no restart is needed.
// 2. Otherwise, verifies that workload error rate <= 5%. If error rate > 5%, halts restart immediately.
// 3. If within safety limits, triggers a controlled graceful rolling restart.
func (r *DriftReconciler) ReconcileSecretDataHash(ctx context.Context, policy domain.ExternalSecretPolicy, targetDeployment string) error {
	key := policy.Namespace + "/" + policy.Name

	r.mu.RLock()
	cachedHash, exists := r.trackedHashes[key]
	r.mu.RUnlock()

	// If no prior hash, register baseline and return
	if !exists {
		r.mu.Lock()
		r.trackedHashes[key] = policy.SecretDataHash
		r.mu.Unlock()
		return nil
	}

	// Hash unchanged -> no drift
	if cachedHash == policy.SecretDataHash {
		return nil
	}

	// Secret data hash drifted!
	atomic.AddUint64(&r.telemetry.DriftDetected, 1)

	// If auto-restart / dynamic reloader is enabled, workload reloads internally
	if policy.ReloaderAutoRestart {
		r.mu.Lock()
		r.trackedHashes[key] = policy.SecretDataHash
		r.mu.Unlock()
		return nil
	}

	// Workload requires rolling restart. First evaluate safety guardrails.
	if r.healthChecker != nil && targetDeployment != "" {
		errRate, err := r.healthChecker.GetWorkloadErrorRate(ctx, targetDeployment)
		if err != nil {
			return fmt.Errorf("evaluating workload health for %s: %w", targetDeployment, err)
		}
		if errRate > domain.MaxTolerableErrorRate {
			atomic.AddUint64(&r.telemetry.RolloutsHalted, 1)
			return fmt.Errorf("rollout safety guardrail triggered (error rate %.2f%% > 5%%): %w", errRate*100, domain.ErrRolloutDegraded)
		}
	}

	// Trigger controlled graceful rolling restart
	if err := r.detector.TriggerGracefulRollout(ctx, targetDeployment); err != nil {
		return fmt.Errorf("triggering rollout for %s: %w", targetDeployment, err)
	}

	atomic.AddUint64(&r.telemetry.RolloutsDone, 1)

	r.mu.Lock()
	r.trackedHashes[key] = policy.SecretDataHash
	r.mu.Unlock()

	return nil
}

// VerifyMasterKeyBackupBeforeRotation verifies that off-cluster DR backups exist
// for the specified KMS or SealedSecrets key before key rotation or migration.
func (r *DriftReconciler) VerifyMasterKeyBackupBeforeRotation(ctx context.Context, kmsKeyURI string) (bool, error) {
	if kmsKeyURI == "" {
		return false, fmt.Errorf("kms key URI cannot be empty: %w", domain.ErrMasterKeyBackupMissing)
	}

	verified, err := r.detector.VerifySealedKeyBackup(ctx, kmsKeyURI)
	if err != nil {
		return false, fmt.Errorf("verifying master key backup: %w", err)
	}
	if !verified {
		return false, domain.ErrMasterKeyBackupMissing
	}
	return true, nil
}

// GetTelemetrySnapshot returns a point-in-time copy of the reconciler counters.
func (r *DriftReconciler) GetTelemetrySnapshot() TelemetryState {
	return TelemetryState{
		AuditRuns:      atomic.LoadUint64(&r.telemetry.AuditRuns),
		MutationsDone:  atomic.LoadUint64(&r.telemetry.MutationsDone),
		RolloutsDone:   atomic.LoadUint64(&r.telemetry.RolloutsDone),
		RolloutsHalted: atomic.LoadUint64(&r.telemetry.RolloutsHalted),
		DriftDetected:  atomic.LoadUint64(&r.telemetry.DriftDetected),
	}
}

// calculateJitterInterval generates a randomized interval of 3600s + rand(0..600s)
// using crypto/rand for cryptographically uniform distribution.
func calculateJitterInterval() (time.Duration, error) {
	// 600 seconds max jitter
	maxJitterSeconds := int64(domain.MaxJitter / time.Second)
	n, err := rand.Int(rand.Reader, big.NewInt(maxJitterSeconds+1))
	if err != nil {
		return 0, err
	}
	jitter := time.Duration(n.Int64()) * time.Second
	return domain.MinSafetyInterval + jitter, nil
}
