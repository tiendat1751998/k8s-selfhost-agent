# Task 026 — Kubernetes Network & CNI IPAM Failure Mitigator (P1)
# Checkpoint: 2026-10-12T10:35:00+07:00

> **Severity**: P1 (Cluster Network Health, Pod Scheduling Stalls & DNS Cascading Failures)  
> **Target Agent**: `backend-coder` / `devops`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Zero-PodCIDR Exhaustion, DNS Thundering Herd Prevention, Non-Disruptive Mutation  

---

## 1. BỐI CẢNH KỸ THUẬT & PHÂN TÍCH LỖI THỰC TẾ (ROOT CAUSE ANALYSIS)

### Lỗi 1: CNI IPAM Lease Leakage Gây Treo Toàn Bộ Pod Mới
* **Hiện tượng thực tế**: Các batch job, CI/CD runner pods tạo và xóa liên tục. CNI plugin (Calico IPAM, Flannel, AWS-VPC-CNI) không kịp thu hồi IP lease từ etcd/CRD do daemonset restart hoặc network blip.
* **Hậu quả**: Worker node còn trống 80% CPU/RAM nhưng pod mới kẹt vĩnh viễn ở `ContainerCreating` với lỗi:
  ```
  FailedCreatePodSandBox: failed to allocate for range 0: no IP addresses available in range: podcidr 10.244.1.0/24
  ```
* **Căn nguyên hiện tại**: Platform hiện tại hoàn toàn không theo dõi dung lượng IPAM của từng node. Khi khách hàng bị cạn IP, hệ thống không hề cảnh báo và không có cơ chế tự động dọn rác IPAM mồ côi (Orphaned IPAM Leases).

### Lỗi 2: CoreDNS Cascading Failure do Mặc Định `ndots: 5`
* **Hiện tượng thực tế**: Mặc định Kubernetes inject `options ndots:5` vào `/etc/resolv.conf` của mọi pod. Khi app query một external domain (vd: `payment.gateway.com`), resolver của app gửi liên tiếp 4 query DNS nội bộ trước:
  1. `payment.gateway.com.my-namespace.svc.cluster.local` $\to$ NXDOMAIN
  2. `payment.gateway.com.svc.cluster.local` $\to$ NXDOMAIN
  3. `payment.gateway.com.cluster.local` $\to$ NXDOMAIN
  4. `payment.gateway.com.localdomain` $\to$ NXDOMAIN
  5. `payment.gateway.com` $\to$ Thành công sau 5 vòng!
* **Hậu quả**: Khi một database downstream bị quá tải, hàng trăm microservices retry liên tục, gửi bão query DNS nhân gấp 5 lần khiến CoreDNS nghẽn CPU, DNS latency vọt từ 1ms lên 4000ms, kéo sập toàn bộ các dịch vụ khác trong cụm (Thundering Herd DNS).

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/network/entity.go`:
```go
package network

import (
    "context"
    "time"
)

type IPAMUtilization struct {
    NodeName         string    `json:"node_name"`
    PodCIDR          string    `json:"pod_cidr"`
    TotalAllocatable int       `json:"total_allocatable"` // vd: /24 = 254 IPs
    AllocatedIPs     int       `json:"allocated_ips"`
    ActivePodCount   int       `json:"active_pod_count"`
    LeakedOrphanIPs  int       `json:"leaked_orphan_ips"` // AllocatedIPs - ActivePodCount
    UsagePercentage  float64   `json:"usage_percentage"`
    Status           string    `json:"status"`            // HEALTHY, WARNING, EXHAUSTED
    UpdatedAt        time.Time `json:"updated_at"`
}

type DNSHealthReport struct {
    ClusterID       string        `json:"cluster_id"`
    CoreDNSReplicas int           `json:"coredns_replicas"`
    P99Latency      time.Duration `json:"p99_latency"`
    ErrorRate       float64       `json:"error_rate"`
    NodeLocalActive bool          `json:"node_local_active"`
    AlertTriggered  bool          `json:"alert_triggered"`
}

type NetworkManager interface {
    GetIPAMUtilization(ctx context.Context, clusterID string) ([]IPAMUtilization, error)
    TriggerIPAMGarbageCollection(ctx context.Context, clusterID, nodeName string) (int, error)
    CheckDNSHealth(ctx context.Context, clusterID string) (*DNSHealthReport, error)
    InjectDNSConfigOptimization(ctx context.Context, clusterID, namespace string) error
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Quy trình 1: Giám Sát & Thu Hồi Rò Rỉ CNI IPAM
1. Định kỳ 60 giây, background poller quét danh sách worker nodes:
   - Đọc `spec.podCIDR` của từng Node.
   - Đếm số lượng `IPAMBlock` hoặc `IPAMHandle` (Calico/Cilium CRD) đã cấp phát trên node.
   - Đếm số lượng Pod đang chạy thực tế trên node (`fieldSelector: spec.nodeName=N`).
2. Tính toán: `LeakedOrphanIPs = AllocatedIPs - ActivePodCount`.
3. Nếu `UsagePercentage > 80%` hoặc `LeakedOrphanIPs > 20`:
   - Kích hoạt cảnh báo P2 qua Telegram.
   - Nếu `UsagePercentage > 95%`: Tự động kích hoạt cơ chế `TriggerIPAMGarbageCollection`:
     - Quét các IP block được cấp cho pod đã không còn tồn tại trong K8s API.
     - Giải phóng an toàn các IP handle mồ côi thông qua Calico/Cilium API hoặc Kubelet CNI cache.
     - Cập nhật số IP đã thu hồi vào audit log.

### Quy trình 2: Tối Ưu Hóa & Cứu Hộ CoreDNS (Thundering Herd Defense)
1. Giám sát Prometheus metric `coredns_dns_request_duration_seconds_bucket`.
2. Nếu P99 latency $> 100ms$ trong 3 phút liên tục:
   - Tự động scale up số lượng CoreDNS pods (`kubectl scale deploy coredns --replicas=N+2`).
   - Cảnh báo SRE kèm nút bấm: `[🚀 Kích hoạt NodeLocal DNSCache]` trên Telegram.
3. Cung cấp Mutating Admission Webhook:
   - Tự động bổ sung `dnsConfig` vào các Pod Deployment:
     ```yaml
     dnsConfig:
       options:
         - name: ndots
           value: "2"
         - name: single-request-reopen
     ```
   - Giảm 60-80% số lượng truy vấn DNS không cần thiết ra CoreDNS.

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_CNI_IPAM_EXHAUSTED`: `HTTP 503 Service Unavailable` — "Node '%s' PodCIDR is exhausted (%d/%d allocated). Pod scheduling blocked."
* `ERR_COREDNS_HIGH_LATENCY`: `HTTP 500 Internal Server Error` — "CoreDNS cluster P99 latency exceeded critical threshold (Latency: %s, Threshold: 100ms)."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestIPAM_DetectOrphanedLeases`: Giả lập node có 200 IPs allocated nhưng chỉ có 120 pods chạy. Assert `LeakedOrphanIPs == 80` và `Status == "WARNING"`.
2. `TestIPAM_GarbageCollection_ReclaimsIPs`: Mock Calico CRD có 80 orphan handles. Thực thi `TriggerIPAMGarbageCollection`. Assert thu hồi thành công 80 IPs, `LeakedOrphanIPs` giảm về 0.
3. `TestDNS_NDotsMutation`: Gửi AdmissionReview request của một Pod thông thường. Assert response patch bổ sung `ndots: "2"` và `single-request-reopen`.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Khi giải phóng IPAM, tuyệt đối cấm thu hồi IP của pod đang có trạng thái `Running` hoặc `ContainerCreating` hợp lệ (chống thu hồi nhầm làm rớt mạng app đang chạy).
