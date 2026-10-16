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

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

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
	return m.socketStatus != nil && m.socketStatus[sockPath]
}

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

// ------------------- Phase 7.1 & 8.1: K8s Health & Container Detection Tests -------------------

func TestK8sHealth_CheckHealth_HealthyNode(t *testing.T) {
	prober := &mockRuntimeProber{
		kubeletHealthy: true,
		kubeletLatency: 12.4,
		plegHealthy:    true,
		socketStatus:   map[string]bool{"/run/containerd/containerd.sock": true},
	}
	collector := NewSystemCollector("", "", func(mount string) (int64, int64, int64, int64, error) {
		return 1000, 500, 1000, 500, nil
	})
	checker := NewK8sHealthChecker(collector, WithRuntimeProber(prober))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp := checker.CheckHealth(ctx)
	if !resp.KubeletHealthy || !resp.PLEGHealthy || !resp.ContainerdHealthy {
		t.Fatalf("expected node to be fully healthy, got: %+v", resp)
	}
	if resp.CRISocketPath != "/run/containerd/containerd.sock" {
		t.Errorf("expected CRISocketPath='/run/containerd/containerd.sock', got '%s'", resp.CRISocketPath)
	}
	if len(resp.DetectedAnomalies) != 0 || len(resp.UnhealthyContainers) != 0 {
		t.Errorf("expected 0 anomalies and 0 unhealthy containers, got anomalies=%v, containers=%v", resp.DetectedAnomalies, resp.UnhealthyContainers)
	}
}

func TestK8sHealth_CheckHealth_AnomaliesAndPressure(t *testing.T) {
	tempProc := t.TempDir()
	for pid := 101; pid <= 105; pid++ {
		dir := filepath.Join(tempProc, string([]byte{'1', '0', byte('0' + (pid - 100))}))
		_ = os.MkdirAll(dir, 0755)
		_ = os.WriteFile(filepath.Join(dir, "stat"), []byte("101 (stuck-shim) D 1 1 0 0 0 0 0 0 0 0 0 0 0 0 0"), 0644)
	}
	for _, pidStr := range []string{"201", "202"} {
		dir := filepath.Join(tempProc, pidStr)
		_ = os.MkdirAll(dir, 0755)
		_ = os.WriteFile(filepath.Join(dir, "stat"), []byte(pidStr+" (zombie-app) Z 1 1 0 0 0 0 0 0 0 0 0 0 0 0 0"), 0644)
	}

	prober := &mockRuntimeProber{kubeletHealthy: false, plegHealthy: false}
	collector := NewSystemCollector(tempProc, "", func(mount string) (int64, int64, int64, int64, error) {
		return 1000, 900, 1000, 100, nil
	}, WithProcPath(tempProc))

	checker := NewK8sHealthChecker(collector, WithRuntimeProber(prober), WithK8sProcPath(tempProc))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp := checker.CheckHealth(ctx)
	if resp.KubeletHealthy || resp.ContainerdHealthy || !resp.DiskPressure || !resp.InodePressure {
		t.Fatalf("expected pressure and runtime down, got: %+v", resp)
	}
	if resp.ZombieCount < 2 || resp.DStateProcesses < 5 {
		t.Errorf("expected ZombieCount>=2 and DState>=5, got Z=%d, D=%d", resp.ZombieCount, resp.DStateProcesses)
	}

	anomalyMap := make(map[string]bool)
	for _, a := range resp.DetectedAnomalies {
		anomalyMap[a] = true
	}
	for _, exp := range []string{"KUBELET_UNRESPONSIVE", "CONTAINERD_SOCKET_DOWN", "DISK_PRESSURE_HIGH", "INODE_PRESSURE_HIGH", "D_STATE_PROCESSES_STUCK", "ZOMBIE_PROCESSES_DETECTED"} {
		if !anomalyMap[exp] {
			t.Errorf("missing expected anomaly %s in %v", exp, resp.DetectedAnomalies)
		}
	}
}

func TestK8sHealth_UnhealthyContainersDetection(t *testing.T) {
	runner := newMockCommandRunner()
	runner.outputs["docker ps -a --filter health=unhealthy --format {{.ID}}|{{.Names}}|{{.Image}}|{{.State}}|{{.Status}}"] = []byte("id1|web-api|nginx:alpine|running|unhealthy (failed health check)\n")
	runner.outputs["docker ps -a --filter status=exited --format {{.ID}}|{{.Names}}|{{.Image}}|{{.State}}|{{.Status}}"] = []byte("id2|worker-task|node:18|exited|Exited (137) 2m ago\nid3|job-completed|busybox|exited|Exited (0) 10m ago\nid4|k8s_POD_pod-123|k8s.gcr.io/pause:3.8|exited|Exited (1)\n")

	checker := NewK8sHealthChecker(NewSystemCollector("", "", nil), WithHealthCommandRunner(runner))
	resp := checker.CheckHealth(context.Background())

	if len(resp.UnhealthyContainers) != 2 {
		t.Fatalf("expected exactly 2 unhealthy containers, got %d: %+v", len(resp.UnhealthyContainers), resp.UnhealthyContainers)
	}
	if resp.UnhealthyContainers[0].Name != "web-api" || resp.UnhealthyContainers[1].Name != "worker-task" {
		t.Errorf("unexpected containers: %+v", resp.UnhealthyContainers)
	}
	hasAnomaly := false
	for _, a := range resp.DetectedAnomalies {
		if a == "UNHEALTHY_CONTAINERS_DETECTED" {
			hasAnomaly = true
		}
	}
	if !hasAnomaly {
		t.Errorf("expected UNHEALTHY_CONTAINERS_DETECTED in anomalies, got: %v", resp.DetectedAnomalies)
	}
}

func TestK8sHealth_HTTPHandler(t *testing.T) {
	checker := NewK8sHealthChecker(NewSystemCollector("", "", nil), WithRuntimeProber(&mockRuntimeProber{kubeletHealthy: true, plegHealthy: true}))
	mux := http.NewServeMux()
	checker.RegisterRoutes(mux, func(r *http.Request) bool {
		return r.Header.Get("Authorization") == "Bearer secret-token"
	})

	for _, tc := range []struct {
		name       string
		method     string
		auth       string
		wantStatus int
	}{
		{"Unauthorized", http.MethodGet, "", http.StatusUnauthorized},
		{"MethodNotAllowed", http.MethodPost, "Bearer secret-token", http.StatusMethodNotAllowed},
		{"AuthorizedGET", http.MethodGet, "Bearer secret-token", http.StatusOK},
	} {
		req := httptest.NewRequest(tc.method, "/k8s/health", nil)
		if tc.auth != "" {
			req.Header.Set("Authorization", tc.auth)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != tc.wantStatus {
			t.Errorf("[%s] expected status %d, got %d", tc.name, tc.wantStatus, rec.Code)
		}
	}
}

// ------------------- Phase 7.2 & 8.1: Remediation Engine Tests -------------------

func TestRemediator_StrictAllowlist(t *testing.T) {
	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(WithCommandRunner(runner))

	// Invalid action
	if _, err := remediator.Execute(context.Background(), RemediationRequest{Action: "format_disk"}); err == nil {
		t.Errorf("expected error for unsupported action")
	}

	// Unapproved target
	if _, err := remediator.Execute(context.Background(), RemediationRequest{Action: "restart_service", Target: "random_daemon"}); err == nil {
		t.Errorf("expected error for unapproved target")
	}

	// Shell injection attempt
	if _, err := remediator.Execute(context.Background(), RemediationRequest{Action: "restart_service", Target: "kubelet; rm -rf /"}); err == nil {
		t.Errorf("expected error for shell injection attempt")
	}

	// Valid default targets
	for _, valid := range []string{"kubelet", "nginx", "redis", "postgresql"} {
		res, err := remediator.Execute(context.Background(), RemediationRequest{Action: "restart_service", Target: valid})
		if err != nil || !res.Success {
			t.Errorf("expected service %s to succeed, err=%v", valid, err)
		}
	}
}

func TestRemediator_ServiceAllowlistAndDenylist(t *testing.T) {
	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(WithCommandRunner(runner))

	// 1. Critical OS/SSH hard denylist must always be rejected
	for _, denied := range []string{"sshd", "ssh", "networking", "systemd-networkd", "NetworkManager", "dbus", "udev", "init", "systemd-journald", "systemd-logind"} {
		if remediator.IsServiceAllowed(denied) {
			t.Errorf("denylisted service %s should NOT be allowed", denied)
		}
		res, err := remediator.Execute(context.Background(), RemediationRequest{Action: "restart_service", Target: denied})
		if err == nil || res.Success {
			t.Errorf("restart_service for denylisted %s should have failed", denied)
		}
	}

	// 2. AddAllowedServices dynamically
	remediator.AddAllowedServices([]string{"custom-app", "worker-node@1,metrics-exporter"})
	for _, allowed := range []string{"custom-app", "worker-node@1", "metrics-exporter"} {
		if !remediator.IsServiceAllowed(allowed) {
			t.Errorf("expected dynamically added service %s to be allowed", allowed)
		}
	}

	// 3. Attempting to add denylisted or invalid services must be rejected
	remediator.AddAllowedServices([]string{"sshd", "dbus", "evil;service", "--flag-service", strings.Repeat("a", 70)})
	for _, forbidden := range []string{"sshd", "dbus", "evil;service", "--flag-service"} {
		if remediator.IsServiceAllowed(forbidden) {
			t.Errorf("service %s should NOT be allowed after AddAllowedServices", forbidden)
		}
	}

	// 4. Case-insensitive denylist checks (Sshd, DBUS)
	for _, deniedCase := range []string{"Sshd", "SSHD", "DBUS", "Dbus"} {
		if remediator.IsServiceAllowed(deniedCase) {
			t.Errorf("case-variant denylisted service %s should NOT be allowed", deniedCase)
		}
	}
	remediator.AddAllowedServices([]string{"Sshd", "DBUS"})
	if remediator.IsServiceAllowed("Sshd") || remediator.IsServiceAllowed("DBUS") {
		t.Errorf("case-variant denylisted services should NOT be allowed after AddAllowedServices")
	}

	// 5. Populate res.Status verification
	resOK, errOK := remediator.Execute(context.Background(), RemediationRequest{Action: "restart_container", Target: "my-app-backend"})
	if errOK != nil || resOK.Status != "success" {
		t.Errorf("expected resOK.Status='success', got %q (err=%v)", resOK.Status, errOK)
	}
	resFail, errFail := remediator.Execute(context.Background(), RemediationRequest{Action: "restart_service", Target: "Sshd"})
	if errFail == nil || resFail.Status != "failed" {
		t.Errorf("expected resFail.Status='failed', got %q", resFail.Status)
	}
}

func TestRemediator_RestartContainer(t *testing.T) {
	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(WithCommandRunner(runner))
	ctx := context.Background()

	// 1. Valid container restart
	res, err := remediator.Execute(ctx, RemediationRequest{Action: "restart_container", Target: "my-app-backend"})
	if err != nil || !res.Success {
		t.Fatalf("expected valid container restart to succeed: %v", err)
	}
	if !runner.hasExecuted("docker restart my-app-backend") {
		t.Errorf("expected docker restart my-app-backend to be executed")
	}

	// 2. Invalid container names
	for _, invalid := range []string{"--privileged", "-it", "my/path/traversal", "app;rm -rf /", strings.Repeat("x", 130)} {
		res, err := remediator.Execute(ctx, RemediationRequest{Action: "restart_container", Target: invalid})
		if err == nil || res.Success {
			t.Errorf("expected invalid target %q to be rejected", invalid)
		}
	}

	// 3. Circuit breaker for restart_container (max 3 executions per 30m)
	for i := 2; i <= 3; i++ {
		if _, err := remediator.Execute(ctx, RemediationRequest{Action: "restart_container", Target: "my-app-backend"}); err != nil {
			t.Fatalf("attempt %d failed unexpectedly: %v", i, err)
		}
	}
	// 4th execution should trip circuit breaker
	_, err4 := remediator.Execute(ctx, RemediationRequest{Action: "restart_container", Target: "my-app-backend"})
	if !errors.Is(err4, ErrRemediationCooldown) {
		t.Errorf("expected ErrRemediationCooldown on 4th container restart, got: %v", err4)
	}
}

func TestRemediator_CircuitBreaker_RollingWindow(t *testing.T) {
	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(WithCommandRunner(runner))
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		res, err := remediator.Execute(ctx, RemediationRequest{Action: "restart_service", Target: "containerd"})
		if err != nil || !res.Success {
			t.Fatalf("restart %d failed: err=%v, res=%v", i, err, res)
		}
	}

	res, err := remediator.Execute(ctx, RemediationRequest{Action: "restart_service", Target: "containerd"})
	if !errors.Is(err, ErrRemediationCooldown) {
		t.Fatalf("expected ErrRemediationCooldown on 4th attempt, got: %v (res=%v)", err, res)
	}

	resDiff, errDiff := remediator.Execute(ctx, RemediationRequest{Action: "restart_service", Target: "docker"})
	if errDiff != nil || !resDiff.Success {
		t.Errorf("expected docker target to succeed despite containerd circuit breaker: %v", errDiff)
	}
}

func TestRemediator_DiskPressureCleanup(t *testing.T) {
	tempLogDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempLogDir, "syslog.1"), []byte(strings.Repeat("A", 1024)), 0644)
	_ = os.WriteFile(filepath.Join(tempLogDir, "kern.log.gz"), []byte(strings.Repeat("B", 2048)), 0644)
	_ = os.WriteFile(filepath.Join(tempLogDir, "app.log.old"), []byte(strings.Repeat("C", 512)), 0644)
	activeLog := filepath.Join(tempLogDir, "active.log")
	_ = os.WriteFile(activeLog, []byte("active content"), 0644)

	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(WithCommandRunner(runner), WithRemediatorLogDir(tempLogDir))

	res, err := remediator.Execute(context.Background(), RemediationRequest{Action: "disk_pressure_cleanup"})
	if err != nil || !res.Success {
		t.Fatalf("cleanup failed: %v", err)
	}
	if res.BytesFreed < 3584 || res.InodesFreed != 3 {
		t.Errorf("expected BytesFreed >= 3584 and InodesFreed == 3, got B=%d, I=%d", res.BytesFreed, res.InodesFreed)
	}
	if _, err := os.Stat(activeLog); err != nil {
		t.Errorf("active log was deleted unexpectedly: %v", err)
	}
	if !runner.hasExecuted("crictl rmi --prune") && !runner.hasExecuted("docker system prune -f") {
		t.Errorf("expected container prune command to be invoked")
	}
}

func TestRemediator_CNIIPAMCleanup(t *testing.T) {
	tempCNIDir := t.TempDir()
	tempProcDir := t.TempDir()

	activePIDDir := filepath.Join(tempProcDir, "1234")
	_ = os.MkdirAll(activePIDDir, 0755)
	_ = os.WriteFile(filepath.Join(activePIDDir, "cgroup"), []byte("0::/kubepods.slice/cri-containerd-container-live-active-99.scope\n"), 0644)

	_ = os.WriteFile(filepath.Join(tempCNIDir, "10.244.0.55"), []byte("container-dead-id\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempCNIDir, "10.244.0.56"), []byte(""), 0644)
	_ = os.WriteFile(filepath.Join(tempCNIDir, "lock"), []byte("stale-lock"), 0644)
	activeIPFile := filepath.Join(tempCNIDir, "10.244.0.99")
	_ = os.WriteFile(activeIPFile, []byte("container-live-active-99\n"), 0644)
	reservedIPFile := filepath.Join(tempCNIDir, "last_reserved_ip.0")
	_ = os.WriteFile(reservedIPFile, []byte("10.244.0.56"), 0644)

	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(WithCommandRunner(runner), WithCNINetworksDir(tempCNIDir), WithRemediatorProcPath(tempProcDir))

	res, err := remediator.Execute(context.Background(), RemediationRequest{Action: "cni_ipam_cleanup"})
	if err != nil || !res.Success || res.InodesFreed != 3 {
		t.Fatalf("CNI IPAM cleanup failed or freed unexpected inodes: res=%+v, err=%v", res, err)
	}
	if _, err := os.Stat(activeIPFile); os.IsNotExist(err) {
		t.Errorf("active IP allocation file %s was deleted unexpectedly", activeIPFile)
	}
	if _, err := os.Stat(reservedIPFile); os.IsNotExist(err) {
		t.Errorf("reserved IP metadata file %s was deleted unexpectedly", reservedIPFile)
	}
}

func TestRemediator_ReapZombies(t *testing.T) {
	tempProcDir := t.TempDir()
	for _, f := range []struct{ pid, stat string }{
		{"1001", "1001 (containerd-shim) Z 500 1001 1001 0 -1 4194304\n"},
		{"1002", "1002 (containerd-shim) Z 500 1002 1002 0 -1 4194304\n"},
		{"1003", "1003 (containerd-shim) Z 1 1003 1003 0 -1 4194304\n"},
		{"1004", "1004 (containerd-shim) S 500 1004 1004 0 -1 4194304\n"},
	} {
		_ = os.MkdirAll(filepath.Join(tempProcDir, f.pid), 0755)
		_ = os.WriteFile(filepath.Join(tempProcDir, f.pid, "stat"), []byte(f.stat), 0644)
	}

	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(WithCommandRunner(runner), WithRemediatorProcPath(tempProcDir))

	res, err := remediator.Execute(context.Background(), RemediationRequest{Action: "reap_zombies"})
	if err != nil || !res.Success {
		t.Fatalf("reap_zombies failed: %v", err)
	}
	if !runner.hasExecuted("kill -s CHLD 500") {
		t.Errorf("expected runner to execute 'kill -s CHLD 500' to wake parent subreaper")
	}
	if !runner.hasExecuted("kill -9 1003") {
		t.Errorf("expected runner to execute 'kill -9 1003' for ppid <= 1 fallback")
	}
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

	// 1. Unauthorized
	recUnauth := httptest.NewRecorder()
	mux.ServeHTTP(recUnauth, httptest.NewRequest(http.MethodPost, "/remediate", bytes.NewBufferString(`{"action":"restart_service","target":"kubelet"}`)))
	if recUnauth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", recUnauth.Code)
	}

	// 2. Bad target -> 400
	reqBad := httptest.NewRequest(http.MethodPost, "/remediate", bytes.NewBufferString(`{"action":"restart_service","target":"evil_proc"}`))
	reqBad.Header.Set("Authorization", "Bearer secret-token")
	recBad := httptest.NewRecorder()
	mux.ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", recBad.Code)
	}

	// 3. Valid POST -> 200
	reqGood := httptest.NewRequest(http.MethodPost, "/remediate", bytes.NewBufferString(`{"action":"restart_service","target":"kubelet"}`))
	reqGood.Header.Set("Authorization", "Bearer secret-token")
	recGood := httptest.NewRecorder()
	mux.ServeHTTP(recGood, reqGood)
	if recGood.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", recGood.Code)
	}

	// 4. GET /remediate/history -> 200
	reqHist := httptest.NewRequest(http.MethodGet, "/remediate/history", nil)
	reqHist.Header.Set("Authorization", "Bearer secret-token")
	recHist := httptest.NewRecorder()
	mux.ServeHTTP(recHist, reqHist)
	if recHist.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on history, got %d", recHist.Code)
	}
	var history []RemediationActionResult
	if err := json.Unmarshal(recHist.Body.Bytes(), &history); err != nil || len(history) != 1 {
		t.Fatalf("unexpected history: %v, err=%v", history, err)
	}
}

func TestWatchdog_HookPodTailer(t *testing.T) {
	runner := newMockCommandRunner()
	remediator := NewNodeRemediator(WithCommandRunner(runner))
	collector := NewSystemCollector("", "", nil)
	checker := NewK8sHealthChecker(collector)
	watchdog := NewSelfHealingWatchdog(checker, remediator)

	podTailer := &PodTailer{baseDir: t.TempDir()}
	watchdog.HookPodTailer(podTailer)
	if podTailer.entryWriter == nil {
		t.Fatalf("expected podTailer.entryWriter to be non-nil after HookPodTailer")
	}

	// Trigger entryWriter with a CNI anomaly log line
	_ = podTailer.entryWriter(logengine.Entry{
		Message: "kubelet: FailedCreatePodSandBox: network setup failed",
	})
	if _, exists := watchdog.pendingAnomalies["CNI_SANDBOX_FAILURE"]; !exists {
		t.Fatalf("expected CNI_SANDBOX_FAILURE recorded via hooked pod tailer")
	}
}
