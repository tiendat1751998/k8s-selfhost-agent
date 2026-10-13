# Task 038 — Multi-Datacenter Safe Failover & Storage Fencing Engine (STONITH) (P1)
# Checkpoint: 2026-10-12T10:45:00+07:00

> **Severity**: P1 (Multi-Datacenter Split-Brain Prevention, Dual-Master Elimination & Cloud Metadata Hardening)  
> **Target Agent**: `devops` / `backend-coder`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: STONITH Hard Fencing, Failover Grace Delays, IMDSv2 Hard Enforcement  

---

## 1. BỐI CẢNH KỸ THUẬT & PHÂN TÍCH LỖI THỰC TẾ (ROOT CAUSE ANALYSIS)

### Lỗi 1: Thảm Họa GitHub 2018 — Failover Vội Vã Gây Split-Brain Writes
* **Hiện tượng**: Đường truyền cáp quang giữa 2 Datacenter bị chập chờn 43 giây. Orchestrator tự động vội vã thăng cấp (promote) Database replica ở DC phụ lên làm Master mới trong khi Primary cũ ở DC chính vẫn đang nhận traffic ghi. Dữ liệu bị xé làm 2 nhánh (Split-Brain Writes) với khóa chính trùng lặp, gây hỏng hóc cơ sở dữ liệu trên diện rộng.
* **Cách xử lý triệt để**:
  1. **Cơ Chế STONITH (Shoot The Other Node In The Head)**:
     - Tuyệt đối KHÔNG BAO GIỜ thăng cấp node replica nếu chưa nhận được tín hiệu xác thực 100% rằng node Primary cũ đã bị ngắt mạng hoàn toàn (Network Isolation) hoặc đã bị tắt nguồn vật lý (IPMI / Power Switch Off).
  2. **Transient Hiccup Grace Period**:
     - Thiết lập độ trễ an toàn tối thiểu 60-120 giây trước khi kích hoạt quy trình failover tự động. Nếu đường truyền mạng phục hồi trong 60s, hủy bỏ lệnh failover để tránh phản ứng thái quá với các sự cố chập chờn ngắn hạn.
  3. **Semi-Synchronous Replication Strictness**:
     - Bắt buộc cấu hình `rpl_semi_sync_master_wait_no_slave = 1`: Nếu không có ít nhất 1 replica xác nhận đã nhận WAL/binlog, Primary tự động từ chối các giao dịch ghi mới thay vì âm thầm ghi tiếp.

### Lỗi 2: Khai Thác IMDSv1 Đánh Cắp Cloud Credentials Qua Lỗ Hổng SSRF
* **Hiện tượng**: Pod ứng dụng bị lỗi SSRF, kẻ tấn công đọc `http://169.254.169.254/latest/meta-data/iam/...` và đánh cắp token của Node IAM Role.
* **Cách xử lý triệt để**:
  1. Tự động áp dụng NetworkPolicy mặc định chặn 100% egress traffic tới CIDR `169.254.169.254/32` cho tất cả application namespaces.
  2. Kiểm tra và cưỡng chế bật IMDSv2 (`HttpTokens=required`) kèm `HttpPutResponseHopLimit=1` trên toàn bộ máy chủ cloud.

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/cluster/fencing.go`:
```go
package cluster

import (
    "context"
    "time"
)

type FencingMethod string

const (
    FencingMethodIPMIReset  FencingMethod = "IPMI_POWER_OFF"
    FencingMethodNetworkCut FencingMethod = "NETWORK_ISOLATION"
    FencingMethodCloudAPI   FencingMethod = "CLOUD_INSTANCE_STOP"
)

type NodeFencingReport struct {
    TargetNodeID   string        `json:"target_node_id"`
    Datacenter     string        `json:"datacenter"`
    Method         FencingMethod `json:"method"`
    IsFenced       bool          `json:"is_fenced"`
    ConfirmedAt    *time.Time    `json:"confirmed_at,omitempty"`
    Duration       time.Duration `json:"duration"`
    OutputLog      string        `json:"output_log"`
}

type FailoverReadinessStatus struct {
    CanPromote       bool          `json:"can_promote"`
    OldPrimaryFenced bool          `json:"old_primary_fenced"`
    ReplicationLag   int64         `json:"replication_lag_bytes"`
    GraceDelayPassed bool          `json:"grace_delay_passed"`
    RejectReason     string        `json:"reject_reason,omitempty"`
}

type MultiDCFencingManager interface {
    ExecuteSTONITH(ctx context.Context, targetNodeID, dc string) (*NodeFencingReport, error)
    ValidateFailoverReadiness(ctx context.Context, clusterID string) (*FailoverReadinessStatus, error)
    EnforceIMDSv2Security(ctx context.Context, clusterID string) error
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Quy trình STONITH & Failover An Toàn Tuyệt Đối:
1. Phát hiện Primary node $P_1$ tại DC-East không phản hồi heartbeat trong 30 giây.
2. **Kích hoạt Grace Period Watcher (60s)**:
   - Nếu trong 60 giây $P_1$ online trở lại $\implies$ Hủy quy trình failover, ghi log transient hiccup.
3. Nếu sau 60 giây $P_1$ vẫn mất kết nối:
   - Khởi chạy lệnh STONITH:
     - Gọi Cloud API (AWS/GCP/OpenStack) hoặc IPMI gửi lệnh tắt máy cứng: `POST /instances/{id}/stop` hoặc `ipmitool power off`.
   - Chờ nhận tín hiệu xác thực phần cứng rằng $P_1$ đã tắt hoàn toàn (`IsFenced == true`).
4. **Kiểm Tra Trạng Thái Sẵn Sàng (Readiness Gate)**:
   - Nếu $P_1$ chưa được xác nhận Fenced $\implies$ **CHẶN ĐỨNG THĂNG CẤP REPLICA**, phát chuông báo động P0 cho ban chỉ huy SRE.
   - Nếu $P_1$ đã Fenced thành công: Thăng cấp Replica $P_2$ tại DC-West lên Primary mới.
5. Cập nhật DNS / Virtual IP trỏ về DC-West.
6. **Triệt tiêu 100% thảm họa Split-Brain Writes.**

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_FAILOVER_PRIMARY_NOT_FENCED`: `HTTP 409 Conflict` — "Failover aborted: Old primary node '%s' is not confirmed fenced. Promoting replica risks split-brain writes."
* `ERR_IMDS_ACCESS_BLOCKED`: `HTTP 403 Forbidden` — "Access to cloud metadata service 169.254.169.254 is restricted by Zero-Trust network policy."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestFailover_RejectsPromotionIfPrimaryNotFenced`: Giả lập Primary mất mạng nhưng lệnh STONITH trả về timeout. Assert hàm `ValidateFailoverReadiness` trả về `CanPromote == false`.
2. `TestFailover_PromotesOnlyAfterSTONITHConfirmed`: Giả lập STONITH thành công sau 40s grace period. Assert hàm trả về `CanPromote == true`.
3. `TestIMDS_NetworkPolicyGeneration`: Kiểm tra generator sinh đúng NetworkPolicy block CIDR `169.254.169.254/32`.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Lệnh STONITH bắt buộc phải có ít nhất 2 kênh kiểm tra độc lập (ví dụ vừa ping network vừa kiểm tra IPMI power state) trước khi kết luận node đã chết để tránh bắn nhầm node đang sống.
