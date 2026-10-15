package rdmawatch

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/rdmawatch"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

type mockProber struct {
	links []rdmawatch.GPUFabricBandwidth
	err   error
}

func (m *mockProber) ProbeFabricLinks(ctx context.Context) ([]rdmawatch.GPUFabricBandwidth, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.links, nil
}

type mockCollector struct {
	rxPauses         uint64
	txPauses         uint64
	pauseRateRxSec   float64
	gpuUtil          float64
	memlockUnlimited bool
	err              error
}

func (m *mockCollector) CollectNICMetrics(ctx context.Context, nicName string) (uint64, uint64, float64, float64, bool, error) {
	if m.err != nil {
		return 0, 0, 0, 0, false, m.err
	}
	return m.rxPauses, m.txPauses, m.pauseRateRxSec, m.gpuUtil, m.memlockUnlimited, nil
}

type mockTainter struct {
	mu     sync.Mutex
	taints map[string]string
}

func (m *mockTainter) ApplyNodeTaint(ctx context.Context, nodeName, taint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.taints == nil {
		m.taints = make(map[string]string)
	}
	m.taints[nodeName] = taint
	return nil
}

type mockAlerter struct {
	mu     sync.Mutex
	alerts []string
}

func (m *mockAlerter) SendEmergencyAlert(ctx context.Context, level, title, detail string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts = append(m.alerts, level+":"+title+":"+detail)
	return nil
}

func TestPFCDeadlockWatcher_StormDetected(t *testing.T) {
	ctx := context.Background()
	collector := &mockCollector{
		rxPauses:         50000,
		txPauses:         1200,
		pauseRateRxSec:   15000.0, // > 10,000 pauses/sec
		gpuUtil:          0.0,     // 0% GPU utilization -> deadlock
		memlockUnlimited: true,
	}
	alerter := &mockAlerter{}

	mgr := NewFabricManager(nil, collector, nil, alerter)

	status, err := mgr.DetectPFCDeadlock(ctx, "mlx5_0")
	require.Error(t, err)

	var domErr *domainerrors.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, rdmawatch.CodeRDMAPFCDeadlock, domErr.Code)

	require.NotNil(t, status)
	assert.True(t, status.DeadlockDetected)
	assert.True(t, status.AlertEmitted)
	assert.Equal(t, "mlx5_0", status.InterfaceName)
	assert.Equal(t, uint64(50000), status.PFCPausesRx)
	assert.Equal(t, 15000.0, status.PauseRateRxSec)
	assert.Equal(t, 0.0, status.GPUUtilization)

	// Check alerter received P0 alert
	alerter.mu.Lock()
	defer alerter.mu.Unlock()
	require.Len(t, alerter.alerts, 1)
	assert.Contains(t, alerter.alerts[0], "P0")
	assert.Contains(t, alerter.alerts[0], "PFC Deadlock")

	// Padded counters check
	assert.Equal(t, uint64(1), mgr.GetCounters().GetDeadlockAlerts())
}

func TestPFCDeadlockWatcher_NormalTraffic(t *testing.T) {
	ctx := context.Background()
	collector := &mockCollector{
		rxPauses:         200,
		txPauses:         150,
		pauseRateRxSec:   250.0, // < 10,000 pauses/sec
		gpuUtil:          95.5,  // Active training
		memlockUnlimited: true,
	}

	mgr := NewFabricManager(nil, collector, nil, nil)
	status, err := mgr.DetectPFCDeadlock(ctx, "mlx5_1")
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.False(t, status.DeadlockDetected)
	assert.False(t, status.AlertEmitted)
}

func TestNVLinkFabricAsymmetry_DegradedDetectionAndTaint(t *testing.T) {
	ctx := context.Background()
	tainter := &mockTainter{}
	links := []rdmawatch.GPUFabricBandwidth{
		{DeviceID: "gpu-0", NVLinkID: 0, BandwidthGBps: 900.0, IsDegraded: false},
		{DeviceID: "gpu-0", NVLinkID: 1, BandwidthGBps: 895.0, IsDegraded: false},
		{DeviceID: "gpu-1", NVLinkID: 0, BandwidthGBps: 50.0, IsDegraded: false}, // Severely degraded link (50GB/s vs 900GB/s)
		{DeviceID: "gpu-1", NVLinkID: 1, BandwidthGBps: 900.0, IsDegraded: false},
	}
	prober := &mockProber{links: links}

	mgr := NewFabricManager(prober, nil, tainter, nil)
	mgr.SetDefaultNodeName("worker-gpu-node-42")

	audited, err := mgr.AuditGPUFabricBandwidth(ctx)
	require.Error(t, err)

	var domErr *domainerrors.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, rdmawatch.CodeGPUFabricAsymmetric, domErr.Code)

	require.Len(t, audited, 4)
	assert.True(t, audited[2].IsDegraded, "50 GB/s link must be marked degraded")

	// Verify node taint was generated
	tainter.mu.Lock()
	defer tainter.mu.Unlock()
	assert.Equal(t, rdmawatch.FabricDegradedTaintString, tainter.taints["worker-gpu-node-42"])
}

func TestNVLinkFabricAsymmetry_Healthy(t *testing.T) {
	ctx := context.Background()
	links := []rdmawatch.GPUFabricBandwidth{
		{DeviceID: "gpu-0", NVLinkID: 0, BandwidthGBps: 900.0},
		{DeviceID: "gpu-0", NVLinkID: 1, BandwidthGBps: 898.0},
		{DeviceID: "gpu-1", NVLinkID: 0, BandwidthGBps: 902.0},
		{DeviceID: "gpu-1", NVLinkID: 1, BandwidthGBps: 899.0},
	}
	prober := &mockProber{links: links}

	mgr := NewFabricManager(prober, nil, nil, nil)
	audited, err := mgr.AuditGPUFabricBandwidth(ctx)
	require.NoError(t, err)
	for _, l := range audited {
		assert.False(t, l.IsDegraded)
	}
}

func TestPodRDMAPreflight_ValidPod(t *testing.T) {
	ctx := context.Background()
	validYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: pytorch-train-rank0
  annotations:
    ulimit/memlock: unlimited
spec:
  containers:
  - name: trainer
    image: pytorch/pytorch:2.2.0-cuda12.1-cudnn8-devel
    resources:
      limits:
        nvidia.com/gpu: "8"
    securityContext:
      capabilities:
        add:
        - IPC_LOCK
`
	mgr := NewFabricManager(nil, nil, nil, nil)
	isValid, reason, err := mgr.ValidatePodRDMAPreflight(ctx, []byte(validYAML))
	require.NoError(t, err)
	assert.True(t, isValid)
	assert.Contains(t, reason, "passed")
}

func TestPodRDMAPreflight_MissingIPCLock(t *testing.T) {
	ctx := context.Background()
	missingIPCLockYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: pytorch-train-rank0
  annotations:
    ulimit/memlock: unlimited
spec:
  containers:
  - name: trainer
    image: pytorch/pytorch:2.2.0-cuda12.1-cudnn8-devel
    resources:
      limits:
        nvidia.com/gpu: "8"
    securityContext:
      capabilities:
        add:
        - SYS_PTRACE
`
	mgr := NewFabricManager(nil, nil, nil, nil)
	isValid, reason, err := mgr.ValidatePodRDMAPreflight(ctx, []byte(missingIPCLockYAML))
	require.Error(t, err)
	assert.False(t, isValid)
	assert.Contains(t, reason, "IPC_LOCK")

	var domErr *domainerrors.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, rdmawatch.CodeRDMAMemlockRestricted, domErr.Code)
}

func TestPodRDMAPreflight_MissingMemlock(t *testing.T) {
	ctx := context.Background()
	missingMemlockYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: pytorch-train-rank0
spec:
  containers:
  - name: trainer
    image: pytorch/pytorch:2.2.0-cuda12.1-cudnn8-devel
    resources:
      limits:
        nvidia.com/gpu: "4"
    securityContext:
      capabilities:
        add:
        - IPC_LOCK
`
	mgr := NewFabricManager(nil, nil, nil, nil)
	isValid, reason, err := mgr.ValidatePodRDMAPreflight(ctx, []byte(missingMemlockYAML))
	require.Error(t, err)
	assert.False(t, isValid)
	assert.Contains(t, reason, "memlock")
}

func TestPodRDMAPreflight_NonGPUPodSkipped(t *testing.T) {
	ctx := context.Background()
	nonGPUYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: web-frontend
spec:
  containers:
  - name: nginx
    image: nginx:latest
`
	mgr := NewFabricManager(nil, nil, nil, nil)
	isValid, reason, err := mgr.ValidatePodRDMAPreflight(ctx, []byte(nonGPUYAML))
	require.NoError(t, err)
	assert.True(t, isValid)
	assert.Contains(t, strings.ToLower(reason), "skipped")
}

func TestPodRDMAPreflight_MutatingPatchGeneration(t *testing.T) {
	rawYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: bad-pod
spec:
  containers:
  - name: worker
    resources:
      limits:
        nvidia.com/gpu: "2"
`
	mgr := NewFabricManager(nil, nil, nil, nil)
	patchedYAML, err := mgr.GenerateMutatingPatch([]byte(rawYAML))
	require.NoError(t, err)

	isValid, reason, err := mgr.ValidatePodRDMAPreflight(context.Background(), patchedYAML)
	require.NoError(t, err)
	assert.True(t, isValid)
	assert.Contains(t, reason, "passed")
}

func TestCoordinateStaggeredCheckpoint(t *testing.T) {
	ctx := context.Background()
	policy := rdmawatch.CheckpointWritePolicy{
		MaxConcurrentWrites: 4,
		LocalSpoolPath:      "/mnt/fast-nvme/checkpoints",
		RemoteStorageTarget: "s3://ml-models/checkpoints",
		MaxJitterWindow:     16 * time.Second,
	}

	mgr := NewFabricManager(nil, nil, nil, nil)
	const totalRanks = 16

	delays := make([]time.Duration, totalRanks)
	for r := 0; r < totalRanks; r++ {
		d, err := mgr.CoordinateStaggeredCheckpoint(ctx, r, totalRanks, policy)
		require.NoError(t, err)
		delays[r] = d
	}

	// Verify rank 0 starts at 0s
	assert.Equal(t, time.Duration(0), delays[0])

	// Verify monotonically increasing delays and no collisions
	for i := 1; i < totalRanks; i++ {
		assert.Greater(t, delays[i], delays[i-1], "Rank delay must be monotonically increasing")
		assert.LessOrEqual(t, delays[i], policy.MaxJitterWindow)
	}

	// Rank 8 of 16 with 16s window should be 8s
	assert.Equal(t, 8*time.Second, delays[8])
}

func TestRollbackGuardrail_CheckpointActivePreventsHCAIsolation(t *testing.T) {
	ctx := context.Background()
	mgr := NewFabricManager(nil, nil, nil, nil)

	nic := "mlx5_0"
	err := mgr.BeginCheckpoint(3, nic)
	require.NoError(t, err)
	assert.True(t, mgr.IsCheckpointInProgress(nic))

	// Attempt HCA isolation during active checkpoint write -> must be rejected
	isolated, reason, err := mgr.SafeIsolateHCA(ctx, nic)
	require.NoError(t, err)
	assert.False(t, isolated)
	assert.Contains(t, reason, "checkpoint write in progress")

	// Complete checkpoint write
	err = mgr.CompleteCheckpoint(3, nic)
	require.NoError(t, err)
	assert.False(t, mgr.IsCheckpointInProgress(nic))

	// Now HCA isolation should succeed
	isolated, reason, err = mgr.SafeIsolateHCA(ctx, nic)
	require.NoError(t, err)
	assert.True(t, isolated)
	assert.Contains(t, reason, "safely")
}

func TestFabricManager_NilProber_ReturnsError(t *testing.T) {
	mgr := NewFabricManager(nil, nil, nil, nil)
	links, err := mgr.AuditGPUFabricBandwidth(context.Background())
	assert.Error(t, err)
	assert.Nil(t, links)
	assert.Contains(t, err.Error(), "uninitialized")
}
