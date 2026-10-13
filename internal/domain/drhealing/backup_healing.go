// Package drhealing defines domain contracts, data structures, and resilience metrics
// for Velero / Kopia snapshot repository lock healing, air-gapped edge SQLite DR, and KMS guards.
//
// ==============================================================================================
// ENGINEERING NOTE: PHYSICAL HARDWARE BOTTLENECKS & REAL-WORLD DISASTER RECOVERY FAILURE MODES
// ==============================================================================================
//
// 1. S3/MinIO Object Storage Latency & Metadata Consistency Bottlenecks:
//    Cloud object stores (AWS S3, GCP Cloud Storage, on-prem MinIO) guarantee read-after-write
//    consistency for PUTs of new keys, but LIST-after-DELETE operations and lock lease renewal
//    mechanisms suffer from network RTT spikes (50ms - 350ms) and metadata tier eventual consistency.
//    Velero and Kopia rely on lock files (e.g., `kopia.maintenance`, `velero-repo-lock`) with atomic
//    time leases. When network partitions or high p99 S3 PUT latencies exceed the lease refresh
//    heartbeat window, nodes falsely assume ownership is contested or leave stale locks behind.
//
// 2. Edge NVMe/SATA SSDs without DRAM Cache & WAL fsync Barriers:
//    Consumer-grade and industrial edge appliances frequently utilize DRAM-less NAND flash controllers.
//    In SQLite WAL (Write-Ahead Logging) mode with `PRAGMA synchronous = FULL` or `NORMAL`, the engine
//    issues `fdatasync`/`fsync` barriers across transaction commit checkpoints. On DRAM-less SSDs,
//    every write barrier forces a high-latency NAND block erase/program cycle (up to 30ms-50ms),
//    stalling synchronous execution loops and blocking Kubernetes edge runtime reconcile loops.
//
// 3. Sudden Power Outage & SQLite Dirty Page Corruption:
//    When sudden power loss strikes an edge node while dirty pages reside in volatile drive buffers,
//    partial page writes occur. Upon reboot, the database header or B-Tree pointers may yield
//    `SQLITE_CORRUPT: database disk image is malformed`. Naive restarts repeatedly crash the control
//    loop. The resilient recovery protocol MUST immediately preserve the damaged file as a `.corrupt`
//    forensic snapshot before executing in-place WAL checkpointing and salvage recovery.
//
// 4. AWS / GCP KMS Key Policy Deletion Propagation Delays:
//    Cloud KMS operations (such as `kms:ScheduleKeyDeletion`) do not purge immediately; AWS enforces
//    a 7-to-30 day pending deletion grace window, whereas misconfigured IAM policies or multi-tenant
//    automation can orphan encryption keys instantly if multi-party quorum or deletion delays are disabled.
//    Propagating KMS state changes across multi-region edge caches can take up to several minutes.
//    Any backup system operating without proactive key deletion protection auditing risks writing
//    backups with keys doomed to expire, rendering multi-terabyte encrypted snapshots permanently unreadable.
//
// 5. Cache-Line False Sharing & MESI/MOESI CPU Degradation:
//    High-frequency metrics and counters updated concurrently across multi-core processors suffer from
//    "false sharing" when independent 8-byte variables share the same 64-byte L1/L2 cache line.
//    Cache coherence protocols (MESI/MOESI) continuously invalidate lines across cores, causing bus
//    storms and 10x throughput degradation. Padded structures with explicit 56-byte cache line buffers
//    guarantee dedicated 64-byte cache lines for each atomic counter.
// ==============================================================================================
package drhealing

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// RFC7807Error represents an RFC 7807 Problem Details compliant domain error.
type RFC7807Error struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance,omitempty"`
	Code     string `json:"code"`
}

func (e *RFC7807Error) Error() string {
	return fmt.Sprintf("[%s] %s: %s (status %d)", e.Code, e.Title, e.Detail, e.Status)
}

// Standard domain resilience errors conforming to RFC 7807 and status codes.
var (
	ErrStaleBackupLockCleared = &RFC7807Error{
		Type:   "https://k8sselfhost.io/dr/errors/ERR_STALE_BACKUP_LOCK_CLEARED",
		Title:  "Stale Backup Lock Cleared",
		Status: http.StatusOK,
		Detail: "stale backup repository lock cleared successfully",
		Code:   "ERR_STALE_BACKUP_LOCK_CLEARED",
	}

	ErrBackupPodStillRunning = &RFC7807Error{
		Type:   "https://k8sselfhost.io/dr/errors/ERR_BACKUP_POD_STILL_RUNNING",
		Title:  "Backup Pod Still Running",
		Status: http.StatusConflict,
		Detail: "backup controller pod is still actively running; break-lock rejected",
		Code:   "ERR_BACKUP_POD_STILL_RUNNING",
	}

	ErrDatabaseCorrupt = &RFC7807Error{
		Type:   "https://k8sselfhost.io/dr/errors/ERR_DATABASE_CORRUPT",
		Title:  "Database Corrupt",
		Status: http.StatusInternalServerError,
		Detail: "edge database disk image is malformed; recovery failed or unrecoverable",
		Code:   "ERR_DATABASE_CORRUPT",
	}

	ErrKMSDeletionUnprotected = &RFC7807Error{
		Type:   "https://k8sselfhost.io/dr/errors/ERR_KMS_DELETION_UNPROTECTED",
		Title:  "KMS Deletion Unprotected",
		Status: http.StatusBadRequest,
		Detail: "kms key deletion policy lacks multi-party quorum or minimum grace delay",
		Code:   "ERR_KMS_DELETION_UNPROTECTED",
	}
)

// BackupStorageLockStatus represents the lock state of a snapshot repository.
type BackupStorageLockStatus struct {
	LocationName  string        `json:"location_name"`
	Provider      string        `json:"provider"`
	Bucket        string        `json:"bucket"`
	IsLocked      bool          `json:"is_locked"`
	LockCreatedBy string        `json:"lock_created_by"`
	LockAge       time.Duration `json:"lock_age"`
	IsStale       bool          `json:"is_stale"` // Age > 60m with no active process
	LastCheckedAt time.Time     `json:"last_checked_at"`
}

// IsLockStale checks whether the lock has exceeded the given threshold.
func (s BackupStorageLockStatus) IsLockStale(threshold time.Duration) bool {
	return s.IsLocked && s.LockAge > threshold
}

// EdgeDatabaseIntegrity represents SQLite integrity state on an edge node.
type EdgeDatabaseIntegrity struct {
	NodeID         string    `json:"node_id"`
	DBPath         string    `json:"db_path"`
	JournalMode    string    `json:"journal_mode"`
	Synchronous    string    `json:"synchronous"`
	IsCorrupt      bool      `json:"is_corrupt"`
	IntegrityCheck string    `json:"integrity_check"`
	LastVerifiedAt time.Time `json:"last_verified_at"`
}

// DRResilienceManager defines the core disaster recovery resilience manager interface.
type DRResilienceManager interface {
	InspectBackupLocks(ctx context.Context) ([]BackupStorageLockStatus, error)
	BreakStaleLock(ctx context.Context, locationName string) error
	RepairEdgeDatabase(ctx context.Context, nodeID, dbPath string) error
	AuditKMSKeyProtection(ctx context.Context, keyARN string) (bool, error)
}

// Domain Ports (Abstractions for clean architecture):

// RepositoryLockInspector provides lock inspection and low-level break operations.
type RepositoryLockInspector interface {
	InspectLocks(ctx context.Context) ([]BackupStorageLockStatus, error)
	ExecuteBreakLock(ctx context.Context, locationName string) error
}

// PodLivenessChecker checks if the backup controller pod is active.
type PodLivenessChecker interface {
	IsBackupPodRunning(ctx context.Context, locationName string) (isRunning bool, podName string, err error)
}

// BackupTrigger triggers scheduled backups.
type BackupTrigger interface {
	TriggerScheduledBackup(ctx context.Context, locationName string) error
}

// EdgeDatabaseOperator handles edge SQLite diagnosis and WAL repair operations.
type EdgeDatabaseOperator interface {
	CheckIntegrity(ctx context.Context, nodeID, dbPath string) (*EdgeDatabaseIntegrity, error)
	CreateCorruptSnapshot(ctx context.Context, nodeID, dbPath string) (snapshotPath string, err error)
	RecoverFromWAL(ctx context.Context, nodeID, dbPath string) error
}

// KMSKeyPolicyAuditor audits cloud KMS key deletion and protection policies.
type KMSKeyPolicyAuditor interface {
	AuditKeyPolicy(ctx context.Context, keyARN string) (isProtected bool, details string, err error)
}

// ==============================================================================================
// CACHE-LINE PADDED CONCURRENT TELEMETRY & HIGH-FREQUENCY COUNTERS
// ==============================================================================================
// Each 8-byte uint64 counter is buffered with 56 bytes of padding to align to 64-byte boundaries,
// isolating CPU cache lines and preventing false sharing across MESI/MOESI processor cores.

// LockStateTelemetry records high-frequency lock state telemetry with cache line isolation.
type LockStateTelemetry struct {
	ActiveLocks       uint64
	_pad0             [56]byte
	StaleLocksCleared uint64
	_pad1             [56]byte
	LockBreaksFailed  uint64
	_pad2             [56]byte
}

// NewLockStateTelemetry initializes an isolated LockStateTelemetry structure.
func NewLockStateTelemetry() *LockStateTelemetry {
	return &LockStateTelemetry{}
}

func (t *LockStateTelemetry) IncActiveLocks(delta uint64) {
	atomic.AddUint64(&t.ActiveLocks, delta)
}

func (t *LockStateTelemetry) SetActiveLocks(val uint64) {
	atomic.StoreUint64(&t.ActiveLocks, val)
}

func (t *LockStateTelemetry) IncStaleLocksCleared() {
	atomic.AddUint64(&t.StaleLocksCleared, 1)
}

func (t *LockStateTelemetry) IncLockBreaksFailed() {
	atomic.AddUint64(&t.LockBreaksFailed, 1)
}

func (t *LockStateTelemetry) Snapshot() (active, cleared, failed uint64) {
	return atomic.LoadUint64(&t.ActiveLocks),
		atomic.LoadUint64(&t.StaleLocksCleared),
		atomic.LoadUint64(&t.LockBreaksFailed)
}

// BackupProgressCounter tracks backup progress with cache line isolation.
type BackupProgressCounter struct {
	BytesProcessed   uint64
	_pad0            [56]byte
	ObjectsScanned   uint64
	_pad1            [56]byte
	SnapshotsTracked uint64
	_pad2            [56]byte
}

// NewBackupProgressCounter initializes a padded BackupProgressCounter.
func NewBackupProgressCounter() *BackupProgressCounter {
	return &BackupProgressCounter{}
}

func (c *BackupProgressCounter) AddBytesProcessed(delta uint64) {
	atomic.AddUint64(&c.BytesProcessed, delta)
}

func (c *BackupProgressCounter) IncObjectsScanned() {
	atomic.AddUint64(&c.ObjectsScanned, 1)
}

func (c *BackupProgressCounter) IncSnapshotsTracked() {
	atomic.AddUint64(&c.SnapshotsTracked, 1)
}

func (c *BackupProgressCounter) Snapshot() (bytes, objects, snapshots uint64) {
	return atomic.LoadUint64(&c.BytesProcessed),
		atomic.LoadUint64(&c.ObjectsScanned),
		atomic.LoadUint64(&c.SnapshotsTracked)
}

// EdgeDatabaseIntegrityMetrics tracks edge DB repair operations with cache line isolation.
type EdgeDatabaseIntegrityMetrics struct {
	ChecksPerformed   uint64
	_pad0             [56]byte
	CorruptionsFound  uint64
	_pad1             [56]byte
	RepairsSuccessful uint64
	_pad2             [56]byte
	RepairsFailed     uint64
	_pad3             [56]byte
}

// NewEdgeDatabaseIntegrityMetrics initializes a padded EdgeDatabaseIntegrityMetrics.
func NewEdgeDatabaseIntegrityMetrics() *EdgeDatabaseIntegrityMetrics {
	return &EdgeDatabaseIntegrityMetrics{}
}

func (m *EdgeDatabaseIntegrityMetrics) IncChecksPerformed() {
	atomic.AddUint64(&m.ChecksPerformed, 1)
}

func (m *EdgeDatabaseIntegrityMetrics) IncCorruptionsFound() {
	atomic.AddUint64(&m.CorruptionsFound, 1)
}

func (m *EdgeDatabaseIntegrityMetrics) IncRepairsSuccessful() {
	atomic.AddUint64(&m.RepairsSuccessful, 1)
}

func (m *EdgeDatabaseIntegrityMetrics) IncRepairsFailed() {
	atomic.AddUint64(&m.RepairsFailed, 1)
}

func (m *EdgeDatabaseIntegrityMetrics) Snapshot() (checks, corruptions, success, failed uint64) {
	return atomic.LoadUint64(&m.ChecksPerformed),
		atomic.LoadUint64(&m.CorruptionsFound),
		atomic.LoadUint64(&m.RepairsSuccessful),
		atomic.LoadUint64(&m.RepairsFailed)
}
