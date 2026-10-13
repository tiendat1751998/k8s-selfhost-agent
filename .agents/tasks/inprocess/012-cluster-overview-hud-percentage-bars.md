# TASK 012: Cluster Overview HUD & Symmetric PercentageBar Redesign

> **Location**: .agents/tasks/enterprise-ui-redesign/012-cluster-overview-hud-percentage-bars.md  
> **Benchmark Source**: Rancher 2.8+ (PercentageBar), Lens 2024 (Cluster HUD), Vercel Metrics  
> **Status**: READY_FOR_DISPATCH  

## 1. Problem Statement
In OverviewView.vue and OverviewHud.vue:
- 5 KPI cards are rendered with an asymmetric "4 + 1" layout: 4 cards on row 1, and 1 lonely card on row 2 with 75% dead empty black space next to it.
- Cards have excessive height (~150px) with low information density.
- Random emojis (🧠, 🪙, 🔥, 🚀, 💻, 📅) and disjointed colors.
- In-between element LIVE REQUEST FLOW has long dashed tracks with awkward dots.

## 2. Industry Standard Pattern
- **Grid Symmetry**: A single 5-column responsive grid on desktop (epeat(5, 1fr)), 3+2 on tablet, or 2+2+1 with auto-fill. Zero stranded cards with empty space.
- **Card Anatomy (Rancher & Lens style)**:
  - Height capped at 84px–92px.
  - Header: 14px SVG icon + 11px uppercase label + status dot.
  - Value: 24px bold tabular number (ont-variant-numeric: tabular-nums).
  - Progress: Sleek 3px dual-tone PercentageBar (used vs allocatable).
- **Request Flow**: Clean, compact telemetry bar with micro-sparkline.

## 3. Action Items
1. Create src/components/ui/PercentageBar.vue:
   - Height: 3px–4px, rounded-full.
   - Track: gba(255, 255, 255, 0.08).
   - Fill: Dynamic semantic gradient (emerald <70%, mber 70–84%, ose >=85%).
2. Refactor rontend-vue/src/components/overview/OverviewHud.vue:
   - Change .metrics-grid to grid-template-columns: repeat(5, minmax(0, 1fr)) on desktop.
   - Replace emojis with BaseIcon.
   - Use PercentageBar in CPU, Memory, and Storage cards.
3. Clean up LIVE REQUEST FLOW strip to be high-signal and compact.
