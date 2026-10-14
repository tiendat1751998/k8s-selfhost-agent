// Package sentinel implements autonomous edge self-healing and state-transition noise reduction.
package sentinel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SentinelState represents discrete lifecycle stability states of an edge node.
type SentinelState string

const (
	// StateNominal indicates all hardware metrics within safe thresholds.
	StateNominal SentinelState = "NOMINAL"
	// StateWarning indicates threshold breach detected before remediation completes.
	StateWarning SentinelState = "WARNING"
	// StateRemediated indicates successful autonomous intervention.
	StateRemediated SentinelState = "REMEDIATED"
	// StateFailed indicates autonomous remediation could not recover node health.
	StateFailed SentinelState = "FAILED"
)

// NodeHardwareSnapshot captures instantaneous hardware conditions evaluated by the 100ms watchdog.
type NodeHardwareSnapshot struct {
	NodeID                  string    `json:"node_id"`
	DiskUsagePct            float64   `json:"disk_usage_pct"`
	CgroupMemUsagePct       float64   `json:"cgroup_mem_usage_pct"`
	CgroupMemGrowthRateMBps float64   `json:"cgroup_mem_growth_rate_mbps"`
	FrozenSystemdUnits      []string  `json:"frozen_systemd_units"`
	Timestamp               time.Time `json:"timestamp"`
}

// RemediationExecutor defines the low-level recovery actions available to the sentinel.
type RemediationExecutor interface {
	PruneContainersAndLogs(ctx context.Context, nodeID string) error
	PreemptBatchProcesses(ctx context.Context, nodeID string) error
	RestartSystemdUnit(ctx context.Context, nodeID string, unit string) error
}

// StateTransitionEvent describes an edge state transition emitted across the mesh.
type StateTransitionEvent struct {
	NodeID       string        `json:"node_id"`
	FromState    SentinelState `json:"from_state"`
	ToState      SentinelState `json:"to_state"`
	Reason       string        `json:"reason"`
	Remediations []string      `json:"remediations"`
	Timestamp    time.Time     `json:"timestamp"`
}

// TransitionListener handles state-transition delta events.
type TransitionListener interface {
	OnStateTransition(ctx context.Context, event StateTransitionEvent) error
}

// EdgeSentinel manages autonomous deterministic watchdog evaluations and delta emission filtering.
type EdgeSentinel struct {
	mu          sync.RWMutex
	executor    RemediationExecutor
	listener    TransitionListener
	nodeStates  map[string]SentinelState
	tickCounter atomic.Uint64
	emitCounter atomic.Uint64
}

// NewEdgeSentinel initializes an autonomous sentinel with required executor and optional listener.
func NewEdgeSentinel(executor RemediationExecutor, listener TransitionListener) *EdgeSentinel {
	return &EdgeSentinel{
		executor:   executor,
		listener:   listener,
		nodeStates: make(map[string]SentinelState),
	}
}

// EvaluateSnapshot executes 100ms deterministic rule checks against the hardware snapshot.
func (s *EdgeSentinel) EvaluateSnapshot(ctx context.Context, snapshot NodeHardwareSnapshot) (SentinelState, error) {
	if snapshot.NodeID == "" {
		return StateFailed, errors.New("invalid snapshot: nodeID cannot be empty")
	}

	s.tickCounter.Add(1)

	var remediations []string
	var remediationErrors []error
	var reasons []string

	// Rule 1: Disk > 92% -> auto-prune containers and rotated logs
	if snapshot.DiskUsagePct > 92.0 {
		reasons = append(reasons, fmt.Sprintf("disk usage %.1f%% > 92%%", snapshot.DiskUsagePct))
		if s.executor != nil {
			if err := s.executor.PruneContainersAndLogs(ctx, snapshot.NodeID); err != nil {
				remediationErrors = append(remediationErrors, fmt.Errorf("pruning disk: %w", err))
			} else {
				remediations = append(remediations, "prune_containers_and_logs")
			}
		}
	}

	// Rule 2: Cgroup OOM Imminent (mem >= 95% and positive growth rate) -> preempt batch processes
	if snapshot.CgroupMemUsagePct >= 95.0 && snapshot.CgroupMemGrowthRateMBps > 0 {
		reasons = append(reasons, fmt.Sprintf("cgroup mem %.1f%% >= 95%% (growth %.1f MB/s)", snapshot.CgroupMemUsagePct, snapshot.CgroupMemGrowthRateMBps))
		if s.executor != nil {
			if err := s.executor.PreemptBatchProcesses(ctx, snapshot.NodeID); err != nil {
				remediationErrors = append(remediationErrors, fmt.Errorf("preempting batch processes: %w", err))
			} else {
				remediations = append(remediations, "preempt_batch_processes")
			}
		}
	}

	// Rule 3: Frozen Systemd Units -> restart affected units
	for _, unit := range snapshot.FrozenSystemdUnits {
		reasons = append(reasons, fmt.Sprintf("frozen systemd unit: %s", unit))
		if s.executor != nil {
			if err := s.executor.RestartSystemdUnit(ctx, snapshot.NodeID, unit); err != nil {
				remediationErrors = append(remediationErrors, fmt.Errorf("restarting unit %s: %w", unit, err))
			} else {
				remediations = append(remediations, fmt.Sprintf("restart_systemd_unit:%s", unit))
			}
		}
	}

	// Determine resulting state
	var targetState SentinelState
	if len(remediationErrors) > 0 {
		targetState = StateFailed
	} else if len(remediations) > 0 {
		targetState = StateRemediated
	} else {
		targetState = StateNominal
	}

	// State-Transition Delta Filter
	if err := s.applyStateTransition(ctx, snapshot.NodeID, targetState, reasons, remediations); err != nil {
		return targetState, err
	}

	if len(remediationErrors) > 0 {
		return targetState, errors.Join(remediationErrors...)
	}

	return targetState, nil
}

func (s *EdgeSentinel) applyStateTransition(
	ctx context.Context,
	nodeID string,
	targetState SentinelState,
	reasons []string,
	remediations []string,
) error {
	s.mu.Lock()
	currentState, tracked := s.nodeStates[nodeID]
	if !tracked {
		currentState = StateNominal
	}

	// Suppress identical state ticks (Delta Filter)
	if tracked && currentState == targetState {
		s.mu.Unlock()
		return nil
	}

	s.nodeStates[nodeID] = targetState
	s.mu.Unlock()

	// If transitioning to or between non-nominal states, or leaving non-nominal back to nominal
	if currentState != targetState {
		s.emitCounter.Add(1)
		if s.listener != nil {
			event := StateTransitionEvent{
				NodeID:       nodeID,
				FromState:    currentState,
				ToState:      targetState,
				Reason:       strings.Join(reasons, "; "),
				Remediations: remediations,
				Timestamp:    time.Now().UTC(),
			}
			return s.listener.OnStateTransition(ctx, event)
		}
	}

	return nil
}

// TickCount returns total watchdog evaluations performed.
func (s *EdgeSentinel) TickCount() uint64 {
	return s.tickCounter.Load()
}

// EmitCount returns count of delta transition events emitted.
func (s *EdgeSentinel) EmitCount() uint64 {
	return s.emitCounter.Load()
}

// NoiseReductionPct calculates noise reduction ratio vs naive continuous heartbeats.
func (s *EdgeSentinel) NoiseReductionPct() float64 {
	ticks := s.tickCounter.Load()
	if ticks == 0 {
		return 100.0
	}
	emitted := s.emitCounter.Load()
	return (1.0 - (float64(emitted) / float64(ticks))) * 100.0
}
