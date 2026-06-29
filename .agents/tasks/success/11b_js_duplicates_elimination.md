---
title: "Phase 11b: JS Duplicates Elimination (Enterprise Polish)"
status: "pending"
priority: "medium"
assignee: "frontend-agent"
---

# Objective
Auditing duplicates identified redundant helper declarations across different UI modules. We must extract shared logic and centralize them into utility helper namespaces.

# Tasks
- [ ] **Unify Badge Builders**:
  - Extract `resultBadge()` from `action-center.js` and `audit-logs.js` into a reusable `UIComponents.resultBadge()` method inside `components.js`.
- [ ] **Unify Tab Switchers**:
  - Extract and parameterize the repetitive `initTabs` logic (present in `compliance.js`, `cost-management.js`, `observability.js`, `topology.js`, etc.) into a utility helper `UIComponents.initTabs(containerId, callback)` to handle tab button states and view toggles.
- [ ] **Clean Helper Clones**:
  - Clean up duplicated helper functions like `escapeHTML` (in `incidents.js`) by mapping to `Security.escapeHTML` or `UIComponents.escapeHtml`.

# Acceptance Criteria
- Duplicated local function declarations are eliminated.
- Refactored views and tabs switch seamlessly without regressions.
