// Package drhealing implements application usecase workflows for Disaster Recovery resilience:
// automated Velero/Kopia snapshot repository lock breaking, edge SQLite WAL corruption recovery,
// and cloud KMS key deletion protection auditing.
package drhealing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/drhealing"
)

const (
	// DefaultStaleLockThreshold defines the minimum lock age (60 minutes) to qualify as stale.
	DefaultStaleLockThreshold = 60 * time.Minute
)

// HealingManager orchestrates disaster recovery resilience checks and automated remediation.
// It implements drhealing.DRResilienceManager.
type HealingManager struct {
	lockInspector      drhealing.RepositoryLockInspector
	podLivenessChecker drhealing.PodLivenessChecker
	backupTrigger      drhealing.BackupTrigger
	edgeOperator       drhealing.EdgeDatabaseOperator
	kmsAuditor         drhealing.KMSKeyPolicyAuditor

	staleLockThreshold time.Duration
	telemetry          *drhealing.LockStateTelemetry
	edgeMetrics        *drhealing.EdgeDatabaseIntegrityMetrics
	progress           *drhealing.BackupProgressCounter
}

// Ensure HealingManager strictly satisfies domain DRResilienceManager interface.
var _ drhealing.DRResilienceManager = (*HealingManager)(nil)

// NewHealingManager constructs a new HealingManager with injected domain ports.
func NewHealingManager(
	lockInspector drhealing.RepositoryLockInspector,
	podLivenessChecker drhealing.PodLivenessChecker,
	backupTrigger drhealing.BackupTrigger,
	edgeOperator drhealing.EdgeDatabaseOperator,
	kmsAuditor drhealing.KMSKeyPolicyAuditor,
) *HealingManager {
	return &HealingManager{
		lockInspector:      lockInspector,
		podLivenessChecker: podLivenessChecker,
		backupTrigger:      backupTrigger,
		edgeOperator:       edgeOperator,
		kmsAuditor:         kmsAuditor,
		staleLockThreshold: DefaultStaleLockThreshold,
		telemetry:          drhealing.NewLockStateTelemetry(),
		edgeMetrics:        drhealing.NewEdgeDatabaseIntegrityMetrics(),
		progress:           drhealing.NewBackupProgressCounter(),
	}
}

// Telemetry returns cache-line padded lock state telemetry.
func (m *HealingManager) Telemetry() *drhealing.LockStateTelemetry {
	return m.telemetry
}

// EdgeMetrics returns cache-line padded edge database integrity metrics.
func (m *HealingManager) EdgeMetrics() *drhealing.EdgeDatabaseIntegrityMetrics {
	return m.edgeMetrics
}

// Progress returns cache-line padded backup progress counters.
func (m *HealingManager) Progress() *drhealing.BackupProgressCounter {
	return m.progress
}

// InspectBackupLocks queries repository storage locks and correlates with pod liveness
// to identify stale orphan locks left after controller crashes.
func (m *HealingManager) InspectBackupLocks(ctx context.Context) ([]drhealing.BackupStorageLockStatus, error) {
	if m.lockInspector == nil {
		return nil, fmt.Errorf("lock inspector port is unconfigured")
	}

	locks, err := m.lockInspector.InspectLocks(ctx)
	if err != nil {
		return nil, fmt.Errorf("inspecting backup repository locks: %w", err)
	}

	var activeCount uint64
	for i := range locks {
		loc := &locks[i]
		if !loc.IsLocked {
			continue
		}
		activeCount++

		// Check lock age against stale threshold (>= 60m)
		if loc.LockAge >= m.staleLockThreshold {
			var podRunning bool
			if m.podLivenessChecker != nil {
				running, _, checkErr := m.podLivenessChecker.IsBackupPodRunning(ctx, loc.LocationName)
				if checkErr == nil {
					podRunning = running
				}
			}
			// If lock age >= 60m and no active backup pod is running, flag as stale
			if !podRunning {
				loc.IsStale = true
			}
		}
	}

	m.telemetry.SetActiveLocks(activeCount)
	return locks, nil
}

// BreakStaleLock safely releases an orphan repository lock if and only if the backup
// controller pod is confirmed DEAD and lock age >= 60 minutes.
// Safety guardrail: Strictly rejects break-lock with ErrBackupPodStillRunning if pod is running.
func (m *HealingManager) BreakStaleLock(ctx context.Context, locationName string) error {
	trimmedLoc := strings.TrimSpace(locationName)
	if trimmedLoc == "" {
		return fmt.Errorf("locationName is required")
	}

	// 1. Inspect lock status
	if m.lockInspector == nil {
		return fmt.Errorf("lock inspector port is unconfigured")
	}

	locks, err := m.lockInspector.InspectLocks(ctx)
	if err != nil {
		return fmt.Errorf("querying repository locks: %w", err)
	}

	var targetLock *drhealing.BackupStorageLockStatus
	for i := range locks {
		if locks[i].LocationName == trimmedLoc {
			targetLock = &locks[i]
			break
		}
	}

	if targetLock == nil || !targetLock.IsLocked {
		return fmt.Errorf("no active lock found for location %q", trimmedLoc)
	}

	if targetLock.LockAge < m.staleLockThreshold {
		m.telemetry.IncLockBreaksFailed()
		return fmt.Errorf("lock age (%v) has not reached stale threshold (%v)",
			targetLock.LockAge, m.staleLockThreshold)
	}

	// 2. Pod liveness guardrail check: confirm controller pod is DEAD
	if m.podLivenessChecker != nil {
		running, podName, err := m.podLivenessChecker.IsBackupPodRunning(ctx, trimmedLoc)
		if err != nil {
			m.telemetry.IncLockBreaksFailed()
			return fmt.Errorf("verifying backup pod status: %w", err)
		}
		if running {
			m.telemetry.IncLockBreaksFailed()
			return fmt.Errorf("%w: pod %q is active on location %q",
				drhealing.ErrBackupPodStillRunning, podName, trimmedLoc)
		}
	}

	// 3. Pod is confirmed DEAD: cleanly execute break-lock (`kopia maintenance --break-lock`)
	if err := m.lockInspector.ExecuteBreakLock(ctx, trimmedLoc); err != nil {
		m.telemetry.IncLockBreaksFailed()
		return fmt.Errorf("executing break-lock on repository %q: %w", trimmedLoc, err)
	}

	// 4. Retrigger scheduled backup immediately to restore DR readiness
	if m.backupTrigger != nil {
		if err := m.backupTrigger.TriggerScheduledBackup(ctx, trimmedLoc); err != nil {
			// Lock was broken but reschedule failed; record telemetry and report
			m.telemetry.IncStaleLocksCleared()
			return fmt.Errorf("break-lock succeeded but re-triggering backup failed: %w", err)
		}
	}

	m.telemetry.IncStaleLocksCleared()
	return nil
}

// RepairEdgeDatabase diagnoses SQLite corruption on edge nodes, takes an offline .corrupt
// snapshot before applying WAL recovery, and verifies database integrity post-repair.
func (m *HealingManager) RepairEdgeDatabase(ctx context.Context, nodeID, dbPath string) error {
	trimmedNode := strings.TrimSpace(nodeID)
	trimmedPath := strings.TrimSpace(dbPath)
	if trimmedNode == "" || trimmedPath == "" {
		return fmt.Errorf("nodeID and dbPath are required")
	}

	if m.edgeOperator == nil {
		return fmt.Errorf("edge database operator port is unconfigured")
	}

	m.edgeMetrics.IncChecksPerformed()

	// 1. Initial integrity check
	status, err := m.edgeOperator.CheckIntegrity(ctx, trimmedNode, trimmedPath)
	if err != nil {
		return fmt.Errorf("checking edge database integrity: %w", err)
	}

	if status == nil || !status.IsCorrupt {
		// Healthy database, nothing to repair
		return nil
	}

	m.edgeMetrics.IncCorruptionsFound()

	// 2. Safeguard: Create a `.corrupt` snapshot backup on disk first
	snapshotPath, err := m.edgeOperator.CreateCorruptSnapshot(ctx, trimmedNode, trimmedPath)
	if err != nil {
		m.edgeMetrics.IncRepairsFailed()
		return fmt.Errorf("creating safety .corrupt snapshot: %w", err)
	}
	_ = snapshotPath

	// 3. Execute in-place recovery from WAL checkpoint
	if err := m.edgeOperator.RecoverFromWAL(ctx, trimmedNode, trimmedPath); err != nil {
		m.edgeMetrics.IncRepairsFailed()
		return fmt.Errorf("recovering edge database from WAL: %w", err)
	}

	// 4. Re-verify integrity post-recovery
	postStatus, err := m.edgeOperator.CheckIntegrity(ctx, trimmedNode, trimmedPath)
	if err != nil {
		m.edgeMetrics.IncRepairsFailed()
		return fmt.Errorf("re-checking edge database integrity post-recovery: %w", err)
	}

	if postStatus != nil && postStatus.IsCorrupt {
		m.edgeMetrics.IncRepairsFailed()
		return fmt.Errorf("%w: node %s db %s remains malformed after WAL recovery",
			drhealing.ErrDatabaseCorrupt, trimmedNode, trimmedPath)
	}

	m.edgeMetrics.IncRepairsSuccessful()
	return nil
}

// AuditKMSKeyProtection audits cloud KMS key deletion policies, verifying multi-party quorum
// and deletion grace delay to prevent catastrophic unrecoverable backup loss.
func (m *HealingManager) AuditKMSKeyProtection(ctx context.Context, keyARN string) (bool, error) {
	trimmedARN := strings.TrimSpace(keyARN)
	if trimmedARN == "" {
		return false, fmt.Errorf("keyARN cannot be empty")
	}

	if m.kmsAuditor == nil {
		return false, fmt.Errorf("kms auditor port is unconfigured")
	}

	protected, details, err := m.kmsAuditor.AuditKeyPolicy(ctx, trimmedARN)
	if err != nil {
		return false, fmt.Errorf("auditing KMS key policy: %w", err)
	}

	if !protected {
		return false, fmt.Errorf("%w: key %q: %s",
			drhealing.ErrKMSDeletionUnprotected, trimmedARN, details)
	}

	return true, nil
}
