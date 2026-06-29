# TASK: Fix 11 Broken Section IDs/Classes

## Priority: 🟡 HIGH — 11 sidebar menus show blank page
## Status: COMPLETED
## Estimated Effort: 20 minutes

---

## Problem Description

The Router at `frontend/core/router/index.js` (L49-52) finds sections using:
```js
document.querySelectorAll('.section').forEach(function (el) {
    var id = el.id.replace('section-', '');
    sections[id] = el;
});
```

But 11 sections at the bottom of `index.html` use a different convention:
- Class: `module-section` instead of `section`
- ID: `capacity` instead of `section-capacity`
- Inline: `style="display:none;"` instead of relying on CSS `.section { display: none; }`

This means the Router never finds them → clicking sidebar links shows a blank page.

## Sub-Tasks

### 2.1.1 — Fix `capacity` section
- **File**: `frontend/index.html` (~L2227)
- **Before**: `<section id="capacity" class="module-section" style="display:none;">`
- **After**: `<section class="section" id="section-capacity">`

### 2.1.2 — Fix `drift` section
- **File**: `frontend/index.html` (~L2232)
- **Before**: `<section id="drift" class="module-section" style="display:none;">`
- **After**: `<section class="section" id="section-drift">`

### 2.1.3 — Fix `correlation` section
- **File**: `frontend/index.html` (~L2237)
- **Before**: `<section id="correlation" class="module-section" style="display:none;">`
- **After**: `<section class="section" id="section-correlation">`

### 2.1.4 — Fix `change` section
- **File**: `frontend/index.html` (~L2242)
- **Before**: `<section id="change" class="module-section" style="display:none;">`
- **After**: `<section class="section" id="section-change">`

### 2.1.5 — Fix `promotion` section
- **File**: `frontend/index.html` (~L2247)
- **Before**: `<section id="promotion" class="module-section" style="display:none;">`
- **After**: `<section class="section" id="section-promotion">`

### 2.1.6 — Fix `explorer` section
- **File**: `frontend/index.html` (~L2252)
- **Before**: `<section id="explorer" class="module-section" style="display:none;">`
- **After**: `<section class="section" id="section-explorer">`

### 2.1.7 — Fix `tagging` section
- **File**: `frontend/index.html` (~L2257)
- **Before**: `<section id="tagging" class="module-section" style="display:none;">`
- **After**: `<section class="section" id="section-tagging">`

### 2.1.8 — Fix `reporting` section
- **File**: `frontend/index.html` (~L2262)
- **Before**: `<section id="reporting" class="module-section" style="display:none;">`
- **After**: `<section class="section" id="section-reporting">`

### 2.1.9 — Fix `health` section
- **File**: `frontend/index.html` (~L2267)
- **Before**: `<section id="health" class="module-section" style="display:none;">`
- **After**: `<section class="section" id="section-health">`

### 2.1.10 — Fix `fleet` section
- **File**: `frontend/index.html` (~L2272)
- **Before**: `<section id="fleet" class="module-section" style="display:none;">`
- **After**: `<section class="section" id="section-fleet">`

### 2.1.11 — Fix `audit` section
- **File**: `frontend/index.html` (~L2277)
- **Before**: `<section id="audit" class="module-section" style="display:none;">`
- **After**: `<section class="section" id="section-audit">`

## Also Check

After changing the IDs, verify that no JS module references the old IDs. Search for:
```
document.getElementById('capacity')
document.getElementById('drift')
... etc
```
These may exist in the corresponding JS modules (e.g., `capacity-planning.js`, `drift-detection.js`).
The JS modules use inner container IDs like `capacity-planning`, `drift-detection`, etc., so they should be unaffected — but verify.

## Verification
- Click each of the 11 sidebar items → content appears (not blank page)
- Browser console shows no errors about missing elements
