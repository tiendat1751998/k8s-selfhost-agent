package backup

import (
	"time"

	"github.com/google/uuid"
)

// Lifecycle status constants for BackupJob and RestoreJob.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusVerified  = "verified"

	VerificationStatusUnverified = "unverified"
	VerificationStatusVerified   = "verified"
	VerificationStatusMismatch   = "checksum_mismatch"
	VerificationStatusFailed     = "verification_failed"
)

type BackupStorage struct {
	ID          string            `json:"id"`
	TenantID    string            `json:"tenant_id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Endpoint    string            `json:"endpoint"`
	Bucket      string            `json:"bucket"`
	Credentials map[string]string `json:"credentials,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type BackupPolicy struct {
	ID                 string    `json:"id"`
	TenantID           string    `json:"tenant_id"`
	Name               string    `json:"name"`
	DBType             string    `json:"db_type"`
	DBHost             string    `json:"db_host"`
	DBPort             int       `json:"db_port"`
	DBName             string    `json:"db_name"`
	StorageID          string    `json:"storage_id"`
	SecondaryStorageID string    `json:"secondary_storage_id,omitempty"`
	Schedule           string    `json:"schedule"`
	RetentionCount     int       `json:"retention_count"`
	BackupType         string    `json:"backup_type"`
	CompressionLevel   int       `json:"compression_level"`
	EncryptionEnabled  bool      `json:"encryption_enabled"`
	EncryptionKeyID    string    `json:"encryption_key_id,omitempty"`
	Enabled            bool      `json:"enabled"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// Enable activates the backup policy.
func (p *BackupPolicy) Enable() {
	p.Enabled = true
	p.UpdatedAt = time.Now().UTC()
}

// Disable deactivates the backup policy.
func (p *BackupPolicy) Disable() {
	p.Enabled = false
	p.UpdatedAt = time.Now().UTC()
}

type BackupJob struct {
	ID                  string     `json:"id"`
	TenantID            string     `json:"tenant_id"`
	PolicyID            string     `json:"policy_id"`
	Status              string     `json:"status"`
	BackupType          string     `json:"backup_type"`
	StoragePath         string     `json:"storage_path"`
	LocalStoragePath    string     `json:"local_storage_path"`
	CloudStoragePath    string     `json:"cloud_storage_path"`
	SizeBytes           int64      `json:"size_bytes"`
	CompressedSizeBytes int64      `json:"compressed_size_bytes"`
	DurationMs          int64      `json:"duration_ms"`
	ChecksumSHA256      string     `json:"checksum_sha256"`
	WALStartLSN         string     `json:"wal_start_lsn"`
	WALEndLSN           string     `json:"wal_end_lsn"`
	VerifiedAt          *time.Time `json:"verified_at,omitempty"`
	VerificationStatus  string     `json:"verification_status"`
	ErrorMessage        string     `json:"error_message,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	StartedAt           *time.Time `json:"started_at,omitempty"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
}

// NewBackupJob creates a new BackupJob in pending status.
func NewBackupJob(policyID, storageID, dbName string) *BackupJob {
	now := time.Now().UTC()
	return &BackupJob{
		ID:                 uuid.New().String(),
		PolicyID:           policyID,
		Status:             StatusPending,
		VerificationStatus: VerificationStatusUnverified,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// Start transitions the backup job to running status.
func (j *BackupJob) Start() {
	now := time.Now().UTC()
	j.Status = StatusRunning
	j.StartedAt = &now
	j.UpdatedAt = now
}

// Complete transitions the backup job to completed status with size and duration metrics.
func (j *BackupJob) Complete(size int64, duration time.Duration) {
	now := time.Now().UTC()
	j.Status = StatusCompleted
	j.SizeBytes = size
	j.DurationMs = duration.Milliseconds()
	j.CompletedAt = &now
	j.UpdatedAt = now
}

// Fail transitions the backup job to failed status with the given error message.
func (j *BackupJob) Fail(err string) {
	now := time.Now().UTC()
	j.Status = StatusFailed
	j.ErrorMessage = err
	j.UpdatedAt = now
}

// MarkVerified updates the verification status and verified timestamp of the backup job.
func (j *BackupJob) MarkVerified(result VerificationResult) {
	checkedAt := result.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	j.VerifiedAt = &checkedAt
	if result.Passed || result.ChecksumOK {
		j.VerificationStatus = VerificationStatusVerified
	} else {
		j.VerificationStatus = VerificationStatusMismatch
	}
	j.UpdatedAt = time.Now().UTC()
}

type RestoreJob struct {
	ID                string     `json:"id"`
	TenantID          string     `json:"tenant_id"`
	BackupJobID       string     `json:"backup_job_id"`
	TargetDBHost      string     `json:"target_db_host"`
	TargetDBName      string     `json:"target_db_name"`
	PITRTimestamp     *time.Time `json:"pitr_timestamp,omitempty"`
	DryRun            bool       `json:"dry_run"`
	SourceStorageType string     `json:"source_storage_type"`
	VerificationLog   string     `json:"verification_log"`
	Status            string     `json:"status"`
	ErrorMessage      string     `json:"error_message,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
}

// NewRestoreJob creates a new RestoreJob in pending status.
func NewRestoreJob(backupJobID, targetDBHost, targetDBName string) *RestoreJob {
	now := time.Now().UTC()
	return &RestoreJob{
		ID:           uuid.New().String(),
		BackupJobID:  backupJobID,
		TargetDBHost: targetDBHost,
		TargetDBName: targetDBName,
		Status:       StatusPending,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// Start transitions the restore job to running status.
func (r *RestoreJob) Start() {
	now := time.Now().UTC()
	r.Status = StatusRunning
	r.StartedAt = &now
	r.UpdatedAt = now
}

// Complete transitions the restore job to completed status with verification details.
func (r *RestoreJob) Complete(verificationLog string) {
	now := time.Now().UTC()
	r.Status = StatusCompleted
	r.VerificationLog = verificationLog
	r.CompletedAt = &now
	r.UpdatedAt = now
}

// Fail transitions the restore job to failed status with the given error message.
func (r *RestoreJob) Fail(err string) {
	r.Status = StatusFailed
	r.ErrorMessage = err
	r.UpdatedAt = time.Now().UTC()
}

