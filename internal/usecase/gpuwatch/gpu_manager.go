// Package gpuwatch provides application usecases for GPU telemetry, XID fault isolation,
// NCCL AllReduce distributed training progress watchdogs, and dynamic KV-cache pressure preemption.
package gpuwatch

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/gpuwatch"
)

// PodReference specifies a Kubernetes pod target for eviction.
type PodReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// XIDRemediationResult details actions performed during XID event handling.
type XIDRemediationResult struct {
	Node               string               `json:"node"`
	CriticalFaults     []gpuwatch.XIDEvent  `json:"critical_faults"`
	NonCriticalFaults  []gpuwatch.XIDEvent  `json:"non_critical_faults"`
	TaintApplied       bool                 `json:"taint_applied"`
	EvictedPods        []string             `json:"evicted_pods"`
	Status             string               `json:"status"`
}

// KVCachePreemptResult details proactive KV-cache offload/swap actions.
type KVCachePreemptResult struct {
	WorkloadID      string  `json:"workload_id"`
	UsagePct        float64 `json:"usage_pct"`
	Preempted       bool    `json:"preempted"`
	BlocksOffloaded int     `json:"blocks_offloaded"`
	Status          string  `json:"status"`
}

// NCCLProgressStatus details training ring synchronization status.
type NCCLProgressStatus struct {
	JobID        string        `json:"job_id"`
	IdleDuration time.Duration `json:"idle_duration"`
	IsHanging    bool          `json:"is_hanging"`
	ActionTaken  string        `json:"action_taken"`
}

// KVCacheSwapper abstracts memory offloading to CPU host RAM via PCIe DMA.
type KVCacheSwapper interface {
	OffloadKVCacheToHostRAM(ctx context.Context, workloadID string, numBlocks int) (int, error)
}

// GPUManagerService coordinates hardware telemetry, XID remediation, and AI workloads.
type GPUManagerService struct {
	clusterMgr gpuwatch.GPUClusterManager
	swapper    KVCacheSwapper
	counters   gpuwatch.PaddedGPUCounters
	mu         sync.RWMutex
}

// NewGPUManagerService constructs a GPUManagerService instance.
func NewGPUManagerService(clusterMgr gpuwatch.GPUClusterManager, swapper KVCacheSwapper) *GPUManagerService {
	return &GPUManagerService{
		clusterMgr: clusterMgr,
		swapper:    swapper,
	}
}

// GetCounters returns the 64-byte cache-line padded telemetry counters.
func (s *GPUManagerService) GetCounters() *gpuwatch.PaddedGPUCounters {
	return &s.counters
}

// GetNodeTelemetry queries node GPU cards and records telemetry metrics.
func (s *GPUManagerService) GetNodeTelemetry(ctx context.Context, node string) ([]gpuwatch.GPUCardTelemetry, error) {
	if s.clusterMgr == nil {
		return nil, fmt.Errorf("cluster manager is uninitialized")
	}

	cards, err := s.clusterMgr.ScanNodeGPUs(ctx, node)
	if err != nil {
		return nil, fmt.Errorf("scanning node %s GPUs: %w", node, err)
	}

	var totalVRAMUsed uint64
	var maxKVCacheMilli uint64

	for _, card := range cards {
		totalVRAMUsed += card.VRAMUsedBytes
		kvMilli := uint64(math.Round(card.KVCacheUsagePct * 1000))
		if kvMilli > maxKVCacheMilli {
			maxKVCacheMilli = kvMilli
		}
	}

	s.counters.RecordPoll(totalVRAMUsed, maxKVCacheMilli)
	return cards, nil
}

// ProcessNodeXIDLogs parses dmesg error events, isolates hardware faults with node taints,
// and terminates zombie training pods.
// SAFETY GUARDRAIL: Pod eviction and node taints are strictly prohibited for non-critical XIDs (e.g. 13, 32).
func (s *GPUManagerService) ProcessNodeXIDLogs(
	ctx context.Context,
	node string,
	dmesgLog string,
	affectedPods []PodReference,
) (XIDRemediationResult, error) {
	events := gpuwatch.ParseDmesgXIDErrors(dmesgLog)

	var criticalFaults []gpuwatch.XIDEvent
	var nonCriticalFaults []gpuwatch.XIDEvent

	for _, ev := range events {
		if ev.IsCritical {
			criticalFaults = append(criticalFaults, ev)
		} else {
			nonCriticalFaults = append(nonCriticalFaults, ev)
		}
	}

	result := XIDRemediationResult{
		Node:              node,
		CriticalFaults:    criticalFaults,
		NonCriticalFaults: nonCriticalFaults,
		TaintApplied:      false,
		Status:            "OK",
	}

	// If no critical faults detected, apply safety guardrail and return early without eviction
	if len(criticalFaults) == 0 {
		if len(nonCriticalFaults) > 0 {
			result.Status = "NON_CRITICAL_INFORMATIONAL_WARNING"
		}
		return result, nil
	}

	s.counters.RecordFault()

	// Apply node taint for each critical XID event
	if s.clusterMgr != nil {
		for _, fault := range criticalFaults {
			if err := s.clusterMgr.TaintFaultyGPUNode(ctx, node, fault.PCIAddress, fault.XID); err != nil {
				return result, fmt.Errorf("applying node taint %s for XID %d: %w", node, fault.XID, err)
			}
			result.TaintApplied = true
		}

		// Evict zombie pods to unfreeze distributed NCCL AllReduce rings
		for _, pod := range affectedPods {
			if err := s.clusterMgr.EvictZombiePod(ctx, pod.Name, pod.Namespace); err != nil {
				return result, fmt.Errorf("evicting zombie pod %s/%s: %w", pod.Namespace, pod.Name, err)
			}
			result.EvictedPods = append(result.EvictedPods, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
		}
	}

	result.Status = "CRITICAL_HARDWARE_FAULT_CONTAINED"
	return result, gpuwatch.ErrGPUXIDHardwareFault
}

// EvaluateAndPreemptKVCache monitors PagedAttention KV-cache pressure and proactively
// offloads blocks to host CPU memory when usage exceeds 85%.
func (s *GPUManagerService) EvaluateAndPreemptKVCache(
	ctx context.Context,
	workloadID string,
	usagePct float64,
	totalBlocks int,
) (KVCachePreemptResult, error) {
	res := KVCachePreemptResult{
		WorkloadID: workloadID,
		UsagePct:   usagePct,
		Preempted:  false,
		Status:     "HEALTHY",
	}

	if err := gpuwatch.EvaluateKVCacheRisk(usagePct); err != nil {
		// Calculate blocks to offload to bring pressure down to ~70% (safe watermark)
		excessPct := usagePct - 70.0
		if excessPct <= 0 {
			excessPct = 15.0
		}
		blocksToOffload := int(math.Ceil(float64(totalBlocks) * (excessPct / 100.0)))
		if blocksToOffload <= 0 {
			blocksToOffload = 1
		}

		res.Preempted = true
		res.Status = "PREEMPTIVE_SWAP_TRIGGERED"

		if s.swapper != nil {
			offloaded, swapErr := s.swapper.OffloadKVCacheToHostRAM(ctx, workloadID, blocksToOffload)
			if swapErr != nil {
				return res, fmt.Errorf("offloading KV-cache blocks for workload %s: %w", workloadID, swapErr)
			}
			res.BlocksOffloaded = offloaded
		} else {
			res.BlocksOffloaded = blocksToOffload
		}

		return res, gpuwatch.ErrKVCacheOOMRisk
	}

	return res, nil
}

// EvaluateNCCLProgress monitors distributed training progress and terminates deadlocked jobs.
func (s *GPUManagerService) EvaluateNCCLProgress(
	ctx context.Context,
	jobID string,
	idleDuration time.Duration,
	podNamespace string,
	podName string,
) (NCCLProgressStatus, error) {
	status := NCCLProgressStatus{
		JobID:        jobID,
		IdleDuration: idleDuration,
		IsHanging:    false,
		ActionTaken:  "NONE",
	}

	isHang := gpuwatch.IsNCCLHang(idleDuration)
	if !isHang && s.clusterMgr != nil {
		var err error
		isHang, err = s.clusterMgr.CheckNCCLProgress(ctx, jobID, idleDuration)
		if err != nil {
			return status, fmt.Errorf("checking NCCL progress for job %s: %w", jobID, err)
		}
	}

	if isHang {
		status.IsHanging = true
		status.ActionTaken = "EVICTED_DEADLOCKED_TRAINING_JOB"

		if s.clusterMgr != nil && podName != "" && podNamespace != "" {
			if err := s.clusterMgr.EvictZombiePod(ctx, podName, podNamespace); err != nil {
				return status, fmt.Errorf("evicting hanging NCCL worker %s/%s: %w", podNamespace, podName, err)
			}
		}

		return status, gpuwatch.ErrNCCLHangDetected
	}

	return status, nil
}
