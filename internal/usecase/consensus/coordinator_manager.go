package consensus

import (
	"context"
	"fmt"
	"sync"
	"time"

	domainconsensus "github.com/datdt/k8sselfhost/internal/domain/consensus"
)

// Default cooldown duration between rolling etcd member defragmentations.
const DefaultDefragCooldown = 30 * time.Second

// CoordinatorOption configures CoordinatorManager runtime settings.
type CoordinatorOption func(*CoordinatorManager)

// WithCooldownDuration configures the cooldown duration between defragmentation operations.
func WithCooldownDuration(d time.Duration) CoordinatorOption {
	return func(cm *CoordinatorManager) {
		if d >= 0 {
			cm.cooldownDuration = d
		}
	}
}

// WithLeaseStore configures the distributed HA consensus lease store.
func WithLeaseStore(store domainconsensus.LeaseStore) CoordinatorOption {
	return func(cm *CoordinatorManager) {
		cm.leaseStore = store
	}
}

type lockRecord struct {
	ownerID    string
	generation int64
	expiresAt  time.Time
}

// CoordinatorManager orchestrates distributed consensus fencing, safe rolling etcd defragmentation,
// and PgBouncer post-failover connection pool hygiene.
type CoordinatorManager struct {
	etcdClient       domainconsensus.EtcdClusterClient
	pgClient         domainconsensus.PgBouncerClient
	leaseStore       domainconsensus.LeaseStore
	cooldownDuration time.Duration

	telemetry    *domainconsensus.ConsensusTelemetryTracker
	stateMachine *domainconsensus.EtcdDefragStateMachine

	mu                sync.RWMutex
	activeLocks       map[string]lockRecord
	committedGen      map[string]int64
	activePrimaries   map[string]string
	previousPrimaries map[string]string
}

// NewCoordinatorManager initializes a new CoordinatorManager with required infrastructure clients.
func NewCoordinatorManager(
	etcdClient domainconsensus.EtcdClusterClient,
	pgClient domainconsensus.PgBouncerClient,
	opts ...CoordinatorOption,
) *CoordinatorManager {
	cm := &CoordinatorManager{
		etcdClient:        etcdClient,
		pgClient:          pgClient,
		cooldownDuration:  DefaultDefragCooldown,
		telemetry:         domainconsensus.NewConsensusTelemetryTracker(),
		stateMachine:      domainconsensus.NewEtcdDefragStateMachine(),
		activeLocks:       make(map[string]lockRecord),
		committedGen:      make(map[string]int64),
		activePrimaries:   make(map[string]string),
		previousPrimaries: make(map[string]string),
	}

	for _, opt := range opts {
		opt(cm)
	}

	return cm
}

// SetLeaseStore dynamically attaches or updates the distributed HA consensus lease store.
func (cm *CoordinatorManager) SetLeaseStore(store domainconsensus.LeaseStore) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.leaseStore = store
}

// SafeDefragCluster safely defragments etcd members one-by-one, enforcing quorum preservation
// and mandatory leader stepdown prior to leader defragmentation.
func (cm *CoordinatorManager) SafeDefragCluster(ctx context.Context, endpoints []string) error {
	if cm.etcdClient == nil {
		return fmt.Errorf("%w: etcd client is not configured", domainconsensus.ErrInvalidConsensusInput)
	}

	cm.stateMachine.Transition(domainconsensus.DefragStateDiscovering)

	members, err := cm.etcdClient.ListMembers(ctx)
	if err != nil {
		cm.stateMachine.Transition(domainconsensus.DefragStateFailed)
		return fmt.Errorf("listing etcd members: %w", err)
	}

	healthyCount := len(members)
	totalNodes := len(endpoints)
	if totalNodes < healthyCount {
		totalNodes = healthyCount
	}

	requiredQuorum := (totalNodes / 2) + 1
	if healthyCount < requiredQuorum {
		cm.stateMachine.Transition(domainconsensus.DefragStateFailed)
		return domainconsensus.ErrConsensusQuorumLost
	}

	var leader *domainconsensus.EtcdMemberStatus
	followers := make([]*domainconsensus.EtcdMemberStatus, 0, len(members))

	for _, m := range members {
		if m.RaftIndex > 0 {
			cm.telemetry.RecordRaftIndex(m.RaftIndex)
		}
		if m.IsLeader {
			leader = m
		} else {
			followers = append(followers, m)
		}
	}

	cm.stateMachine.Transition(domainconsensus.DefragStateFollowersInProgress)

	// Step 1: Defragment all non-leader follower nodes one-by-one
	for _, f := range followers {
		select {
		case <-ctx.Done():
			cm.stateMachine.Transition(domainconsensus.DefragStateFailed)
			return ctx.Err()
		default:
		}

		if err := cm.etcdClient.Defragment(ctx, f.Endpoint); err != nil {
			cm.stateMachine.Transition(domainconsensus.DefragStateFailed)
			return fmt.Errorf("defragmenting follower %s (%s): %w", f.ID, f.Endpoint, err)
		}
		cm.stateMachine.IncNodesProcessed()

		if cm.cooldownDuration > 0 {
			select {
			case <-ctx.Done():
				cm.stateMachine.Transition(domainconsensus.DefragStateFailed)
				return ctx.Err()
			case <-time.After(cm.cooldownDuration):
			}
		}
	}

	// Step 2: Safe handling of leader node
	if leader != nil {
		if len(followers) == 0 {
			// Single node cluster cannot step down leadership
			if err := cm.etcdClient.Defragment(ctx, leader.Endpoint); err != nil {
				cm.stateMachine.Transition(domainconsensus.DefragStateFailed)
				return fmt.Errorf("defragmenting standalone leader: %w", err)
			}
			cm.stateMachine.IncNodesProcessed()
		} else {
			cm.stateMachine.Transition(domainconsensus.DefragStateLeaderStepdown)

			transferee := followers[0]
			if err := cm.etcdClient.MoveLeader(ctx, transferee.ID); err != nil {
				cm.stateMachine.Transition(domainconsensus.DefragStateFailed)
				return fmt.Errorf("%w: failed to step down leader %s: %v", domainconsensus.ErrEtcdDefragLeaderBlocked, leader.ID, err)
			}
			cm.telemetry.IncLeaderStepDown()

			// Wait for Raft election stabilization
			if cm.cooldownDuration > 0 {
				select {
				case <-ctx.Done():
					cm.stateMachine.Transition(domainconsensus.DefragStateFailed)
					return ctx.Err()
				case <-time.After(cm.cooldownDuration):
				}
			}

			cm.stateMachine.Transition(domainconsensus.DefragStateOldLeaderDefrag)
			if err := cm.etcdClient.Defragment(ctx, leader.Endpoint); err != nil {
				cm.stateMachine.Transition(domainconsensus.DefragStateFailed)
				return fmt.Errorf("defragmenting demoted leader %s (%s): %w", leader.ID, leader.Endpoint, err)
			}
			cm.stateMachine.IncNodesProcessed()
		}
	}

	cm.stateMachine.Transition(domainconsensus.DefragStateCompleted)
	cm.stateMachine.SetLastDefragUnix(time.Now().Unix())
	cm.telemetry.IncDefragCycles()
	return nil
}

// AcquireLockWithFencing acquires a distributed lock lease protected by an atomic monotonic fencing token.
// When leaseStore is configured, it delegates atomically to the distributed store;
// otherwise it falls back to the thread-safe local in-memory tracker.
func (cm *CoordinatorManager) AcquireLockWithFencing(
	ctx context.Context,
	key, ownerID string,
	ttl time.Duration,
) (*domainconsensus.FencingToken, error) {
	if key == "" || ownerID == "" {
		return nil, fmt.Errorf("%w: key and ownerID cannot be empty", domainconsensus.ErrInvalidConsensusInput)
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("%w: ttl must be positive", domainconsensus.ErrInvalidConsensusInput)
	}

	cm.mu.RLock()
	store := cm.leaseStore
	cm.mu.RUnlock()

	if store != nil {
		lease, err := store.AcquireOrRenewLease(ctx, key, ownerID, ttl)
		if err != nil {
			return nil, err
		}
		return domainconsensus.NewFencingToken(key, ownerID, lease.FencingToken, lease.ExpiresAt)
	}

	generation := cm.telemetry.NextGeneration()
	expiresAt := time.Now().UTC().Add(ttl)

	cm.mu.Lock()
	cm.activeLocks[key] = lockRecord{
		ownerID:    ownerID,
		generation: generation,
		expiresAt:  expiresAt,
	}
	cm.mu.Unlock()

	return domainconsensus.NewFencingToken(key, ownerID, generation, expiresAt)
}

// AcquireLease acquires or renews a distributed fencing lease for an HA cluster.
// When leaseStore is configured, it delegates atomically to the distributed store;
// otherwise it falls back to the thread-safe local in-memory tracker.
func (cm *CoordinatorManager) AcquireLease(
	ctx context.Context,
	clusterID, holderID string,
	ttl time.Duration,
) (*domainconsensus.FencingLease, error) {
	if clusterID == "" || holderID == "" {
		return nil, fmt.Errorf("%w: clusterID and holderID cannot be empty", domainconsensus.ErrInvalidConsensusInput)
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("%w: ttl must be positive", domainconsensus.ErrInvalidConsensusInput)
	}

	cm.mu.RLock()
	store := cm.leaseStore
	cm.mu.RUnlock()

	if store != nil {
		prevHolder, _, _ := store.GetActivePrimary(ctx, clusterID)
		lease, err := store.AcquireOrRenewLease(ctx, clusterID, holderID, ttl)
		if err != nil {
			return nil, err
		}
		cm.mu.Lock()
		if prevHolder != "" && prevHolder != holderID {
			cm.previousPrimaries[clusterID] = prevHolder
		}
		cm.activePrimaries[clusterID] = holderID
		cm.mu.Unlock()
		return lease, nil
	}

	// Thread-safe in-memory fallback
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now().UTC()
	current, exists := cm.activeLocks[clusterID]
	if exists && now.Before(current.expiresAt) && current.ownerID != holderID {
		return nil, domainconsensus.ErrLeaseConflict
	}

	if exists && current.ownerID != holderID && current.ownerID != "" {
		cm.previousPrimaries[clusterID] = current.ownerID
	}

	generation := cm.telemetry.NextGeneration()
	expiresAt := now.Add(ttl)
	cm.activeLocks[clusterID] = lockRecord{
		ownerID:    holderID,
		generation: generation,
		expiresAt:  expiresAt,
	}
	cm.activePrimaries[clusterID] = holderID

	return domainconsensus.NewFencingLease(clusterID, holderID, generation, now, expiresAt)
}

// SetActivePrimary explicitly records the active primary host for a connection pooler.
func (cm *CoordinatorManager) SetActivePrimary(poolerAddr, primaryAddr string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.activePrimaries[poolerAddr] = primaryAddr
}

// GetActivePrimary returns the tracked primary host for a connection pooler.
func (cm *CoordinatorManager) GetActivePrimary(poolerAddr string) string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.activePrimaries[poolerAddr]
}

// ValidateFencingToken validates that a fencing token is strictly greater than the committed generation,
// is monotonic against active lease generations, and has not expired.
// When leaseStore is configured, validation is delegated to the shared distributed store.
func (cm *CoordinatorManager) ValidateFencingToken(ctx context.Context, key string, token int64) (bool, error) {
	if key == "" {
		return false, fmt.Errorf("%w: key cannot be empty", domainconsensus.ErrInvalidConsensusInput)
	}
	if token <= 0 {
		return false, fmt.Errorf("%w: token must be greater than zero", domainconsensus.ErrInvalidConsensusInput)
	}

	cm.mu.RLock()
	store := cm.leaseStore
	cm.mu.RUnlock()

	if store != nil {
		valid, err := store.ValidateToken(ctx, key, token)
		if err != nil {
			cm.telemetry.IncStaleRejects()
			return false, err
		}
		if !valid {
			cm.telemetry.IncStaleRejects()
			return false, domainconsensus.ErrFencingTokenStale
		}
		cm.telemetry.IncValidations()
		return true, nil
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	committed := cm.committedGen[key]
	if token <= committed {
		cm.telemetry.IncStaleRejects()
		return false, domainconsensus.ErrFencingTokenStale
	}

	// Validate monotonicity and expiration against active lock record
	if record, exists := cm.activeLocks[key]; exists {
		if token < record.generation {
			cm.telemetry.IncStaleRejects()
			return false, domainconsensus.ErrFencingTokenStale
		}
		if time.Now().UTC().After(record.expiresAt) {
			cm.telemetry.IncStaleRejects()
			return false, fmt.Errorf("%w: lock lease has expired", domainconsensus.ErrFencingTokenStale)
		}
	}

	cm.committedGen[key] = token
	cm.telemetry.IncValidations()
	return true, nil
}

// ExecuteStonithFence dynamically fences the stale or previously active primary node for clusterID.
// It queries the shared distributed lease store (or local memory) to resolve the target node,
// severs any stale backend connection pools via PgBouncer, and records telemetry.
func (cm *CoordinatorManager) ExecuteStonithFence(ctx context.Context, clusterID string, targetNode ...string) (string, error) {
	if clusterID == "" {
		return "", fmt.Errorf("%w: clusterID cannot be empty", domainconsensus.ErrInvalidConsensusInput)
	}

	var target string
	if len(targetNode) > 0 && targetNode[0] != "" {
		target = targetNode[0]
	}

	cm.mu.RLock()
	store := cm.leaseStore
	prevPrimary := cm.previousPrimaries[clusterID]
	activePrimary := cm.activePrimaries[clusterID]
	cm.mu.RUnlock()

	if target == "" {
		if prevPrimary != "" {
			target = prevPrimary
		} else if store != nil {
			primary, _, err := store.GetActivePrimary(ctx, clusterID)
			if err != nil {
				return "", fmt.Errorf("resolving active primary from lease store: %w", err)
			}
			target = primary
		} else if activePrimary != "" {
			target = activePrimary
		} else if rec, exists := cm.activeLocks[clusterID]; exists {
			target = rec.ownerID
		}
	}

	if target == "" {
		return "", fmt.Errorf("no active or previous primary found to fence for cluster %s", clusterID)
	}

	if cm.pgClient != nil {
		killed, err := cm.pgClient.KillStaleConnections(ctx, clusterID, target)
		if err != nil {
			return "", fmt.Errorf("severing stale connections during stonith fencing for host %s: %w", target, err)
		}
		cm.telemetry.AddStaleSocketsKilled(uint64(killed))
	}

	return target, nil
}

// FlushPgBouncerStaleConnections executes connection pause, severs stale backend sockets,
// updates pool relocation to new master, and resumes traffic.
func (cm *CoordinatorManager) FlushPgBouncerStaleConnections(
	ctx context.Context,
	poolerAddr, newMasterAddr string,
) error {
	if poolerAddr == "" || newMasterAddr == "" {
		return fmt.Errorf("%w: poolerAddr and newMasterAddr cannot be empty", domainconsensus.ErrInvalidConsensusInput)
	}
	if cm.pgClient == nil {
		return fmt.Errorf("%w: pgbouncer client is not configured", domainconsensus.ErrInvalidConsensusInput)
	}

	// Resolve the target stale node deterministically:
	// 1. Check if an active primary is tracked for this pooler
	// 2. Or check active lock for poolerAddr or database primary
	// 3. Or derive deterministically from pooler address
	cm.mu.Lock()
	staleHost := cm.activePrimaries[poolerAddr]
	if staleHost == "" {
		if rec, exists := cm.activeLocks[poolerAddr]; exists && rec.ownerID != "" {
			staleHost = rec.ownerID
		} else if rec, exists := cm.activeLocks["primary"]; exists && rec.ownerID != "" {
			staleHost = rec.ownerID
		} else if rec, exists := cm.activeLocks["patroni/master"]; exists && rec.ownerID != "" {
			staleHost = rec.ownerID
		} else {
			staleHost = fmt.Sprintf("stale-%s", poolerAddr)
		}
	}
	cm.mu.Unlock()

	if err := cm.pgClient.Pause(ctx, poolerAddr); err != nil {
		return fmt.Errorf("pausing pgbouncer pool: %w", err)
	}

	killed, err := cm.pgClient.KillStaleConnections(ctx, poolerAddr, staleHost)
	if err != nil {
		_ = cm.pgClient.Resume(ctx, poolerAddr)
		return fmt.Errorf("killing stale connections for host %s: %w", staleHost, err)
	}
	cm.telemetry.AddStaleSocketsKilled(uint64(killed))

	if err := cm.pgClient.RelocatePool(ctx, poolerAddr, newMasterAddr); err != nil {
		_ = cm.pgClient.Resume(ctx, poolerAddr)
		return fmt.Errorf("relocating pool to new master: %w", err)
	}

	if err := cm.pgClient.Resume(ctx, poolerAddr); err != nil {
		return fmt.Errorf("resuming pgbouncer pool: %w", err)
	}

	cm.mu.Lock()
	cm.activePrimaries[poolerAddr] = newMasterAddr
	cm.mu.Unlock()

	cm.telemetry.IncPgBouncerFlushes()
	return nil
}

// GetTelemetryTracker returns the cache-line padded consensus telemetry tracker.
func (cm *CoordinatorManager) GetTelemetryTracker() *domainconsensus.ConsensusTelemetryTracker {
	return cm.telemetry
}

// GetDefragStateMachine returns the rolling etcd defragmentation state machine.
func (cm *CoordinatorManager) GetDefragStateMachine() *domainconsensus.EtcdDefragStateMachine {
	return cm.stateMachine
}
