package drhealing

import (
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"
	"unsafe"
)

func TestCacheLinePadding(t *testing.T) {
	// Verify that struct offsets or field separations guarantee 64-byte spacing
	// preventing false sharing across MESI/MOESI cache coherence lines.
	var telemetry LockStateTelemetry
	offsetActive := unsafe.Offsetof(telemetry.ActiveLocks)
	offsetCleared := unsafe.Offsetof(telemetry.StaleLocksCleared)
	offsetFailed := unsafe.Offsetof(telemetry.LockBreaksFailed)

	if offsetCleared-offsetActive < 64 {
		t.Errorf("expected spacing between ActiveLocks and StaleLocksCleared >= 64, got %d", offsetCleared-offsetActive)
	}
	if offsetFailed-offsetCleared < 64 {
		t.Errorf("expected spacing between StaleLocksCleared and LockBreaksFailed >= 64, got %d", offsetFailed-offsetCleared)
	}

	var progress BackupProgressCounter
	offsetBytes := unsafe.Offsetof(progress.BytesProcessed)
	offsetObjs := unsafe.Offsetof(progress.ObjectsScanned)
	offsetSnaps := unsafe.Offsetof(progress.SnapshotsTracked)

	if offsetObjs-offsetBytes < 64 {
		t.Errorf("expected spacing between BytesProcessed and ObjectsScanned >= 64, got %d", offsetObjs-offsetBytes)
	}
	if offsetSnaps-offsetObjs < 64 {
		t.Errorf("expected spacing between ObjectsScanned and SnapshotsTracked >= 64, got %d", offsetSnaps-offsetObjs)
	}

	var edge EdgeDatabaseIntegrityMetrics
	offsetChecks := unsafe.Offsetof(edge.ChecksPerformed)
	offsetCorrupt := unsafe.Offsetof(edge.CorruptionsFound)
	offsetSuccess := unsafe.Offsetof(edge.RepairsSuccessful)
	offsetRepairFailed := unsafe.Offsetof(edge.RepairsFailed)

	if offsetCorrupt-offsetChecks < 64 {
		t.Errorf("expected spacing between ChecksPerformed and CorruptionsFound >= 64, got %d", offsetCorrupt-offsetChecks)
	}
	if offsetSuccess-offsetCorrupt < 64 {
		t.Errorf("expected spacing between CorruptionsFound and RepairsSuccessful >= 64, got %d", offsetSuccess-offsetCorrupt)
	}
	if offsetRepairFailed-offsetSuccess < 64 {
		t.Errorf("expected spacing between RepairsSuccessful and RepairsFailed >= 64, got %d", offsetRepairFailed-offsetSuccess)
	}
}

func TestConcurrentTelemetryCounters(t *testing.T) {
	telemetry := NewLockStateTelemetry()
	const numGoroutines = 100
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				telemetry.IncActiveLocks(1)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				telemetry.IncStaleLocksCleared()
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				telemetry.IncLockBreaksFailed()
			}
		}()
	}

	wg.Wait()

	active, cleared, failed := telemetry.Snapshot()
	expected := uint64(numGoroutines * iterations)
	if active != expected {
		t.Errorf("expected active=%d, got %d", expected, active)
	}
	if cleared != expected {
		t.Errorf("expected cleared=%d, got %d", expected, cleared)
	}
	if failed != expected {
		t.Errorf("expected failed=%d, got %d", expected, failed)
	}
}

func TestRFC7807ErrorDefinitions(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "ErrStaleBackupLockCleared",
			err:            ErrStaleBackupLockCleared,
			expectedStatus: http.StatusOK,
			expectedCode:   "ERR_STALE_BACKUP_LOCK_CLEARED",
		},
		{
			name:           "ErrBackupPodStillRunning",
			err:            ErrBackupPodStillRunning,
			expectedStatus: http.StatusConflict,
			expectedCode:   "ERR_BACKUP_POD_STILL_RUNNING",
		},
		{
			name:           "ErrDatabaseCorrupt",
			err:            ErrDatabaseCorrupt,
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "ERR_DATABASE_CORRUPT",
		},
		{
			name:           "ErrKMSDeletionUnprotected",
			err:            ErrKMSDeletionUnprotected,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "ERR_KMS_DELETION_UNPROTECTED",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var prob *RFC7807Error
			if !errors.As(tc.err, &prob) {
				t.Fatalf("expected error %v to be RFC7807Error", tc.err)
			}
			if prob.Status != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, prob.Status)
			}
			if prob.Code != tc.expectedCode {
				t.Errorf("expected code %s, got %s", tc.expectedCode, prob.Code)
			}
			if prob.Error() == "" {
				t.Errorf("expected non-empty error string")
			}
		})
	}
}

func TestBackupStorageLockStatusEvaluation(t *testing.T) {
	status := BackupStorageLockStatus{
		LocationName:  "default-kopia-s3",
		Provider:      "aws",
		Bucket:        "corp-velero-backups",
		IsLocked:      true,
		LockCreatedBy: "velero-backup-controller-6d7c88b9-x21v",
		LockAge:       75 * time.Minute,
		LastCheckedAt: time.Now().UTC(),
	}

	if !status.IsLockStale(60 * time.Minute) {
		t.Errorf("expected lock with age 75m to be marked stale (>60m)")
	}

	freshStatus := BackupStorageLockStatus{
		LocationName:  "default-kopia-s3",
		Provider:      "aws",
		Bucket:        "corp-velero-backups",
		IsLocked:      true,
		LockAge:       15 * time.Minute,
		LastCheckedAt: time.Now().UTC(),
	}

	if freshStatus.IsLockStale(60 * time.Minute) {
		t.Errorf("expected lock with age 15m to NOT be marked stale")
	}
}
