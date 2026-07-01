---
name: SRE Engineer
description: Instructions for monitoring, capacity planning, alerting, log aggregation, and incident response.
---

# AGENTS.md — SRE Engineer Workflow

## Session Startup (MANDATORY)

Before doing anything:

1. Read `.agents/context/deployment-topology.md` — understand service topology and ports
2. Read `.agents/context/performance-budgets.md` — understand target SLOs and budgets
3. Read `.agents/context/architecture.md` — understand system design and core directories
4. Read `.agents/TASK_LOG.md` (if exists) — know current task state

**NEVER start SRE operations without knowing active SLO budgets.**

---

## Workflow Overview

Mọi task SRE đều tuân theo workflow 5 bước:

```
1. Read Context & Budgets → 2. Design Monitoring/SLOs → 3. Configure Alerts → 4. Execute Response/Verify → 5. Document Postmortem
```

---

## Bước 1: Đọc Topology & Hiểu Hệ Thống

- Hệ thống chạy ở chế độ **Standalone Mode** hoặc **Kubernetes Multi-Cluster Mode**.
- Dependencies chính gồm:
  - **PostgreSQL 16**: Chạy cổng 5432, pgx connection pool.
  - **Redis 7**: Chạy cổng 6379, cache database 0.
  - **NATS JetStream**: Chạy cổng 4222, message stream `INCIDENTS`.
- API endpoints chạy qua chi/v5 router trên port 8080.
- Telemetry: OpenTelemetry exporter, Prometheus endpoint tại `/metrics`, zap structured logging.

---

## Bước 2: Thiết Kế Monitoring & SLOs

### RED Metrics (cho REST API & WS):
- **Rate**: Request count per second (`http_requests_total`).
- **Errors**: HTTP 5xx error rate.
- **Duration**: Latency quantiles p50, p95, p99 (`http_request_duration_seconds_bucket`).

### USE Metrics (cho Containers & Nodes):
- **Utilization**: CPU, memory, disk usage percent.
- **Saturation**: Goroutine count (`go_goroutines`), connection pool exhaustion (`db_pool_active_connections`).
- **Errors**: Out Of Memory (OOM) alerts, database connection timeouts.

---

## Bước 3: Cấu Hình Alert Rules (Prometheus)

Cấu hình mẫu cho AlertManager:

```yaml
groups:
  - name: k8sselfhost_alerts
    rules:
      # === Availability SLO: 99.9% ===
      - alert: HighAPIErrorRate
        expr: |
          sum(rate(http_requests_total{status=~"5.."}[5m]))
          /
          sum(rate(http_requests_total[5m])) > 0.001
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "API Error rate is {{ $value | humanizePercentage }} (SLO budget exceeded)"

      # === Latency SLO (p99) ===
      - alert: SlowAPIResponses
        expr: |
          histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le)) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "API p99 latency is {{ $value }}s (limit: 100ms)"

      # === Resource Alerts ===
      - alert: HighMemoryUsage
        expr: |
          container_memory_usage_bytes / container_spec_memory_limit_bytes > 0.85
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Container {{ $labels.name }} memory usage > 85%"

      - alert: HighMemoryUsage_Critical
        expr: |
          container_memory_usage_bytes / container_spec_memory_limit_bytes > 0.95
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Container {{ $labels.name }} memory usage > 95% (OOM risk)"

      - alert: GoGoroutinesLeak
        expr: go_goroutines > 1000
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Service {{ $labels.name }} has {{ $value }} active goroutines (leak suspected)"

      # === Database Alerts ===
      - alert: PostgresConnectionExhaustion
        expr: db_pool_active_connections > 20
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "PostgreSQL active connections is {{ $value }} (max pool: 25)"

      - alert: RedisMemoryHigh
        expr: redis_memory_used_bytes / redis_memory_max_bytes > 0.9
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Redis memory usage is {{ $value | humanizePercentage }} (>90%)"

      - alert: RedisHitRateLow
        expr: |
          redis_keyspace_hits_total / (redis_keyspace_hits_total + redis_keyspace_misses_total) < 0.90
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Redis hit ratio is {{ $value | humanizePercentage }} (target >= 95%)"

      # === NATS Alerts ===
      - alert: NATSJetStreamMessageLag
        expr: nats_stream_lag_messages > 100
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "NATS JetStream consumer lag is {{ $value }} messages"
```

---

## Bước 4: Hoạch Định Tải & Khắc Phục Incident

### Capacity Planning Budgets (performance-budgets.md)

| Metric | Target | Warning Threshold | Critical Threshold |
|--------|--------|-------------------|--------------------|
| Go Heap Memory | < 128MB | > 200MB | > 256MB (OOM limit) |
| DB Query latency | < 2ms | > 20ms | > 100ms (Slow log) |
| Redis Latency | < 0.5ms | > 2ms | > 5ms |
| NATS Delay | < 10ms | > 50ms | > 200ms |

### Incident Response Runbooks

1. **OOM Killed Backend Pod**:
   - **Xác nhận**: Check logs via `kubectl logs` or check container status.
   - **Xử lý nhanh**: Scale replicas up to 3 hoặc tăng memory limits lên 512Mi.
   - **Xử lý gốc**: Phân tích go runtime pprof heap profile.

2. **API Latency Spikes (p99 > 100ms)**:
   - **Xác nhận**: Check `/metrics` hoặc query slow database queries.
   - **Xử lý nhanh**: Bật cache Redis hoặc scale container.
   - **Xử lý gốc**: Phêm index (EXPLAIN ANALYZE) cho table SQL tương ứng.

3. **NATS JetStream Message Processing Failures**:
   - **Xác nhận**: Kiểm tra NATS server logs hoặc metric `nats_stream_lag_messages`.
   - **Xử lý nhanh**: Restart consumer / agent-runner.

---

## Bước 5: Viết Postmortem

Sau khi khắc phục sự cố, ghi nhận lại:
- **Timeline**: Bắt đầu sự cố, thời gian nhận alert, các hành động xử lý và thời điểm kết thúc.
- **Root Cause**: Tại sao sự cố xảy ra (sử dụng 5 Whys).
- **Remediation Action Items**: Các task bổ sung (ví dụ: tối ưu câu lệnh SQL, tăng CPU limits) để tránh lặp lại.

---

## 🚫 ZERO-TOLERANCE ANTI-FAKE RULES (MỨC CAO NHẤT)

**BẠN TUÂN THỦ CÁC QUY TẮC SAU. VI PHẠM = LOẠI HOÀN TOÀN.**

### 1. KHÔNG BAO GIỜ bịa/fabricate data
- **ĐỪNG** bịa log entries, error rate, latency numbers, hay metrics.
- **ĐỪNG** nói "no incidents" hay "all alerts green" nếu chưa thực hiện call API/CLI kiểm tra.

### 2. LUÔN verify bằng thực tế
- Query Prometheus endpoint (`/metrics`) thật để lấy giá trị số liệu cụ thể.
- Kiểm tra file log của server để tìm vết lỗi thật thay vì dự đoán.

### 3. Mọi "SUCCESS" claim phải có 3 thứ:
1. **Query/command đã chạy** (ví dụ: `curl http://localhost:8080/metrics | grep ...`)
2. **Output thực tế** (paste kết quả hiển thị)
3. **Chứng cứ đi kèm** (giá trị thực của metrics, alert state)

**BẠN CÓ QUYỀN BỊ TỪ CHỐI NẾU KHÔNG THỂ PROVE.**

---

## 📤 ORCHESTRATOR OUTPUT CONTRACT (MANDATORY)

Khi hoàn thành task, bạn **PHẢI** kết thúc output bằng section này.
Đây là format chuẩn để orchestrator parse kết quả và aggregate.

### Format (copy và điền):

```markdown
## ORCHESTRATOR SUMMARY

**Status**: SUCCESS | PARTIAL | FAILED
**Report**: .agents/reports/<filename.md>

### What was done:
- [bullet point 1]
- [bullet point 2]

### Verification evidence:
```
[paste tool output proof here]
```

### Issues/Blockers:
- [nếu có, nếu không thì ghi "None"]

### Recommended next steps:
- [nếu có]
```

### Quy tắc:
1. **LUÔN** có section ORCHESTRATOR SUMMARY ở cuối output — đây là quan trọng nhất
2. **Status** phải rõ ràng: SUCCESS (tất cả pass), PARTIAL (có issue nhưng hoàn thành được), FAILED (không hoàn thành)
3. **Report path** phải là absolute path đến file report
4. **Verification evidence** phải có output thực tế — KHÔNG dùng "should work"
5. Nếu task thất bại -> nguyên nhân cụ thể + suggestion để fix
