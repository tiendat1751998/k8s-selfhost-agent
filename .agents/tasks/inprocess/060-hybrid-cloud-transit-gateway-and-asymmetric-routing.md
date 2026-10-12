# Task 060: Hybrid Cloud Transit Gateway & Asymmetric Routing Shield
**Status**: `READY_FOR_IMPLEMENTATION`  
**Priority**: `P1_HIGH`  
**FMEA Scenarios Covered**: Scenario 166, 167, 168, 169, 170 (Tầng 37)  
**Target Package Scope**: `internal/infrastructure/cloudnet/` & `internal/usecase/cloudnet/`

---

## 1. Root Cause Analysis (Verbatim Production Failure Mode)

### Problematic Scenario
Trong kiến trúc mạng Hybrid Cloud (AWS/GCP/On-Premises):
1. **Asymmetric Routing**: Gói tin đi qua DirectConnect nhưng về qua IPSec VPN, tường lửa stateful ở Datacenter drop toàn bộ kết nối do thiếu session state.
2. **VPC Peering Jumbo MTU Drop**: Gói tin > 1500 bytes bị nuốt âm thầm (Path MTU Black Hole) trên đường peering giữa VPC AI (MTU 9001) và VPC App (MTU 1500).
3. **NLB Cross-Zone Imbalance**: Tắt cross-zone khiến 1 AZ chịu 80% tải, pods ở AZ đó quá tải sập hàng loạt dù toàn cụm còn thừa 60% CPU.
4. **Split-View DNS NXDOMAIN Poisoning**: VPN chập chờn khiến CoreDNS fallback ra public DNS và cache âm (negative TTL 300s) tên miền nội bộ, chặn đứng kết nối DB 5 phút.

---

## 2. Go Structs & Interface Contracts

```go
package cloudnet

import (
	"context"
	"time"
)

type RoutingPathIntegrity struct {
	PrimaryPath     string        `json:"primary_path"`
	ReturnPath      string        `json:"return_path"`
	IsAsymmetric    bool          `json:"is_asymmetric"`
	RTTLatency      time.Duration `json:"rtt_latency"`
	PacketLossRate  float64       `json:"packet_loss_rate"`
}

type PeeringMTUValidation struct {
	SourceVPCID       string `json:"source_vpc_id"`
	TargetVPCID       string `json:"target_vpc_id"`
	ConfiguredMSS     int    `json:"configured_mss"`
	EffectivePathMTU  int    `json:"effective_path_mtu"`
	BlackHoleDetected bool   `json:"black_hole_detected"`
}

type NLBZoneDistribution struct {
	LoadBalancerARN   string             `json:"load_balancer_arn"`
	CrossZoneEnabled  bool               `json:"cross_zone_enabled"`
	ZoneRequestShares map[string]float64 `json:"zone_request_shares"`
	ImbalanceDetected bool               `json:"imbalance_detected"`
}

type HybridCloudNetworkManager interface {
	AuditRoutingSymmetry(ctx context.Context, targetEndpoint string) (*RoutingPathIntegrity, error)
	ValidateVPCPathMTU(ctx context.Context, targetHost string, testBytes int) (*PeeringMTUValidation, error)
	AuditNLBCrossZoneBalance(ctx context.Context, lbARN string) (*NLBZoneDistribution, error)
	SanitizeCoreDNSNegativeCache(ctx context.Context, domainZone string, maxNegativeTTLSeconds int) error
}
```

---

## 3. Step-by-Step State Machine Algorithm

```
State: HEALTHY
  │
  ├─► [Event: BGP_PATH_PROBE (every 30s)]
  │     ├─ Trace route outward and match against inbound BGP AS-Path
  │     ├─ If Primary != Return:
  │     │     ├─ State: ASYMMETRIC_ROUTING_DETECTED
  │     │     ├─ Alert P0: BGP_ASYMMETRIC_PATH
  │     │     └─ Apply BGP AS-Path Prepending (x3) on backup VPN link
  │
  ├─► [Event: NLB_BALANCE_CHECK (every 60s)]
  │     ├─ Scrape CloudWatch RequestCountPerTarget by AZ
  │     ├─ If any AZ share > 60% and CrossZone == false:
  │     │     ├─ Auto-enable cross_zone_load_balancing.enabled = true
  │     │     └─ Verify traffic distributes evenly -> State: HEALTHY
  │
  └─► [Event: COREDNS_NEGATIVE_CACHE_DEFENSE]
        ├─ Ensure CoreDNS ConfigMap contains cache denial 5 for corp.internal
        └─ Prevent caching NXDOMAIN for longer than 5 seconds
```

---

## 4. RFC 7807 Problem Details Error Codes

- `ERROR_BGP_ASYMMETRIC_PATH`: Phát hiện đường đi và về không đồng nhất qua stateful firewall.
- `ERROR_VPC_PATH_MTU_BLACKHOLE`: Gói tin Jumbo Frame bị drop âm thầm trên peering link.
- `ERROR_NLB_AZ_IMBALANCE`: Mất cân bằng tải nghiêm trọng giữa các Availability Zones.

---

## 5. Verification Test Matrix

1. `TestRoutingSymmetry_DetectsAsymmetricPath`: Inject divergent route, verify alert triggered.
2. `TestVPCPathMTU_FlagsBlackHole`: Mock 1500 byte drop without ICMP, verify flag set.
3. `TestNLBZoneDistribution_EnablesCrossZoneOnImbalance`: Inject 80% share on AZ-a, verify cross-zone patch generated.

---

## 6. Rollback Guardrails

- Không tự động sửa AS-Path BGP trên router phần cứng nếu chưa có tín hiệu probe xác nhận đường truyền DirectConnect chính hoàn toàn sẵn sàng.
