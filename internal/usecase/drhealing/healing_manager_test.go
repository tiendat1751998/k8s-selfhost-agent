package drhealing

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/drhealing"
)

// Fake implementations for domain ports (zero external mocks)
type fakeRepoLockInspector struct {
	mu           sync.Mutex
	locks        []drhealing.BackupStorageLockStatus
	breakCalled  map[string]bool
	breakLockErr error
}

func (f *fakeRepoLockInspector) InspectLocks(ctx context.Context) ([]drhealing.BackupStorageLockStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	res := make([]drhealing.BackupStorageLockStatus, len(f.locks))
	copy(res, f.locks)
	return res, nil
}

func (f *fakeRepoLockInspector) ExecuteBreakLock(ctx context.Context, locationName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.breakLockErr != nil {
		return f.breakLockErr
	}
	if f.breakCalled == nil {
		f.breakCalled = make(map[string]bool)
	}
	f.breakCalled[locationName] = true
	// clear lock
	for i := range f.locks {
		if f.locks[i].LocationName == locationName {
			f.locks[i].IsLocked = false
			f.locks[i].IsStale = false
		}
	}
	return nil
}

type fakePodLivenessChecker struct {
	mu          sync.Mutex
	podRunning  map[string]bool
	podNames    map[string]string
	checkErr    error
}

func (f *fakePodLivenessChecker) IsBackupPodRunning(ctx context.Context, locationName string) (bool, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.checkErr != nil {
		return false, "", f.checkErr
	}
	running := f.podRunning[locationName]
	name := f.podNames[locationName]
	return running, name, nil
}

type fakeBackupTrigger struct {
	mu            sync.Mutex
	triggered     map[string]bool
	triggerErr    error
}

func (f *fakeBackupTrigger) TriggerScheduledBackup(ctx context.Context, locationName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.triggerErr != nil {
		return f.triggerErr
	}
	if f.triggered == nil {
		f.triggered = make(map[string]bool)
	}
	f.triggered[locationName] = true
	return nil
}

type fakeEdgeDBOperator struct {
	mu                sync.Mutex
	integrityStatus   map[string]*drhealing.EdgeDatabaseIntegrity
	snapshotCreated   map[string]string
	walRecoverCalled  map[string]bool
	failPostRecovery  bool
}

func (f *fakeEdgeDBOperator) CheckIntegrity(ctx context.Context, nodeID, dbPath string) (*drhealing.EdgeDatabaseIntegrity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := nodeID + ":" + dbPath
	status, exists := f.integrityStatus[key]
	if !exists {
		return &drhealing.EdgeDatabaseIntegrity{
			NodeID:         nodeID,
			DBPath:         dbPath,
			JournalMode:    "wal",
			Synchronous:    "NORMAL",
			IsCorrupt:      false,
			IntegrityCheck: "ok",
			LastVerifiedAt: time.Now().UTC(),
		}, nil
	}
	return status, nil
}

func (f *fakeEdgeDBOperator) CreateCorruptSnapshot(ctx context.Context, nodeID, dbPath string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.snapshotCreated == nil {
		f.snapshotCreated = make(map[string]string)
	}
	snapPath := dbPath + ".corrupt." + time.Now().UTC().Format("20060102150405")
	key := nodeID + ":" + dbPath
	f.snapshotCreated[key] = snapPath
	return snapPath, nil
}

func (f *fakeEdgeDBOperator) RecoverFromWAL(ctx context.Context, nodeID, dbPath string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.walRecoverCalled == nil {
		f.walRecoverCalled = make(map[string]bool)
	}
	key := nodeID + ":" + dbPath
	f.walRecoverCalled[key] = true
	if f.failPostRecovery {
		// leave it corrupt
		return nil
	}
	// mark recovered
	if status, exists := f.integrityStatus[key]; exists {
		status.IsCorrupt = false
		status.IntegrityCheck = "ok"
	}
	return nil
}

type fakeKMSAuditor struct {
	mu         sync.Mutex
	protection map[string]bool
	details    map[string]string
	auditErr   error
}

func (f *fakeKMSAuditor) AuditKeyPolicy(ctx context.Context, keyARN string) (bool, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.auditErr != nil {
		return false, "", f.auditErr
	}
	protected, exists := f.protection[keyARN]
	if !exists {
		return false, "policy not found", nil
	}
	return protected, f.details[keyARN], nil
}

func TestBreakStaleLock_SuccessWhenPodDead(t *testing.T) {
	ctx := context.Background()
	location := "kopia-primary-s3"

	inspector := &fakeRepoLockInspector{
		locks: []drhealing.BackupStorageLockStatus{
			{
				LocationName:  location,
				Provider:      "aws",
				Bucket:        "velero-backups-bucket",
				IsLocked:      true,
				LockCreatedBy: "velero-backup-controller-crash",
				LockAge:       75 * time.Minute,
				LastCheckedAt: time.Now().UTC(),
			},
		},
	}
	podChecker := &fakePodLivenessChecker{
		podRunning: map[string]bool{location: false},
		podNames:   map[string]string{location: ""},
	}
	trigger := &fakeBackupTrigger{}
	edgeOp := &fakeEdgeDBOperator{}
	kmsAud := &fakeKMSAuditor{}

	manager := NewHealingManager(inspector, podChecker, trigger, edgeOp, kmsAud)

	// Inspect locks first to verify IsStale evaluation
	locks, err := manager.InspectBackupLocks(ctx)
	if err != nil {
		t.Fatalf("unexpected error inspecting locks: %v", err)
	}
	if len(locks) != 1 || !locks[0].IsStale {
		t.Fatalf("expected lock to be marked stale, got %+v", locks)
	}

	// Break stale lock
	err = manager.BreakStaleLock(ctx, location)
	if err != nil {
		t.Fatalf("expected nil error on breaking stale lock, got: %v", err)
	}

	if !inspector.breakCalled[location] {
		t.Errorf("expected ExecuteBreakLock to be called for %s", location)
	}
	if !trigger.triggered[location] {
		t.Errorf("expected TriggerScheduledBackup to be called for %s", location)
	}

	active, cleared, failed := manager.Telemetry().Snapshot()
	if cleared != 1 {
		t.Errorf("expected cleared=1, got %d", cleared)
	}
	if failed != 0 {
		t.Errorf("expected failed=0, got %d", failed)
	}
	_ = active
}

func TestBreakStaleLock_RejectsWhenPodRunning(t *testing.T) {
	ctx := context.Background()
	location := "kopia-primary-s3"

	inspector := &fakeRepoLockInspector{
		locks: []drhealing.BackupStorageLockStatus{
			{
				LocationName:  location,
				Provider:      "aws",
				Bucket:        "velero-backups-bucket",
				IsLocked:      true,
				LockCreatedBy: "velero-backup-controller-healthy",
				LockAge:       90 * time.Minute,
				LastCheckedAt: time.Now().UTC(),
			},
		},
	}
	podChecker := &fakePodLivenessChecker{
		podRunning: map[string]bool{location: true},
		podNames:   map[string]string{location: "velero-backup-controller-healthy"},
	}
	trigger := &fakeBackupTrigger{}
	edgeOp := &fakeEdgeDBOperator{}
	kmsAud := &fakeKMSAuditor{}

	manager := NewHealingManager(inspector, podChecker, trigger, edgeOp, kmsAud)

	// Breaking stale lock must fail with ErrBackupPodStillRunning
	err := manager.BreakStaleLock(ctx, location)
	if err == nil {
		t.Fatalf("expected error when pod is running, got nil")
	}
	if !errors.Is(err, drhealing.ErrBackupPodStillRunning) {
		t.Fatalf("expected ErrBackupPodStillRunning, got: %v", err)
	}

	if inspector.breakCalled[location] {
		t.Errorf("break lock must NOT be called when pod is running")
	}
	if trigger.triggered[location] {
		t.Errorf("backup trigger must NOT be called when pod is running")
	}

	_, cleared, failed := manager.Telemetry().Snapshot()
	if cleared != 0 {
		t.Errorf("expected cleared=0, got %d", cleared)
	}
	if failed != 1 {
		t.Errorf("expected failed=1, got %d", failed)
	}
}

func TestRepairEdgeDatabase_SuccessWithCorruptSnapshot(t *testing.T) {
	ctx := context.Background()
	nodeID := "edge-node-k3s-01"
	dbPath := "/var/lib/rancher/k3s/server/db/state.db"
	key := nodeID + ":" + dbPath

	edgeOp := &fakeEdgeDBOperator{
		integrityStatus: map[string]*drhealing.EdgeDatabaseIntegrity{
			key: {
				NodeID:         nodeID,
				DBPath:         dbPath,
				JournalMode:    "wal",
				Synchronous:    "FULL",
				IsCorrupt:      true,
				IntegrityCheck: "SQLITE_CORRUPT: database disk image is malformed",
				LastVerifiedAt: time.Now().UTC(),
			},
		},
	}

	manager := NewHealingManager(nil, nil, nil, edgeOp, nil)

	err := manager.RepairEdgeDatabase(ctx, nodeID, dbPath)
	if err != nil {
		t.Fatalf("expected repair to succeed, got: %v", err)
	}

	if edgeOp.snapshotCreated[key] == "" {
		t.Errorf("expected .corrupt snapshot to be created prior to WAL recovery")
	}
	if !edgeOp.walRecoverCalled[key] {
		t.Errorf("expected RecoverFromWAL to be called")
	}

	checks, corrupt, success, failed := manager.EdgeMetrics().Snapshot()
	if checks != 1 || corrupt != 1 || success != 1 || failed != 0 {
		t.Errorf("unexpected edge metrics: checks=%d, corrupt=%d, success=%d, failed=%d",
			checks, corrupt, success, failed)
	}
}

func TestRepairEdgeDatabase_FailurePostRecovery(t *testing.T) {
	ctx := context.Background()
	nodeID := "edge-node-k3s-02"
	dbPath := "/var/lib/k3s/corrupt.db"
	key := nodeID + ":" + dbPath

	edgeOp := &fakeEdgeDBOperator{
		failPostRecovery: true,
		integrityStatus: map[string]*drhealing.EdgeDatabaseIntegrity{
			key: {
				NodeID:         nodeID,
				DBPath:         dbPath,
				JournalMode:    "wal",
				Synchronous:    "FULL",
				IsCorrupt:      true,
				IntegrityCheck: "SQLITE_CORRUPT: database disk image is malformed",
				LastVerifiedAt: time.Now().UTC(),
			},
		},
	}

	manager := NewHealingManager(nil, nil, nil, edgeOp, nil)

	err := manager.RepairEdgeDatabase(ctx, nodeID, dbPath)
	if err == nil {
		t.Fatalf("expected error when DB remains corrupt post-recovery")
	}
	if !errors.Is(err, drhealing.ErrDatabaseCorrupt) {
		t.Fatalf("expected ErrDatabaseCorrupt, got: %v", err)
	}

	_, _, success, failed := manager.EdgeMetrics().Snapshot()
	if success != 0 || failed != 1 {
		t.Errorf("expected success=0, failed=1, got success=%d, failed=%d", success, failed)
	}
}

func TestAuditKMSKeyProtection(t *testing.T) {
	ctx := context.Background()
	protectedKey := "arn:aws:kms:us-east-1:123456789012:key/protected-mrk-key"
	unprotectedKey := "arn:aws:kms:us-east-1:123456789012:key/unprotected-orphan-key"

	kmsAud := &fakeKMSAuditor{
		protection: map[string]bool{
			protectedKey:   true,
			unprotectedKey: false,
		},
		details: map[string]string{
			protectedKey:   "Multi-party quorum enabled; 30-day grace period",
			unprotectedKey: "No multi-party quorum; zero grace delay",
		},
	}

	manager := NewHealingManager(nil, nil, nil, nil, kmsAud)

	// Protected key
	isProtected, err := manager.AuditKMSKeyProtection(ctx, protectedKey)
	if err != nil || !isProtected {
		t.Fatalf("expected protected=true without error, got isProtected=%v, err=%v", isProtected, err)
	}

	// Unprotected key
	isProtected, err = manager.AuditKMSKeyProtection(ctx, unprotectedKey)
	if isProtected {
		t.Fatalf("expected unprotected key to return protected=false")
	}
	if !errors.Is(err, drhealing.ErrKMSDeletionUnprotected) {
		t.Fatalf("expected ErrKMSDeletionUnprotected, got: %v", err)
	}
}

func TestConcurrentManagerLoad(t *testing.T) {
	ctx := context.Background()
	location := "kopia-loc-concurrent"

	inspector := &fakeRepoLockInspector{
		locks: []drhealing.BackupStorageLockStatus{
			{
				LocationName:  location,
				IsLocked:      true,
				LockAge:       80 * time.Minute,
				LastCheckedAt: time.Now().UTC(),
			},
		},
	}
	podChecker := &fakePodLivenessChecker{
		podRunning: map[string]bool{location: false},
	}
	trigger := &fakeBackupTrigger{}
	edgeOp := &fakeEdgeDBOperator{}
	kmsAud := &fakeKMSAuditor{
		protection: map[string]bool{"arn:test": true},
	}

	manager := NewHealingManager(inspector, podChecker, trigger, edgeOp, kmsAud)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, _ = manager.InspectBackupLocks(ctx)
		}()
		go func() {
			defer wg.Done()
			_ = manager.RepairEdgeDatabase(ctx, "node-1", "/db.sqlite")
		}()
		go func() {
			defer wg.Done()
			_, _ = manager.AuditKMSKeyProtection(ctx, "arn:test")
		}()
	}

	wg.Wait()
}
