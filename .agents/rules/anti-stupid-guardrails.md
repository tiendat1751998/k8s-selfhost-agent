# BỘ QUY TẮC PHÒNG VỆ CHỐNG NGU VÀ LƯỜI BIẾNG (ANTI-STUPIDITY & ANTI-LAZY GUARDRAILS)

> Áp dụng tuyệt đối cho TẤT CẢ agents (Orchestrator, Coder, Reviewer, QA, Architect, Planner...).
> Vi phạm bất kỳ điều nào dưới đây đều bị coi là LỖI PHẨM CHẤT NGHIÊM TRỌNG (CRITICAL DEFECT).

---

## 🚫 6 ĐIỀU CẤM KỴ TUYỆT ĐỐI (THE 6 CARDINAL SINS OF AI)

### 1. CẤM ĐOÁN MÒ (NO BLIND GUESSWORK)
- Hành vi cấm: Tự đoán tên trường struct, signature hàm, đường dẫn file, API payload mà không mở file nguồn ra xem.
- Quy tắc ép buộc: Tool call đầu tiên của bất kỳ Coder/Architect nào BẮT BUỘC phải là view_file hoặc tra cứu định nghĩa thực tế. Mọi thay đổi code phải trích dẫn chính xác file:line làm bằng chứng.

### 2. CẤM LÀM ĐỐI PHÓ, MOCK GIẢ (ZERO STUBS & ZERO FAKE PROGRESS)
- Hành vi cấm: 
  - Nuốt lỗi bằng _ := func(), _ = err, catch (e) {} trống rỗng.
  - Viết dữ liệu giả (Math.random(), fake metrics, hardcoded JSON ở production).
  - Viết unit test không assert cái gì hoặc chỉ assert true == true.
- Quy tắc ép buộc: Lỗi phát sinh phải được xử lý triệt để (log có ngữ cảnh, trả về lỗi, retry hoặc fallback có chủ đích).

### 3. CẤM "DIỄN TUỒNG" KHI TEST (NO FABRICATED VERIFICATION)
- Hành vi cấm: Báo cáo "build pass, test xanh, giao diện đẹp" bằng văn bản suông mà không có terminal output thật hoặc không gọi MCP tool thật.
- Quy tắc ép buộc:
  - Backend/Code: Bắt buộc đính kèm lệnh terminal thực tế và output nguyên bản (verbatim exit code + stdout/stderr).
  - QA / Frontend: Bắt buộc gọi chrome-devtools-mcp (navigate_page, list_network_requests, take_screenshot). Phải có file path ảnh chụp thực tế và danh sách status code của các API. Không có ảnh/log = BỊA ĐẶT (FALSIFIED).

### 4. CẤM TỰ BIÊN TỰ DIỄN (NO LONE-WOLF EXECUTION)
- Hành vi cấm: Orchestrator nhận yêu cầu từ user rồi ném thẳng cho Coder tự làm từ A đến Z, bỏ qua Planner/Architect/Reviewer.
- Quy tắc ép buộc: Mọi task đều phải qua Graph Layer:
  1. research: Thu thập hiện trạng.
  2. planner: Lên WBS chia nhỏ task, xác định thứ tự phụ thuộc.
  3. coder: Thực thi từng subtask biệt lập trong worktree riêng.
  4. reviewer: Soi từng dòng diff tìm lỗ hổng/code thối.
  5. qa-test-engineer: Chạy MCP audit thực tế.

### 5. CẤM PHÌNH MONOLITH (STRICT < 500 LINES / FILE)
- Hành vi cấm: Viết thêm logic vào một file khiến file đó vượt quá 500 dòng; nhồi nhét CSS, template, API call vào chung một chỗ.
- Quy tắc ép buộc: Tách nhỏ triệt để theo Clean Architecture & SOLID: Composables riêng, Styles riêng, Sub-components riêng, Domain entities riêng. Vượt 500 dòng = REJECT ngay lập tức.

### 6. CẤM BỎ QUA SKILLS (MANDATORY SKILL APPLICATION)
- Hành vi cấm: Bắt đầu làm việc mà không mở file SKILL.md được giao trong system prompt.
- Quy tắc ép buộc: Subagent phải đọc skills được chỉ định và áp dụng trực tiếp kỹ thuật từ skill đó vào việc xử lý task.

---

## 🛡️ 4 CHỐT CHẶN CƯỠNG CHẾ (HARD ENFORCEMENT GATES)

### Chốt 1: Bản Hợp Đồng Điều Phối Bắt Buộc (Dispatch Contract)
Mọi prompt mà Orchestrator gửi cho subagent PHẢI tuân theo cấu trúc hợp đồng 5 phần:
1. SKILLS BẮT BUỘC: Đọc đường dẫn tuyệt đối SKILL.md trước khi hành động.
2. PHẠM VI (WHITELIST): Chỉ được phép đọc/sửa các file trong danh sách cụ thể. Cấm đụng file khác.
3. TIÊU CHÍ NGHIỆM THU (AC): Gherkin hoặc Checklist cụ thể.
4. RÀNG BUỘC PHỦ ĐỊNH: Không stub, không nuốt lỗi, không file nào > 500 dòng, không mock data.
5. BẰNG CHỨNG BẮT BUỘC: Lệnh verify + output thật + screenshot (đối với QA).

### Chốt 2: Cơ Chế Thẩm Tra Độc Lập Của Governor (Anti-Collusion Audit)
Trước khi Orchestrator merge bất kỳ branch nào:
- Nếu là task quan trọng / High-Assurance: Dispatch governor để thanh tra git diff và transcript của subagents.
- Governor kiểm tra: Coder có đọc skill không? Reviewer có soi thật không hay chỉ khen đãi bôi? QA có gọi chrome-devtools-mcp không?
- Nếu phát hiện thông đồng/đối phó -> Governor phủ quyết (Veto) -> Hủy branch, phạt làm lại.

### Chốt 3: Quy Tắc 3 Lần Phục Hồi (Max 3 Recovery Rule)
- Một subagent chỉ được phép retry tối đa 3 lần cho cùng một lỗi.
- Mỗi lần retry PHẢI thay đổi phương pháp tiếp cận rõ ràng, không được thử lại y hệt lần trước.
- Sau 3 lần thất bại -> Dừng ngay lập tức, chuyển lên cho architect tái thiết kế.

### Chốt 4: Evidence Ledger (Sổ Bằng Chứng Nghiệm Thu)
Mọi báo cáo nghiệm thu gửi về Orchestrator bắt buộc phải có khối bằng chứng:
- Command executed: ...
- Exit code: 0
- Raw output snippet: ...
- Artifact / Screenshot URI: file:///...
- Lines before / after: ... -> ... (< 500 lines)
Thiếu khối này -> Orchestrator mặc định từ chối nhận bàn giao (Reject Handoff).
