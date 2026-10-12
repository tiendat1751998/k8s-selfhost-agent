# Task 055: Hardware ECC EDAC & Thermal Throttling Watchdog
**Status**: `READY_FOR_IMPLEMENTATION`  
**Priority**: `P1_HIGH`  
**FMEA Scenarios Covered**: Scenario 141, 142, 143, 144, 145 (Tầng 32)  
**Target Package Scope**: `internal/infrastructure/hardware/` & `internal/usecase/hardware/`

---

## 1. Root Cause Analysis (Verbatim Production Failure Mode)

### Problematic Scenario
Trên các cụm Bare-metal Datacenter và On-Premises:
1. **ECC Correctable Error Spike**: Thanh RAM bị suy thoái vật lý sinh ra 10,000 lỗi sửa được/phút, dẫn tới Multi-bit uncorrectable error làm sập máy hoặc Silent Data Corruption.
2. **PCIe AER Flooding**: Khe cắm NVMe phát sinh ngắt AER chiếm 100% CPU Core 0, biến filesystem thành Read-Only.
3. **CPU Thermal Throttling Thầm Lặng**: Quạt tản nhiệt hỏng khiến CPU nóng >95°C, xung nhịp bị bóp nghẹt từ 3.6GHz xuống 800MHz khiến API latency tăng 25 lần mà node vẫn báo `Ready`.
4. **PSU Redundancy Loss**: Một bộ nguồn hỏng âm thầm, khi tải đỉnh làm nhảy aptomat PDU, kéo sập toàn bộ 40 node trong rack.

---

## 2. Go Structs & Interface Contracts

```go
package hardware

import (
	"context"
	"time"
)

type ECCDIMMStatus struct {
	SlotID            string    `json:"slot_id"`
	CorrectableCount  int64     `json:"correctable_count"`
	ErrorRatePerMin   float64   `json:"error_rate_per_min"`
	IsDegraded        bool      `json:"is_degraded"`
	LastEventTime     time.Time `json:"last_event_time"`
}

type CPUThermalStatus struct {
	PackageTempC      float64 `json:"package_temp_c"`
	IsThrottled       bool    `json:"is_throttled"`
	BaseFrequencyMHz  int     `json:"base_frequency_mhz"`
	CurrentFrequencyMHz int   `json:"current_frequency_mhz"`
}

type PowerSupplyStatus struct {
	PSUID          string  `json:"psu_id"`
	IsPresent      bool    `json:"is_present"`
	InputVoltage   float64 `json:"input_voltage"`
	OutputPowerW   float64 `json:"output_power_w"`
	RedundancyLost bool    `json:"redundancy_lost"`
}

type HardwareWatchdogEngine interface {
	AuditECCMemoryErrors(ctx context.Context) ([]ECCDIMMStatus, error)
	AuditCPUThermalThrottling(ctx context.Context) (*CPUThermalStatus, error)
	AuditPowerSupplyRedundancy(ctx context.Context) ([]PowerSupplyStatus, error)
	DrainAndCordonDegradedNode(ctx context.Context, nodeName string, reason string) error
	TaintThermalThrottledNode(ctx context.Context, nodeName string) error
}
```

---

## 3. Step-by-Step State Machine Algorithm

```
State: HEALTHY
  │
  ├─► [Event: ECC_POLL_TICK (every 30s)]
  │     ├─ Parse /sys/devices/system/edac/mc/mc*/ce_count
  │     ├─ If Rate > 100 errors/hour on any DIMM slot:
  │     │     ├─ State: DEGRADED_HARDWARE
  │     │     ├─ Apply label node.kubernetes.io/hardware-degraded=true
  │     │     ├─ Trigger graceful kubectl cordon and drain (grace-period 120s)
  │     │     └─ Send P0 Telegram Alert with physical DIMM slot ID
  │
  ├─► [Event: THERMAL_POLL_TICK (every 15s)]
  │     ├─ Read coretemp / MSR thermal throttling bit
  │     ├─ If Temp > 85°C OR CurrentFreq < 0.5 * BaseFreq:
  │     │     ├─ State: THERMAL_THROTTLED
  │     │     ├─ Apply taint node.kubernetes.io/thermal-throttled:NoSchedule
  │     │     └─ Evict latency-sensitive workloads -> State: HEALTHY
  │
  └─► [Event: PSU_STATUS_POLL (every 60s)]
        ├─ Read IPMI Power Supply Redundancy Status
        ├─ If RedundancyLost == true:
        │     ├─ Send High-Priority P1 Alert to Datacenter Ops
        │     └─ Cap CPU RAPL power limit to 70% to avoid PDU trip
```

---

## 4. RFC 7807 Problem Details Error Codes

- `ERROR_ECC_MEMORY_CORRECTABLE_SURGE`: Tần suất lỗi RAM ECC vượt ngưỡng cảnh báo hỏng hóc vật lý.
- `ERROR_CPU_THERMAL_THROTTLED`: CPU bị hạ xung nhịp nghiêm trọng do quá nhiệt.
- `ERROR_PSU_REDUNDANCY_FAILED`: Mất dự phòng nguồn kép 1+1 trên máy chủ vật lý.

---

## 5. Verification Test Matrix

1. `TestECCWatchdog_CordonOnHighErrorRate`: Inject 150 errors/min, verify cordon API invoked.
2. `TestThermalWatchdog_TaintsNodeOnFrequencyDrop`: Inject 800MHz against 3200MHz base, verify NoSchedule taint applied.
3. `TestPSUWatchdog_AlertsOnLostRedundancy`: Inject PSU failure, verify PDU protection threshold triggered.

---

## 6. Rollback Guardrails

- Không thực hiện hard power cut máy chủ nếu không có tín hiệu xác nhận hoàn tất drain từ Kubernetes controller.
