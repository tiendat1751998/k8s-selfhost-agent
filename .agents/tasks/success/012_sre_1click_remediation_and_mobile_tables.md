# Task 012: 1-Click SRE Node Remediation, Cluster Self-Healing & Mobile Table Conversions

**Created**: 2026-09-07T13:53:00+07:00  
**Status**: IN_PROCESS  
**Owner**: Orchestrator (tiendat1751998/k8s-selfhost-agent)  
**Supreme Directive**: Craftsmanship > Speed (AGENTS.md §00). Proactive MCP visual audits across all breakpoints.

## Objective
1. Wire up the Go backend's <30s fast-failover remediation controller (internal/usecase/sre/node_remediation.go and internal/adapter/http/dr_handler.go) to the frontend UI with a 1-Click Failover button in:
   - AlertCenterModal.vue
   - TopHudAlertBell.vue
   - NodeDiagnosticsDrawer.vue (with [ ⚡ 1-Click Failover ], [ 🛡️ Cordon / Uncordon ], [ 🔍 Live Probe ])
2. Eradicate remaining desktop 1154px tables on mobile (375x812):
   - /explorer: ExplorerMobileCards.vue (<250 lines)
   - /automation: AutomationMobileCards.vue (<200 lines)
   - /cost: CostMobileCards.vue (<200 lines)
3. Conduct autonomous platform-wide MCP visual audits on Mobile (375x812), Tablet (768x1024), and Desktop (1440x900) with zero user manual testing needed.

## Work Breakdown Structure (WBS)
- [ ] Wave 1: 1-Click SRE Remediation & Self-Healing Modal + Buttons
  - [ ] NodeRemediationModal.vue (<250 lines)
  - [ ] NodeDiagnosticsDrawer.vue SRE action buttons
  - [ ] AlertCenterModal.vue 1-click failover button
  - [ ] TopHudAlertBell.vue quick failover action
- [ ] Wave 2: Mobile Responsive Card Streams
  - [ ] ExplorerMobileCards.vue + ExplorerView.vue
  - [ ] AutomationMobileCards.vue + AutomationView.vue
  - [ ] CostMobileCards.vue + CostFinOpsView.vue
- [ ] Wave 3: Proactive Autonomous MCP Audit
  - [ ] Mobile 375x812 visual inspection
  - [ ] Tablet 768x1024 visual inspection
  - [ ] Desktop 1440x900 visual inspection
  - [ ] Full regression verification (npm run build, go test)
