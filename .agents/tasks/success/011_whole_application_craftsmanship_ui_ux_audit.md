# Task 011: Whole-Application UI/UX Craftsmanship Audit & Mobile PWA Command Bar Overhaul

> **Supreme Directive**: Quality & Craftsmanship > Speed. Zero tolerance for superficial progress ("làm đối phó"). Slower is fine; take all necessary hours/days to achieve high-end Enterprise polish.
> **Binding Constitution**: Section 00 in `AGENTS.md` and `.agents/rules/vbc-verification.md`.

---

## 1. Executive Summary & Problem Diagnosis

### Root Defect Identified via Chrome DevTools MCP Live Audit:
Every single view in `frontend-vue/src/views/` uses a desktop-centric template structure:
1. `.view-header`: Renders an uppercase tag, giant 2-line H1, and 3-5 line paragraph descriptions.
2. `header-actions`: Renders multiple rows of bulky full-width buttons.
3. `.metrics-grid`: Renders 4 to 6 large KPI cards that stack vertically on mobile into 400px - 500px of height (or leave awkward empty gaps on odd counts like 5 cards).
4. **Result on Mobile (<640px)**: 80% to 90% of the first screen is completely consumed by headers, descriptions, buttons, and cards before a user can see a single incident, workload, deployment, node, or backup policy.
5. **Button Inconsistencies**: Buttons have mixed heights (24px, 32px, 36px, 42px), double emojis (`[ 🚀 Deployments 🚀 Apps ]`), cut-off badge text (e.g., `ATTACHEI` on `/backup`), and missing search icons.

---

## 2. Global Design Standard (Tier 1 to Tier 4)

### Tier 1: Mobile PWA (< 640px)
- **Top HUD**: 48px fixed (`[ ☰ ] K8SCONTROL [ 🔍 ] [ 🔔 5 ]`).
- **Slim View Command Bar (40px - 44px)**:
  - Left: View Title + Count badge (e.g. `Incidents (50)` or `Deployments (28)`).
  - Right: Compact 32px icon action buttons (`[ 🔍 ]`, `[ 🔄 ]`, `[ + ]`).
- **Micro-Telemetry / Stats (20px)**:
  - Replace 4-card / 5-card bulky KPI grids with a 1-line micro-stat bar:
    `⚡ 50 incidents · 🔥 50 critical · 🤖 0 active · 🛡️ 0 resolved`
  - Zero horizontal scrollbar, zero cutoffs.
- **Mobile Card Stream (~70px/item)**:
  - The actual operational list/stream starts immediately on the first screen.
  - Users see 4-5 workloads, incidents, or nodes on the first screen without scrolling!
- **Viewport Ergonomics**:
  - Zero double scrollbars on the page body.
  - Native touch inertia scrolling (`-webkit-overflow-scrolling: touch;`).

### Tier 2: Tablet (768px - 1024px)
- Auto-collapsing 64px icon sidebar.
- 2x2 metric KPI grid.
- Tables show primary 4-5 columns, drawer for extra attributes.

### Tier 3: Desktop Full HD (1440px - 1920px)
- Standard 260px sidebar navigation.
- Full 4-column metric cards HUD.
- 100% data table with standardized 32px labeled action buttons.

---

## 3. Phased Execution Ledger (6 Waves)

| Wave | Scope | Views / Components | Status | Target Defect Remediation |
|---|---|---|---|---|
| **Wave 1** | Core Observability & SRE | `/`, `/incidents`, `/slo`, `/logs`, Drawer Terminal | 🟢 COMPLETED | Odd 5th KPI card, Trends Deep-Dive duplication, 400px stacked KPI cards on `/incidents`, multi-window button clutter on `/slo`, Drawer Terminal text squish & cutoffs fixed, fake mock log generator eradicated, 100% verified with Chrome DevTools MCP ✅ |
| **Wave 2** | Compute & Fleet Orchestration | `/fleet`, `/deployments`, `/explorer`, `/hosts`, `/docker`, `/helm`, `/promotions` | 🟢 COMPLETED | Missing search icon, 5-line subtitle on Deployments, 6 stacked buttons on Explorer, carousel cutoff, swarm cards, /docker, /helm, /promotions mobile command bar (40px) + micro-telemetry (20px), 100% verified with Chrome DevTools MCP ✅ |
| **Wave 3** | Security, Governance & DR | `/backup`, `/audit`, `/security`, `/compliance`, `/drift` | 🟢 COMPLETED | Truncated `ATTACHEI` badge on Backup, 4 stacked cards on DR, tamper-evident audit ledger, CVE severity bars, 40px Command Bar + 20px micro-telemetry on all 5 views, 100% verified with Chrome DevTools MCP ✅ |
| **Wave 4** | Automation, FinOps & Ops | `/automation`, `/runbooks`, `/cost`, `/capacity`, `/changes` | 🟢 COMPLETED | Runbook execution drawers, FinOps idle cost charts on mobile, capacity exhaustion cards, JSON telemetry defect eradicated, 40px Command Bar + 20px micro-telemetry on all 5 views, 100% verified with Chrome DevTools MCP ✅ |
| **Wave 5** | Enterprise Management & Hub | `/tenancy`, `/ai-hub`, `/alerts`, `/reports`, `/catalog`, `/scaffolder`, `/settings` | 🟡 PARTIAL (5A DONE, 5B QUEUED) | Wave 5A Completed & MCP-Verified: Tenancy, AI Hub, Alerts, Reports. Wave 5B Queued: Catalog, Scaffolder, Settings |
| **Wave 6** | Global Shell & Button Polish | Top HUD, Sidebar, Bottom Bar, Modals | ⚪ QUEUED | Standardize all action buttons across all views to 32px height, verify all touch targets on real devices |

---

## 4. Wave 1 Detailed Specification: Core Observability & SRE

### Subtask 1.1: Overview (`/`) Refinement & Modularization Overhaul (COMPLETED & VERIFIED ✅)
- **Files**: `frontend-vue/src/views/OverviewView.vue` (331 lines < 350 lines), `frontend-vue/src/assets/styles/views/overview.css`, `frontend-vue/src/composables/useOverviewDashboard.ts` (455 lines), `frontend-vue/src/composables/useNodeDiagnostics.ts` (138 lines), `frontend-vue/src/components/overview/hud/OverviewHud.vue` (434 lines), `frontend-vue/src/components/overview/hud/OverviewSaturationTrends.vue` (447 lines).
- **Remediation Delivered**:
  1. Monolith completely eradicated: 1,352-line file shredded down to 331 lines. All inline `<style scoped>` extracted into `overview.css`.
  2. Mobile 44px Command Bar with status dot and compact actions (`[ 🔄 ]`, `[ 📊 Deep-Dive ]`).
  3. Odd 5th KPI card fixed: On mobile and tablet, Card 5 (Storage) cleanly spans 100% full width (`grid-column: span 2` / `grid-column: 1 / -1`), leaving zero awkward gaps.
  4. Duplicate `🔍 Deep-Dive` button removed from Saturation Trends card.
  5. Verified via Chrome DevTools MCP: Desktop (1440x900) and Mobile (375x812) verified with 0 errors. All line count invariants strictly respected.

### Subtask 1.2: Incidents Command Center (`/incidents`) Mobile Overhaul
- **Files**: `frontend-vue/src/views/IncidentsView.vue`, `frontend-vue/src/assets/styles/views/incidents.css`
- **Defects to Fix**:
  1. On mobile (<640px): Completely suppress the 3-line subtitle and bulky H1.
  2. Replace the 4 full-width stacked metric cards (400px height!) with an ultra-compact 20px micro-stat bar:
     `⚡ 50 incidents · 🔥 50 critical · 🤖 0 active · 🛡️ 0 resolved`.
  3. Show `Incident Queue` immediately at the top of the screen so 4-5 incident items are visible without scrolling!
  4. Ensure button heights are standard 32px with clean icons.

### Subtask 1.3: SLO & Error Budgets (`/slo`) Mobile Overhaul (COMPLETED & VERIFIED ✅)
- **Backend Files**: `internal/usecase/slo/seed.go` (93 lines), `internal/usecase/slo/seed_test.go` (115 lines), `cmd/standalone/main.go`.
- **Frontend Files**: `frontend-vue/src/views/SLOView.vue` (329 lines < 350 lines), `frontend-vue/src/assets/styles/views/slo.css` (356 lines), `frontend-vue/src/components/slo/SloCardsGrid.vue` (346 lines < 450 lines).
- **Remediation Delivered**:
  1. Automated seeding for 3 enterprise SLOs (`traefik`, `postgres`, `nats`) on backend startup with initial baseline snapshots.
  2. Mobile 44px Command Bar (`🎯 SLOs (3)` + `[ ➕ ]` `[ 🔄 ]` `[ 🔍 ]`).
  3. Micro-telemetry strip (20px): `🎯 3 slos · 🛡️ 3 ok · ⚠️ 0 warn · 🔥 0.00x burn`.
  4. Compact 1-row segmented pill strip for Multi-Window analysis (`1h`, `6h`, `24h`, `30d`).
  5. Hidden desktop 4-card metric stack on mobile, bringing the 3 live service cards directly to Screen 1 without scrolling!
  6. Standardized button heights to 32px (`[ 🔍 Inspect ]`, `[ ⚡ Simulate Burn ]`, `[ 🗑 Delete ]`).
  7. Verified via Chrome DevTools MCP: Desktop (1440x900) and Mobile (375x812) with 0 errors.

### Subtask 1.4: Real-Time Logs Explorer (`/logs`) Full-Stack Overhaul (COMPLETED & VERIFIED ✅)
- **Backend Files**: `internal/infrastructure/logging/aggregator.go`, `internal/adapter/http/log_stream_handler.go`, `cmd/standalone/main.go`
- **Frontend Files**: `frontend-vue/src/stores/logStore.ts`, `frontend-vue/src/components/logs/LogTargetTree.vue`, `frontend-vue/src/views/LogStreamView.vue`, `frontend-vue/src/components/logs/LogViewerTerminal.vue`
- **Full-Stack Deliverables**:
  1. Multi-dimensional log aggregation supporting `node`, `service`, `container`, `namespace`, `pod`, `level`, and `keyword` filters.
  2. Background telemetry and container log streaming in `cmd/standalone/main.go` tagged with exact Node and Service identity.
  3. 2-column drill-down tree with live buffer count badges (`k8smater`, `worker1`, `postgres`, `traefik`, etc.).
  4. Real-time WebSocket connection dynamically synchronizing to selected node or service.
  5. Verified via Chrome DevTools MCP: 12+ log lines on mobile screen 1 without horizontal scroll, exact node/service isolation verified on `worker1` and `masterdb`. All Go tests pass 100%, Vue build 0 errors.

---

## 5. Verification Protocol (Anti-Collusion & Proactive MCP Audit)

Before any task in any wave is accepted as complete:
1. Run automated build & type check: `cmd /c "npm run build"` (must pass with 0 errors).
2. Use `chrome-devtools-mcp`:
   - `emulate` mobile: `375x812x3,mobile,touch` (iPhone)
   - `emulate` mobile: `360x740x2,mobile,touch` (Android)
   - `emulate` tablet: `768x1024x2` (iPad)
   - `resize_page`: `1440x900` (Desktop)
3. Take screenshots of every viewport and inspect:
   - Are 4-5 items visible on screen 1 on mobile?
   - Are all buttons standard 32px?
   - Is there any horizontal cutoff or double scrollbars?
   - Is the contrast and typography crisp?
4. Document the exact command outputs and visual proof in `walkthrough.md`.
