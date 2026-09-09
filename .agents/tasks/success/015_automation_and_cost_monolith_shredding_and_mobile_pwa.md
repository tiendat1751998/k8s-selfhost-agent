# Task 015: Whole-Repository Audit Resolution — Shredding Final Monolith Views & Mobile PWA Overhaul

## 1. Executive Summary & Context
In accordance with the user's explicit directive:
> *"kiểm tra tất cả các file khác đi :D"*
A comprehensive recursive scan was conducted across all `.vue`, `.ts`, and `.go` files in the repository.

### Audit Discoveries
1. **`frontend-vue/src/views/AutomationView.vue`**: **842 lines** (VIOLATION of AGENTS.md §5 max 500-line absolute rule and 350-line view rule). Contains 402 lines of inline `<style scoped>`. Subcomponents in `src/components/automation/`, composable `useAutomationEngine.ts` (320 lines), and stylesheet `assets/styles/views/automation.css` already exist, but the view was never rewired!
2. **`frontend-vue/src/views/CostFinOpsView.vue`**: **745 lines** (VIOLATION of AGENTS.md §5 max 500-line absolute rule and 350-line view rule). Contains 386 lines of inline `<style scoped>`. Subcomponents in `src/components/cost/`, composable `useCostFinOps.ts` (270 lines), and stylesheet `assets/styles/views/cost.css` already exist, but the view was never rewired!
3. **`frontend-vue/src/views/LogStreamView.vue`**: **391 lines** (exceeds 350-line view limit due to 197 lines of inline CSS).
4. **`frontend-vue/src/views/OverviewView.vue`**: **353 lines** (3 lines over 350-line limit).
5. **Mobile PWA Defects on `/automation` and `/cost`**:
   - Neither view has a 44px Mobile Command Bar or 20px micro-telemetry strip.
   - Bulky desktop headers and 4 vertically stacked KPI cards eat 450px+ of viewport height on mobile `<640px`, pushing actionable tables and cards far off screen.

---

## 2. Mandatory Acceptance Criteria
1. **Line-Count Strict Compliance**:
   - `AutomationView.vue`: MUST be `< 250 lines` (orchestrator template).
   - `CostFinOpsView.vue`: MUST be `< 250 lines` (orchestrator template).
   - `LogStreamView.vue`: MUST be `< 220 lines` (orchestrator template).
   - `OverviewView.vue`: MUST be `< 350 lines` (orchestrator template).
   - **Zero Monolithic Views**: 100% of ALL 34 views in `src/views/` will strictly comply with `< 350 lines`.
2. **Modular Wire-Up**:
   - `AutomationView.vue`: Wire to `useAutomationEngine.ts` and `src/components/automation/`:
     - `AutomationHudCards.vue`
     - `AutomationRulesTable.vue`
     - `AutomationMobileCards.vue`
     - `AutomationExecutionHistory.vue`
     - `CreateWorkflowModal.vue`
     - Externalize styles to `assets/styles/views/automation.css`.
   - `CostFinOpsView.vue`: Wire to `useCostFinOps.ts` and `src/components/cost/`:
     - `CostHudMetrics.vue`
     - `CostBreakdownChart.vue`
     - `NamespaceCostTable.vue`
     - `CostMobileCards.vue`
     - Externalize styles to `assets/styles/views/cost.css`.
   - `LogStreamView.vue`: Move inline `<style scoped>` to `assets/styles/views/logstream.css`.
   - `OverviewView.vue`: Consolidate comments to bring line count strictly `< 350 lines`.
3. **4-Tier RWD & Mobile-First PWA Ergonomics**:
   - On `@media (max-width: 640px)`:
     - Completely hide desktop headers and bulky 4-card KPI grids (`.desktop-header-wrap { display: none !important; }`).
     - Render 44px `.mobile-command-bar` with 32px standardized icon buttons (`[ 🔄 ] [ ➕ ] [ 🔍 ]`).
     - Render 20px `.mobile-micro-telemetry` strip.
     - Stream cards (`AutomationMobileCards.vue` / `CostMobileCards.vue`) immediately on Screen 1 without horizontal scrolling or button truncation.
4. **Autonomous Visual MCP Verification**:
   - Verify Mobile (375x812), Tablet (768x1024), and Desktop (1440x900) via Chrome DevTools MCP for both `/automation` and `/cost`.
   - `npm run build` must compile with exit code 0.

---

## 3. Work Breakdown Structure (WBS)
- **Phase 1**: Dispatch `frontend-coder` to shred and rewire `AutomationView.vue`, `CostFinOpsView.vue`, `LogStreamView.vue`, and `OverviewView.vue`.
- **Phase 2**: Verify `npm run build` and run Python line count verification across all 34 views.
- **Phase 3**: Chrome DevTools MCP visual inspection on Mobile (375x812), Tablet (768x1024), and Desktop (1440x900).
- **Phase 4**: Commit changes, log `DEC-095`, and archive task to `tasks/success/`.
