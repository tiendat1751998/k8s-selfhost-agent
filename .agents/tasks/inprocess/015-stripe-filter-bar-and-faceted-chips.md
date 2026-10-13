# TASK 015: Stripe-Style Filter Bar & Faceted Chips

> **Location**: .agents/tasks/enterprise-ui-redesign/015-stripe-filter-bar-and-faceted-chips.md  
> **Benchmark Source**: Stripe Workbench, Datadog Faceted Query Bar  
> **Status**: READY_FOR_DISPATCH  

## 1. Goal:
Implement a standardized filter & search toolbar mounted above all data tables with interactive, dismissible token chips ([ Status: Running ✕ ] [ Cluster: prod-east ✕ ] [ + Filter ]).

## 2. Acceptance Criteria:
- Instant client-side & server-side filtering with debounce (150ms).
- Removable filter tokens with 1-click Clear All.
- Keyboard shortcut: / or Ctrl+F focuses search input.
