# Task 034 — Message Queue & Streaming SRE Auto-Tuner (Kafka, RabbitMQ, NATS) (P1)
# Checkpoint: 2026-10-12T10:45:00+07:00

> **Severity**: P1 (Message Stream Liveness, Rebalance Storm Mitigation & Memory Deadlock Prevention)  
> **Target Agent**: `backend-coder` / `database-engineer`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Cooperative Rebalance Enforcement, Quorum Queue Segregation, Dirty Memory Throttling  

---

## 1. BỐI CẢNH KỸ THUẬT & PHÂN TÍCH LỖI THỰC TẾ (ROOT CAUSE ANALYSIS)

### Lỗi 1: Kafka Consumer Group Rebalance Storm Khi Worker Node Drain
* **Hiện tượng**: Khi nâng cấp cụm K8s hoặc drain node, các consumer pod bị restart lần lượt. Giao thức Eager Rebalance mặc định thu hồi (revoke) toàn bộ partition của TẤT CẢ consumer trong group. Xử lý tin nhắn đóng băng hàng chục phút, consumer lag nhảy vọt hàng triệu messages.
* **Cách xử lý triệt để**:
  1. Tự động kiểm tra và cấu hình `partition.assignment.strategy` bắt buộc dùng `org.apache.kafka.clients.consumer.CooperativeStickyAssignor` (KIP-429: Incremental Cooperative Rebalance). Khi 1 pod restart, các pod khác tiếp tục tiêu thụ dữ liệu bình thường, chỉ partition của pod chết bị tái phân bổ.
  2. Bổ sung cấu hình `group.instance.id` (KIP-345: Static Membership) cho các StatefulSet consumer pods. Khi pod restart trong vòng `session.timeout.ms = 45000`, coordinator giữ nguyên partition mapping mà không kích hoạt rebalance.

### Lỗi 2: RabbitMQ High Memory Watermark Gây Deadlock Hàng Đợi
* **Hiện tượng**: RabbitMQ broker chạm ngưỡng RAM `vm_memory_high_watermark = 0.4`, tự động kích hoạt flow control chặn kết nối socket của publisher. Nếu ứng dụng dùng chung 1 connection TCP cho cả publish lẫn consume, toàn bộ hệ thống rơi vào deadlock vĩnh viễn.
* **Cách xử lý triệt để**:
  1. Linter kiểm tra code client: bắt buộc tách biệt connection pool cho Producer và Consumer.
  2. Tự động chuyển đổi các hàng đợi truyền thống (classic queues) sang **Quorum Queues** (dựa trên Raft consensus engine có tính năng streaming xuống disk liên tục).
  3. Cấu hình `vm_memory_high_watermark_paging_ratio = 0.5` để chủ động flush page cache xuống disk trước khi chạm ngưỡng khóa kết nối.

### Lỗi 3: Kafka Under-Replicated Partitions (URP) Do Linux `vm.dirty_ratio` Flush Stall
* **Hiện tượng**: Máy chủ Linux tích tụ hàng chục GB dirty pages rồi xả đồng loạt, nghẽn I/O disk khiến Kafka broker đơ trong 20s, rớt heartbeat, kích hoạt bầu cử leader liên tục.
* **Cách xử lý triệt để**:
  - Tự động áp dụng profile tinh chỉnh sysctl cho Kafka node:
    ```bash
    vm.dirty_background_ratio = 3
    vm.dirty_ratio = 10
    vm.dirty_expire_centisecs = 1000
    ```

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/messaging/sre.go`:
```go
package messaging

import (
    "context"
    "time"
)

type KafkaGroupHealth struct {
    GroupID           string        `json:"group_id"`
    Topic             string        `json:"topic"`
    PartitionCount    int           `json:"partition_count"`
    ConsumerCount     int           `json:"consumer_count"`
    TotalLag          int64         `json:"total_lag"`
    AssignmentStrategy string       `json:"assignment_strategy"`
    IsCooperative     bool          `json:"is_cooperative"`
    RebalanceCount24h int           `json:"rebalance_count_24h"`
    Status            string        `json:"status"` // HEALTHY, REBALANCING_STORM, DEGRADED
}

type RabbitMQHealthReport struct {
    ClusterName       string  `json:"cluster_name"`
    MemoryUsedBytes   int64   `json:"memory_used_bytes"`
    MemoryLimitBytes  int64   `json:"memory_limit_bytes"`
    MemoryPercentage  float64 `json:"memory_percentage"`
    FlowBlockedConns  int     `json:"flow_blocked_conns"`
    QuorumQueueCount  int     `json:"quorum_queue_count"`
    ClassicQueueCount int     `json:"classic_queue_count"`
    Status            string  `json:"status"`
}

type MessageQueueSREManager interface {
    InspectKafkaConsumerGroups(ctx context.Context, clusterID string) ([]KafkaGroupHealth, error)
    EnforceKafkaCooperativePolicy(ctx context.Context, clusterID, groupID string) error
    AuditRabbitMQMemoryAndTopology(ctx context.Context, clusterID string) (*RabbitMQHealthReport, error)
    ApplyKafkaKernelTuning(ctx context.Context, nodeName string) error
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Quy trình 1: Phát Hiện & Dập Tắt Kafka Rebalance Storm
1. Cứ mỗi 30 giây, background poller gọi `AdminClient.DescribeConsumerGroups`:
   - Nếu `State == "CompletingRebalance"` hoặc `State == "PreparingRebalance"` liên tục quá 3 lần:
     $\implies$ Kích hoạt cảnh báo P1 `ALERT_KAFKA_REBALANCE_STORM`.
2. Kiểm tra `AssignmentStrategy` của các consumer pod:
   - Nếu phát hiện đang dùng `RangeAssignor` hoặc `RoundRobinAssignor` cũ:
     - Gửi đề xuất bản vá YAML tự động cập nhật `partition.assignment.strategy` sang `CooperativeStickyAssignor`.
     - Inject `group.instance.id` theo pod ordinal: `fmt.Sprintf("%s-%s", deploymentName, podIndex)`.

### Quy trình 2: Tự Động Giải Cứu RabbitMQ Khi Chạm Memory Watermark
1. Đọc Prometheus metric `rabbitmq_process_resident_memory_bytes`:
2. Nếu `MemoryPercentage > 85%`:
   - Gửi tín hiệu điều phối đến các producer client: Kích hoạt Exponential Backoff delay 500ms cho mỗi lệnh publish.
   - Gọi API RabbitMQ kích hoạt Garbage Collection memory: `POST /api/nodes/{node}/memory/purge`.
   - Cảnh báo SRE kèm danh sách các queue có dung lượng lớn nhất chưa được chuyển sang Quorum Queue.

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_KAFKA_REBALANCE_STORM`: `HTTP 500 Internal Server Error` — "Kafka consumer group '%s' is stuck in rebalance storm (Lag: %d, Rebalances: %d)."
* `ERR_RABBITMQ_FLOW_BLOCKED`: `HTTP 503 Service Unavailable` — "RabbitMQ node memory threshold exceeded (Memory: %.2f%%). Publishers flow blocked."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestKafka_DetectRebalanceStorm`: Giả lập nhóm consumer liên tục đổi state trong 5 phút. Assert hàm inspect phát hiện status `REBALANCING_STORM`.
2. `TestKafka_InjectCooperativeConfig`: Nạp cấu hình consumer mặc định. Assert hàm mutate chèn đúng `CooperativeStickyAssignor`.
3. `TestRabbitMQ_DetectMemoryAlarm`: Mock RabbitMQ trả về memory usage 92%. Assert hệ thống kích hoạt backpressure và phát cảnh báo P1.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Khi chuyển đổi assignment strategy của Kafka consumer group, bắt buộc rolling restart từng instance một để tránh làm gãy toàn bộ consumer group.
