# TASK 011: Enterprise Table Actions — Kebab Menu & Slide-Over Drawer Standardization

> **Location**: .agents/tasks/enterprise-ui-redesign/011-table-actions-kebab-and-drawer-standard.md  
> **Benchmark Source**: Lens IDE (More Actions ⋮), Rancher (ActionMenu), Headlamp (Row Drawer)  
> **Status**: READY_FOR_DISPATCH  

## 1. Problem Statement
Current data tables (FleetClustersTable.vue, NodeHeadroomTable.vue, NodeTableView.vue) render 3-4 colored buttons on every row (⚡ Details, 🔍 Probe, ⬆️ Upgrade, 🗑️ Evict or ⚡ Rebalance, 🔍 Inspect).
This causes:
- Severe visual clutter (cognitive overload from rainbow buttons on every row).
- Table horizontal overflow and scrollbars on smaller viewports.
- Inability to scale actions without breaking table column layouts.

## 2. Industry Standard Pattern
- **Row Interaction**: Clicking the row (or resource name link) opens a right-docked **Slide-over Drawer** (450px–600px) showing live metrics, YAML, and diagnostics.
- **Row Actions**: Far right column contains a single 36px–40px cell with a **3-Dot Kebab Menu (⋮)** (ActionDropdown.vue).
- **Context Menu**: Click opens floating dropdown:
  - Primary inspect (View Details, Terminal Shell)
  - Operational actions (Probe, Rebalance, Cordon / Drain)
  - Destructive action (Evict, Delete) in muted red.

## 3. Action Items
1. Create src/components/ui/ActionDropdown.vue:
   - Single ghost trigger button with 3-dot vertical icon.
   - Teleported floating menu with auto-positioning, keyboard navigation, and click-outside dismissal.
2. Refactor FleetClustersTable.vue:
   - Remove the 4 action buttons from the row template.
   - Replace with <ActionDropdown :items="clusterActions" @select="handleAction" />.
3. Refactor NodeHeadroomTable.vue and NodeTableView.vue:
   - Remove inline buttons (⚡ Rebalance, 🔍 Inspect).
   - Replace with <ActionDropdown>.
4. Verify table fits full width without horizontal scrollbars.
