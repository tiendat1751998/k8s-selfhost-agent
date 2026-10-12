# Task 054: NUMA Memory Zone Optimizer & HugePages Allocator
**Status**: `READY_FOR_IMPLEMENTATION`  
**Priority**: `P1_HIGH`  
**FMEA Scenarios Covered**: Scenario 136, 137, 138, 139, 140 (Tầng 31)  
**Target Package Scope**: `internal/infrastructure/kernel/` & `internal/usecase/kernel/`

---

## 1. Root Cause Analysis (Verbatim Production Failure Mode)

### Problematic Scenario
Trên máy chủ Linux đa socket và bộ nhớ lớn (>128GB RAM):
1. **NUMA zone_reclaim_mode Lockup**: `vm.zone_reclaim_mode = 1` khiến kernel thực hiện nén và thu dọn bộ nhớ nội bộ NUMA Node 0 gây CPU spinlock kéo dài hàng trăm ms dù Node 1 còn trống hàng chục GB RAM.
2. **Synchronous Dirty Page Flush**: `vm.dirty_ratio = 20` khiến máy chủ 256GB tích tụ tới 50GB dirty pages. Khi kernel xả đồng bộ, toàn bộ I/O của máy chủ bị đông cứng 100%.
3. **1Gi HugePages Allocation Fragmentation**: RAM bị phân mảnh sau nhiều tuần khiến pod DPDK/LLM yêu cầu `hugepages-1Gi` bị kẹt `Pending/Unschedulable`.
4. **Kernel Slab Leak**: Container tạo và xóa liên tục khiến `SUnreclaim` trong slab ngốn tới 80% RAM, Kubelet không evict được pod dẫn tới OOM sập node.

---

## 2. Go Structs & Interface Contracts

```go
package kernel

import (
	"context"
)

type NUMAZoneStatus struct {
	NodeID          int   `json:"node_id"`
	TotalRAMBytes   int64 `json:"total_ram_bytes"`
	FreeRAMBytes    int64 `json:"free_ram_bytes"`
	ZoneReclaimMode int   `json:"zone_reclaim_mode"`
	SpinlockDetected bool  `json:"spinlock_detected"`
}

type DirtyPageConfig struct {
	DirtyBackgroundBytes int64 `json:"dirty_background_bytes"`
	DirtyBytes           int64 `json:"dirty_bytes"`
	CurrentDirtyBytes    int64 `json:"current_dirty_bytes"`
	SynchronousFlushActive bool `json:"synchronous_flush_active"`
}

type HugePagePoolStatus struct {
	PageSizeKB      int   `json:"page_size_kb"`
	TotalPages      int   `json:"total_pages"`
	FreePages       int   `json:"free_pages"`
	FragmentationIdx float64 `json:"fragmentation_index"`
}

type LinuxMemorySubsystemManager interface {
	AuditNUMAZones(ctx context.Context) ([]NUMAZoneStatus, error)
	TuneNUMAZoneReclaim(ctx context.Context, targetMode int) error
	ApplyDirtyPageByteLimits(ctx context.Context, backgroundBytes, totalBytes int64) error
	CheckHugePageAvailability(ctx context.Context, sizeKB int, requestedPages int) (*HugePagePoolStatus, error)
	TriggerCompactionIfFragmented(ctx context.Context, threshold float64) error
	AuditSlabUnreclaimable(ctx context.Context) (unreclaimMB int64, err error)
}
```

---

## 3. Step-by-Step State Machine Algorithm

```
State: MONITORING
  │
  ├─► [Event: NUMA_AUDIT_TICK (every 60s)]
  │     ├─ Read /proc/sys/vm/zone_reclaim_mode
  │     ├─ If mode != 0 -> Set vm.zone_reclaim_mode = 0
  │     └─ Verify cross-NUMA allocation allowed -> State: MONITORING
  │
  ├─► [Event: DIRTY_PAGE_CHECK (every 30s)]
  │     ├─ Read Dirty in /proc/meminfo
  │     ├─ If Dirty bytes > 1GB -> Apply vm.dirty_background_bytes=256MB, vm.dirty_bytes=1GB
  │     └─ Ensure asynchronous flushing remains active -> State: MONITORING
  │
  └─► [Event: HUGEPAGE_REQUEST_PREFLIGHT]
        ├─ Inspect HugePages_Free for 1048576 kB
        ├─ If free < requested and Unusable Index > 0.7:
        │     ├─ Trigger echo 1 > /proc/sys/vm/compact_memory
        │     ├─ Sleep 500ms
        │     └─ Re-evaluate availability -> If pass Return OK, else Return ErrFragmented
```

---

## 4. RFC 7807 Problem Details Error Codes

- `ERROR_NUMA_ZONE_LOCKED`: NUMA node 0 bị lockup trong local compaction.
- `ERROR_DIRTY_PAGE_FLUSH_SATURATION`: Kernel rơi vào synchronous flush đĩa.
- `ERROR_HUGEPAGES_ALLOCATION_FRAGMENTED`: Không đủ trang 1Gi liền kề do phân mảnh.
- `ERROR_SLAB_UNRECLAIMABLE_LEAK`: SUnreclaim vượt quá 30% tổng RAM máy chủ.

---

## 5. Verification Test Matrix

1. `TestNUMAOptimizer_SetsZoneReclaimModeZero`: Verify sysctl write correctly applies mode 0.
2. `TestDirtyPageLimits_ConvertsPercentageToBytes`: Verify calculation of 256MB background bytes.
3. `TestHugePageAllocator_TriggersCompactionOnFragmentation`: Verify compaction trigger when index > 0.7.

---

## 6. Rollback Guardrails

- Không hạ `vm.dirty_bytes` xuống dưới 128MB để tránh overhead context switch I/O quá cao trên máy chủ đĩa cơ/SATA SSD cũ.
