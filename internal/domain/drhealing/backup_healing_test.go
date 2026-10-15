package drhealing

import (
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestTelemetryStructures(t *testing.T) {
	telemetry := NewLockStateTelemetry()
	telemetry.IncActiveLocks(2)
	telemetry.IncStaleLocksCleared()
	telemetry.IncLockBreaksFailed()
	act, clr, fld := telemetry.Snapshot()
	if act != 2 || clr != 1 || fld != 1 {
		t.Errorf("unexpected telemetry snapshot: %d, %d, %d", act, clr, fld)
	}

	progress := NewBackupProgressCounter()
	progress.AddBytesProcessed(1024)
	progress.IncObjectsScanned()
	progress.IncSnapshotsTracked()
	b, o, s := progress.Snapshot()
	if b != 1024 || o != 1 || s != 1 {
		t.Errorf("unexpected progress snapshot: %d, %d, %d", b, o, s)
	}

	edge := NewEdgeDatabaseIntegrityMetrics()
	edge.IncChecksPerformed()
	edge.IncCorruptionsFound()
	edge.IncRepairsSuccessful()
	edge.IncRepairsFailed()
	chk, crp, rep, fld2 := edge.Snapshot()
	if chk != 1 || crp != 1 || rep != 1 || fld2 != 1 {
		t.Errorf("unexpected edge metrics snapshot: %d, %d, %d, %d", chk, crp, rep, fld2)
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
