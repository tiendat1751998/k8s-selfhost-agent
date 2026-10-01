---
name: ui-ux-promax
description: Workflow for generating agency-grade UI/UX designs and handing them off to frontend-coder using UI/UX Pro Max engine.
activation: always_on
---

# 🎨 UI/UX Pro Max Workflow & Design System Protocol

## 1. Engine Location & CLI
The **UI/UX Pro Max** design intelligence engine is located at:
`D:\MCP-server\UIUXPROMAX`

It provides searchable local databases containing 79 UI styles, 192 color palettes, 74 font pairings, 119 UX guidelines, 25 chart types, and 22 framework stacks.

### Available Commands:
```bash
# 1. Generate full design system (Pattern, Style, Colors, Typography, Key Effects, Checklist)
python D:\MCP-server\UIUXPROMAX\src\ui-ux-pro-max\scripts\search.py "<product/feature>" --design-system

# 2. Query specific domain (style, color, typography, icons, chart, ux, landing)
python D:\MCP-server\UIUXPROMAX\src\ui-ux-pro-max\scripts\search.py "<query>" --domain <domain>

# 3. Query Vue 3 stack best practices (Composition API, Pinia, Reactivity)
python D:\MCP-server\UIUXPROMAX\src\ui-ux-pro-max\scripts\search.py "<topic>" --stack vue
```

---

## 2. Division of Labor (Separation of Powers)

### A. Phase Thiết kế (`ux-designer`)
1. **Query UI/UX Pro Max**: Chạy lệnh `--design-system` với từ khóa của feature (ví dụ: `"kubernetes cluster nodes dashboard"`).
2. **Khai thác Design System cốt lõi**: Đối chiếu với `design-system/k8sselfhost/MASTER.md` để đảm bảo bảng màu (`#0F172A`, `#22C55E`, `#1B2336`), font (`JetBrains Mono` + `IBM Plex Sans`), và khoảng cách nhất quán.
3. **Tạo Interactive Preview**: Dùng skill `generative_ui` render widget HTML/CSS trực tiếp trong chat để user và coder preview trước.
4. **Xuất bản Handover Spec cho `frontend-coder`**:
   - Bảng biến CSS / Tokens
   - Sơ đồ phân rã Sub-components (< 300 dòng / file)
   - Bảng RWD 4-tier: Mobile (375px), Tablet (768px), Desktop (1440px), 4K
   - 5 trạng thái: Initial, Loading (Skeleton), Empty (CTA), Error, Interactive (Hover/Active/Focus)

### B. Phase Lập trình (`frontend-coder`)
1. Nhận Handover Spec từ `ux-designer`.
2. Kiểm tra Vue conventions nếu cần: `python D:\MCP-server\UIUXPROMAX\src\ui-ux-pro-max\scripts\search.py "<topic>" --stack vue`.
3. Viết Single File Component (SFC) với `<script setup lang="ts">`, tách logic vào composables `src/composables/`, style vào `src/assets/styles/`.
4. Không vượt quá 500 dòng/file (chuẩn dưới 300 dòng).
5. Self-verify bằng `npm.cmd run build` hoặc type-check trước khi bàn giao.
