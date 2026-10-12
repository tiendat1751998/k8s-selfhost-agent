# Task 058: RoCEv2 PFC Deadlock Watchdog & GPU RDMA Fabric Resilience
**Status**: `READY_FOR_IMPLEMENTATION`  
**Priority**: `P1_HIGH`  
**FMEA Scenarios Covered**: Scenario 156, 157, 158, 159, 160 (Tầng 35)  
**Target Package Scope**: `internal/infrastructure/ai/` & `internal/usecase/ai/`

---

## 1. Root Cause Analysis (Verbatim Production Failure Mode)

### Problematic Scenario
Trên các cụm GPU AI Training quy mô lớn (64–1024 GPUs):
1. **RoCEv2 PFC Deadlock**: Priority Flow Control pause frames lan truyền ngược theo chu trình vòng tròn giữa các switch buffers làm tê liệt mạng RDMA, 1024 GPU đồng loạt rơi vào 0% utilization.
2. **RDMA Memlock Saturation**: Thiếu `IPC_LOCK` hoặc `ulimit -l unlimited` khiến PyTorch NCCL ném lỗi `Call to ibv_reg_mr failed: Cannot allocate memory` và crash loop.
3. **Asymmetric NVLink Degradation**: Cáp NVLink bị lỗi hạ tốc độ từ 900GB/s xuống 50GB/s, kéo tụt hiệu năng toàn bộ phiên huấn luyện 85% do NCCL AllReduce là rào cản đồng bộ.
4. **CUDA Fork Deadlock**: Gunicorn/FastAPI khởi tạo CUDA trong tiến trình cha trước khi `fork()`, khóa cứng driver mutex và làm treo toàn bộ worker pods.

---

## 2. Go Structs & Interface Contracts

```go
package ai

import (
	"context"
	"time"
)

type GPUFabricBandwidth struct {
	DeviceID        int     `json:"device_id"`
	NVLinkID        int     `json:"nvlink_id"`
	BandwidthGBps   float64 `json:"bandwidth_gbps"`
	IsDegraded      bool    `json:"is_degraded"`
	ErrorCount      int64   `json:"error_count"`
}

type RDMAWatchdogStatus struct {
	InterfaceName   string  `json:"interface_name"`
	PFCPausesRx     uint64  `json:"pfc_pauses_rx"`
	PFCPausesTx     uint64  `json:"pfc_pauses_tx"`
	DeadlockDetected bool    `json:"deadlock_detected"`
	MemlockUnlimited bool    `json:"memlock_unlimited"`
}

type CheckpointWritePolicy struct {
	MaxConcurrentWrites int           `json:"max_concurrent_writes"`
	LocalSpoolPath      string        `json:"local_spool_path"`
	RemoteStorageTarget string        `json:"remote_storage_target"`
	MaxJitterWindow     time.Duration `json:"max_jitter_window"`
}

type GPUAIResilienceEngine interface {
	AuditGPUFabricBandwidth(ctx context.Context) ([]GPUFabricBandwidth, error)
	DetectPFCDeadlock(ctx context.Context, nicName string) (*RDMAWatchdogStatus, error)
	ValidatePodRDMAPreflight(ctx context.Context, podYAML []byte) (isValid bool, reason string, err error)
	CoordinateStaggeredCheckpoint(ctx context.Context, rankID, totalRanks int, policy CheckpointWritePolicy) (delay time.Duration, err error)
}
```

---

## 3. Step-by-Step State Machine Algorithm

```
State: MONITORING
  │
  ├─► [Event: PFC_METRIC_TICK (every 10s)]
  │     ├─ Read /sys/class/infiniband/<dev>/ports/1/counters/pfc_pauses_rx
  │     ├─ If delta > 10,000 pauses/sec and GPU Util drops to 0%:
  │     │     ├─ State: PFC_DEADLOCK_SUSPECTED
  │     │     ├─ Send ECN trigger packet to break buffer queue
  │     │     └─ Alert P0: RDMA_PFC_DEADLOCK_CONFIRMED
  │
  ├─► [Event: FABRIC_BANDWIDTH_PROBE (pre-job run)]
  │     ├─ Run NCCL all-reduce synthetic burst (100MB)
  │     ├─ Calculate standard deviation across links:
  │     │     ├─ If stddev > 15% from baseline:
  │     │     │     ├─ Taint node gpu.kubernetes.io/degraded-fabric=true
  │     │     │     └─ Exclude node from distributed training topology
  │     │     └─ Else: Return READY_FOR_TRAINING
  │
  └─► [Event: POD_ADMISSION_MUTATION]
        ├─ Inspect pod resource requests
        ├─ If nvidia.com/gpu > 0:
        │     ├─ Inject IPC_LOCK capability
        │     ├─ Set ulimit memlock = unlimited
        │     └─ Ensure multiprocessing start method is 'spawn'
```

---

## 4. RFC 7807 Problem Details Error Codes

- `ERROR_RDMA_PFC_DEADLOCK`: Mạng RoCEv2 bị deadlock hàng đợi PFC.
- `ERROR_GPU_FABRIC_ASYMMETRIC`: Băng thông NVLink bị suy thoái vật lý.
- `ERROR_RDMA_MEMLOCK_RESTRICTED`: Thiếu quyền IPC_LOCK / memlock unlimited.

---

## 5. Verification Test Matrix

1. `TestRDMAWatchdog_DetectsPFCStorm`: Inject high pause counter, verify alert and mitigation trigger.
2. `TestFabricBandwidth_TaintsDegradedNVLinkNode`: Inject 50GB/s against 900GB/s, verify taint applied.
3. `TestPodAdmission_InjectsMemlockUnlimited`: Verify mutating webhook sets memlock unlimited.

---

## 6. Rollback Guardrails

- Không ngắt kết nối HCA nếu training job đang trong giai đoạn checkpoint quan trọng mà chưa hoàn tất lưu local.
