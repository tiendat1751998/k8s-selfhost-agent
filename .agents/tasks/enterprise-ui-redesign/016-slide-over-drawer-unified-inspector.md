# TASK 016: Unified Slide-Over Contextual Inspector Drawer

> **Location**: .agents/tasks/enterprise-ui-redesign/016-slide-over-drawer-unified-inspector.md  
> **Benchmark Source**: Cloudflare Edge Script Inspector, Stripe Drawer, Lens Inspector  
> **Status**: READY_FOR_DISPATCH  

## 1. Goal:
Upgrade ModalDrawer.vue into a unified SlideOverInspector.vue supporting tabbed detail exploration across Pods, Nodes, Deployments, and Clusters without unmounting or obscuring the background table.

## 2. Acceptance Criteria:
- 580px fixed width on desktop, 100vw on mobile (<640px).
- Tabbed views: Overview, YAML with syntax highlighting, Live Logs, Quick Actions.
- Zero table re-render when opening/closing drawer.
