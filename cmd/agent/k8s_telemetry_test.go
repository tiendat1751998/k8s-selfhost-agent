package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

func TestDiskInodes_Collection(t *testing.T) {
	c := NewSystemCollector("", "", nil)
	disks := c.collectDisks()
	require.NotEmpty(t, disks)

	d := disks[0]
	assert.GreaterOrEqual(t, d.InodesTotal, int64(0), "InodesTotal should be >= 0")
	assert.GreaterOrEqual(t, d.InodesFree, int64(0), "InodesFree should be >= 0")
	assert.GreaterOrEqual(t, d.InodesUsed, int64(0), "InodesUsed should be >= 0")
	assert.GreaterOrEqual(t, d.InodesUsagePercent, float64(0), "InodesUsagePercent should be >= 0")
}

func TestNetworkErrorsAndConntrack(t *testing.T) {
	tmpDir := t.TempDir()
	procNetDir := filepath.Join(tmpDir, "net")
	require.NoError(t, os.MkdirAll(procNetDir, 0755))

	// Fixture with eth0 having errors and drops
	devContent := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  eth0: 1000000    1000   12   34    0     0          0         0  2000000    2000   56   78    0     0       0          0
    lo:  500000     500    0    0    0     0          0         0   500000     500    0    0    0     0       0          0
`
	require.NoError(t, os.WriteFile(filepath.Join(procNetDir, "dev"), []byte(devContent), 0644))

	// Fixture for conntrack
	conntrackDir := filepath.Join(tmpDir, "sys", "net", "netfilter")
	require.NoError(t, os.MkdirAll(conntrackDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(conntrackDir, "nf_conntrack_count"), []byte("12345\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(conntrackDir, "nf_conntrack_max"), []byte("262144\n"), 0644))

	c := NewSystemCollector(tmpDir, "", nil)
	netMetrics := c.collectNetwork(time.Now())

	require.NotEmpty(t, netMetrics.Interfaces)
	var eth0 *NetworkInterfaceMetrics
	for i := range netMetrics.Interfaces {
		if netMetrics.Interfaces[i].Name == "eth0" {
			eth0 = &netMetrics.Interfaces[i]
			break
		}
	}
	require.NotNil(t, eth0)
	assert.Equal(t, int64(12), eth0.RxErrors)
	assert.Equal(t, int64(34), eth0.RxDrops)
	assert.Equal(t, int64(56), eth0.TxErrors)
	assert.Equal(t, int64(78), eth0.TxDrops)

	assert.Equal(t, int64(12345), netMetrics.ConntrackCount)
	assert.Equal(t, int64(262144), netMetrics.ConntrackMax)
}

func TestLinuxPSI_Parsing(t *testing.T) {
	tmpDir := t.TempDir()
	pressureDir := filepath.Join(tmpDir, "pressure")
	require.NoError(t, os.MkdirAll(pressureDir, 0755))

	cpuPSI := "some avg10=1.23 avg60=4.56 avg300=7.89 total=1234567\n"
	memPSI := "some avg10=2.34 avg60=5.67 avg300=8.90 total=2345678\nfull avg10=0.12 avg60=0.34 avg300=0.56 total=34567\n"
	ioPSI := "some avg10=3.45 avg60=6.78 avg300=9.01 total=3456789\nfull avg10=1.11 avg60=2.22 avg300=3.33 total=45678\n"

	require.NoError(t, os.WriteFile(filepath.Join(pressureDir, "cpu"), []byte(cpuPSI), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(pressureDir, "memory"), []byte(memPSI), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(pressureDir, "io"), []byte(ioPSI), 0644))

	c := NewSystemCollector(tmpDir, "", nil)
	psi := c.collectPSI()

	assert.InDelta(t, 1.23, psi.CPU.Avg10, 0.001)
	assert.InDelta(t, 4.56, psi.CPU.Avg60, 0.001)
	assert.Equal(t, uint64(1234567), psi.CPU.TotalMicros)

	assert.InDelta(t, 2.34, psi.Memory.Avg10, 0.001)
	assert.InDelta(t, 5.67, psi.Memory.Avg60, 0.001)
	assert.Equal(t, uint64(2345678), psi.Memory.TotalMicros)

	assert.InDelta(t, 3.45, psi.IO.Avg10, 0.001)
	assert.InDelta(t, 6.78, psi.IO.Avg60, 0.001)
	assert.Equal(t, uint64(3456789), psi.IO.TotalMicros)
}

func TestK8sNodeCounters(t *testing.T) {
	tmpDir := t.TempDir()

	// vmstat for oom_kill
	vmstatContent := "nr_free_pages 1000\noom_kill 42\nnr_inactive_anon 500\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "vmstat"), []byte(vmstatContent), 0644))

	// cpu.stat for nr_throttled
	cgroupDir := filepath.Join(tmpDir, "sys", "fs", "cgroup", "kubepods.slice")
	require.NoError(t, os.MkdirAll(cgroupDir, 0755))
	cpuStatContent := "usage_usec 100000\nnr_periods 500\nnr_throttled 88\nthrottled_usec 12345\n"
	require.NoError(t, os.WriteFile(filepath.Join(cgroupDir, "cpu.stat"), []byte(cpuStatContent), 0644))

	c := NewSystemCollector(tmpDir, "", nil)
	k8s := c.collectK8sMetrics()

	assert.Equal(t, int64(42), k8s.OOMKills)
	assert.Equal(t, int64(88), k8s.CPUThrottledPeriods)
}

func TestPodTailer_OffsetCheckpointing(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "pods", "ns1_pod1_uid1", "container")
	require.NoError(t, os.MkdirAll(logDir, 0755))
	logFile := filepath.Join(logDir, "0.log")

	checkpointPath := filepath.Join(tmpDir, "tail_offsets.json")

	// Write initial CRI lines
	line1 := "2026-10-15T12:00:00.000000000Z stdout F first line\n"
	require.NoError(t, os.WriteFile(logFile, []byte(line1), 0644))

	var collected []logengine.Entry
	tailer := NewPodTailer(filepath.Join(tmpDir, "pods"), nil, nil, checkpointPath)
	tailer.entryWriter = func(e logengine.Entry) error {
		collected = append(collected, e)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tailer.pollOnce(ctx)
	require.Len(t, collected, 1)
	assert.Equal(t, "first line", collected[0].Message)

	// Verify checkpoint file was written
	assert.FileExists(t, checkpointPath)

	// Write second line
	line2 := "2026-10-15T12:00:01.000000000Z stdout F second line\n"
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, _ = f.WriteString(line2)
	_ = f.Close()

	// Create NEW tailer instance simulating agent restart
	collectedRestart := []logengine.Entry{}
	tailerRestart := NewPodTailer(filepath.Join(tmpDir, "pods"), nil, nil, checkpointPath)
	tailerRestart.entryWriter = func(e logengine.Entry) error {
		collectedRestart = append(collectedRestart, e)
		return nil
	}

	tailerRestart.pollOnce(ctx)
	// Must ONLY read line2 because line1 was already checkpointed!
	require.Len(t, collectedRestart, 1, "restarted tailer should resume from saved offset without duplicate lines")
	assert.Equal(t, "second line", collectedRestart[0].Message)
}

func TestLogSearch_NoOOMFallbackOnEngineEmptyMatch(t *testing.T) {
	engineDir := t.TempDir()
	engSrc, err := NewEngineLogSource(engineDir)
	require.NoError(t, err)
	defer engSrc.Close()

	// Write 1 entry to engine
	err = engSrc.Writer().Write(logengine.Entry{
		Timestamp: time.Now().UTC(),
		Service:   "test-app",
		Level:     "info",
		Message:   "hello world",
	})
	require.NoError(t, err)
	require.NoError(t, engSrc.Writer().Flush())

	// Create a mock legacy log source that panics or flags if called (simulating heavy OOM scanning)
	var fallbackQueried int32
	heavyFallbackSource := &mockHeavyFallbackSource{onQuery: func() {
		atomic.AddInt32(&fallbackQueried, 1)
	}}

	srv := NewLogServer(
		WithLogSource(engSrc),
		WithLogSource(heavyFallbackSource),
	)

	// Search for query that definitely has 0 matches
	results, err := srv.QueryLogs(context.Background(), "test-app", 100, "", "", "NON_EXISTENT_STRING_999", "")
	require.NoError(t, err)
	assert.Empty(t, results)
	assert.Equal(t, int32(0), atomic.LoadInt32(&fallbackQueried), "engine confirmed 0 matches; heavy fallback sources MUST NOT be scanned")
}

type mockHeavyFallbackSource struct {
	onQuery func()
}

func (m *mockHeavyFallbackSource) ListServices(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockHeavyFallbackSource) GetLogs(ctx context.Context, app string, tail int, sinceTime, untilTime *time.Time, query string, level string) ([]LogEntry, error) {
	if m.onQuery != nil {
		m.onQuery()
	}
	return nil, nil
}

func TestRegistrar_AutoRegistration(t *testing.T) {
	var registeredPayload AgentRegistrationPayload
	var registerCalls int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agents/register" && r.Method == http.MethodPost {
			atomic.AddInt32(&registerCalls, 1)
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &registeredPayload)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"registered"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	reg := NewRegistrar(RegistrarConfig{
		ControlPlaneURL: ts.URL,
		NodeIP:          "192.168.1.100",
		Port:            9100,
		AuthToken:       "test-token",
		Interval:        100 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go reg.Start(ctx)

	// Wait for registration
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&registerCalls) >= 1
	}, 1*time.Second, 20*time.Millisecond)

	assert.Equal(t, "192.168.1.100", registeredPayload.NodeIP)
	assert.Equal(t, 9100, registeredPayload.Port)
	assert.Equal(t, "online", registeredPayload.Status)
	assert.Contains(t, registeredPayload.Capabilities, "metrics")
	assert.Contains(t, registeredPayload.Capabilities, "logs")
}
