# TASK 014: Enterprise StatusBadge Standardization Across All 34 Screens

> **Location**: .agents/tasks/enterprise-ui-redesign/014-status-badge-enterprise-standardization.md  
> **Benchmark Source**: Rancher (BadgeState), Headlamp Status Chips, Datadog Entity Badges  
> **Status**: READY_FOR_DISPATCH  

## 1. Problem Statement
Status indicators across views are currently inconsistent:
- Some views use emoji buttons (● ACTIVE, ⚡ Rebalance).
- Some views use monospace font pills that look like 8-bit games.
- Colors and paddings vary between views.

## 2. Industry Standard Pattern
- 6px circular dot indicator with subtle CSS pulse for active states.
- Translucent background (10–14% opacity) + 1px subtle border.
- Strict sans-serif font (Inter), 11px uppercase, letter-spacing: 0.04em, ont-weight: 600.
- Standard Kubernetes state mapping:
  - Ready / Healthy / Active: Emerald #10b981
  - Pending / Warning / Degraded: Amber #f59e0b
  - Failed / Critical / Offline: Rose #f43f5e
  - Cordoned / Draining / Terminating: Slate #94a3b8

## 3. Action Items
1. Harden src/components/ui/StatusBadge.vue:
   - Enforce dot indicator + translucent pill styling.
   - Accept standard status variants.
2. Replace ad-hoc status spans in all views with <StatusBadge :status="status" :label="label" />.
3. Verify on Chrome DevTools MCP across viewports.
