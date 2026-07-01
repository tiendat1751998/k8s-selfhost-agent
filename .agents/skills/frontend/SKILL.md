---
name: Frontend Engineer
description: Instructions for developing premium dashboard views, styles, layouts, and responsive interfaces.
---

# AGENTS.md — Frontend Engineer Workflow

## Session Startup (MANDATORY)

Before doing anything:

1. Read `.agents/context/deployment-topology.md` — know the backend endpoint and structure
2. Read `.agents/context/api-contracts.md` — know the REST API endpoints and WebSocket messages
3. Read `.agents/context/architecture.md` — know the directories and layer boundaries
4. Read `.agents/TASK_LOG.md` (if exists) — know current task state

**NEVER start modifying frontend files without reading the API contract.**

---

## Technical Stack

- **Core**: Vanilla HTML5, Vanilla JavaScript (ES6 Modules).
- **Styling**: Vanilla CSS3 (Custom Properties / CSS Variables).
- **Icons / Assets**: SVG and custom UI badges.
- **Data Transfer**: `fetch` API, WebSocket (event-based updates).

---

## Workflow Overview

Mọi task frontend đều tuân theo workflow 4 bước:

```
1. Read API Contract → 2. Implement Component/View → 3. Style with CSS classes → 4. Verification
```

---

## Bước 1: Read API Contract

- Xác định REST endpoint cần gọi từ `.agents/context/api-contracts.md`
- Xác định payload structure và headers (Authorization Bearer token)
- Xác định WebSocket messages (ví dụ: incident alerts) để cập nhật UI realtime
- Kiểm tra `frontend/core/services/api-client.js` xem đã có REST method tương ứng chưa để tái sử dụng

---

## Bước 2: Implement Component/View

Khi tạo hoặc sửa các component/view:

- **Directory Structure**:
  - `frontend/core/` — shared services (api-client, websocket, router)
  - `frontend/components/` — reusable components (dialogs, charts, panels)
  - `frontend/modules/` — page modules (incidents, drift, fleet, audit, etc.)
  - `frontend/css/` — CSS stylesheets
- **JS Length Limit**: Giữ file JS dưới **500 dòng**. Tách logic ra helper hoặc helper modules.
- **Vanilla JS Component Pattern**: Đăng ký component/module thông qua router/app controller.
- **XSS Prevention**: Validate và sanitize input nhận vào. Dùng `textContent` thay cho `innerHTML` cho untrusted strings. Dùng sanitizer utility trong `frontend/core/utils/sanitizer.js`.
- **Zero Mock Policy**: Kết nối trực tiếp vào API qua `api-client.js`. Không dùng `setTimeout` giả lập độ trễ API.

---

## Bước 3: Style with CSS Classes

- **Harmonious Palette (HSL)**: Sử dụng các biến CSS được định nghĩa trong `frontend/css/styles.css` và `enterprise.css` (ví dụ: `var(--color-canvas-dark)`, `var(--color-primary)`). Không dùng primary red, blue, green thuần.
- **CSS Class Management**: Thay đổi style state qua CSS class (ví dụ: `element.classList.add('active')`). KHÔNG viết inline style attribute trong Javascript (trừ trường hợp dynamic position/chart calculations).
- **Responsive design**:
  - Mobile-first approach (styles mặc định cho mobile, dùng media queries `@media (min-width: ...)` cho tablet/desktop).
  - Breakpoints:
    - 640px (large phones)
    - 768px (tablets)
    - 1024px (laptops/desktops)
- **Touch targets**: Đảm bảo nút bấm tối thiểu 44x44px trên mobile.

---

## Bước 4: Verification

- Chạy cục bộ bằng Go server (`make run` hoặc `go run ./cmd/server`)
- Mở trình duyệt, đăng nhập bằng tài khoản Platform Admin (`admin@k8sselfhost.local` / `admin`)
- Thực hiện kiểm tra trực quan (Visual validation):
  - Kiểm tra console log xem có báo lỗi Javascript đỏ không (Error-free)
  - Kiểm tra network tab xem API requests/responses có đúng schema không
  - Kiểm tra responsive bằng Device Mode trong Chrome DevTools
  - Kiểm tra real-time update qua WebSocket

---

## 🚫 ZERO-TOLERANCE ANTI-FAKE RULES (MỨC CAO NHẤT)

**BẠN TUÂN THỦ CÁC QUY TẮC SAU. VI PHẠM = LOẠI HOÀN TOÀN.**

### 1. KHÔNG BAO GIỜ bịa/fabricate data
- **ĐỪNG** bịa kết quả test UI, response mock, hay CSS verification.
- **ĐỪNG** nói "UI works" nếu chưa chạy server thật và verify trên browser.
- **ĐỪNG** nói "API integrated" nếu chưa verify network tab của browser.
- **ĐỪNG** tạo file rồi nói "đã tạo" mà chưa verify file tồn tại.

### 2. LUÔN verify bằng thực tế
- Mọi claim hoạt động phải đi kèm với **chạy thử nghiệm**:
  - `curl -I http://localhost:8080/` để kiểm tra web server hoạt động.
  - Phân tích log của Go server xem có request từ client đến không.
  - Test browser qua công cụ tự động hoặc kiểm thử bằng mắt và paste log/chứng cứ.

### 3. ĐỪNG dùng placeholder
- Không dùng placeholder text hoặc fake charts. Dùng data thật từ DB/API.

### 4. Mọi "SUCCESS" claim phải có 3 thứ:
1. **Command/hành động đã chạy** (ví dụ: `go run ./cmd/server` & click tabs)
2. **Output thực tế** (console logs, network logs, console warnings)
3. **Chứng cứ liên quan** (file diff, line numbers, CSS classes used)

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
[paste tool output proof here — e.g. curl response, browser log, file check]
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
