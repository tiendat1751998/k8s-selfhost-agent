# TASK 013: Capacity & Headroom Matrix Anti-Neon Redesign

> **Location**: .agents/tasks/enterprise-ui-redesign/013-capacity-headroom-anti-neon-redesign.md  
> **Benchmark Source**: Lens Resource Allocation, Kubernetes Scheduling Headroom Formula  
> **Status**: READY_FOR_DISPATCH  

## 1. Problem Statement
In CapacityView.vue and NodeHeadroomTable.vue:
- The table looks like a neon casino: yellow 70.0%, green 69.0%, blue 81.2%, yellow 30.0%, green Rebalance, grey Inspect.
- In the HEALTH column, there is an interactive button ⚡ Rebalance with a struck-through label (~~Rebalance~~). An action button should never be placed in a health status column!
- Overly cluttered columns with mixed neon bars distract the operator.

## 2. Industry Standard Pattern
- **Unified Semantic Spectrum**: Single color logic across all resource allocation bars based strictly on risk:
  - < 70%: Muted Slate/Cyan (Normal allocation)
  - 70% – 84.9%: Warm Amber (Caution / Approaching saturation)
  - >= 85%: Rose/Coral (Critical saturation / Eviction danger)
- **Column Separation**:
  - HEALTH: Strictly displays <StatusBadge> (e.g. Ready, Cordoned, High Commitment), never an action button.
  - ACTIONS: Single 3-dot Kebab menu ... holding Rebalance Pods, Inspect Allocations, Drain Node.
- **Allocation Bar Anatomy**: Clean 6px progress bar with allocation percentage and allocatable GiB/cores subtext.

## 3. Action Items
1. Refactor rontend-vue/src/components/capacity/NodeHeadroomTable.vue:
   - Remove ⚡ Rebalance button from the HEALTH column.
   - Replace HEALTH column content with a clean <StatusBadge>.
   - Move Rebalance action into <ActionDropdown> in the ACTIONS column.
   - Apply unified semantic color threshold to CPU Allocation and Memory Allocation bars.
2. Verify table renders flat on Full HD with 0 horizontal scrollbar.
