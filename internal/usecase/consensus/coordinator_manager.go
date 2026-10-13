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
	cooldownDuration time.Duration

	telemetry    *domainconsensus.ConsensusTelemetryTracker
	stateMachine *domainconsensus.EtcdDefragStateMachine

	mu           sync.RWMutex
	activeLocks  map[string]lockRecord
	committedGen map[string]int64
}

// NewCoordinatorManager initializes a new CoordinatorManager with required infrastructure clients.
func NewCoordinatorManager(
	etcdClient domainconsensus.EtcdClusterClient,
	pgClient domainconsensus.PgBouncerClient,
	opts ...CoordinatorOption,
) *CoordinatorManager {
	cm := &CoordinatorManager{
		etcdClient:       etcdClient,
		pgClient:         pgClient,
		cooldownDuration: DefaultDefragCooldown,
		telemetry:        domainconsensus.NewConsensusTelemetryTracker(),
		stateMachine:     domainconsensus.NewEtcdDefragStateMachine(),
		activeLocks:      make(map[string]lockRecord),
		committedGen:     make(map[string]int64),
	}

	for _, opt := range opts {
		opt(cm)
	}

	return cm
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

// ValidateFencingToken validates that a fencing token is strictly greater than the committed generation.
func (cm *CoordinatorManager) ValidateFencingToken(ctx context.Context, key string, token int64) (bool, error) {
	if key == "" {
		return false, fmt.Errorf("%w: key cannot be empty", domainconsensus.ErrInvalidConsensusInput)
	}
	if token <= 0 {
		return false, fmt.Errorf("%w: token must be greater than zero", domainconsensus.ErrInvalidConsensusInput)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	committed := cm.committedGen[key]
	if token <= committed {
		cm.telemetry.IncStaleRejects()
		return false, domainconsensus.ErrFencingTokenStale
	}

	cm.committedGen[key] = token
	cm.telemetry.IncValidations()
	return true, nil
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

	if err := cm.pgClient.Pause(ctx, poolerAddr); err != nil {
		return fmt.Errorf("pausing pgbouncer pool: %w", err)
	}

	killed, err := cm.pgClient.KillStaleConnections(ctx, poolerAddr, "demoted-primary")
	if err != nil {
		_ = cm.pgClient.Resume(ctx, poolerAddr)
		return fmt.Errorf("killing stale connections: %w", err)
	}
	cm.telemetry.AddStaleSocketsKilled(uint64(killed))

	if err := cm.pgClient.RelocatePool(ctx, poolerAddr, newMasterAddr); err != nil {
		_ = cm.pgClient.Resume(ctx, poolerAddr)
		return fmt.Errorf("relocating pool to new master: %w", err)
	}

	if err := cm.pgClient.Resume(ctx, poolerAddr); err != nil {
		return fmt.Errorf("resuming pgbouncer pool: %w", err)
	}

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
