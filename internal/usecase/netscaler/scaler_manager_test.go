package netscaler_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/datdt/k8sselfhost/internal/domain/netscaler"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/netscaler"
)

// memorySysctlExecutor implements usecase.SysctlExecutor in-memory with optional write error simulation.
type memorySysctlExecutor struct {
	mu         sync.Mutex
	values     map[string]string
	failKey    string
	failErr    error
	writeOrder []string
}

func newMemorySysctlExecutor(initial map[string]string) *memorySysctlExecutor {
	cpy := make(map[string]string)
	for k, v := range initial {
		cpy[k] = v
	}
	return &memorySysctlExecutor{
		values:     cpy,
		writeOrder: make([]string, 0),
	}
}

func (m *memorySysctlExecutor) ReadSysctl(_ context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.values[key]
	if !ok {
		return "0", nil
	}
	return v, nil
}

func (m *memorySysctlExecutor) WriteSysctl(_ context.Context, key string, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failKey != "" && m.failKey == key {
		return m.failErr
	}
	m.values[key] = value
	m.writeOrder = append(m.writeOrder, key)
	return nil
}

// staticMetricProvider supplies predetermined socket metrics for testing.
type staticMetricProvider struct {
	metrics *domain.SocketMetrics
	err     error
}

func (p *staticMetricProvider) CollectMetrics(_ context.Context) (*domain.SocketMetrics, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.metrics, nil
}

func TestEvaluateAndAutoTune_PortStarvation(t *testing.T) {
	// Ephemeral port pressure at 85%
	metrics, err := domain.NewSocketMetrics(
		23500, // TIME_WAIT > 22000
		24000, // Allocated ports
		28232, // Total ephemeral ports -> ~85.01%
		0,     // No listen drops
		50000, // Low conntrack entries
		262144,// Conntrack max
	)
	require.NoError(t, err)

	executor := newMemorySysctlExecutor(map[string]string{
		"net.core.somaxconn":                             "1024",
		"net.ipv4.tcp_max_syn_backlog":                  "1024",
		"net.ipv4.tcp_tw_reuse":                         "0",
		"net.netfilter.nf_conntrack_max":                "262144",
		"net.netfilter.nf_conntrack_tcp_timeout_time_wait": "60",
	})

	tmpDir := t.TempDir()
	backupPath := filepath.Join(tmpDir, "sysctl_backup.conf")

	mgr := usecase.NewScalerManager(
		usecase.WithMetricProvider(&staticMetricProvider{metrics: metrics}),
		usecase.WithSysctlExecutor(executor),
		usecase.WithBackupPath(backupPath),
	)

	result, err := mgr.EvaluateAndAutoTune(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.TuningApplied)
	assert.True(t, result.EnforceHTTPConnectionPool, "HTTP client connection pooling must be enforced when port starved")
	assert.Equal(t, 1, result.AppliedConfig.TCPTwReuse, "tcp_tw_reuse should be tuned to 1")

	// Verify sysctl executor was updated
	val, err := executor.ReadSysctl(context.Background(), "net.ipv4.tcp_tw_reuse")
	require.NoError(t, err)
	assert.Equal(t, "1", val)

	// Verify backup snapshot file exists on disk
	_, err = os.Stat(backupPath)
	assert.NoError(t, err, "Backup file should exist on disk")
}

func TestEvaluateAndAutoTune_ConntrackDynamicExpansion(t *testing.T) {
	// Conntrack usage at 76.29% (> 75%)
	metrics, err := domain.NewSocketMetrics(
		1000,
		2000,
		28232,
		0,
		200000, // ct entries
		262144, // ct max -> ~76.29%
	)
	require.NoError(t, err)

	executor := newMemorySysctlExecutor(map[string]string{
		"net.core.somaxconn":                             "1024",
		"net.ipv4.tcp_max_syn_backlog":                  "1024",
		"net.ipv4.tcp_tw_reuse":                         "0",
		"net.netfilter.nf_conntrack_max":                "262144",
		"net.netfilter.nf_conntrack_tcp_timeout_time_wait": "60",
	})

	tmpDir := t.TempDir()
	backupPath := filepath.Join(tmpDir, "sysctl_backup.conf")

	mgr := usecase.NewScalerManager(
		usecase.WithMetricProvider(&staticMetricProvider{metrics: metrics}),
		usecase.WithSysctlExecutor(executor),
		usecase.WithBackupPath(backupPath),
	)

	result, err := mgr.EvaluateAndAutoTune(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.TuningApplied)
	// Max was 262144, doubled -> 524288
	assert.Equal(t, int64(524288), result.AppliedConfig.ConntrackMax)

	val, err := executor.ReadSysctl(context.Background(), "net.netfilter.nf_conntrack_max")
	require.NoError(t, err)
	assert.Equal(t, "524288", val)
}

func TestEvaluateAndAutoTune_ConntrackHardSafetyCeiling(t *testing.T) {
	// Conntrack max already at 1,500,000, doubling would reach 3,000,000 (> 2,097,152)
	metrics, err := domain.NewSocketMetrics(
		1000,
		2000,
		28232,
		0,
		1200000, // ct entries (80% usage)
		1500000, // ct max
	)
	require.NoError(t, err)

	executor := newMemorySysctlExecutor(map[string]string{
		"net.core.somaxconn":              "1024",
		"net.ipv4.tcp_max_syn_backlog":   "1024",
		"net.ipv4.tcp_tw_reuse":          "0",
		"net.netfilter.nf_conntrack_max": "1500000",
		"net.netfilter.nf_conntrack_tcp_timeout_time_wait": "60",
	})

	tmpDir := t.TempDir()
	backupPath := filepath.Join(tmpDir, "sysctl_backup.conf")

	mgr := usecase.NewScalerManager(
		usecase.WithMetricProvider(&staticMetricProvider{metrics: metrics}),
		usecase.WithSysctlExecutor(executor),
		usecase.WithBackupPath(backupPath),
	)

	result, err := mgr.EvaluateAndAutoTune(context.Background())
	require.NoError(t, err)
	assert.Equal(t, domain.ConntrackHardSafetyCeiling, result.AppliedConfig.ConntrackMax,
		"Must clamp to ConntrackHardSafetyCeiling to avoid kernel OOM panic")
}

func TestEvaluateAndAutoTune_ListenDropsBacklogScaling(t *testing.T) {
	// Listen backlog drops detected
	metrics, err := domain.NewSocketMetrics(
		1000,
		2000,
		28232,
		15, // 15 listen backlog drops
		50000,
		262144,
	)
	require.NoError(t, err)

	executor := newMemorySysctlExecutor(map[string]string{
		"net.core.somaxconn":              "128",
		"net.ipv4.tcp_max_syn_backlog":   "128",
		"net.ipv4.tcp_tw_reuse":          "0",
		"net.netfilter.nf_conntrack_max": "262144",
		"net.netfilter.nf_conntrack_tcp_timeout_time_wait": "60",
	})

	mgr := usecase.NewScalerManager(
		usecase.WithMetricProvider(&staticMetricProvider{metrics: metrics}),
		usecase.WithSysctlExecutor(executor),
		usecase.WithBackupPath(filepath.Join(t.TempDir(), "sysctl_backup.conf")),
	)

	result, err := mgr.EvaluateAndAutoTune(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 65535, result.AppliedConfig.Somaxconn)
	assert.Equal(t, 65535, result.AppliedConfig.TCPMaxSynBacklog)
}

func TestApplyKernelTuning_RollbackGuardrail(t *testing.T) {
	initialValues := map[string]string{
		"net.core.somaxconn":                             "128",
		"net.ipv4.tcp_max_syn_backlog":                  "256",
		"net.ipv4.tcp_tw_reuse":                         "0",
		"net.netfilter.nf_conntrack_max":                "262144",
		"net.netfilter.nf_conntrack_tcp_timeout_time_wait": "60",
	}
	executor := newMemorySysctlExecutor(initialValues)
	// Simulate failure when applying net.ipv4.tcp_tw_reuse
	executor.failKey = "net.ipv4.tcp_tw_reuse"
	executor.failErr = errors.New("kernel: permission denied writing to /proc/sys/net/ipv4/tcp_tw_reuse")

	tmpDir := t.TempDir()
	backupPath := filepath.Join(tmpDir, "sysctl_backup.conf")

	mgr := usecase.NewScalerManager(
		usecase.WithSysctlExecutor(executor),
		usecase.WithBackupPath(backupPath),
	)

	targetCfg := domain.KernelNetConfig{
		Somaxconn:            65535,
		TCPMaxSynBacklog:     65535,
		TCPTwReuse:           1,
		ConntrackMax:         524288,
		ConntrackTCPTimeWait: 30,
	}

	err := mgr.ApplyKernelTuning(context.Background(), targetCfg)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrKernelSysctlFailed), "Must return ErrKernelSysctlFailed")

	// Ensure rollback happened: somaxconn and tcp_max_syn_backlog must be restored to original values!
	somaxVal, _ := executor.ReadSysctl(context.Background(), "net.core.somaxconn")
	assert.Equal(t, "128", somaxVal, "somaxconn must be rolled back to initial value")
	synVal, _ := executor.ReadSysctl(context.Background(), "net.ipv4.tcp_max_syn_backlog")
	assert.Equal(t, "256", synVal, "tcp_max_syn_backlog must be rolled back to initial value")

	// Verify backup file was created
	_, err = os.Stat(backupPath)
	assert.NoError(t, err, "Backup file should have been written before tuning was attempted")
}

func TestCheckHealth(t *testing.T) {
	// Case 1: Healthy
	healthy, _ := domain.NewSocketMetrics(100, 500, 28232, 0, 1000, 262144)
	mgrHealthy := usecase.NewScalerManager(
		usecase.WithMetricProvider(&staticMetricProvider{metrics: healthy}),
	)
	assert.NoError(t, mgrHealthy.CheckHealth(context.Background()))

	// Case 2: Port Starved
	starved, _ := domain.NewSocketMetrics(25000, 26000, 28232, 0, 1000, 262144)
	mgrStarved := usecase.NewScalerManager(
		usecase.WithMetricProvider(&staticMetricProvider{metrics: starved}),
	)
	err := mgrStarved.CheckHealth(context.Background())
	assert.True(t, errors.Is(err, domain.ErrNetEphemeralPortsExhausted))

	// Case 3: Conntrack Saturated
	saturated, _ := domain.NewSocketMetrics(100, 500, 28232, 0, 250000, 262144) // >75%
	mgrSaturated := usecase.NewScalerManager(
		usecase.WithMetricProvider(&staticMetricProvider{metrics: saturated}),
	)
	err = mgrSaturated.CheckHealth(context.Background())
	assert.True(t, errors.Is(err, domain.ErrNetConntrackSaturation))
}

func TestConcurrentSafety(t *testing.T) {
	metrics, _ := domain.NewSocketMetrics(500, 1000, 28232, 0, 10000, 262144)
	executor := newMemorySysctlExecutor(map[string]string{
		"net.core.somaxconn":                             "1024",
		"net.ipv4.tcp_max_syn_backlog":                  "1024",
		"net.ipv4.tcp_tw_reuse":                         "0",
		"net.netfilter.nf_conntrack_max":                "262144",
		"net.netfilter.nf_conntrack_tcp_timeout_time_wait": "60",
	})
	mgr := usecase.NewScalerManager(
		usecase.WithMetricProvider(&staticMetricProvider{metrics: metrics}),
		usecase.WithSysctlExecutor(executor),
		usecase.WithBackupPath(filepath.Join(t.TempDir(), "sysctl_backup.conf")),
	)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, _ = mgr.CollectMetrics(context.Background())
			_ = mgr.CheckHealth(context.Background())
		}()
		go func() {
			defer wg.Done()
			_, _ = mgr.EvaluateAndAutoTune(context.Background())
		}()
	}

	wg.Wait()
}
