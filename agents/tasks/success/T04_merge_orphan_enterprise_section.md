# TASK: Merge Orphan Enterprise-Console Section

## Priority: 🟡 HIGH — Duplicate/unreachable content
## Status: COMPLETED
## Estimated Effort: 15 minutes

---

## Problem Description

Two enterprise sections exist in `index.html`:

1. `<section id="section-enterprise">` (~L1270) — Reachable via sidebar "Enterprise Control"
2. `<section id="section-enterprise-console">` (~L1347) — **UNREACHABLE** — no sidebar link

The `enterprise-console` section contains important features:
- Multi-Tenancy management (org/project/env selectors)
- RBAC Matrix
- Marketplace
- Backup & DR
- Provisioning Wizard

These are fully built with HTML + JS but hidden from users.

## Sub-Tasks

### 4.1 — Audit content of both sections
- **File**: `frontend/index.html`
- Check `section-enterprise` (L1270-1345) content
- Check `section-enterprise-console` (L1347-1550+) content
- Determine if they overlap or are complementary

### 4.2 — Merge or link
- **Option A (Merge)**: Move the `enterprise-console` tabs (Multi-Tenancy, RBAC, Marketplace, Backup, Provisioning) into `section-enterprise` as sub-tabs
- **Option B (Link)**: Change sidebar item to point to `enterprise-console` instead
- **Option C (Both)**: Keep both but add `enterprise-console` to sidebar as a separate item

### 4.3 — Clean up JS references
- Check `enterprise-tenancy.js`, `enterprise-rbac.js`, `enterprise-marketplace.js`, `enterprise-backup.js`, `enterprise-provisioning.js`
- These JS modules render into `section-enterprise-console` containers. If merging, update their target container IDs.

### 4.4 — Remove orphan section
- After merging, delete the orphan `<section>` block from `index.html`

## Files Involved
- `frontend/index.html` — sections at L1270 and L1347
- `frontend/modules/platform/enterprise.js`
- `frontend/modules/platform/enterprise-tenancy.js`
- `frontend/modules/platform/enterprise-rbac.js`
- `frontend/modules/platform/enterprise-marketplace.js`
- `frontend/modules/platform/enterprise-backup.js`
- `frontend/modules/platform/enterprise-provisioning.js`

## Verification
- Sidebar "Enterprise Control" → shows all enterprise features (tenancy, RBAC, marketplace, etc.)
- No orphan/unreachable sections remain in HTML
