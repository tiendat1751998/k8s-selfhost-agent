package main

import (
	"context"
	"os"
	"path/filepath"
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

func TestWatchdog_RecordLogLine(t *testing.T) {
	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(WithCommandRunner(runner))
	collector := NewSystemCollector("", "", nil)
	checker := NewK8sHealthChecker(collector)
	watchdog := NewSelfHealingWatchdog(checker, remediator)

	watchdog.RecordLogLine("kubelet: FailedCreatePodSandBox: network setup failed")
	if _, exists := watchdog.pendingAnomalies["CNI_SANDBOX_FAILURE"]; !exists {
		t.Errorf("expected CNI_SANDBOX_FAILURE to be recorded in pendingAnomalies")
	}

	watchdog.RecordLogLine("random info message")
	if len(watchdog.pendingAnomalies) != 1 {
		t.Errorf("expected only 1 pending anomaly, got %d", len(watchdog.pendingAnomalies))
	}
}

func TestWatchdog_Rule3_ZombieReap(t *testing.T) {
	tempProc := t.TempDir()
	// Create a zombie process
	pidDir := filepath.Join(tempProc, "201")
	_ = os.MkdirAll(pidDir, 0755)
	_ = os.WriteFile(filepath.Join(pidDir, "stat"), []byte("201 (containerd-shim) Z 1 1 0 0 0 0 0 0 0 0 0 0 0 0 0"), 0644)

	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(WithCommandRunner(runner), WithRemediatorProcPath(tempProc))
	collector := NewSystemCollector("", "", nil)
	checker := NewK8sHealthChecker(collector, WithK8sProcPath(tempProc))

	watchdog := NewSelfHealingWatchdog(checker, remediator)
	ctx := context.Background()

	// First execution should trigger reap_zombies
	watchdog.runOnce(ctx)

	if !runner.hasExecuted("kill -9 201") {
		t.Errorf("expected watchdog to trigger reap_zombies terminating zombie pid 201")
	}

	// Immediate second execution must be in 10-minute cooldown
	runner.executed = make([][]string, 0)
	watchdog.runOnce(ctx)
	if runner.hasExecuted("kill -9 201") {
		t.Errorf("expected zombie reap to be throttled by 10m cooldown on second run")
	}
}

func TestWatchdog_Rule4_CNIIPAMCleanup(t *testing.T) {
	tempCNI := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempCNI, "lock"), []byte("stale"), 0644)

	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(WithCommandRunner(runner), WithCNINetworksDir(tempCNI))
	collector := NewSystemCollector("", "", nil)
	checker := NewK8sHealthChecker(collector)

	watchdog := NewSelfHealingWatchdog(checker, remediator)
	ctx := context.Background()

	// Record anomaly
	watchdog.RecordLogLine("kubelet: FailedCreatePodSandBox: failed network")

	// Trigger runOnce
	watchdog.runOnce(ctx)

	// Verify lock file was removed by cni_ipam_cleanup
	if _, err := os.Stat(filepath.Join(tempCNI, "lock")); !os.IsNotExist(err) {
		t.Errorf("expected CNI lock file to be cleaned up after CNI_SANDBOX_FAILURE anomaly")
	}

	// Verify anomaly was cleared
	if _, exists := watchdog.pendingAnomalies["CNI_SANDBOX_FAILURE"]; exists {
		t.Errorf("expected CNI_SANDBOX_FAILURE anomaly to be cleared after cleanup")
	}
}

func TestWatchdog_Rule5_RestartUnhealthyContainers(t *testing.T) {
	runner := newMockCommandRunner()
	// Mock docker ps output: 3 unhealthy containers
	runner.outputs["docker ps -a --filter health=unhealthy --format {{.ID}}|{{.Names}}|{{.Image}}|{{.State}}|{{.Status}}"] = []byte("c1|app-1|repo/app:v1|running|unhealthy\nc2|app-2|repo/app:v2|running|unhealthy\nc3|app-3|repo/app:v3|running|unhealthy\n")

	remediator := NewNodeRemediator(WithCommandRunner(runner))
	collector := NewSystemCollector("", "", nil)
	checker := NewK8sHealthChecker(collector, WithHealthCommandRunner(runner))

	watchdog := NewSelfHealingWatchdog(checker, remediator)
	ctx := context.Background()

	watchdog.runOnce(ctx)

	// Rule 5 caps at max 2 containers per tick
	if !runner.hasExecuted("docker restart app-1") {
		t.Errorf("expected docker restart app-1 to be executed")
	}
	if !runner.hasExecuted("docker restart app-2") {
		t.Errorf("expected docker restart app-2 to be executed")
	}
	if runner.hasExecuted("docker restart app-3") {
		t.Errorf("expected app-3 NOT to be restarted on first tick due to max 2 cap")
	}
}

