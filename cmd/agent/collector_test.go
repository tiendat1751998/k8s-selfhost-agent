package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectCPU(t *testing.T) {
	tempDir := t.TempDir()
	statFile := filepath.Join(tempDir, "stat")

	initialStat := `cpu  10000 2000 3000 50000 1000 500 200 0 0 0
cpu0 5000 1000 1500 25000 500 250 100 0 0 0
cpu1 5000 1000 1500 25000 500 250 100 0 0 0
`
	if err := os.WriteFile(statFile, []byte(initialStat), 0644); err != nil {
		t.Fatalf("failed to write initial stat fixture: %v", err)
	}

	collector := NewSystemCollector(tempDir, "", nil)

	// First pass
	resp1, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collect pass 1 failed: %v", err)
	}
	if resp1.CPU.Count != 2 {
		t.Errorf("expected CPU count 2, got %d", resp1.CPU.Count)
	}

	// Updated stat fixture
	updatedStat := `cpu  12000 2500 3500 51000 1000 600 200 0 0 0
cpu0 6000 1250 1750 25500 500 300 100 0 0 0
cpu1 6000 1250 1750 25500 500 300 100 0 0 0
`
	if err := os.WriteFile(statFile, []byte(updatedStat), 0644); err != nil {
		t.Fatalf("failed to write updated stat fixture: %v", err)
	}

	// Second pass
	resp2, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collect pass 2 failed: %v", err)
	}
	if resp2.CPU.Count != 2 {
		t.Errorf("expected CPU count 2, got %d", resp2.CPU.Count)
	}
	if resp2.CPU.UsagePercent <= 0.0 || resp2.CPU.UsagePercent > 100.0 {
		t.Errorf("expected positive valid CPU usage percent, got %f", resp2.CPU.UsagePercent)
	}
}

func TestCollectMemory(t *testing.T) {
	tempDir := t.TempDir()
	memFile := filepath.Join(tempDir, "meminfo")

	memContent := `MemTotal:       16384000 kB
MemFree:         4000000 kB
MemAvailable:    8000000 kB
Buffers:          500000 kB
Cached:          3500000 kB
`
	if err := os.WriteFile(memFile, []byte(memContent), 0644); err != nil {
		t.Fatalf("failed to write meminfo fixture: %v", err)
	}

	collector := NewSystemCollector(tempDir, "", nil)
	resp, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	expectedTotal := int64(16384000) * 1024
	expectedAvail := int64(8000000) * 1024
	expectedUsed := expectedTotal - expectedAvail

	if resp.Memory.TotalBytes != expectedTotal {
		t.Errorf("expected TotalBytes %d, got %d", expectedTotal, resp.Memory.TotalBytes)
	}
	if resp.Memory.AvailableBytes != expectedAvail {
		t.Errorf("expected AvailableBytes %d, got %d", expectedAvail, resp.Memory.AvailableBytes)
	}
	if resp.Memory.UsedBytes != expectedUsed {
		t.Errorf("expected UsedBytes %d, got %d", expectedUsed, resp.Memory.UsedBytes)
	}
	if resp.Memory.UsagePercent <= 0 {
		t.Errorf("expected positive UsagePercent, got %f", resp.Memory.UsagePercent)
	}
}

func TestCollectDisk(t *testing.T) {
	tempDir := t.TempDir()
	mountsFile := filepath.Join(tempDir, "mounts")

	mountsContent := `/dev/sda1 / ext4 rw,relatime 0 0
/dev/sdb1 /data xfs rw,relatime 0 0
proc /proc proc rw 0 0
tmpfs /run tmpfs rw 0 0
`
	if err := os.WriteFile(mountsFile, []byte(mountsContent), 0644); err != nil {
		t.Fatalf("failed to write mounts fixture: %v", err)
	}

	mockDiskFn := func(mountPoint string) (int64, int64, error) {
		switch mountPoint {
		case "/":
			return 100 * 1024 * 1024 * 1024, 40 * 1024 * 1024 * 1024, nil
		case "/data":
			return 200 * 1024 * 1024 * 1024, 150 * 1024 * 1024 * 1024, nil
		default:
			return 0, 0, nil
		}
	}

	collector := NewSystemCollector(tempDir, mountsFile, mockDiskFn)
	resp, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if len(resp.Disks) != 2 {
		t.Fatalf("expected 2 disks, got %d: %+v", len(resp.Disks), resp.Disks)
	}

	disk0 := resp.Disks[0]
	if disk0.MountPoint != "/" || disk0.Filesystem != "ext4" || disk0.UsagePercent != 40.0 {
		t.Errorf("unexpected disk 0: %+v", disk0)
	}

	disk1 := resp.Disks[1]
	if disk1.MountPoint != "/data" || disk1.Filesystem != "xfs" || disk1.UsagePercent != 75.0 {
		t.Errorf("unexpected disk 1: %+v", disk1)
	}
}

func TestCollectDisk_FilterAndDeduplicate(t *testing.T) {
	tempDir := t.TempDir()
	mountsFile := filepath.Join(tempDir, "mounts")

	mountsContent := `/dev/sda1 / ext4 rw,relatime 0 0
/dev/sda1 /mnt/bind_root ext4 rw,relatime 0 0
/dev/sdb1 /boot ext4 rw,relatime 0 0
overlay /var/lib/docker/overlay2/abc/merged overlay rw 0 0
overlay /var/lib/docker/overlay2/def/merged overlay rw 0 0
tmpfs /dev/shm tmpfs rw 0 0
squashfs /snap/core/123 squashfs ro 0 0
fuse.snapfuse /snap/bare/5 fuse.snapfuse ro 0 0
/dev/sdc1 /data xfs rw,relatime 0 0
`
	if err := os.WriteFile(mountsFile, []byte(mountsContent), 0644); err != nil {
		t.Fatalf("failed to write mounts fixture: %v", err)
	}

	mockDiskFn := func(mountPoint string) (int64, int64, error) {
		switch mountPoint {
		case "/":
			return 102971269120, 31782694912, nil
		case "/mnt/bind_root":
			return 102971269120, 31782694912, nil
		case "/boot":
			return 2040373248, 204037324, nil
		case "/data":
			return 200 * 1024 * 1024 * 1024, 50 * 1024 * 1024 * 1024, nil
		case "/var/lib/docker/overlay2/abc/merged":
			return 102971269120, 31782694912, nil
		default:
			return 0, 0, nil
		}
	}

	collector := NewSystemCollector(tempDir, mountsFile, mockDiskFn)
	resp, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	// Should only have 3 disks: "/", "/boot", and "/data" (overlay, tmpfs, snap, and duplicate /mnt/bind_root filtered)
	if len(resp.Disks) != 3 {
		t.Fatalf("expected 3 disks, got %d: %+v", len(resp.Disks), resp.Disks)
	}

	expectedMounts := map[string]string{
		"/":     "ext4",
		"/boot": "ext4",
		"/data": "xfs",
	}

	for _, d := range resp.Disks {
		fs, ok := expectedMounts[d.MountPoint]
		if !ok {
			t.Errorf("unexpected mount point kept: %s", d.MountPoint)
		} else if d.Filesystem != fs {
			t.Errorf("for mount point %s expected fs %s, got %s", d.MountPoint, fs, d.Filesystem)
		}
	}
}

func TestCollectNetwork(t *testing.T) {
	tempDir := t.TempDir()
	netDir := filepath.Join(tempDir, "net")
	if err := os.MkdirAll(netDir, 0755); err != nil {
		t.Fatalf("failed to create net dir: %v", err)
	}
	devFile := filepath.Join(netDir, "dev")

	initialDev := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  eth0: 10000000    100    0    0    0     0          0         0  5000000     50    0    0    0     0       0          0
  eth1:  2000000     20    0    0    0     0          0         0  1000000     10    0    0    0     0       0          0
`
	if err := os.WriteFile(devFile, []byte(initialDev), 0644); err != nil {
		t.Fatalf("failed to write initial net/dev: %v", err)
	}

	collector := NewSystemCollector(tempDir, "", nil)

	// Pass 1: sets baseline
	_, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collect pass 1 failed: %v", err)
	}

	updatedDev := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  eth0: 11024000    110    0    0    0     0          0         0  5512000     60    0    0    0     0       0          0
  eth1:  2100000     25    0    0    0     0          0         0  1050000     15    0    0    0     0       0          0
`
	if err := os.WriteFile(devFile, []byte(updatedDev), 0644); err != nil {
		t.Fatalf("failed to write updated net/dev: %v", err)
	}

	// Override timestamp to simulate 1 second elapsed
	collector.prevNetTime = time.Now().UTC().Add(-1 * time.Second)

	resp, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collect pass 2 failed: %v", err)
	}

	if len(resp.Network.Interfaces) != 2 {
		t.Fatalf("expected 2 network interfaces, got %d", len(resp.Network.Interfaces))
	}

	var foundEth0 bool
	for _, iface := range resp.Network.Interfaces {
		if iface.Name == "eth0" {
			foundEth0 = true
			if iface.RxBytesPerSec <= 0 || iface.TxBytesPerSec <= 0 {
				t.Errorf("expected positive rate for eth0, got rx=%d, tx=%d", iface.RxBytesPerSec, iface.TxBytesPerSec)
			}
		}
	}

	if !foundEth0 {
		t.Errorf("eth0 interface not found in network metrics")
	}
	if resp.Network.TotalRxBytesPerSec <= 0 || resp.Network.TotalTxBytesPerSec <= 0 {
		t.Errorf("expected positive total network rates, got rx=%d, tx=%d", resp.Network.TotalRxBytesPerSec, resp.Network.TotalTxBytesPerSec)
	}
}

func TestReadTopProcesses_ProcFixtures(t *testing.T) {
	tempDir := t.TempDir()

	// Initial system stat
	statFile := filepath.Join(tempDir, "stat")
	initialStat := `cpu  10000 2000 3000 50000 1000 500 200 0 0 0
cpu0 5000 1000 1500 25000 500 250 100 0 0 0
cpu1 5000 1000 1500 25000 500 250 100 0 0 0
`
	if err := os.WriteFile(statFile, []byte(initialStat), 0644); err != nil {
		t.Fatalf("failed to write initial stat: %v", err)
	}

	// PID 1 (systemd)
	p1Dir := filepath.Join(tempDir, "1")
	_ = os.MkdirAll(p1Dir, 0755)
	_ = os.WriteFile(filepath.Join(p1Dir, "stat"), []byte("1 (systemd) S 0 1 1 0 -1 4194304 1000 0 0 0 10 5 0 0 20 0 1 0 100 1000 2560 0 0 0 0 0 0 0 0 0 0 0 0 0 0"), 0644)
	_ = os.WriteFile(filepath.Join(p1Dir, "cmdline"), []byte("/sbin/init\x00"), 0644)
	_ = os.WriteFile(filepath.Join(p1Dir, "status"), []byte("Name:\tsystemd\nState:\tS (sleeping)\nUid:\t0\t0\t0\t0\nVmRSS:\t10240 kB\n"), 0644)
	_ = os.WriteFile(filepath.Join(p1Dir, "io"), []byte("read_bytes: 1000\nwrite_bytes: 2000\n"), 0644)

	// PID 101 (nginx)
	p101Dir := filepath.Join(tempDir, "101")
	_ = os.MkdirAll(p101Dir, 0755)
	_ = os.WriteFile(filepath.Join(p101Dir, "stat"), []byte("101 (nginx) R 1 101 101 0 -1 4194304 1000 0 0 0 100 50 0 0 20 0 1 0 100 1000 12800 0 0 0 0 0 0 0 0 0 0 0 0 0 0"), 0644)
	_ = os.WriteFile(filepath.Join(p101Dir, "cmdline"), []byte("nginx\x00-g\x00daemon off;\x00"), 0644)
	_ = os.WriteFile(filepath.Join(p101Dir, "status"), []byte("Name:\tnginx\nState:\tR (running)\nUid:\t1000\t1000\t1000\t1000\nVmRSS:\t51200 kB\n"), 0644)
	_ = os.WriteFile(filepath.Join(p101Dir, "io"), []byte("read_bytes: 50000\nwrite_bytes: 30000\n"), 0644)

	// PID 202 (postgres)
	p202Dir := filepath.Join(tempDir, "202")
	_ = os.MkdirAll(p202Dir, 0755)
	_ = os.WriteFile(filepath.Join(p202Dir, "stat"), []byte("202 (postgres) S 1 202 202 0 -1 4194304 1000 0 0 0 50 25 0 0 20 0 1 0 100 1000 25600 0 0 0 0 0 0 0 0 0 0 0 0 0 0"), 0644)
	_ = os.WriteFile(filepath.Join(p202Dir, "cmdline"), []byte("postgres: writer\x00"), 0644)
	_ = os.WriteFile(filepath.Join(p202Dir, "status"), []byte("Name:\tpostgres\nState:\tS (sleeping)\nUid:\t1001\t1001\t1001\t1001\nVmRSS:\t102400 kB\n"), 0644)
	_ = os.WriteFile(filepath.Join(p202Dir, "io"), []byte("read_bytes: 20000\nwrite_bytes: 10000\n"), 0644)

	collector := NewSystemCollector(tempDir, "", nil)

	// Pass 1: Baseline
	resp1, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collect pass 1 failed: %v", err)
	}

	if len(resp1.TopProcesses) != 3 {
		t.Fatalf("expected 3 top processes in pass 1, got %d", len(resp1.TopProcesses))
	}

	// In pass 1 (no CPU delta yet), processes should be sorted by MemoryBytes desc (PID 202 (100MB), PID 101 (50MB), PID 1 (10MB))
	if resp1.TopProcesses[0].PID != 202 {
		t.Errorf("expected heaviest mem process PID 202 first in pass 1, got %d", resp1.TopProcesses[0].PID)
	}
	if resp1.TopProcesses[0].MemoryBytes != 102400*1024 {
		t.Errorf("expected 102400 kB for PID 202, got %d", resp1.TopProcesses[0].MemoryBytes)
	}
	if resp1.TopProcesses[0].State != "sleeping" {
		t.Errorf("expected state 'sleeping' for PID 202, got '%s'", resp1.TopProcesses[0].State)
	}

	// Update fixture for pass 2
	updatedStat := `cpu  12000 2500 3500 51000 1000 600 200 0 0 0
cpu0 6000 1250 1750 25500 500 300 100 0 0 0
cpu1 6000 1250 1750 25500 500 300 100 0 0 0
`
	_ = os.WriteFile(statFile, []byte(updatedStat), 0644)
	_ = os.WriteFile(filepath.Join(p101Dir, "stat"), []byte("101 (nginx) R 1 101 101 0 -1 4194304 1000 0 0 0 300 150 0 0 20 0 1 0 100 1000 12800 0 0 0 0 0 0 0 0 0 0 0 0 0 0"), 0644)
	_ = os.WriteFile(filepath.Join(p101Dir, "io"), []byte("read_bytes: 150000\nwrite_bytes: 80000\n"), 0644)

	collector.prevProcTime = time.Now().UTC().Add(-1 * time.Second)

	// Pass 2
	resp2, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collect pass 2 failed: %v", err)
	}

	if len(resp2.TopProcesses) != 3 {
		t.Fatalf("expected 3 top processes in pass 2, got %d", len(resp2.TopProcesses))
	}

	// In pass 2, PID 101 had major CPU increase and I/O increase, should be #1 by CPUPercent
	p0 := resp2.TopProcesses[0]
	if p0.PID != 101 {
		t.Errorf("expected PID 101 to be #1 top process, got PID %d (%s)", p0.PID, p0.Name)
	}
	if p0.Name != "nginx" {
		t.Errorf("expected name 'nginx', got '%s'", p0.Name)
	}
	if p0.CommandLine != "nginx -g daemon off;" {
		t.Errorf("expected command line 'nginx -g daemon off;', got '%s'", p0.CommandLine)
	}
	if p0.CPUPercent <= 0 {
		t.Errorf("expected positive CPUPercent for PID 101, got %f", p0.CPUPercent)
	}
	if p0.ReadBytesPerSec <= 0 || p0.WriteBytesPerSec <= 0 {
		t.Errorf("expected positive IO rates for PID 101, got read=%d write=%d", p0.ReadBytesPerSec, p0.WriteBytesPerSec)
	}
	if p0.State != "running" {
		t.Errorf("expected state 'running' for PID 101, got '%s'", p0.State)
	}
}

func TestReadTopProcesses_Fallback(t *testing.T) {
	collector := NewSystemCollector("/non/existent/path", "", nil)
	topProcs := collector.readTopProcesses(5, 16*1024*1024*1024)

	if len(topProcs) != 5 {
		t.Fatalf("expected 5 fallback processes, got %d", len(topProcs))
	}

	for _, p := range topProcs {
		if p.PID <= 0 {
			t.Errorf("expected positive PID, got %d", p.PID)
		}
		if p.Name == "" {
			t.Errorf("expected non-empty process name")
		}
		if p.State == "" {
			t.Errorf("expected non-empty process state")
		}
		if p.MemoryBytes <= 0 {
			t.Errorf("expected positive MemoryBytes")
		}
		if p.MemoryPercent <= 0 {
			t.Errorf("expected positive MemoryPercent")
		}
	}
}

func TestMetricsEndpoint_ReturnsJSON(t *testing.T) {
	collector := NewSystemCollector("", "", nil)
	handler := setupHandler(collector, "")

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp MetricsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	if resp.Hostname == "" {
		t.Errorf("expected non-empty hostname")
	}
	if resp.OS == "" {
		t.Errorf("expected non-empty OS")
	}
	if resp.Arch == "" {
		t.Errorf("expected non-empty Arch")
	}
	if len(resp.TopProcesses) == 0 {
		t.Errorf("expected top_processes to be populated in metrics response")
	}
}

func TestHealthEndpoint(t *testing.T) {
	collector := NewSystemCollector("", "", nil)
	handler := setupHandler(collector, "")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode health JSON: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.Status)
	}
}

func TestAuthToken_Required(t *testing.T) {
	collector := NewSystemCollector("", "", nil)
	handler := setupHandler(collector, "secret-token-123")

	// Missing header
	{
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401 for missing token, got %d", rec.Code)
		}
	}

	// Invalid token
	{
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401 for invalid token, got %d", rec.Code)
		}
	}

	// Health endpoint should remain unauthenticated
	{
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 for health endpoint without auth, got %d", rec.Code)
		}
	}
}

func TestAuthToken_Valid(t *testing.T) {
	collector := NewSystemCollector("", "", nil)
	handler := setupHandler(collector, "secret-token-123")

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer secret-token-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for valid token, got %d", rec.Code)
	}

	var resp MetricsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if resp.Hostname == "" {
		t.Errorf("expected non-empty hostname")
	}
	if len(resp.TopProcesses) == 0 {
		t.Errorf("expected top_processes in authenticated response")
	}
}

func TestCollectOSInfo_Fixtures(t *testing.T) {
	tempDir := t.TempDir()

	// 1. os-release fixture
	osReleaseFile := filepath.Join(tempDir, "os-release")
	osReleaseContent := `NAME="Ubuntu"
VERSION="22.04.3 LTS (Jammy Jellyfish)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 22.04.3 LTS"
VERSION_ID="22.04"
`
	if err := os.WriteFile(osReleaseFile, []byte(osReleaseContent), 0644); err != nil {
		t.Fatalf("failed to write os-release fixture: %v", err)
	}

	// 2. sys/kernel/osrelease fixture
	sysKernelDir := filepath.Join(tempDir, "sys", "kernel")
	if err := os.MkdirAll(sysKernelDir, 0755); err != nil {
		t.Fatalf("failed to create sys/kernel dir: %v", err)
	}
	osReleaseProc := filepath.Join(sysKernelDir, "osrelease")
	if err := os.WriteFile(osReleaseProc, []byte("5.15.0-91-generic\n"), 0644); err != nil {
		t.Fatalf("failed to write kernel osrelease fixture: %v", err)
	}

	collector := NewSystemCollector(tempDir, "", nil, WithOSReleasePath(osReleaseFile))
	resp, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if resp.OSDistro != "Ubuntu 22.04.3 LTS" {
		t.Errorf("expected OSDistro 'Ubuntu 22.04.3 LTS', got '%s'", resp.OSDistro)
	}
	if resp.KernelVersion != "5.15.0-91-generic" {
		t.Errorf("expected KernelVersion '5.15.0-91-generic', got '%s'", resp.KernelVersion)
	}
}

func TestCollectOSInfo_ProcVersionFallback(t *testing.T) {
	tempDir := t.TempDir()

	osReleaseFile := filepath.Join(tempDir, "os-release")
	osReleaseContent := `NAME="Debian GNU/Linux"
VERSION_ID="12"
VERSION="12 (bookworm)"
`
	if err := os.WriteFile(osReleaseFile, []byte(osReleaseContent), 0644); err != nil {
		t.Fatalf("failed to write os-release fixture: %v", err)
	}

	versionProc := filepath.Join(tempDir, "version")
	versionContent := "Linux version 6.1.0-18-amd64 (debian-kernel@lists.debian.org) (gcc-12 (Debian 12.2.0-14) 12.2.0, GNU ld (GNU Binutils for Debian) 2.40) #1 SMP PREEMPT_DYNAMIC Debian 6.1.76-1 (2024-02-01)\n"
	if err := os.WriteFile(versionProc, []byte(versionContent), 0644); err != nil {
		t.Fatalf("failed to write version fixture: %v", err)
	}

	collector := NewSystemCollector(tempDir, "", nil, WithOSReleasePath(osReleaseFile))
	resp, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if resp.OSDistro != "Debian GNU/Linux 12 (bookworm)" {
		t.Errorf("expected OSDistro 'Debian GNU/Linux 12 (bookworm)', got '%s'", resp.OSDistro)
	}
	if resp.KernelVersion != "6.1.0-18-amd64" {
		t.Errorf("expected KernelVersion '6.1.0-18-amd64', got '%s'", resp.KernelVersion)
	}
}

