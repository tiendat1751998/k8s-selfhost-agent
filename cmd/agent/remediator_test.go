package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// MockRuntimeProber for deterministic unit testing of K8s health checks
type mockRuntimeProber struct {
	kubeletHealthy bool
	kubeletLatency float64
	plegHealthy    bool
	socketStatus   map[string]bool
}

func (m *mockRuntimeProber) ProbeKubelet(ctx context.Context) (bool, float64, bool) {
	return m.kubeletHealthy, m.kubeletLatency, m.plegHealthy
}

func (m *mockRuntimeProber) ProbeSocket(sockPath string, timeout time.Duration) bool {
	if m.socketStatus == nil {
		return false
	}
	return m.socketStatus[sockPath]
}

// MockCommandRunner for capturing executed commands
type mockCommandRunner struct {
	mu       sync.Mutex
	executed [][]string
	outputs  map[string][]byte
	errs     map[string]error
}

func newMockCommandRunner() *mockCommandRunner {
	return &mockCommandRunner{
		executed: make([][]string, 0),
		outputs:  make(map[string][]byte),
		errs:     make(map[string]error),
	}
}

func (m *mockCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cmd := append([]string{name}, args...)
	m.executed = append(m.executed, cmd)
	key := strings.Join(cmd, " ")
	if err, exists := m.errs[key]; exists {
		return nil, err
	}
	if out, exists := m.outputs[key]; exists {
		return out, nil
	}
	return []byte("success"), nil
}

func (m *mockCommandRunner) hasExecuted(cmdPrefix string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, cmd := range m.executed {
		if strings.HasPrefix(strings.Join(cmd, " "), cmdPrefix) {
			return true
		}
	}
	return false
}

// ------------------- Phase 7.1: K8s Health Diagnostics Tests -------------------

func TestK8sHealth_CheckHealth_HealthyNode(t *testing.T) {
	prober := &mockRuntimeProber{
		kubeletHealthy: true,
		kubeletLatency: 12.4,
		plegHealthy:    true,
		socketStatus: map[string]bool{
			"/run/containerd/containerd.sock": true,
		},
	}

	collector := NewSystemCollector("", "", func(mount string) (int64, int64, int64, int64, error) {
		// 50% usage
		return 1000, 500, 1000, 500, nil
	})

	checker := NewK8sHealthChecker(collector, WithRuntimeProber(prober))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp := checker.CheckHealth(ctx)

	if !resp.KubeletHealthy {
		t.Errorf("expected KubeletHealthy=true, got false")
	}
	if !resp.PLEGHealthy {
		t.Errorf("expected PLEGHealthy=true, got false")
	}
	if !resp.ContainerdHealthy {
		t.Errorf("expected ContainerdHealthy=true, got false")
	}
	if resp.CRISocketPath != "/run/containerd/containerd.sock" {
		t.Errorf("expected CRISocketPath='/run/containerd/containerd.sock', got '%s'", resp.CRISocketPath)
	}
	if resp.DiskPressure {
		t.Errorf("expected DiskPressure=false")
	}
	if resp.InodePressure {
		t.Errorf("expected InodePressure=false")
	}
	if len(resp.DetectedAnomalies) != 0 {
		t.Errorf("expected 0 anomalies, got %v", resp.DetectedAnomalies)
	}
}

func TestK8sHealth_CheckHealth_AnomaliesAndPressure(t *testing.T) {
	tempProc := t.TempDir()
	// Create mock /proc structure with D-state and Zombie processes
	// 5 D-state processes (PIDs 101-105)
	for pid := 101; pid <= 105; pid++ {
		dir := filepath.Join(tempProc, string([]byte{'1', '0', byte('0' + (pid - 100))}))
		_ = os.MkdirAll(dir, 0755)
		_ = os.WriteFile(filepath.Join(dir, "stat"), []byte("101 (stuck-shim) D 1 1 0 0 0 0 0 0 0 0 0 0 0 0 0"), 0644)
	}
	// 2 Zombie processes (PIDs 201-202)
	for _, pidStr := range []string{"201", "202"} {
		dir := filepath.Join(tempProc, pidStr)
		_ = os.MkdirAll(dir, 0755)
		_ = os.WriteFile(filepath.Join(dir, "stat"), []byte(pidStr+" (zombie-app) Z 1 1 0 0 0 0 0 0 0 0 0 0 0 0 0"), 0644)
	}

	prober := &mockRuntimeProber{
		kubeletHealthy: false,
		kubeletLatency: 0,
		plegHealthy:    false,
		socketStatus: map[string]bool{
			"/run/containerd/containerd.sock": false,
		},
	}

	collector := NewSystemCollector(tempProc, "", func(mount string) (int64, int64, int64, int64, error) {
		// 90% disk usage, 90% inode usage
		return 1000, 900, 1000, 100, nil
	}, WithProcPath(tempProc))

	checker := NewK8sHealthChecker(collector, WithRuntimeProber(prober), WithK8sProcPath(tempProc))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp := checker.CheckHealth(ctx)

	if resp.KubeletHealthy {
		t.Errorf("expected KubeletHealthy=false")
	}
	if resp.ContainerdHealthy {
		t.Errorf("expected ContainerdHealthy=false")
	}
	if !resp.DiskPressure {
		t.Errorf("expected DiskPressure=true")
	}
	if !resp.InodePressure {
		t.Errorf("expected InodePressure=true")
	}
	if resp.ZombieCount < 2 {
		t.Errorf("expected ZombieCount >= 2, got %d", resp.ZombieCount)
	}
	if resp.DStateProcesses < 5 {
		t.Errorf("expected DStateProcesses >= 5, got %d", resp.DStateProcesses)
	}

	// Verify anomalies detected
	anomalyMap := make(map[string]bool)
	for _, a := range resp.DetectedAnomalies {
		anomalyMap[a] = true
	}

	expectedAnomalies := []string{
		"KUBELET_UNRESPONSIVE",
		"CONTAINERD_SOCKET_DOWN",
		"DISK_PRESSURE_HIGH",
		"INODE_PRESSURE_HIGH",
		"D_STATE_PROCESSES_STUCK",
		"ZOMBIE_PROCESSES_DETECTED",
	}

	for _, exp := range expectedAnomalies {
		if !anomalyMap[exp] {
			t.Errorf("expected anomaly '%s' in detected_anomalies, but not found in %v", exp, resp.DetectedAnomalies)
		}
	}
}

func TestK8sHealth_HTTPHandler(t *testing.T) {
	prober := &mockRuntimeProber{kubeletHealthy: true, plegHealthy: true}
	collector := NewSystemCollector("", "", nil)
	checker := NewK8sHealthChecker(collector, WithRuntimeProber(prober))

	mux := http.NewServeMux()
	checker.RegisterRoutes(mux, func(r *http.Request) bool {
		return r.Header.Get("Authorization") == "Bearer secret-token"
	})

	// 1. Unauthorized
	req := httptest.NewRequest(http.MethodGet, "/k8s/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
	}

	// 2. Method Not Allowed
	reqPost := httptest.NewRequest(http.MethodPost, "/k8s/health", nil)
	reqPost.Header.Set("Authorization", "Bearer secret-token")
	recPost := httptest.NewRecorder()
	mux.ServeHTTP(recPost, reqPost)
	if recPost.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 MethodNotAllowed, got %d", recPost.Code)
	}

	// 3. Authorized GET
	reqAuth := httptest.NewRequest(http.MethodGet, "/k8s/health", nil)
	reqAuth.Header.Set("Authorization", "Bearer secret-token")
	recAuth := httptest.NewRecorder()
	mux.ServeHTTP(recAuth, reqAuth)

	if recAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", recAuth.Code, recAuth.Body.String())
	}

	var resp NodeK8sHealthResponse
	if err := json.Unmarshal(recAuth.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.KubeletHealthy {
		t.Errorf("expected KubeletHealthy=true in HTTP response")
	}
}

// ------------------- Phase 7.2: Remediation Engine Tests -------------------

func TestRemediator_StrictAllowlist(t *testing.T) {
	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(WithCommandRunner(runner))

	// Invalid action
	res, err := remediator.Execute(context.Background(), RemediationRequest{
		Action: "format_disk",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported action") {
		t.Errorf("expected error for unsupported action, got %v (res=%v)", err, res)
	}

	// Invalid target for restart_service
	res, err = remediator.Execute(context.Background(), RemediationRequest{
		Action: "restart_service",
		Target: "nginx",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported target") {
		t.Errorf("expected error for invalid target, got %v (res=%v)", err, res)
	}

	// Target with shell injection attempt
	res, err = remediator.Execute(context.Background(), RemediationRequest{
		Action: "restart_service",
		Target: "kubelet; rm -rf /",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported target") {
		t.Errorf("expected error for shell injection in target, got %v (res=%v)", err, res)
	}

	// Valid target
	res, err = remediator.Execute(context.Background(), RemediationRequest{
		Action: "restart_service",
		Target: "kubelet",
	})
	if err != nil {
		t.Fatalf("expected valid target to succeed, got %v", err)
	}
	if !res.Success {
		t.Errorf("expected Success=true, got %v", res)
	}
	if !runner.hasExecuted("systemctl restart kubelet") {
		t.Errorf("expected runner to execute systemctl restart kubelet")
	}
}

func TestRemediator_CircuitBreaker_RollingWindow(t *testing.T) {
	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(WithCommandRunner(runner))
	ctx := context.Background()

	// Perform 3 restarts (allowed)
	for i := 1; i <= 3; i++ {
		res, err := remediator.Execute(ctx, RemediationRequest{
			Action: "restart_service",
			Target: "containerd",
		})
		if err != nil || !res.Success {
			t.Fatalf("restart %d failed: err=%v, res=%v", i, err, res)
		}
	}

	// 4th restart within 30 min window must fail with ErrRemediationCooldown
	res, err := remediator.Execute(ctx, RemediationRequest{
		Action: "restart_service",
		Target: "containerd",
	})
	if !errors.Is(err, ErrRemediationCooldown) {
		t.Fatalf("expected ErrRemediationCooldown on 4th attempt, got: %v (res=%v)", err, res)
	}

	// Different target is unaffected
	resDiff, errDiff := remediator.Execute(ctx, RemediationRequest{
		Action: "restart_service",
		Target: "docker",
	})
	if errDiff != nil || !resDiff.Success {
		t.Errorf("expected docker target to succeed despite containerd circuit breaker: %v", errDiff)
	}
}

func TestRemediator_DiskPressureCleanup(t *testing.T) {
	tempLogDir := t.TempDir()
	// Create rotated logs
	_ = os.WriteFile(filepath.Join(tempLogDir, "syslog.1"), []byte(strings.Repeat("A", 1024)), 0644)
	_ = os.WriteFile(filepath.Join(tempLogDir, "kern.log.gz"), []byte(strings.Repeat("B", 2048)), 0644)
	_ = os.WriteFile(filepath.Join(tempLogDir, "app.log.old"), []byte(strings.Repeat("C", 512)), 0644)
	// Active log should NOT be deleted
	activeLog := filepath.Join(tempLogDir, "active.log")
	_ = os.WriteFile(activeLog, []byte("active content"), 0644)

	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(
		WithCommandRunner(runner),
		WithRemediatorLogDir(tempLogDir),
	)

	res, err := remediator.Execute(context.Background(), RemediationRequest{
		Action: "disk_pressure_cleanup",
	})
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected Success=true, got %v", res)
	}
	if res.BytesFreed < 3584 {
		t.Errorf("expected BytesFreed >= 3584, got %d", res.BytesFreed)
	}
	if res.InodesFreed != 3 {
		t.Errorf("expected InodesFreed=3, got %d", res.InodesFreed)
	}

	// Verify active log is still intact
	if _, err := os.Stat(activeLog); err != nil {
		t.Errorf("active log was deleted unexpectedly: %v", err)
	}

	// Verify crictl/docker prune command was called
	if !runner.hasExecuted("crictl rmi --prune") && !runner.hasExecuted("docker system prune -f") {
		t.Errorf("expected container prune command to be invoked")
	}
}

func TestRemediator_CNIIPAMCleanup(t *testing.T) {
	tempCNIDir := t.TempDir()
	tempProcDir := t.TempDir()

	// 1. Create active container cgroup in mock proc
	activePIDDir := filepath.Join(tempProcDir, "1234")
	_ = os.MkdirAll(activePIDDir, 0755)
	_ = os.WriteFile(filepath.Join(activePIDDir, "cgroup"), []byte("0::/kubepods.slice/cri-containerd-container-live-active-99.scope\n"), 0644)

	// 2. Create CNI files:
	// Dead container IP -> should be removed
	_ = os.WriteFile(filepath.Join(tempCNIDir, "10.244.0.55"), []byte("container-dead-id\n"), 0644)
	// Corrupted/empty IP file -> should be removed
	_ = os.WriteFile(filepath.Join(tempCNIDir, "10.244.0.56"), []byte(""), 0644)
	// Stale lock file -> should be removed
	_ = os.WriteFile(filepath.Join(tempCNIDir, "lock"), []byte("stale-lock"), 0644)
	// Active container IP -> MUST BE PRESERVED
	activeIPFile := filepath.Join(tempCNIDir, "10.244.0.99")
	_ = os.WriteFile(activeIPFile, []byte("container-live-active-99\n"), 0644)
	// Non-IP metadata file -> should NOT be deleted
	reservedIPFile := filepath.Join(tempCNIDir, "last_reserved_ip.0")
	_ = os.WriteFile(reservedIPFile, []byte("10.244.0.56"), 0644)

	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(
		WithCommandRunner(runner),
		WithCNINetworksDir(tempCNIDir),
		WithRemediatorProcPath(tempProcDir),
	)

	res, err := remediator.Execute(context.Background(), RemediationRequest{
		Action: "cni_ipam_cleanup",
	})
	if err != nil {
		t.Fatalf("CNI IPAM cleanup failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected Success=true, got %v", res)
	}
	if res.InodesFreed != 3 {
		t.Errorf("expected exactly 3 inodes freed (dead IP, empty IP, lock), got %d", res.InodesFreed)
	}

	// Active IP file MUST be preserved
	if _, err := os.Stat(activeIPFile); os.IsNotExist(err) {
		t.Errorf("active IP allocation file %s was deleted unexpectedly", activeIPFile)
	}
	// Metadata file MUST be preserved
	if _, err := os.Stat(reservedIPFile); os.IsNotExist(err) {
		t.Errorf("reserved IP metadata file %s was deleted unexpectedly", reservedIPFile)
	}
}

func TestRemediator_ReapZombies(t *testing.T) {
	tempProcDir := t.TempDir()

	// Zombie 1001 with parent 500
	_ = os.MkdirAll(filepath.Join(tempProcDir, "1001"), 0755)
	_ = os.WriteFile(filepath.Join(tempProcDir, "1001", "stat"), []byte("1001 (containerd-shim) Z 500 1001 1001 0 -1 4194304\n"), 0644)

	// Zombie 1002 with same parent 500 (verifies deduplication)
	_ = os.MkdirAll(filepath.Join(tempProcDir, "1002"), 0755)
	_ = os.WriteFile(filepath.Join(tempProcDir, "1002", "stat"), []byte("1002 (containerd-shim) Z 500 1002 1002 0 -1 4194304\n"), 0644)

	// Zombie 1003 with parent 1 (init/systemd -> fallback to kill -9)
	_ = os.MkdirAll(filepath.Join(tempProcDir, "1003"), 0755)
	_ = os.WriteFile(filepath.Join(tempProcDir, "1003", "stat"), []byte("1003 (containerd-shim) Z 1 1003 1003 0 -1 4194304\n"), 0644)

	// Process 1004 sleeping (not a zombie -> should not be killed or signaled)
	_ = os.MkdirAll(filepath.Join(tempProcDir, "1004"), 0755)
	_ = os.WriteFile(filepath.Join(tempProcDir, "1004", "stat"), []byte("1004 (containerd-shim) S 500 1004 1004 0 -1 4194304\n"), 0644)

	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(
		WithCommandRunner(runner),
		WithRemediatorProcPath(tempProcDir),
	)

	res, err := remediator.Execute(context.Background(), RemediationRequest{
		Action: "reap_zombies",
	})
	if err != nil {
		t.Fatalf("reap_zombies failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected Success=true, got %v", res)
	}

	// Must signal parent 500 with SIGCHLD
	if !runner.hasExecuted("kill -s CHLD 500") {
		t.Errorf("expected runner to execute 'kill -s CHLD 500' to wake parent subreaper")
	}

	// Must fallback to kill -9 1003 because ppid <= 1
	if !runner.hasExecuted("kill -9 1003") {
		t.Errorf("expected runner to execute 'kill -9 1003' for ppid <= 1 fallback")
	}

	// Must NOT execute kill -9 1001 because ppid > 1
	if runner.hasExecuted("kill -9 1001") {
		t.Errorf("did not expect kill -9 on zombie pid 1001 whose ppid is 500")
	}
}

func TestRemediator_HTTPAndHistory(t *testing.T) {
	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(WithCommandRunner(runner))

	mux := http.NewServeMux()
	remediator.RegisterRoutes(mux, func(r *http.Request) bool {
		return r.Header.Get("Authorization") == "Bearer secret-token"
	})

	// 1. POST /remediate - Unauthorized
	reqUnauth := httptest.NewRequest(http.MethodPost, "/remediate", bytes.NewBufferString(`{"action":"restart_service","target":"kubelet"}`))
	recUnauth := httptest.NewRecorder()
	mux.ServeHTTP(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", recUnauth.Code)
	}

	// 2. POST /remediate - Invalid payload (bad target) -> 400 Bad Request
	reqBad := httptest.NewRequest(http.MethodPost, "/remediate", bytes.NewBufferString(`{"action":"restart_service","target":"evil_proc"}`))
	reqBad.Header.Set("Authorization", "Bearer secret-token")
	recBad := httptest.NewRecorder()
	mux.ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusBadRequest {
		t.Errorf("expected 400 BadRequest for bad target, got %d: %s", recBad.Code, recBad.Body.String())
	}

	// 3. POST /remediate - Valid -> 200 OK
	reqGood := httptest.NewRequest(http.MethodPost, "/remediate", bytes.NewBufferString(`{"action":"restart_service","target":"kubelet","reason":"test"}`))
	reqGood.Header.Set("Authorization", "Bearer secret-token")
	recGood := httptest.NewRecorder()
	mux.ServeHTTP(recGood, reqGood)
	if recGood.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", recGood.Code, recGood.Body.String())
	}

	// 4. GET /remediate/history - 200 OK
	reqHist := httptest.NewRequest(http.MethodGet, "/remediate/history", nil)
	reqHist.Header.Set("Authorization", "Bearer secret-token")
	recHist := httptest.NewRecorder()
	mux.ServeHTTP(recHist, reqHist)

	if recHist.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on history, got %d", recHist.Code)
	}

	var history []RemediationActionResult
	if err := json.Unmarshal(recHist.Body.Bytes(), &history); err != nil {
		t.Fatalf("failed to decode history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].Target != "kubelet" || history[0].Action != "restart_service" {
		t.Errorf("unexpected history entry: %+v", history[0])
	}
}
