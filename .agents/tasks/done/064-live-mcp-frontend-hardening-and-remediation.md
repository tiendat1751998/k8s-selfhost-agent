# Task 064: Live MCP Frontend Hardening & Defect Remediation

## 1. Context & Objectives
During the live Chrome DevTools MCP visual and operational audit (`qa-test-engineer` session `9332c5ac-4e12-4964-bea4-7e6ae6693a49`) of the running frontend at `http://localhost:3000/`, 7 concrete UI & operational defects, fake telemetry data traps, accessibility violations, mobile layout omissions, and severe CSS bundle bloating were caught directly on the live interface.

This task remediates all 7 verified defects across the frontend, enforcing strict production standards, zero mock fallbacks, 4-tier RWD, and clean asset bundling.

---

## 2. Verified Defect Inventory & Action Plan

### 🔴 Defect 1 (CRITICAL): Mobile Hamburger Drawer Broken with `is-collapsed` (64px width)
- **File & Line**: `frontend-vue/src/assets/styles/layout/app-shell.css`, `frontend-vue/src/components/layout/AppSidebar.vue`
- **Evidence**: `audit_screenshots/mobile_sidebar_collapsed_bug.png`
- **Symptom**: On mobile (<640px), clicking `.mobile-menu-btn` opens the drawer with `.sidebar.is-collapsed` keeping width at `64px` instead of `280px`. Nav text is vertically sliced in half (`Ov`, `Inc`, `SL`, `Re`, `Fle`, `Inf`, `De`), and the close button is hidden.
- **Root Cause**: `.sidebar.is-collapsed { width: 64px !important; }` has equal specificity with `.sidebar` in `@media (max-width: 640px)`.
- **Remediation**:
  - In `app-shell.css` under `@media (max-width: 640px)`, ensure `.sidebar.sidebar-open` strictly overrides:
    ```css
    .sidebar.sidebar-open,
    .sidebar.is-collapsed.sidebar-open {
      width: 280px !important;
      min-width: 280px !important;
      transform: translateX(0) !important;
    }
    .sidebar.sidebar-open .nav-text,
    .sidebar.is-collapsed.sidebar-open .nav-text {
      display: inline-block !important;
      opacity: 1 !important;
      visibility: visible !important;
    }
    ```

### 🔴 Defect 2 (CRITICAL): Mock Workload Fallback Trap on Disconnected Backend
- **File & Line**: `frontend-vue/src/composables/useDeployments.ts:14-52, 131-133, 142-144`
- **Evidence**: `audit_screenshots/deployments_desktop_1440.png`
- **Symptom**: When the backend is offline (502 Bad Gateway), `useDeployments` catches the error and silently populates `deployments.value = FALLBACK_SAMPLE_WORKLOADS` (`ingress-traefik`, `api-gateway`, `auth-service`, `redis-master`, `postgres-operator`), falsely reporting healthy running pods.
- **Remediation**:
  - Purge `FALLBACK_SAMPLE_WORKLOADS` entirely.
  - On empty or failed API response, `deployments.value = []` and set proper `error.value`.
  - Maintain clean, authentic empty states and error notifications.

### 🟠 Defect 3 (HIGH): Missing Mobile Toolbar & Dead Action Hint on `/tenancy`
- **File & Line**: `frontend-vue/src/views/TenancyRbacView.vue:56`, `frontend-vue/src/components/tenancy/TenancyMobileCards.vue:41`
- **Evidence**: `audit_screenshots/tenancy_mobile_375.png`
- **Symptom**: `.tenancy-toolbar-sleek` has class `desktop-only`. On mobile, there is no command bar, no tab switching (stuck on tenants), no search, and no `+ New Org` button. The empty card tells users `"Tap + to onboard a new tenant workspace"`, but no `+` button exists on screen.
- **Remediation**:
  - Implement a mobile-first horizontally scrollable tab-pill bar for mobile viewports (`mobile-tenancy-toolbar`).
  - Ensure tab switching, search input, and creation triggers are accessible across all 4 RWD tiers.

### 🟠 Defect 4 (HIGH): Table Column Error Misplacement in Cluster Explorer (`/explorer`)
- **File & Line**: `frontend-vue/src/views/ExplorerView.vue`
- **Evidence**: `audit_screenshots/explorer_desktop_1440.png`
- **Symptom**: In 502/disconnected state, the error `"API Error 502 (Bad Gateway)"` is rendered inside only the first column (`NAME`) without a full-width `colspan="7"` empty-state row, causing a separator `<hr>` to slice through the error text.
- **Remediation**:
  - Wrap the error/empty message in `<td colspan="7">` or a dedicated full-width container spanning the table grid.

### 🟡 Defect 5 (MEDIUM): Truncated UI Elements on Mobile (375px) & Localization Mix
- **File & Line**:
  - `frontend-vue/src/components/deployments/DeploymentsMobileCards.vue:104, 107`
  - `frontend-vue/src/views/ExplorerView.vue`
  - `frontend-vue/src/views/LogStreamView.vue`
- **Remediation**:
  - In `DeploymentsMobileCards.vue`: Change `"Chi tiết"` to `"Details"` and `"Chi tiết Workload Inspector"` to `"Inspect Workload"`.
  - In `ExplorerView.vue`: Change `"Apply Y"` button to compact icon or full `"Apply YAML"` with `white-space: nowrap; font-size: 11px; padding: 4px 8px;`.
  - In `LogStreamView.vue`: Adjust mobile badge flex wrap/padding so `"60 FPS"` and `"Virtualized"` stay on a single line or pill-stack cleanly.

### 🟡 Defect 6 (MEDIUM): Redundant Stacked Empty States on `/hosts`
- **File & Line**: `frontend-vue/src/views/InfraHostsView.vue`
- **Evidence**: `audit_screenshots/hosts_desktop_1440.png`
- **Symptom**: Desktop renders two separate empty state cards simultaneously: `"No infrastructure hosts registered"` stacked directly above `"No compute hosts found"`. Filter counts are unspaced: `"All 0 Connected 0 Disconnected 0"`.
- **Remediation**:
  - Consolidate to a single clear empty state card when 0 hosts are found.
  - Fix filter count badge spacing (`margin-left: 4px; padding: 2px 6px;`).

### 🟡 Defect 7 (MEDIUM): Static Fallback Log Targets in `/logs`
- **File & Line**: `frontend-vue/src/components/logs/LogTargetTree.vue:60-81`
- **Symptom**: Static arrays `DEFAULT_APPS` (`postgres_db`, `tiki_redis`, `my-nginx`, `nats`, `traefik`), `DEFAULT_SYSTEMD`, and `DEFAULT_NODES` are rendered when backend telemetry is offline.
- **Remediation**:
  - Purge static mock arrays `DEFAULT_APPS` and `DEFAULT_NODES`.
  - Dynamically populate log targets from discovered cluster nodes, pods, and container workloads via props/API.
  - When no targets are discovered, render standard empty state with dynamic search capability.

### 🟡 Defect 8: Form Input Accessibility & Autofill Non-Compliance
- **File & Line**: `frontend-vue/src/views/LoginView.vue`
- **Violation**: Form fields lack explicit `id` and `name` attributes, violating WCAG 2.2 AA and triggering browser console accessibility warnings.
- **Remediation**:
  - Add standard `id="username" name="username"` and `id="password" name="password"` attributes with proper `autocomplete` and `aria-label` tags.

### 🟡 Defect 9: Massive CSS Bundle Explosion
- **Files**: `frontend-vue/src/assets/styles/views/overview.css`, `frontend-vue/src/assets/styles/views/explorer.css`, etc.
- **Violation**: `OverviewView.css` reaches 643.76 kB and `ExplorerView.css` reaches 291.66 kB in Vite production builds due to recursive `@import` of global design-system stylesheets inside individual view CSS files.
- **Remediation**:
  - Purge redundant `@import` statements for base variables, reset, and utilities in scoped/view stylesheets since `main.css` and `App.vue` already load them globally.
  - Verify `npm run build` generates sleek, lightweight CSS assets (< 50 kB per view).

---

## 3. Implementation Subagents & Workflow
- **Coder Subagent**: `frontend-coder` (Workspace: `branch`)
- **Reviewer Subagent**: `reviewer` (independent diff audit)
- **QA Subagent**: `qa-test-engineer` (live Chrome DevTools MCP re-audit with screenshots)
- **Gate Criteria**:
  1. `npm.cmd run type-check` exits with 0 errors.
  2. `npm.cmd run build` completes successfully with dramatic reduction in CSS bundle sizes.
  3. Live MCP DevTools audit verifies 0 console errors, clean mobile navigation on `/tenancy`, authentic empty state on `/deployments`, mobile sidebar opening at 280px, and 100% responsive layouts.
