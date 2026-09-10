# TASK 021: Rancher-Style Global Cluster & Namespace Context Switcher

> **Location**: .agents/tasks/enterprise-ui-redesign/021-rancher-style-global-context-and-namespace-selector.md  
> **Benchmark Source**: SUSE Rancher Prime (Cluster/Project/Namespace Selector), Lens IDE Context Bar  
> **Status**: READY_FOR_DISPATCH  

## 1. Problem Statement
Currently, users must manually select or re-select clusters and namespaces across different views (`/fleet`, `/deployments`, `/logs`, `/capacity`, `/incidents`).
There is no unified global context indicating:
1. Which cluster is currently being inspected.
2. Which namespace is active (or if viewing `All Namespaces`).
3. Switching clusters requires navigating back to the Fleet page.

## 2. Industry Standard Pattern (SUSE Rancher & Lens)
- **Top Header Global Context Bar**: A prominent, compact selector in the top application bar (`GlobalContextSelector.vue`):
  - **Cluster Dropdown**: Displays current cluster with health indicator dot (`staging-k8s [● Healthy]`), with instant search filter to switch clusters anywhere in the app.
  - **Namespace Dropdown**: Displays active namespace (`default`, `kube-system`, `production`, or `All Namespaces`) with quick-search.
- **Reactive Global State (`useGlobalContext.ts`)**:
  - Centralized Pinia store / composable persisting `activeClusterId` and `activeNamespace` in `localStorage`.
  - Views like Deployments, Pods, Services, Logs, and Alerts automatically subscribe to this global context.
- **Visual Clarity**: Provides immediate spatial orientation — the operator always knows which environment and tenant they are executing commands against.

## 3. Action Items
1. **Global Context Composable (`frontend-vue/src/composables/useGlobalContext.ts`)**:
   - Manages `currentCluster`, `currentNamespace`, `availableClusters`, `availableNamespaces`.
   - Persists selections to `localStorage`.
2. **Context Selector Component (`frontend-vue/src/components/navigation/GlobalContextSelector.vue`)**:
   - Compact dual-dropdown in top navbar (Cluster + Namespace).
   - Searchable list with keyboard navigation (Up/Down/Enter).
   - Status badge next to each cluster name in the dropdown.
3. **Integration with AppHeader (`frontend-vue/src/components/navigation/AppHeader.vue` or `App.vue`)**:
   - Mount next to tenant selector / search bar.
4. **View Synchronization**:
   - Sync `activeClusterId` with `useLogStreamer`, `useFleet`, `useDeployments`.

## 4. Verification:
- `npm.cmd run build` passes with 0 compiler errors.
- Chrome DevTools MCP verification: Switching cluster in the header immediately updates the table in `/deployments` and `/logs`.
