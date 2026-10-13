# Task 031 — Automated Canary Progressive Delivery & Adaptive Load Shedding (P1)
# Checkpoint: 2026-10-12T10:35:00+07:00

> **Severity**: P1 (Zero-Downtime Progressive Delivery, Automated Rollback & Cascading Failure Prevention)  
> **Target Agent**: `backend-coder`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Zero Big-Bang Deployment, Statistical Anomaly Canary Abort, Adaptive Load Shedding  

---

## 1. BỐI CẢNH KỸ THUẬT & PHÂN TÍCH LỖI THỰC TẾ (ROOT CAUSE ANALYSIS)

### Lỗi 1: Triển Khai "Big Bang" 100% Gây Sập Toàn Diện Khi Có Lỗi Ẩn
* **Hiện tượng thực tế**: Phiên bản mới của ứng dụng vượt qua toàn bộ unit test ở staging, nhưng khi deploy 100% lên Production thì dính lỗi memory leak hoặc lỗi deadlock database dưới tải lớn (Production Load).
* **Hậu quả thảm khốc**: Toàn bộ các pod đồng loạt dính CrashLoopBackOff hoặc phản hồi chậm. 100% người dùng bị gián đoạn dịch vụ. Kỹ sư phải cuống cuồng mở laptop gõ lệnh rollback thủ công trong sự hoảng loạn.

### Lỗi 2: Thiếu Cơ Chế Adaptive Load Shedding Gây Sụp Đổ Domino (Cascading Collapse)
* **Hiện tượng thực tế**: Khi cơ sở dữ liệu bị quá tải đột ngột (CPU 95%, Connection Pool cạn kiệt), các API pod tiếp tục nhận thêm hàng ngàn request mới, khiến hàng đợi request phình to vô tận, RAM cạn kiệt và toàn bộ cụm pod chết chùm.
* **Căn nguyên**: Hệ thống thiếu cơ chế "xả lũ thông minh" (Load Shedding) để chủ động từ chối các request thứ yếu (non-critical) và bảo vệ giao dịch cốt lõi.

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/promotion/canary.go`:
```go
package promotion

import (
    "context"
    "time"
)

type CanaryStep struct {
    TrafficWeightPercentage int           `json:"traffic_weight_percentage"` // 10%, 25%, 50%, 100%
    EvaluationDuration      time.Duration `json:"evaluation_duration"`       // 5m, 10m
}

type CanaryMetricsComparison struct {
    BaselineErrorRate float64       `json:"baseline_error_rate"`
    CanaryErrorRate   float64       `json:"canary_error_rate"`
    ErrorRateDelta    float64       `json:"error_rate_delta"` // Canary - Baseline
    BaselineP99       time.Duration `json:"baseline_p99"`
    CanaryP99         time.Duration `json:"canary_p99"`
    P99LatencyRatio   float64       `json:"p99_latency_ratio"`
    ResultVerdict     string        `json:"result_verdict"`   // PROCEED, WARN, ABORT
}

type CanaryReleaseExecution struct {
    ReleaseID          string                    `json:"release_id"`
    WorkloadName       string                    `json:"workload_name"`
    Namespace          string                    `json:"namespace"`
    CurrentStepIndex   int                       `json:"current_step_index"`
    CurrentWeight      int                       `json:"current_weight"`
    Status             string                    `json:"status"` // PROGRESSING, PROMOTED, ABORTED
    LatestMetrics      *CanaryMetricsComparison  `json:"latest_metrics,omitempty"`
    StartedAt          time.Time                 `json:"started_at"`
    UpdatedAt          time.Time                 `json:"updated_at"`
}

type ProgressiveDeliveryEngine interface {
    StartCanaryRelease(ctx context.Context, workload, namespace, newImage string, steps []CanaryStep) (*CanaryReleaseExecution, error)
    EvaluateCanaryHealth(ctx context.Context, releaseID string) (*CanaryMetricsComparison, error)
    PromoteNextStep(ctx context.Context, releaseID string) error
    AbortCanaryRollback(ctx context.Context, releaseID string, reason string) error
}
```

---

## 3. THUẬT TOÂN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Quy trình 1: Điều Phối Canary Tự Động (Automated Canary Analysis)
1. **Khởi tạo Canary**:
   - Tạo deployment phụ `workload-canary` chạy image mới.
   - Cập nhật Ingress (Traefik Service Weight / Nginx Annotations):
     - `workload-primary`: 90%
     - `workload-canary`: 10%
2. **Đo Lường & Đánh Giá Tự Động (Evaluation Loop)**:
   - Cứ mỗi 30 giây, truy vấn Prometheus:
     - Tỷ lệ lỗi 5xx của Canary so với Primary: `delta = canary_error_rate - primary_error_rate`.
     - Độ trễ P99 của Canary: `latency_ratio = canary_p99 / primary_p99`.
3. **Quyết Định Tự Động (Auto-Verdict)**:
   - **ABORT Tức Thời**: Nếu `delta > 0.5%` (Canary sinh thêm lỗi $> 0.5\%$) HOẶC `latency_ratio > 1.8` (chậm gần gấp đôi):
     - Lập tức chuyển traffic của Canary về **0%**.
     - Xóa bỏ pod `workload-canary`.
     - Gửi cảnh báo khẩn cấp về Telegram:
       `🚨 Canary Rollout bị hủy tự động! Phát hiện lỗi tăng cao (Delta: +%.2f%%). Đã hoàn nguyên 100% traffic an toàn.`
   - **PROCEED**: Nếu các chỉ số hoàn toàn ổn định trong suốt cửa sổ đánh giá (5 phút):
     - Tự động nâng tỷ lệ traffic lên nấc tiếp theo: $10\% \to 25\% \to 50\% \to 100\%$.
     - Khi đạt $100\%$, cập nhật primary deployment lên image mới và dọn dẹp canary.

### Quy trình 2: Cơ Chế Xả Lũ Thích Ứng (Adaptive Load Shedding)
1. Middleware API theo dõi chỉ số bão hòa CPU và DB pool:
   - Nếu `SystemLoad > 90%` hoặc `DBConnectionWaitCount > 20`:
     - Kích hoạt chế độ Load Shedding: Chấp nhận các request critical (thanh toán, đặt lệnh core).
     - Từ chối các request non-critical (analytics, avatar upload, notification preview) với:
       `HTTP 429 Too Many Requests` hoặc `HTTP 503 Service Unavailable` kèm header `Retry-After: 5`.
2. Tránh làm sập toàn bộ hệ thống trong các đợt Flash Sale hoặc traffic tăng đột biến.

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_CANARY_ABORTED_HIGH_ERRORS`: `HTTP 422 Unprocessable Entity` — "Canary release aborted: Error rate threshold exceeded (Delta: +%.2f%%, MaxAllowed: 0.5%%)."
* `ERR_SYSTEM_OVERLOAD_LOAD_SHEDDING`: `HTTP 503 Service Unavailable` — "System overload detected. Non-critical request shed to preserve core platform stability."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestCanary_AbortOnHighErrorRate`: Giả lập Canary sinh lỗi 5xx tỷ lệ 1.2% trong khi Primary là 0.1%. Assert hàm evaluate trả về `verdict == "ABORT"` và hệ thống lập tức hạ weight về 0.
2. `TestCanary_PromoteSuccessfulSteps`: Giả lập metrics Canary hoàn hảo trong 5 phút. Assert hệ thống tự động thăng cấp từ step 10% lên 25%.
3. `TestLoadShedding_ProtectsCriticalEndpoints`: Giả lập hệ thống đạt ngưỡng tải 95%. Gửi đồng thời 1 request thanh toán và 1 request analytics. Assert request thanh toán được xử lý, request analytics nhận về HTTP 503 `Retry-After: 5`.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Bất kỳ thao tác hạ weight về 0 trong quá trình abort canary bắt buộc phải có độ trễ $< 2$ giây ở tầng Ingress để chặn đứng dòng người dùng tiếp cận phiên bản lỗi.
