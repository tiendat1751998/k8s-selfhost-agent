package sentinel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type mockExecutor struct {
	mu           sync.Mutex
	prunedNodes  []string
	preemptNodes []string
	restarted    []string
	pruneErr     error
	preemptErr   error
	restartErr   error
}

func (m *mockExecutor) PruneContainersAndLogs(ctx context.Context, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pruneErr != nil {
		return m.pruneErr
	}
	m.prunedNodes = append(m.prunedNodes, nodeID)
	return nil
}

func (m *mockExecutor) PreemptBatchProcesses(ctx context.Context, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.preemptErr != nil {
		return m.preemptErr
	}
	m.preemptNodes = append(m.preemptNodes, nodeID)
	return nil
}

func (m *mockExecutor) RestartSystemdUnit(ctx context.Context, nodeID string, unit string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.restartErr != nil {
		return m.restartErr
	}
	m.restarted = append(m.restarted, fmt.Sprintf("%s:%s", nodeID, unit))
	return nil
}

type mockListener struct {
	mu     sync.Mutex
	events []StateTransitionEvent
}

func (m *mockListener) OnStateTransition(ctx context.Context, event StateTransitionEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *mockListener) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

func TestEdgeSentinel_DiskUsagePruning(t *testing.T) {
	ctx := context.Background()
	exec := &mockExecutor{}
	listener := &mockListener{}

	sentinel := NewEdgeSentinel(exec, listener)

	// Snapshot with DiskUsage > 92%
	snapshot := NodeHardwareSnapshot{
		NodeID:                  "node-disk-01",
		DiskUsagePct:            93.5,
		CgroupMemUsagePct:       60.0,
		CgroupMemGrowthRateMBps: 0,
		FrozenSystemdUnits:      nil,
		Timestamp:               time.Now(),
	}

	state, err := sentinel.EvaluateSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state != StateRemediated {
		t.Fatalf("expected state %s, got %s", StateRemediated, state)
	}

	if len(exec.prunedNodes) != 1 || exec.prunedNodes[0] != "node-disk-01" {
		t.Fatalf("expected prune on node-disk-01, got %v", exec.prunedNodes)
	}

	if listener.Count() != 1 {
		t.Fatalf("expected 1 transition event, got %d", listener.Count())
	}
	ev := listener.events[0]
	if ev.ToState != StateRemediated {
		t.Fatalf("expected transition to %s, got %s", StateRemediated, ev.ToState)
	}
}

func TestEdgeSentinel_CgroupOOMPreemption(t *testing.T) {
	ctx := context.Background()
	exec := &mockExecutor{}
	listener := &mockListener{}

	sentinel := NewEdgeSentinel(exec, listener)

	// Snapshot with CgroupMemUsagePct >= 95% and Growth > 0
	snapshot := NodeHardwareSnapshot{
		NodeID:                  "node-oom-01",
		DiskUsagePct:            50.0,
		CgroupMemUsagePct:       96.2,
		CgroupMemGrowthRateMBps: 15.4,
		FrozenSystemdUnits:      nil,
		Timestamp:               time.Now(),
	}

	state, err := sentinel.EvaluateSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state != StateRemediated {
		t.Fatalf("expected state %s, got %s", StateRemediated, state)
	}

	if len(exec.preemptNodes) != 1 || exec.preemptNodes[0] != "node-oom-01" {
		t.Fatalf("expected preempt on node-oom-01, got %v", exec.preemptNodes)
	}
}

func TestEdgeSentinel_FrozenSystemdRestart(t *testing.T) {
	ctx := context.Background()
	exec := &mockExecutor{}
	listener := &mockListener{}

	sentinel := NewEdgeSentinel(exec, listener)

	snapshot := NodeHardwareSnapshot{
		NodeID:             "node-sysd-01",
		DiskUsagePct:       40.0,
		CgroupMemUsagePct:  50.0,
		FrozenSystemdUnits: []string{"kubelet.service", "containerd.service"},
		Timestamp:          time.Now(),
	}

	state, err := sentinel.EvaluateSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state != StateRemediated {
		t.Fatalf("expected state %s, got %s", StateRemediated, state)
	}

	if len(exec.restarted) != 2 {
		t.Fatalf("expected 2 units restarted, got %d (%v)", len(exec.restarted), exec.restarted)
	}
}

func TestEdgeSentinel_RemediationFailure(t *testing.T) {
	ctx := context.Background()
	exec := &mockExecutor{
		pruneErr: errors.New("container engine unresponsive"),
	}
	listener := &mockListener{}

	sentinel := NewEdgeSentinel(exec, listener)

	snapshot := NodeHardwareSnapshot{
		NodeID:       "node-fail-01",
		DiskUsagePct: 95.0,
		Timestamp:    time.Now(),
	}

	state, err := sentinel.EvaluateSnapshot(ctx, snapshot)
	if err == nil {
		t.Fatal("expected remediation error, got nil")
	}

	if state != StateFailed {
		t.Fatalf("expected state %s on failure, got %s", StateFailed, state)
	}

	if listener.Count() != 1 || listener.events[0].ToState != StateFailed {
		t.Fatalf("expected transition to %s, got %v", StateFailed, listener.events)
	}
}

func TestEdgeSentinel_NoiseReductionOver1000Ticks(t *testing.T) {
	ctx := context.Background()
	exec := &mockExecutor{}
	listener := &mockListener{}

	sentinel := NewEdgeSentinel(exec, listener)

	nodeID := "node-steady-01"
	nominalSnapshot := NodeHardwareSnapshot{
		NodeID:                  nodeID,
		DiskUsagePct:            45.0,
		CgroupMemUsagePct:       55.0,
		CgroupMemGrowthRateMBps: 0.0,
		FrozenSystemdUnits:      nil,
		Timestamp:               time.Now(),
	}

	const totalTicks = 1000
	for i := 0; i < totalTicks; i++ {
		_, err := sentinel.EvaluateSnapshot(ctx, nominalSnapshot)
		if err != nil {
			t.Fatalf("unexpected error at tick %d: %v", i, err)
		}
	}

	// Over 1,000 nominal ticks: transitions should be at most 0 or 1, noise reduction > 99%
	emitted := listener.Count()
	reductionPct := (1.0 - float64(emitted)/float64(totalTicks)) * 100.0

	t.Logf("Total ticks: %d, Emitted events: %d, Noise reduction: %.2f%%", totalTicks, emitted, reductionPct)

	if reductionPct < 99.0 {
		t.Fatalf("expected noise reduction >= 99.0%%, got %.2f%% (emitted %d events)", reductionPct, emitted)
	}
}
