# TASK: Fix CSS & Visual Issues

## Priority: 🟣 MEDIUM — UI looks broken/inconsistent
## Status: PENDING
## Estimated Effort: 30 minutes

---

## Problem Description

Multiple CSS issues cause visual bugs, broken theme switching, and inconsistent styling across the application.

## Sub-Tasks

### 7.1 — Fix undefined CSS variable `--color-text` 
- **File**: `frontend/index.html` (L26 and potentially other locations)
- **Problem**: `var(--color-text)` is used but NOT defined in `:root` in `styles.css`. The actual variables are `--color-body` (for body text) and `--color-on-dark` (for high-contrast text).
- **Fix**: Search entire `index.html` for `--color-text` and replace:
  - For headings/titles: `--color-on-dark`
  - For body text: `--color-body`
- **Command to find**: `Select-String -Pattern "color-text" frontend/index.html`

### 7.2 — Complete Light Theme (`[data-theme="light"]`) CSS overrides
- **File**: `frontend/css/styles.css` (~L77-88)
- **Current**: Only 10 variables overridden (surface colors + text colors)
- **Missing overrides needed**:
  ```css
  [data-theme="light"] {
    /* Already defined: canvas, surface, text colors */
    
    /* ADD these: */
    --color-primary: #f0b90b;           /* Slightly adjusted for light bg */
    --color-primary-active: #d4a20a;
    --color-trading-up: #0ba360;        /* Darker green for contrast */
    --color-trading-down: #dc3545;      /* Darker red for contrast */
    --color-info: #2563eb;              /* Darker blue for contrast */
    --color-warning: #d97706;           /* Amber for light mode */
  }
  ```
- **Test**: Toggle to Light Mode → all status indicators, charts, and badges remain readable

### 7.3 — Add skeleton loading states for all sections
- **Problem**: Only `TableRenderer` has `showLoading()`. Other sections show blank content until API responds.
- **Affected modules**:
  - `observability.js` — SLO cards
  - `topology.js` — topology map
  - `capacity-planning.js` — forecast cards
  - `deployment-center.js` — catalog grid
  - `cost-management.js` — cost breakdown
  - `compliance.js` — compliance cards
  - `fleet-view.js` — cluster map
- **Fix per module**: Add skeleton HTML at init time:
  ```js
  init() {
      const container = document.getElementById('xxx');
      container.innerHTML = `
          <div class="skeleton" style="height:200px;border-radius:var(--rounded-lg);"></div>
      `;
      this.loadData(); // Real data replaces skeleton
  }
  ```
- **CSS**: `.skeleton` class already exists in `styles.css` (L707-724) with shimmer animation

### 7.4 — Consolidate small CSS files
- **Problem**: 11 separate CSS files loaded (11 HTTP requests):
  - `styles.css` (21KB) — main
  - `enterprise.css` (16KB) — layout
  - `cost.css` (3KB), `observability.css` (4KB), `copilot.css` (4KB), `docker-swarm.css` (4KB)
  - `automation.css` (1KB), `fleet.css` (2KB), `healthcenter.css` (2KB), `promotion.css` (2KB), `topology.css` (2KB)
- **Fix**: Merge the 6 smallest files (< 3KB each) into `enterprise.css`:
  - Move `automation.css` → `enterprise.css`
  - Move `fleet.css` → `enterprise.css`
  - Move `healthcenter.css` → `enterprise.css`
  - Move `promotion.css` → `enterprise.css`
  - Move `topology.css` → `enterprise.css`
- **Result**: 6 CSS files instead of 11, fewer HTTP requests
- **Update**: Remove merged `<link>` tags from `index.html`

### 7.5 — Add consistent `section-header` styling to all sections
- **Problem**: Some sections have proper headers, others don't. Compare:
  - `section-runbooks` has `<div class="section-header"><h2>📓 Runbook Center</h2>...</div>` ✅
  - `section-capacity` has `<div id="capacity-planning"></div>` (empty, JS renders) ❌
- **Fix**: Ensure all 25+ sections follow the pattern:
  ```html
  <section class="section" id="section-xxx">
      <div class="section-header">
          <h2>🎯 Section Title</h2>
          <div class="section-actions">
              <button class="btn btn-ghost btn-sm" onclick="Module.refresh()">↻ Refresh</button>
          </div>
      </div>
      <div id="xxx-content"></div>
  </section>
  ```

## Files Involved
- `frontend/index.html` — CSS variable fixes, section headers
- `frontend/css/styles.css` — light theme overrides
- `frontend/css/enterprise.css` — merged CSS from small files
- `frontend/css/automation.css` — merge target (delete after)
- `frontend/css/fleet.css` — merge target (delete after)
- `frontend/css/healthcenter.css` — merge target (delete after)
- `frontend/css/promotion.css` — merge target (delete after)
- `frontend/css/topology.css` — merge target (delete after)
- Multiple JS modules — skeleton loading

## Verification
- Toggle Light Mode → all elements readable, no invisible text
- Every section shows skeleton animation before data loads
- Page loads with 6 CSS requests instead of 11
- All sections have consistent header layout
