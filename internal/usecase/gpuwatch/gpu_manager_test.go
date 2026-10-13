package gpuwatch_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/gpuwatch"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/gpuwatch"
)

// fakeClusterManager implements domain.GPUClusterManager for testing without external deps.
type fakeClusterManager struct {
	scannedCards []gpuwatch.GPUCardTelemetry
	taintedNodes []string
	taintedPCIs  []string
	taintedXIDs  []int
	evictedPods  []string
	scanErr      error
	taintErr     error
	evictErr     error
	ncclHang     bool
	ncclErr      error
}

func (f *fakeClusterManager) ScanNodeGPUs(ctx context.Context, node string) ([]gpuwatch.GPUCardTelemetry, error) {
	if f.scanErr != nil {
		return nil, f.scanErr
	}
	return f.scannedCards, nil
}

func (f *fakeClusterManager) TaintFaultyGPUNode(ctx context.Context, node string, pciAddr string, xid int) error {
	if f.taintErr != nil {
		return f.taintErr
	}
	f.taintedNodes = append(f.taintedNodes, node)
	f.taintedPCIs = append(f.taintedPCIs, pciAddr)
	f.taintedXIDs = append(f.taintedXIDs, xid)
	return nil
}

func (f *fakeClusterManager) EvictZombiePod(ctx context.Context, podName, namespace string) error {
	if f.evictErr != nil {
		return f.evictErr
	}
	f.evictedPods = append(f.evictedPods, namespace+"/"+podName)
	return nil
}

func (f *fakeClusterManager) CheckNCCLProgress(ctx context.Context, jobID string, maxIdleDuration time.Duration) (bool, error) {
	if f.ncclErr != nil {
		return false, f.ncclErr
	}
	return f.ncclHang, nil
}

type fakeKVSwapper struct {
	offloadedBlocks int
	offloadErr      error
}

func (f *fakeKVSwapper) OffloadKVCacheToHostRAM(ctx context.Context, workloadID string, numBlocks int) (int, error) {
	if f.offloadErr != nil {
		return 0, f.offloadErr
	}
	f.offloadedBlocks += numBlocks
	return numBlocks, nil
}

func TestProcessNodeXIDLogs_CriticalFault_TaintsAndEvicts(t *testing.T) {
	cm := &fakeClusterManager{}
	swapper := &fakeKVSwapper{}
	mgr := usecase.NewGPUManagerService(cm, swapper)

	dmesg := `
[ 501.000] NVRM: Xid (PCI:0000:07:00.0): 31, pid=8890, name=megatron_train, MMU Fault: ENGINE GRAPHICS GPCCL
`
	pods := []usecase.PodReference{
		{Name: "llama-train-rank-0", Namespace: "ai-workloads"},
	}

	result, err := mgr.ProcessNodeXIDLogs(context.Background(), "node-gpu-h100-01", dmesg, pods)
	if !errors.Is(err, gpuwatch.ErrGPUXIDHardwareFault) {
		t.Fatalf("ProcessNodeXIDLogs err = %v; want ErrGPUXIDHardwareFault", err)
	}

	if !result.TaintApplied {
		t.Errorf("Expected TaintApplied = true")
	}
	if len(cm.taintedNodes) != 1 || cm.taintedNodes[0] != "node-gpu-h100-01" {
		t.Errorf("Expected node-gpu-h100-01 to be tainted; got %v", cm.taintedNodes)
	}
	if len(cm.taintedXIDs) != 1 || cm.taintedXIDs[0] != 31 {
		t.Errorf("Expected XID 31; got %v", cm.taintedXIDs)
	}
	if len(cm.evictedPods) != 1 || cm.evictedPods[0] != "ai-workloads/llama-train-rank-0" {
		t.Errorf("Expected llama-train-rank-0 evicted; got %v", cm.evictedPods)
	}
}

func TestProcessNodeXIDLogs_NonCriticalFault_GuardrailNoEviction(t *testing.T) {
	cm := &fakeClusterManager{}
	swapper := &fakeKVSwapper{}
	mgr := usecase.NewGPUManagerService(cm, swapper)

	dmesg := `
[ 502.000] NVRM: Xid (PCI:0000:07:00.0): 13, pid=8891, name=torchrun, Graphics Exception: ESR 0x40f100
[ 503.000] NVRM: Xid (PCI:0000:07:00.0): 32, pid=8891, name=torchrun, Corrupted Push Buffer
`
	pods := []usecase.PodReference{
		{Name: "llama-train-rank-0", Namespace: "ai-workloads"},
	}

	result, err := mgr.ProcessNodeXIDLogs(context.Background(), "node-gpu-h100-01", dmesg, pods)
	if err != nil {
		t.Fatalf("ProcessNodeXIDLogs err = %v; want nil for non-critical XIDs", err)
	}

	if result.TaintApplied {
		t.Errorf("Guardrail violation: TaintApplied = true for non-critical XIDs")
	}
	if len(cm.taintedNodes) != 0 {
		t.Errorf("Guardrail violation: taintedNodes = %v; want none", cm.taintedNodes)
	}
	if len(cm.evictedPods) != 0 {
		t.Errorf("Guardrail violation: evictedPods = %v; want none", cm.evictedPods)
	}
	if len(result.NonCriticalFaults) != 2 {
		t.Errorf("Expected 2 non-critical faults; got %d", len(result.NonCriticalFaults))
	}
}

func TestDynamicKVCachePreemptor(t *testing.T) {
	cm := &fakeClusterManager{}
	swapper := &fakeKVSwapper{}
	mgr := usecase.NewGPUManagerService(cm, swapper)

	ctx := context.Background()

	// High pressure >= 85%: triggers preemption and swap
	res, err := mgr.EvaluateAndPreemptKVCache(ctx, "vllm-qwen-72b", 89.5, 1000)
	if !errors.Is(err, gpuwatch.ErrKVCacheOOMRisk) {
		t.Errorf("EvaluateAndPreemptKVCache err = %v; want ErrKVCacheOOMRisk", err)
	}
	if !res.Preempted {
		t.Errorf("Expected Preempted = true for 89.5%% usage")
	}
	if res.BlocksOffloaded <= 0 {
		t.Errorf("Expected BlocksOffloaded > 0; got %d", res.BlocksOffloaded)
	}
	if swapper.offloadedBlocks <= 0 {
		t.Errorf("Expected swapper to offload blocks; got %d", swapper.offloadedBlocks)
	}

	// Safe pressure < 85%: no preemption
	resSafe, errSafe := mgr.EvaluateAndPreemptKVCache(ctx, "vllm-qwen-72b", 70.0, 1000)
	if errSafe != nil {
		t.Errorf("EvaluateAndPreemptKVCache safe err = %v; want nil", errSafe)
	}
	if resSafe.Preempted {
		t.Errorf("Expected Preempted = false for 70.0%% usage")
	}
	if resSafe.BlocksOffloaded != 0 {
		t.Errorf("Expected BlocksOffloaded = 0 for safe usage; got %d", resSafe.BlocksOffloaded)
	}
}

func TestNCCLProgressWatchdog(t *testing.T) {
	cm := &fakeClusterManager{
		ncclHang: true,
	}
	mgr := usecase.NewGPUManagerService(cm, nil)

	ctx := context.Background()

	// Idle duration > 5 minutes (hang detected)
	res, err := mgr.EvaluateNCCLProgress(ctx, "job-megatron-32k", 6*time.Minute, "ai-workloads", "megatron-worker-0")
	if !errors.Is(err, gpuwatch.ErrNCCLHangDetected) {
		t.Errorf("EvaluateNCCLProgress err = %v; want ErrNCCLHangDetected", err)
	}
	if !res.IsHanging {
		t.Errorf("Expected IsHanging = true")
	}
	if len(cm.evictedPods) != 1 || cm.evictedPods[0] != "ai-workloads/megatron-worker-0" {
		t.Errorf("Expected zombie worker pod evicted on hang; got %v", cm.evictedPods)
	}

	// Idle duration <= 5 minutes (healthy progress)
	cmHealthy := &fakeClusterManager{
		ncclHang: false,
	}
	mgrHealthy := usecase.NewGPUManagerService(cmHealthy, nil)
	resOk, errOk := mgrHealthy.EvaluateNCCLProgress(ctx, "job-megatron-32k", 2*time.Minute, "ai-workloads", "megatron-worker-0")
	if errOk != nil {
		t.Errorf("EvaluateNCCLProgress healthy err = %v; want nil", errOk)
	}
	if resOk.IsHanging {
		t.Errorf("Expected IsHanging = false")
	}
}

func TestGetNodeTelemetryAndPaddedCounters(t *testing.T) {
	cm := &fakeClusterManager{
		scannedCards: []gpuwatch.GPUCardTelemetry{
			{
				Index:           0,
				PCIAddress:      "0000:01:00.0",
				Model:           "NVIDIA H100 80GB HBM3",
				VRAMTotalBytes:  85899345920,
				VRAMUsedBytes:   42949672960,
				VRAMUsagePct:    50.0,
				KVCacheUsagePct: 60.0,
				TemperatureC:    55,
				HealthState:     gpuwatch.GPUHealthy,
				LastPolledAt:    time.Now().UTC(),
			},
		},
	}
	mgr := usecase.NewGPUManagerService(cm, nil)

	cards, err := mgr.GetNodeTelemetry(context.Background(), "node-h100-01")
	if err != nil {
		t.Fatalf("GetNodeTelemetry err = %v; want nil", err)
	}
	if len(cards) != 1 {
		t.Fatalf("Expected 1 card; got %d", len(cards))
	}

	counters := mgr.GetCounters()
	if counters.GetPollCount() != 1 {
		t.Errorf("GetPollCount() = %d; want 1", counters.GetPollCount())
	}
	if counters.GetVRAMUsedBytes() != 42949672960 {
		t.Errorf("GetVRAMUsedBytes() = %d; want 42949672960", counters.GetVRAMUsedBytes())
	}
}
