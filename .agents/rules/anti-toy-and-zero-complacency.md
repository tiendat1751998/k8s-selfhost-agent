# ANTI-TOY & ZERO-COMPLACENCY BINDING RULE

> Mandatory constitutional directive for all agents and subagents operating in `k8sselfhost`.
> Imposed: 2026-10-12. Priority: SUPREME (Overrides all assumptions of completion).

### CRITICAL CONSTRAINT: ZERO-COMPLACENCY & ANTI-TOY IMPLEMENTATION (BẮT BUỘC)

1. **Chống triển khai sơ sài (Anti-Toy Project):**
   - Tuyệt đối không sinh mã giả (pseudo-code), không viết code kiểu tutorial/minh họa rút gọn, không dùng `// TODO: xử lý sau`.
   - Mọi cấu trúc dữ liệu tải cao phải thể hiện rõ: cơ chế cấp phát bộ nhớ (allocation profiling), căn chỉnh bộ đệm (cache-line padding), kiểm soát tranh chấp luồng (lock-free/CAS), và cơ chế backpressure thực tế.

2. **Cấm tự mãn và ngộ nhận chuẩn hóa (Anti-Complacency):**
   - Cấm sử dụng các cụm từ khẳng định chủ quan như: *"Hệ thống này đã hoàn hảo"*, *"Đạt chuẩn production 100%"*, *"Không còn điểm nghẽn nào"*.
   - Mọi kiến trúc luôn đi kèm đánh đổi (trade-offs). Phải giữ thái độ nghiêm cẩn về mặt kỹ thuật phần cứng (Mechanical Sympathy).

3. **Bắt buộc phân tích điểm gãy (Failure Modes & Edge Cases):**
   - Khi đưa ra bất kỳ đoạn mã hay kiến trúc nào, BẮT BUỘC phải đính kèm:
     - Điểm nghẽn phần cứng thực tế (CPU context-switch, cache bouncing, băng thông PCIe, giới hạn ngắt mạng NIC).
     - Kịch bản hệ thống có thể bị sập (ví dụ: memory leak khi client treo kết nối, starvation khi ring buffer bị tràn, GC pause đột biến khi heap phình to).
     - Điều kiện biên bắt buộc phải đo đạc (benchmark baseline) trước khi đưa vào môi trường thật.
