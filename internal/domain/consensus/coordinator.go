package consensus

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Standard domain error definitions and RFC 7807 problem details.
var (
	// ErrInvalidConsensusInput is returned when input parameters violate domain validation invariants.
	ErrInvalidConsensusInput = errors.New("invalid consensus input parameters")

	// ErrEtcdDefragLeaderBlocked indicates defragmentation was attempted on an active Raft leader node.
	ErrEtcdDefragLeaderBlocked = &RFC7807ProblemDetail{
		Type:   "https://k8sselfhost.io/errors/ERR_ETCD_DEFRAG_LEADER_BLOCKED",
		Title:  "Etcd Defragmentation Blocked on Active Leader",
		Status: http.StatusConflict, // 409
		Detail: "Attempted to execute etcd defragmentation directly on the active Raft leader node; leader stepdown is strictly required to prevent cluster-wide heartbeat loss and election storms",
		Code:   "ERR_ETCD_DEFRAG_LEADER_BLOCKED",
	}

	// ErrFencingTokenStale indicates a fencing token generation is stale or superseded.
	ErrFencingTokenStale = &RFC7807ProblemDetail{
		Type:   "https://k8sselfhost.io/errors/ERR_FENCING_TOKEN_STALE",
		Title:  "Distributed Fencing Token Stale",
		Status: http.StatusPreconditionFailed, // 412
		Detail: "Fencing token generation is stale or superseded by a higher generation token; write operation rejected to prevent Martin Kleppmann split-brain data corruption",
		Code:   "ERR_FENCING_TOKEN_STALE",
	}

	// ErrConsensusQuorumLost indicates the healthy member count is below Raft quorum requirement.
	ErrConsensusQuorumLost = &RFC7807ProblemDetail{
		Type:   "https://k8sselfhost.io/errors/ERR_CONSENSUS_QUORUM_LOST",
		Title:  "Consensus Quorum Lost",
		Status: http.StatusServiceUnavailable, // 503
		Detail: "Cluster healthy member count is below required Raft quorum threshold (healthy < N/2 + 1); defragmentation halted to prevent catastrophic cluster partition",
		Code:   "ERR_CONSENSUS_QUORUM_LOST",
	}

	// ErrLeaseConflict indicates a fencing lease is currently held by another active coordinator node.
	ErrLeaseConflict = &RFC7807ProblemDetail{
		Type:   "https://k8sselfhost.io/errors/ERR_LEASE_CONFLICT",
		Title:  "Consensus Fencing Lease Conflict",
		Status: http.StatusConflict, // 409
		Detail: "Fencing lease is actively held by another coordinator node; lease acquisition rejected to maintain HA single-primary invariant",
		Code:   "ERR_LEASE_CONFLICT",
	}
)

// RFC7807ProblemDetail implements RFC 7807 Problem Details for HTTP APIs.
type RFC7807ProblemDetail struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance,omitempty"`
	Code     string `json:"code"`
}

// Error implements the standard Go error interface.
func (p *RFC7807ProblemDetail) Error() string {
	return fmt.Sprintf("[%s] %s: %s (status %d)", p.Code, p.Title, p.Detail, p.Status)
}

// Is supports errors.Is matching by problem code.
func (p *RFC7807ProblemDetail) Is(target error) bool {
	if t, ok := target.(*RFC7807ProblemDetail); ok {
		return p.Code == t.Code
	}
	return false
}

// EtcdMemberStatus models the real-time operational state of an etcd cluster member.
type EtcdMemberStatus struct {
	ID          string `json:"id"`
	Endpoint    string `json:"endpoint"`
	IsLeader    bool   `json:"is_leader"`
	DbSize      int64  `json:"db_size"`
	DbSizeInUse int64  `json:"db_size_in_use"`
	RaftIndex   uint64 `json:"raft_index"`
}

// NewEtcdMemberStatus constructs and validates an EtcdMemberStatus domain entity.
func NewEtcdMemberStatus(
	id string,
	endpoint string,
	isLeader bool,
	dbSize int64,
	dbSizeInUse int64,
	raftIndex uint64,
) (*EtcdMemberStatus, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: member id cannot be empty", ErrInvalidConsensusInput)
	}
	if endpoint == "" {
		return nil, fmt.Errorf("%w: member endpoint cannot be empty", ErrInvalidConsensusInput)
	}
	if dbSize < 0 {
		return nil, fmt.Errorf("%w: dbSize cannot be negative", ErrInvalidConsensusInput)
	}
	if dbSizeInUse < 0 {
		return nil, fmt.Errorf("%w: dbSizeInUse cannot be negative", ErrInvalidConsensusInput)
	}

	return &EtcdMemberStatus{
		ID:          id,
		Endpoint:    endpoint,
		IsLeader:    isLeader,
		DbSize:      dbSize,
		DbSizeInUse: dbSizeInUse,
		RaftIndex:   raftIndex,
	}, nil
}

// FencingToken represents an epoch-based monotonic fencing token for distributed lock fencing.
type FencingToken struct {
	LockKey    string    `json:"lock_key"`
	OwnerID    string    `json:"owner_id"`
	Generation int64     `json:"generation"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// NewFencingToken constructs and validates a FencingToken domain entity.
func NewFencingToken(lockKey, ownerID string, generation int64, expiresAt time.Time) (*FencingToken, error) {
	if lockKey == "" {
		return nil, fmt.Errorf("%w: lock key cannot be empty", ErrInvalidConsensusInput)
	}
	if ownerID == "" {
		return nil, fmt.Errorf("%w: owner id cannot be empty", ErrInvalidConsensusInput)
	}
	if generation <= 0 {
		return nil, fmt.Errorf("%w: generation token must be greater than zero", ErrInvalidConsensusInput)
	}

	return &FencingToken{
		LockKey:    lockKey,
		OwnerID:    ownerID,
		Generation: generation,
		ExpiresAt:  expiresAt,
	}, nil
}

// IsExpired checks whether the fencing token lease has lapsed against the provided reference clock.
func (f *FencingToken) IsExpired(ref time.Time) bool {
	return ref.After(f.ExpiresAt) || ref.Equal(f.ExpiresAt)
}

// FencingLease represents an HA cluster fencing lease with a monotonic fencing token.
type FencingLease struct {
	ClusterID    string    `json:"cluster_id"`
	HolderID     string    `json:"holder_id"`
	FencingToken int64     `json:"fencing_token"`
	AcquiredAt   time.Time `json:"acquired_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// NewFencingLease constructs and validates a FencingLease domain entity.
func NewFencingLease(
	clusterID, holderID string,
	fencingToken int64,
	acquiredAt, expiresAt time.Time,
) (*FencingLease, error) {
	if clusterID == "" {
		return nil, fmt.Errorf("%w: clusterID cannot be empty", ErrInvalidConsensusInput)
	}
	if holderID == "" {
		return nil, fmt.Errorf("%w: holderID cannot be empty", ErrInvalidConsensusInput)
	}
	if fencingToken <= 0 {
		return nil, fmt.Errorf("%w: fencing token must be greater than zero", ErrInvalidConsensusInput)
	}
	return &FencingLease{
		ClusterID:    clusterID,
		HolderID:     holderID,
		FencingToken: fencingToken,
		AcquiredAt:   acquiredAt,
		ExpiresAt:    expiresAt,
	}, nil
}

// IsExpired checks whether the lease has lapsed against the provided reference clock.
func (l *FencingLease) IsExpired(ref time.Time) bool {
	return ref.After(l.ExpiresAt) || ref.Equal(l.ExpiresAt)
}

// LeaseStore defines the persistence and consensus synchronization contract
// for multi-replica distributed HA fencing leases across coordinator nodes.
type LeaseStore interface {
	AcquireOrRenewLease(ctx context.Context, clusterID, holderID string, ttl time.Duration) (*FencingLease, error)
	GetActivePrimary(ctx context.Context, clusterID string) (holderID string, fencingToken int64, err error)
	ValidateToken(ctx context.Context, clusterID string, token int64) (bool, error)
}

// ConsensusCoordinator is the core domain port for distributed lock fencing, etcd defrag, and pooler hygiene.
type ConsensusCoordinator interface {
	AcquireLockWithFencing(ctx context.Context, key, ownerID string, ttl time.Duration) (*FencingToken, error)
	ValidateFencingToken(ctx context.Context, key string, token int64) (bool, error)
	SafeDefragCluster(ctx context.Context, endpoints []string) error
	FlushPgBouncerStaleConnections(ctx context.Context, poolerAddr, newMasterAddr string) error
}

// EtcdClusterClient represents the infrastructure abstraction for etcd management operations.
type EtcdClusterClient interface {
	ListMembers(ctx context.Context) ([]*EtcdMemberStatus, error)
	MemberStatus(ctx context.Context, endpoint string) (*EtcdMemberStatus, error)
	Defragment(ctx context.Context, endpoint string) error
	MoveLeader(ctx context.Context, transfereeID string) error
}

// PgBouncerClient represents the infrastructure abstraction for PostgreSQL connection pool management.
type PgBouncerClient interface {
	Pause(ctx context.Context, poolerAddr string) error
	Resume(ctx context.Context, poolerAddr string) error
	KillStaleConnections(ctx context.Context, poolerAddr, staleHost string) (int, error)
	RelocatePool(ctx context.Context, poolerAddr, newMasterAddr string) error
}

// =================================================================================================
// PHYSICAL HARDWARE BOTTLENECK & REAL-WORLD FAILURE MODE ANALYSIS
// =================================================================================================
//
// 1. etcd WAL fsync Disk I/O Lockup During Defragmentation (Up to 10s Disk Freeze):
//    etcd uses BoltDB (bbolt) as its underlying key-value storage engine. Over time, bbolt pages become
//    fragmented due to MVCC historical revisions. Running defragmentation rewrites the bbolt database file
//    from scratch. During this rewrite, boltdb locks the file and executes heavy sequential disk I/O followed
//    by synchronous fsync() barriers. On spinning disks or cloud volumes (e.g. AWS gp2/gp3 with exhausted burst
//    credits), fsync can take between 2 to 10 seconds. During this freeze, etcd cannot append raft entries or
//    respond to heartbeats.
//
// 2. Catastrophic Leader Defrag Election Storms (SRE Misoperation):
//    If an operator runs `etcdctl defrag` directly on the active cluster leader, the leader's disk I/O lockup
//    prevents it from dispatching Raft heartbeat RPCs to followers. After electionTimeout (typically 1000ms),
//    followers assume the leader has crashed and initiate election rounds. As the old leader is still alive and
//    locking its bolt file, follower vote requests either fail or trigger a series of cascading split-brain
//    elections, destabilizing Kubernetes API servers and causing widespread kubelet disconnection storms.
//    Mitigation: Never defrag the leader directly. Non-leaders must be defragmented first, followed by a graceful
//    `move-leader` stepdown, allowing election stabilization before old leader defragmentation.
//
// 3. Martin Kleppmann GC Pause Catastrophe (Distributed Lock Leases):
//    Distributed locks based solely on lease TTLs (e.g. standard Redis Redlock) are fundamentally vulnerable
//    to asynchronous process pauses: Linux swap thrashing, JVM/Go stop-the-world garbage collection pauses, or
//    hypervisor vCPU scheduling delays. If Client A acquires a 5-second lease, pauses for 6 seconds due to GC,
//    its lease expires. Client B acquires the lock and initiates writes. Client A awakes believing it still holds
//    the lock and issues an overwrite, corrupting shared data.
//    Mitigation: Monotonically increasing fencing tokens (generation numbers). Every write to persistent storage
//    must validate that the caller's generation token is strictly greater than the committed token generation.
//
// 4. PgBouncer Connection Poisoning Post-Patroni Failover:
//    When PostgreSQL failover occurs via Patroni/etcd consensus, the old primary is demoted to a read-only
//    standby or shut down. However, PgBouncer maintains persistent backend socket connections in transaction or
//    session pooling modes. If PgBouncer does not execute `PAUSE` and immediately sever existing connections
//    pointing to the demoted primary host, client application write queries (`INSERT`/`UPDATE`) hit the demoted
//    read-only replica and fail with `cannot execute INSERT in a read-only transaction`, or worse, silently read
//    stale un-replicated data.
// =================================================================================================

// ConsensusTelemetryTracker provides atomic counters for consensus operations.
type ConsensusTelemetryTracker struct {
	FencingGeneration   atomic.Int64
	FencingValidations  atomic.Uint64
	FencingStaleRejects atomic.Uint64
	RaftIndexObserved   atomic.Uint64
	DefragCyclesRun     atomic.Uint64
	LeaderStepDownCount atomic.Uint64
	PgBouncerFlushes    atomic.Uint64
	StaleSocketsKilled  atomic.Uint64
}

// NewConsensusTelemetryTracker creates an initialized ConsensusTelemetryTracker.
func NewConsensusTelemetryTracker() *ConsensusTelemetryTracker {
	return &ConsensusTelemetryTracker{}
}

// NextGeneration atomically advances and returns the next monotonic generation token.
func (t *ConsensusTelemetryTracker) NextGeneration() int64 {
	return t.FencingGeneration.Add(1)
}

// GetGeneration loads the current fencing generation.
func (t *ConsensusTelemetryTracker) GetGeneration() int64 {
	return t.FencingGeneration.Load()
}

// IncValidations increments the successful fencing validation count.
func (t *ConsensusTelemetryTracker) IncValidations() {
	t.FencingValidations.Add(1)
}

// GetValidations returns the total validation count.
func (t *ConsensusTelemetryTracker) GetValidations() uint64 {
	return t.FencingValidations.Load()
}

// IncStaleRejects increments the count of rejected stale fencing tokens.
func (t *ConsensusTelemetryTracker) IncStaleRejects() {
	t.FencingStaleRejects.Add(1)
}

// GetStaleRejects returns the total stale token rejection count.
func (t *ConsensusTelemetryTracker) GetStaleRejects() uint64 {
	return t.FencingStaleRejects.Load()
}

// RecordRaftIndex atomically records the latest observed Raft index if greater than observed.
func (t *ConsensusTelemetryTracker) RecordRaftIndex(idx uint64) {
	curr := t.RaftIndexObserved.Load()
	for idx > curr {
		if t.RaftIndexObserved.CompareAndSwap(curr, idx) {
			break
		}
		curr = t.RaftIndexObserved.Load()
	}
}

// GetRaftIndex returns the highest observed Raft index.
func (t *ConsensusTelemetryTracker) GetRaftIndex() uint64 {
	return t.RaftIndexObserved.Load()
}

// IncDefragCycles increments the completed defragmentation cycle count.
func (t *ConsensusTelemetryTracker) IncDefragCycles() {
	t.DefragCyclesRun.Add(1)
}

// IncLeaderStepDown increments the count of graceful leader stepdown operations.
func (t *ConsensusTelemetryTracker) IncLeaderStepDown() {
	t.LeaderStepDownCount.Add(1)
}

// IncPgBouncerFlushes increments the total PgBouncer pool flushes executed.
func (t *ConsensusTelemetryTracker) IncPgBouncerFlushes() {
	t.PgBouncerFlushes.Add(1)
}

// AddStaleSocketsKilled adds to the total severed stale socket count.
func (t *ConsensusTelemetryTracker) AddStaleSocketsKilled(count uint64) {
	t.StaleSocketsKilled.Add(count)
}

// DefragState represents the execution stage of rolling etcd defragmentation.
type DefragState int32

const (
	DefragStateIdle DefragState = iota
	DefragStateDiscovering
	DefragStateFollowersInProgress
	DefragStateLeaderStepdown
	DefragStateOldLeaderDefrag
	DefragStateCompleted
	DefragStateFailed
)

// EtcdDefragStateMachine coordinates rolling defragmentation state safely.
type EtcdDefragStateMachine struct {
	State          atomic.Int32
	NodesProcessed atomic.Uint64
	LastDefragUnix atomic.Int64
}

// NewEtcdDefragStateMachine initializes a new EtcdDefragStateMachine in Idle state.
func NewEtcdDefragStateMachine() *EtcdDefragStateMachine {
	sm := &EtcdDefragStateMachine{}
	sm.State.Store(int32(DefragStateIdle))
	return sm
}

// Transition moves the state machine to a new state.
func (sm *EtcdDefragStateMachine) Transition(next DefragState) {
	sm.State.Store(int32(next))
}

// CurrentState returns the current execution state.
func (sm *EtcdDefragStateMachine) CurrentState() DefragState {
	return DefragState(sm.State.Load())
}

// IncNodesProcessed increments the count of defragmented nodes.
func (sm *EtcdDefragStateMachine) IncNodesProcessed() {
	sm.NodesProcessed.Add(1)
}

// SetLastDefragUnix records the timestamp of the last defragmentation run.
func (sm *EtcdDefragStateMachine) SetLastDefragUnix(unixSec int64) {
	sm.LastDefragUnix.Store(unixSec)
}
