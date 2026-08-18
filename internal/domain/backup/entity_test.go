package backup_test

import (
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
)

func TestNewBackupJob(t *testing.T) {
	job := backup.NewBackupJob("policy-123", "storage-456", "db_prod")

	if job.ID == "" {
		t.Error("expected non-empty ID")
	}
	if job.PolicyID != "policy-123" {
		t.Errorf("expected policy_id 'policy-123', got '%s'", job.PolicyID)
	}
	if job.Status != backup.StatusPending {
		t.Errorf("expected status '%s', got '%s'", backup.StatusPending, job.Status)
	}
	if job.VerificationStatus != backup.VerificationStatusUnverified {
		t.Errorf("expected verification status '%s', got '%s'", backup.VerificationStatusUnverified, job.VerificationStatus)
	}
	if job.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if job.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestBackupJob_Lifecycle(t *testing.T) {
	job := backup.NewBackupJob("policy-123", "storage-456", "db_prod")

	// 1. Start
	job.Start()
	if job.Status != backup.StatusRunning {
		t.Errorf("expected status '%s', got '%s'", backup.StatusRunning, job.Status)
	}
	if job.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}

	// 2. Complete
	job.Complete(1048576, 2500*time.Millisecond)
	if job.Status != backup.StatusCompleted {
		t.Errorf("expected status '%s', got '%s'", backup.StatusCompleted, job.Status)
	}
	if job.SizeBytes != 1048576 {
		t.Errorf("expected SizeBytes 1048576, got %d", job.SizeBytes)
	}
	if job.DurationMs != 2500 {
		t.Errorf("expected DurationMs 2500, got %d", job.DurationMs)
	}
	if job.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}

	// 3. MarkVerified (Success)
	job.MarkVerified(backup.VerificationResult{
		Passed:     true,
		ChecksumOK: true,
	})
	if job.VerificationStatus != backup.VerificationStatusVerified {
		t.Errorf("expected verification status '%s', got '%s'", backup.VerificationStatusVerified, job.VerificationStatus)
	}
	if job.VerifiedAt == nil {
		t.Error("expected VerifiedAt to be set")
	}

	// 4. MarkVerified (Failure / Mismatch)
	job.MarkVerified(backup.VerificationResult{
		Passed:     false,
		ChecksumOK: false,
	})
	if job.VerificationStatus != backup.VerificationStatusMismatch {
		t.Errorf("expected verification status '%s', got '%s'", backup.VerificationStatusMismatch, job.VerificationStatus)
	}

	// 5. Fail
	job.Fail("disk write failure")
	if job.Status != backup.StatusFailed {
		t.Errorf("expected status '%s', got '%s'", backup.StatusFailed, job.Status)
	}
	if job.ErrorMessage != "disk write failure" {
		t.Errorf("expected ErrorMessage 'disk write failure', got '%s'", job.ErrorMessage)
	}
}

func TestNewRestoreJob(t *testing.T) {
	restore := backup.NewRestoreJob("job-123", "127.0.0.1", "db_prod_restore")

	if restore.ID == "" {
		t.Error("expected non-empty ID")
	}
	if restore.BackupJobID != "job-123" {
		t.Errorf("expected BackupJobID 'job-123', got '%s'", restore.BackupJobID)
	}
	if restore.TargetDBHost != "127.0.0.1" {
		t.Errorf("expected TargetDBHost '127.0.0.1', got '%s'", restore.TargetDBHost)
	}
	if restore.TargetDBName != "db_prod_restore" {
		t.Errorf("expected TargetDBName 'db_prod_restore', got '%s'", restore.TargetDBName)
	}
	if restore.Status != backup.StatusPending {
		t.Errorf("expected status '%s', got '%s'", backup.StatusPending, restore.Status)
	}
	if restore.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestRestoreJob_Lifecycle(t *testing.T) {
	restore := backup.NewRestoreJob("job-123", "127.0.0.1", "db_prod_restore")

	// 1. Start
	restore.Start()
	if restore.Status != backup.StatusRunning {
		t.Errorf("expected status '%s', got '%s'", backup.StatusRunning, restore.Status)
	}
	if restore.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}

	// 2. Complete
	restore.Complete("Restore completed successfully in 12s.")
	if restore.Status != backup.StatusCompleted {
		t.Errorf("expected status '%s', got '%s'", backup.StatusCompleted, restore.Status)
	}
	if restore.VerificationLog != "Restore completed successfully in 12s." {
		t.Errorf("expected VerificationLog 'Restore completed successfully in 12s.', got '%s'", restore.VerificationLog)
	}
	if restore.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}

	// 3. Fail
	restore.Fail("connection reset by peer")
	if restore.Status != backup.StatusFailed {
		t.Errorf("expected status '%s', got '%s'", backup.StatusFailed, restore.Status)
	}
	if restore.ErrorMessage != "connection reset by peer" {
		t.Errorf("expected ErrorMessage 'connection reset by peer', got '%s'", restore.ErrorMessage)
	}
}

func TestBackupPolicy_EnableDisable(t *testing.T) {
	policy := &backup.BackupPolicy{
		ID:      "policy-1",
		Enabled: false,
	}

	policy.Enable()
	if !policy.Enabled {
		t.Error("expected policy to be enabled")
	}

	policy.Disable()
	if policy.Enabled {
		t.Error("expected policy to be disabled")
	}
}
