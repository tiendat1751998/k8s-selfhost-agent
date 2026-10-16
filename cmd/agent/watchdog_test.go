package main

import (
	"context"
	"testing"
	"time"
)

// ------------------- Watchdog & Anomaly Detection Unit Tests -------------------

func TestWatchdog_DetectLogAnomaly(t *testing.T) {
	tests := []struct {
		line        string
		wantCode    string
		wantMatched bool
	}{
		{"kernel: Out of memory: Killed process 12345 (mysqld)", "OOM_KILL", true},
		{"kubelet: PLEG is not healthy: pleg was last seen active 3m ago", "PLEG_UNHEALTHY", true},
		{"kernel: nf_conntrack: table full, dropping packet", "CONNTRACK_FULL", true},
		{"kubelet: FailedCreatePodSandBox: failed to setup network for sandbox", "CNI_SANDBOX_FAILURE", true},
		{"normal application log info line", "", false},
		{"", "", false},
	}

	for _, tc := range tests {
		code, matched := DetectLogAnomaly(tc.line)
		if matched != tc.wantMatched || code != tc.wantCode {
			t.Errorf("DetectLogAnomaly(%q) = (%q, %v), want (%q, %v)", tc.line, code, matched, tc.wantCode, tc.wantMatched)
		}
	}
}

func TestWatchdog_SelfHealingTrigger_Kubelet(t *testing.T) {
	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(WithCommandRunner(runner))

	// Mock prober that reports kubelet down 3 times
	prober := &mockRuntimeProber{
		kubeletHealthy: false,
		plegHealthy:    false,
	}
	collector := NewSystemCollector("", "", nil)
	checker := NewK8sHealthChecker(collector, WithRuntimeProber(prober))

	watchdog := NewSelfHealingWatchdog(checker, remediator, WithWatchdogInterval(10*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	// Run check iteration manually
	watchdog.runOnce(ctx) // check 1: consecutive = 1
	watchdog.runOnce(ctx) // check 2: consecutive = 2
	watchdog.runOnce(ctx) // check 3: consecutive = 3 -> triggers restart_service(kubelet)

	if !runner.hasExecuted("systemctl restart kubelet") {
		t.Errorf("expected watchdog to trigger systemctl restart kubelet on 3 consecutive failures")
	}
}

func TestWatchdog_RestartContainerdOnFailure(t *testing.T) {
	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(WithCommandRunner(runner))

	// Mock prober that reports containerd unhealthy but kubelet healthy
	prober := &mockRuntimeProber{
		kubeletHealthy: true,
		plegHealthy:    true,
		socketStatus: map[string]bool{
			"/run/containerd/containerd.sock": false,
		},
	}
	collector := NewSystemCollector("", "", nil)
	checker := NewK8sHealthChecker(collector, WithRuntimeProber(prober))

	watchdog := NewSelfHealingWatchdog(checker, remediator, WithWatchdogInterval(10*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	// Trigger 3 consecutive failures for containerd
	watchdog.runOnce(ctx) // 1
	watchdog.runOnce(ctx) // 2
	watchdog.runOnce(ctx) // 3 -> should trigger containerd restart

	if !runner.hasExecuted("systemctl restart containerd") {
		t.Errorf("expected watchdog to trigger systemctl restart containerd on 3 consecutive CRI failures")
	}
}
