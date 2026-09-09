# Task 013: Mobile PWA Craftsmanship, Font Normalization, Button Polish & SRE Controller Robustness

**Created**: 2026-09-07T15:36:00+07:00  
**Status**: COMPLETED  
**Owner**: Orchestrator (tiendat1751998/k8s-selfhost-agent)  
**Supreme Directive**: Craftsmanship > Speed (AGENTS.md §00). Proactive MCP visual audits across all breakpoints. Zero user manual QA.

## Objective & User Feedback Addressed
User reported:
1. *"trông website không hề mượt một tí nào"* -> Eliminate nested dual scrollbars on mobile (`document.body` + `.page-container`), add momentum scrolling `-webkit-overflow-scrolling: touch` and `scroll-behavior: smooth`.
2. *"lỗi front chữ"* -> Import Google Fonts `Inter` (weights 400..800) and `JetBrains Mono` (weights 400..700) in `index.html`. Fix typography and line heights across tokens.
3. *"lỗi buttion có quá nhiều chữ ở pwa mobile-web"* ->
   - In `OverviewView.vue`: On mobile `<640px`, switch to `OverviewMobileStream.vue` (ultra-compact 48px node chips, 4 touch KPIs) and hide bulky 800px HUD, saturation trends chart, and 500px tall node cards.
   - Button text normalization on mobile: Shorten verbose button texts (`[ 🔍 Inspect Telemetry & Apps ]` -> `[ 🔍 Inspect ]`, `[ ⚡ Confirm & Execute Fast Failover ]` -> `[ ⚡ Confirm Failover ]`, `[ 🔕 Mute (Until Restart) ]` -> `[ 🔕 Mute ]`).
4. Fix SRE Remediation Backend:
   - Handle typed nil interface in Go to prevent panic on `client.CoreV1()`.
   - Fallback to `clientManager` for default cluster if primary kubeconfig unavailable.
   - Return clean descriptive errors without panicking.

## Work Breakdown Structure (WBS)
- [x] Wave 1: Typography, Global Smooth Scrolling & Font CDN
  - [x] `frontend-vue/index.html`: Add Inter & JetBrains Mono fonts link
  - [x] `frontend-vue/src/assets/css/base.css`: Smooth scrolling, touch momentum, clean body overflow
  - [x] `frontend-vue/src/assets/styles/layout/app-shell.css`: Mobile page container padding (bottom 76px for nav bar), remove dual scrollbar
- [x] Wave 2: Mobile Overview Stream & Button Text Reduction
  - [x] `frontend-vue/src/views/OverviewView.vue`: Integrate `OverviewMobileStream.vue` on `<640px` breakpoint
  - [x] `frontend-vue/src/assets/styles/views/overview.css`: Add `@media (max-width: 640px)` and `@media (max-width: 768px)` rules
  - [x] `frontend-vue/src/components/overview/alerts/AlertListTable.vue`: Mobile button labels (`Failover`, `Host`, `Mute`, `Dismiss`)
  - [x] `frontend-vue/src/components/overview/drawer/NodeRemediationModal.vue`: Compact safety text and button on mobile
- [x] Wave 3: Backend SRE Controller Nil-Pointer Hardening
  - [x] `internal/usecase/sre/node_remediation.go`: Safe interface dereferencing, graceful cluster fallback, nil check
  - [x] `cmd/standalone/main.go`: Safe typed nil handling for `k8sClient`
  - [x] Verify Go tests (`go test -v ./internal/usecase/sre/...`)
- [x] Wave 4: Proactive Autonomous MCP Visual Audit
  - [x] Mobile 375x812: Overview, Alert Center, Remediation Modal, Deployments, Explorer
  - [x] Tablet 768x1024: Verify 2x2 layout and sidebar
  - [x] Desktop 1440x900: Full HD verification
