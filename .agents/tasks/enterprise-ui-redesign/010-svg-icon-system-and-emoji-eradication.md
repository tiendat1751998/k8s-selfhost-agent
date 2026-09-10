# TASK 010: Centralized SVG Icon System & Complete Emoji Eradication

> **Location**: .agents/tasks/enterprise-ui-redesign/010-svg-icon-system-and-emoji-eradication.md  
> **Benchmark Source**: Rancher (@rancher/icons), KubeSphere (@kubed/icons), Lens IDE, Headlamp  
> **Status**: READY_FOR_DISPATCH  

## 1. Problem Statement
The current UI is heavily contaminated with Unicode emojis (🧠 memory, 🪙 storage, 🔥 peak cpu, 🚀 active containers, 💻 nodes, ⚡ rebalance, 🔍 probe, 🗑️ evict). 
Emojis render inconsistently across Windows, macOS, iOS, and Linux; they fail accessibility screen readers (speaking alt texts aloud); and they give the platform a student hobbyist/discord-bot appearance.

## 2. Industry Standard Pattern
- 100% Vector SVG icons (Lucide / Tabler style), 14x14px or 16x16px, stroke-width: 1.5px, inheriting currentColor.
- Zero Unicode emojis in production UI code.

## 3. Action Items
1. Create src/components/ui/BaseIcon.vue supporting standard enterprise iconography:
   - server, cpu, layers, hard-drive, ox, ctivity, 	erminal, more-vertical, efresh, shield, lert-triangle, check-circle, x-circle, search, external-link.
2. Scan and replace all emojis across:
   - rontend-vue/src/views/OverviewView.vue & OverviewHud.vue
   - rontend-vue/src/views/FleetView.vue & FleetClustersTable.vue
   - rontend-vue/src/views/CapacityView.vue & NodeHeadroomTable.vue
   - rontend-vue/src/views/LogStreamView.vue & LogTargetTree.vue
   - rontend-vue/src/constants/navigation.ts
3. Verify 
pm.cmd run build passes with 0 compiler errors.
