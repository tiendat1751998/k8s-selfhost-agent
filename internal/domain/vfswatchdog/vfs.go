// Package vfswatchdog defines domain entities, safety contracts, and hardware-level
// guardrails for VFS inode exhaustion monitoring, uninterruptible sleep (D-state) process breaking,
// and container zombie subreaper PID table protection.
//
// =========================================================================================
// CRITICAL ENGINEERING NOTE: PHYSICAL HARDWARE BOTTLENECKS & REAL-WORLD FAILURE MODES
// =========================================================================================
// 1. Physical Hardware Bottlenecks:
//   - VFS Inode Cache Dentry Locks: Under high-density directory structures (millions of
//     tiny temporary files), kernel directory path lookups suffer from severe spinlock
//     contention on dcache locks (`d_lock` / `rename_lock`) and inode mutexes (`i_rwsem`).
//     Concurrent userspace directory traversal (`sys_getdents64`) causes CPU cores to burn
//     cycles spinning in kernel space, degrading overall system IPC.
//   - NFS RPC Network Timeout Latency: Remote storage mounts (NFSv3/NFSv4/EFS) communicating
//     over degraded or congested network fabrics experience dropped RPC requests. When an
//     operation is dispatched without soft timeouts or during network partitions, the SunRPC
//     client state machine hangs waiting for server acknowledgment, stalling kernel I/O workers.
//   - Block Device I/O Queue Hangs: NVMe/SATA controller firmware lockups, SAN fiber channel
//     flapping, or backend EBS degradation cause kernel block multiqueue dispatch (`blk_mq`)
//     to freeze. Kernel threads submitting synchronous I/O enter `TASK_UNINTERRUPTIBLE` (D-state)
//     while holding filesystem locks, rendering standard signals (SIGKILL / kill -9) completely
//     ineffectual.
//   - Kernel PID Table Fragmentation: Linux allocates Process IDs via the kernel PID bitmap
//     up to `/proc/sys/kernel/pid_max` (typically 32,768 to 4,194,304). High fork-rate workloads
//     and leaked processes fragment PID allocations and churn the kernel's `task_struct`
//     slab caches (`task_struct` and `cred` caches), leading to allocation exhaustion.
//
// 2. Real-World Failure Modes:
//   - VFS Inode Exhaustion (0 Inodes Free Despite 90% Free Disk Space):
//     Automated CI pipelines, build artifacts, or broken log rotations frequently write millions
//     of zero-byte temporary files to `/tmp` or `/var/tmp`. While storage capacity is plentiful,
//     filesystem inode tables (e.g. ext4 inode ratio or XFS AG inode allocation) hit 100%
//     saturation. Subsequent writes fail immediately with `ENOSPC` (No space left on device),
//     halting host services, container runtimes, and pod scheduling.
//   - Uninterruptible Sleep (D-State) Mount Deadlocks:
//     Processes executing synchronous disk or NFS writes that hang become stuck in D-state.
//     Because signals are ignored in uninterruptible sleep, pods holding these file descriptors
//     cannot terminate, wedging Kubernetes node draining in perpetual `Terminating` state.
//     The only non-reboot recovery is issuing a forced lazy unmount (`umount -f -l <mount_point>`),
//     which detaches the filesystem hierarchy immediately and severs pending VFS lock waits.
//   - Container PID 1 Zombie Process Fork Bombs:
//     Containers running application processes directly as PID 1 without an init subreaper
//     (`tini`, `dumb-init`, or `shareProcessNamespace: true`) fail to harvest dead child
//     processes via `waitpid()`. Terminated children remain in zombie (`Z`) state, holding
//     their entry in the kernel PID table. As zombies accumulate, host `/proc/sys/kernel/pid_max`
//     is exhausted, preventing any new process creation cluster-wide.
// =========================================================================================
package vfswatchdog

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// RFC 7807 error codes for VFS watchdog and process state anomalies.
const (
	CodeVFSInodeExhaustion = "ERR_VFS_INODE_EXHAUSTION"
	CodeVFSDStateDeadlock  = "ERR_VFS_DSTATE_DEADLOCK"
	CodeNFSUnmountFailed   = "ERR_NFS_UNMOUNT_FAILED"
	CodePIDQuotaExceeded   = "ERR_PID_QUOTA_EXCEEDED"
)

// Standard domain errors.
var (
	ErrVFSInodeExhaustion = &domainerrors.DomainError{
		Code:    CodeVFSInodeExhaustion,
		Message: "VFS inode exhaustion detected: filesystem inodes critically saturated (>85%) or exhausted",
		Err:     errors.New("vfs inode table exhausted"),
	}
	ErrVFSDStateDeadlock = &domainerrors.DomainError{
		Code:    CodeVFSDStateDeadlock,
		Message: "Uninterruptible sleep (D-state) deadlock detected: process wedged on hung storage mount",
		Err:     errors.New("process stuck in d-state exceeding timeout"),
	}
	ErrNFSUnmountFailed = &domainerrors.DomainError{
		Code:    CodeNFSUnmountFailed,
		Message: "Lazy unmount failed for hung storage mount",
		Err:     errors.New("failed to remount or lazy unmount storage target"),
	}
	ErrPIDQuotaExceeded = &domainerrors.DomainError{
		Code:    CodePIDQuotaExceeded,
		Message: "Pod PID allocation exceeded limit: zombie accumulation or fork bomb risk",
		Err:     errors.New("pod pid ceiling exceeded"),
	}
)

// Threshold constants for VFS and process safety guardrails.
const (
	// DefaultInodeWarningThresholdPct is the threshold above which inode alerts and culprit detection trigger.
	DefaultInodeWarningThresholdPct = 85.0

	// DefaultDStateMaxDuration is the maximum allowable uninterruptible sleep duration before intervention (5m).
	DefaultDStateMaxDuration = 5 * time.Minute

	// TempFilePurgeMaxAge is the retention limit for temporary files eligible for automatic pruning.
	TempFilePurgeMaxAge = 24 * time.Hour
)

// InodeReport represents point-in-time inode utilization metrics for a mounted filesystem.
type InodeReport struct {
	MountPoint      string   `json:"mount_point"`
	TotalInodes     uint64   `json:"total_inodes"`
	FreeInodes      uint64   `json:"free_inodes"`
	UsagePercentage float64  `json:"usage_percentage"`
	TopCulpritDirs  []string `json:"top_culprit_dirs"`
}

// IsExhausted returns true if inode utilization exceeds the critical 85% threshold or free inodes is zero.
func (r InodeReport) IsExhausted() bool {
	return r.FreeInodes == 0 || r.UsagePercentage >= DefaultInodeWarningThresholdPct
}

// DStateProcessInfo captures metadata for a process stuck in uninterruptible sleep (D-state).
type DStateProcessInfo struct {
	PID          int           `json:"pid"`
	Command      string        `json:"command"`
	WaitDuration time.Duration `json:"wait_duration"`
	WchanSymbol  string        `json:"wchan_symbol"`
	MountPoint   string        `json:"mount_point"`
}

// IsStuck returns true if the process has waited in D-state longer than the specified threshold.
func (p DStateProcessInfo) IsStuck(maxDuration time.Duration) bool {
	if maxDuration <= 0 {
		maxDuration = DefaultDStateMaxDuration
	}
	return p.WaitDuration >= maxDuration
}

// PaddedInodeMetric provides an atomic counter set for high-scale VFS inspection.
type PaddedInodeMetric struct {
	ScannedFiles   atomic.Uint64
	InodesInUse    atomic.Uint64
	ScanDurationNs atomic.Int64
}

// AddScannedFiles atomically increments the scanned files counter.
func (m *PaddedInodeMetric) AddScannedFiles(delta uint64) uint64 {
	return m.ScannedFiles.Add(delta)
}

// SetInodesInUse atomically updates the count of currently utilized inodes.
func (m *PaddedInodeMetric) SetInodesInUse(val uint64) {
	m.InodesInUse.Store(val)
}

// RecordScanDuration atomically accumulates total scanning latency in nanoseconds.
func (m *PaddedInodeMetric) RecordScanDuration(ns int64) int64 {
	return m.ScanDurationNs.Add(ns)
}

// PaddedDStateMetric provides an atomic counter set for D-state process tracking.
type PaddedDStateMetric struct {
	DStateProcessesDetected atomic.Uint64
	StuckDurationMs         atomic.Int64
	LazyUnmountTriggers     atomic.Uint64
}

// IncDStateProcesses atomically records a detected D-state process.
func (m *PaddedDStateMetric) IncDStateProcesses() uint64 {
	return m.DStateProcessesDetected.Add(1)
}

// AddStuckDurationMs atomically accumulates stuck duration in milliseconds.
func (m *PaddedDStateMetric) AddStuckDurationMs(ms int64) int64 {
	return m.StuckDurationMs.Add(ms)
}

// IncLazyUnmountTriggers atomically records a forced lazy unmount operation.
func (m *PaddedDStateMetric) IncLazyUnmountTriggers() uint64 {
	return m.LazyUnmountTriggers.Add(1)
}

// PaddedPIDMetric provides an atomic counter set for PID and zombie metrics.
type PaddedPIDMetric struct {
	TotalPIDs     atomic.Uint64
	ZombiePIDs    atomic.Uint64
	ThrottledPIDs atomic.Uint64
}

// SetTotalPIDs atomically updates total monitored PIDs.
func (m *PaddedPIDMetric) SetTotalPIDs(count uint64) {
	m.TotalPIDs.Store(count)
}

// AddZombiePIDs atomically adds discovered zombie processes.
func (m *PaddedPIDMetric) AddZombiePIDs(delta uint64) uint64 {
	return m.ZombiePIDs.Add(delta)
}

// IncThrottledPIDs atomically increments throttled or flagged PID limit occurrences.
func (m *PaddedPIDMetric) IncThrottledPIDs() uint64 {
	return m.ThrottledPIDs.Add(1)
}

// WhitelistedTempDirs defines the system paths eligible for automatic temporary file pruning.
// Under no circumstances may paths outside this whitelist be purged automatically.
var WhitelistedTempDirs = []string{
	"/tmp",
	"/var/tmp",
	"/cache",
}

// VFSWatchdog defines low-level and orchestration operations for VFS inode auditing,
// stuck D-state process breaking, hung storage lazy unmounting, and container PID limit protection.
type VFSWatchdog interface {
	// ScanInodes scans mounted filesystems and generates utilization reports.
	ScanInodes(ctx context.Context) ([]InodeReport, error)

	// FindTopInodeConsumers identifies directory consumers containing the largest inode counts.
	FindTopInodeConsumers(ctx context.Context, mountPoint string, limit int) ([]string, error)

	// DetectDStateProcesses inspects processes in uninterruptible sleep exceeding maxDuration.
	DetectDStateProcesses(ctx context.Context, maxDuration time.Duration) ([]DStateProcessInfo, error)

	// RemountOrUnmountHungNFS executes a forced lazy unmount (umount -f -l) on a hung storage mount.
	RemountOrUnmountHungNFS(ctx context.Context, mountPoint string) error

	// EnforcePIDLimits audits pod process namespace sharing and subreapers, guarding PID limits.
	EnforcePIDLimits(ctx context.Context, maxPIDsPerPod int) error
}
