# TASK: Handle Phantom Sidebar Entries & Missing Router Titles

## Priority: 🟡 HIGH — 5 sidebar links lead to blank page
## Status: COMPLETED
## Estimated Effort: 30 minutes

---

## Problem Description

The sidebar (defined in `frontend/components/layout/sidebar.js`) contains 5 menu items that have **no corresponding `<section>` in `index.html`** at all:

| Sidebar Item | ID | Group | Problem |
|---|---|---|---|
| Nodes | `nodes` | Infrastructure | No HTML section, no JS module |
| Pods | `pods` | Infrastructure | No HTML section, no JS module |
| Rollouts | `rollouts` | Deployments | No HTML section, no JS module |
| Scaling | `scaling` | Deployments | No HTML section, no JS module |
| Incidents | `incidents` | Operations | No dedicated section (only in Overview panel) |

## Sub-Tasks

### 3.1 — Create `nodes` section + JS module
- **Add to**: `frontend/index.html` (new `<section class="section" id="section-nodes">`)
- **Create**: `frontend/modules/clusters/nodes.js`
- **API**: Wire to `/api/v1/explorer/nodes` (ExplorerHandler already exists)
- **Content**: Table with columns: Name, Status, Role, CPU, Memory, Pods, Age
- **Init**: Add `NodesSection.init()` in `app.js`
- **Script**: Add `<script src="/modules/clusters/nodes.js">` in `index.html`

### 3.2 — Create `pods` section + JS module
- **Add to**: `frontend/index.html` (new `<section class="section" id="section-pods">`)
- **Create**: `frontend/modules/clusters/pods.js`
- **API**: Wire to `/api/v1/explorer/pods` (ExplorerHandler already exists)
- **Content**: Table with columns: Name, Namespace, Status, Restarts, Node, Age
- **Init**: Add `PodsSection.init()` in `app.js`
- **Script**: Add `<script src="/modules/clusters/pods.js">` in `index.html`

### 3.3 — Create `rollouts` section + JS module
- **Add to**: `frontend/index.html` (new `<section class="section" id="section-rollouts">`)
- **Create**: `frontend/modules/deployments/rollouts.js`
- **API**: Wire to `/api/v1/explorer/deployments` filtered by rollout status
- **Content**: Active rollouts table: Deployment, Strategy, Progress, Replicas
- **Init**: Add `RolloutsSection.init()` in `app.js`

### 3.4 — Create `scaling` section + JS module
- **Add to**: `frontend/index.html` (new `<section class="section" id="section-scaling">`)
- **Create**: `frontend/modules/clusters/scaling.js`
- **API**: Wire to `/api/v1/explorer/hpa` (may need new handler)
- **Content**: HPA/VPA list: Name, Target, Min, Max, Current, Metrics
- **Init**: Add `ScalingSection.init()` in `app.js`

### 3.5 — Create dedicated `incidents` section
- **Add to**: `frontend/index.html` (new `<section class="section" id="section-incidents">`)
- **Create**: `frontend/modules/incidents/incidents-page.js`
- **API**: Wire to `/api/v1/incidents` (already exists)
- **Content**: Full incidents table with filtering, pagination, severity badges
- **Note**: The existing `incidents.js` renders the Overview panel. This new module is a full-page view.
- **Init**: Add `IncidentsPage.init()` in `app.js`

### 3.6 — Add missing Router `sectionTitles`
- **File**: `frontend/core/router/index.js` (L9-45)
- **Add entries**:
  ```js
  'nodes': 'Nodes',
  'pods': 'Pods',
  'rollouts': 'Rollouts',
  'scaling': 'Auto Scaling',
  'incidents': 'Incidents',
  ```

## Files Involved
- `frontend/index.html` — add 5 new sections + 5 new script tags
- `frontend/core/router/index.js` — add 5 sectionTitles
- `frontend/app/app.js` — add 5 `.init()` calls
- `frontend/components/layout/sidebar.js` — no change needed
- New files: `nodes.js`, `pods.js`, `rollouts.js`, `scaling.js`, `incidents-page.js`

## Verification
- Click Nodes → shows node table
- Click Pods → shows pod table
- Click Rollouts → shows rollout list
- Click Scaling → shows HPA/VPA list
- Click Incidents → shows full incident table (not just Overview panel)
