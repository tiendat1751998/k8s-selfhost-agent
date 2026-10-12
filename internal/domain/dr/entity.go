package dr

import (
	"context"
	"errors"
	"time"
)

// Standard DR error definitions according to RFC 7807 specifications.
var (
	ErrEtcdSnapshotFailed            = errors.New("ERR_ETCD_SNAPSHOT_FAILED: etcdctl snapshot save failed: connection refused or invalid certificates")
	ErrRestoreDrillIntegrityMismatch = errors.New("ERR_RESTORE_DRILL_INTEGRITY_MISMATCH: restore drill failed: snapshot checksum mismatch or missing tables during test restore")
	ErrSnapshotNotFound              = errors.New("snapshot not found")
	ErrClusterMismatch               = errors.New("snapshot belongs to a different cluster")
)

// EtcdSnapshotResult contains status and verified metadata of an etcd snapshot.
type EtcdSnapshotResult struct {
	SnapshotID  string    `json:"snapshot_id"`
	ClusterID   string    `json:"cluster_id"`
	FilePath    string    `json:"file_path,omitempty"`
	StoragePath string    `json:"storage_path,omitempty"`
	SizeBytes   int64     `json:"size_bytes"`
	Size        int64     `json:"size,omitempty"`
	SHA256Hash  string    `json:"sha256_hash,omitempty"`
	TotalKeys   int64     `json:"total_keys,omitempty"`
	RaftTerm    uint64    `json:"raft_term,omitempty"`
	RaftIndex   uint64    `json:"raft_index,omitempty"`
	Status      string    `json:"status"` // "Completed", "Failed"
	CreatedAt   time.Time `json:"created_at"`
}

// ClusterBackupResult contains status and metadata of a cluster backup (e.g. Velero).
type ClusterBackupResult struct {
	BackupName string    `json:"backup_name"`
	ClusterID  string    `json:"cluster_id"`
	Phase      string    `json:"phase"` // "Completed", "InProgress", "Failed", "New"
	TotalItems int       `json:"total_items"`
	CreatedAt  time.Time `json:"created_at"`
}

// RestoreDrillReport contains metrics and execution logs for an automated restore drill.
type RestoreDrillReport struct {
	DrillID         string        `json:"drill_id"`
	SnapshotID      string        `json:"snapshot_id"`
	TargetClusterID string        `json:"target_cluster_id"`
	Status          string        `json:"status"` // "PASSED", "FAILED", "TIMED_OUT"
	RealRTODuration time.Duration `json:"real_rto_duration"`
	RealRPOLag      time.Duration `json:"real_rpo_lag"`
	TablesRestored  int           `json:"tables_restored"`
	RowsRestored    int64         `json:"rows_restored"`
	ExecutedAt      time.Time     `json:"executed_at"`
	LogOutput       string        `json:"log_output"`
}

// EtcdClientDriver abstracts interactions with etcd for snapshotting and verification.
type EtcdClientDriver interface {
	SaveSnapshot(ctx context.Context, endpoint, caPath, certPath, keyPath string) (*EtcdSnapshotResult, error)
	VerifySnapshot(ctx context.Context, snapshotPath string) (bool, error)
}
