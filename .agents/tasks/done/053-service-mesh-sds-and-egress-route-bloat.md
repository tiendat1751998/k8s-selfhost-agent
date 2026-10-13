# Task 053: Service Mesh SDS Watchdog & Egress Route Memory Shield
**Status**: `READY_FOR_IMPLEMENTATION`  
**Priority**: `P1_HIGH`  
**FMEA Scenarios Covered**: Scenario 131, 132, 133, 134, 135 (Tầng 30)  
**Target Package Scope**: `internal/infrastructure/servicemesh/` & `internal/usecase/servicemesh/`

---

## 1. Root Cause Analysis (Verbatim Production Failure Mode)

### Problematic Scenario
Khi cụm Kubernetes mở rộng trên 500 microservices hoặc đa cụm (Multi-Cluster Mesh):
1. **Envoy OOMKill**: Istio mặc định đẩy toàn bộ route và endpoint của toàn bộ cluster vào mỗi sidecar (`Sidecar.spec.egress` rỗng). Envoy chiếm >1.5GB RAM mỗi pod, gây OOMKill diện rộng.
2. **SDS Socket Hang**: `istio-agent` Unix domain socket bị treo khi xoay vòng chứng chỉ TLS 24h, khiến Envoy không thể cập nhật chứng chỉ mTLS mới, gây lỗi `SSL_do_handshake() failed: certificate expired` trên 100% traffic nội bộ.
3. **Tunnel MTU Black Hole**: Gói tin Cilium ClusterMesh VXLAN/Geneve vượt quá MTU 1500 của mạng vật lý bị drop âm thầm khi có cờ `DF` (Don't Fragment), khiến request POST lớn bị treo vô tận.

---

## 2. Go Structs & Interface Contracts

```go
package servicemesh

import (
	"context"
	"time"
)

type SDSSocketStatus struct {
	SocketPath    string        `json:"socket_path"`
	IsAlive       bool          `json:"is_alive"`
	CertRemaining time.Duration `json:"cert_remaining"`
	LastRotatedAt time.Time     `json:"last_rotated_at"`
	LastError     string        `json:"last_error,omitempty"`
}

type EgressScopePolicy struct {
	Namespace       string   `json:"namespace"`
	AllowedServices []string `json:"allowed_services"`
	MaxMemoryMB     int      `json:"max_memory_mb"`
	MemoryAllocated int64    `json:"memory_allocated_bytes"`
}

type TunnelMTUProbeResult struct {
	TunnelInterface string `json:"tunnel_interface"`
	DetectedPMTU    int    `json:"detected_pmtu"`
	ConfiguredMTU   int    `json:"configured_mtu"`
	BlackHoleFound  bool   `json:"black_hole_found"`
	ActionTaken     string `json:"action_taken"`
}

type ServiceMeshResilienceEngine interface {
	AuditSDSSocket(ctx context.Context, socketPath string) (*SDSSocketStatus, error)
	RecoverSDSSocket(ctx context.Context, socketPath string) error
	EnforceEgressScope(ctx context.Context, namespace string, dependencies []string) error
	AuditTunnelMTUBlackHole(ctx context.Context, targetIP string, testPayloadSize int) (*TunnelMTUProbeResult, error)
}
```

---

## 3. Step-by-Step State Machine Algorithm

```
State: IDLE
  │
  ├─► [Event: SDS_PROBE_TICK (every 30s)]
  │     ├─ Dial Unix Domain Socket with 3s timeout
  │     ├─ If Dial fails 2x -> State: RECOVER_SDS
  │     │     ├─ Send SIGUSR1 / restart signal to istio-agent
  │     │     └─ Verify new cert loaded -> State: IDLE
  │     └─ If cert remaining < 2 hours and not refreshed -> Alert P1
  │
  ├─► [Event: MEMORY_SPIKE_CHECK (every 60s)]
  │     ├─ Query envoy_server_memory_allocated metric
  │     ├─ If Memory > 300MB -> State: ENFORCE_EGRESS_SCOPE
  │     │     ├─ Generate minimal Sidecar.networking.istio.io resource
  │     │     ├─ Apply egress scope restricting to direct dependencies
  │     │     └─ Verify memory drops < 100MB -> State: IDLE
  │
  └─► [Event: MTU_BLACKHOLE_PROBE (every 120s)]
        ├─ Send 1450-byte UDP probe with IP_PMTUDISC_DO flag
        ├─ If packet dropped without ICMP FragNeeded -> State: REMEDIATE_MTU
        │     ├─ Set TCP MSS Clamping to 1410
        │     └─ Patch cilium-config tunnel-mtu=1420 -> State: IDLE
```

---

## 4. RFC 7807 Problem Details Error Codes

- `ERROR_SDS_SOCKET_UNRESPONSIVE`: Unix domain socket không phản hồi sau 3s.
- `ERROR_ENVOY_EGRESS_MEMORY_EXHAUSTED`: Envoy sidecar memory vượt quá 300MB.
- `ERROR_TUNNEL_PMTU_BLACKHOLE`: Gói tin lớn bị nuốt không có ICMP phản hồi.

---

## 5. Verification Test Matrix

1. `TestSDSWatchdog_DetectsHungSocketAndRestartsAgent`: Mock hung socket, verify recovery triggered within 3s.
2. `TestEgressScope_GeneratesMinimalIstioSidecarCR`: Verify generated YAML contains only scoped hosts.
3. `TestTunnelMTU_DetectsBlackholeAndClampsMSS`: Verify PMTU black hole triggers MSS clamp calculation.

---

## 6. Rollback Guardrails

- Nếu việc thu hẹp `Sidecar.egress` làm đứt kết nối tới một legitimate dependency, lập tức nới lỏng fallback về `*/<namespace>` trong 15 giây.
