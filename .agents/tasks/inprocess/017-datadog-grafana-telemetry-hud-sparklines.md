# TASK 017: Datadog & Grafana Telemetry HUD with Inline Sparklines

> **Location**: .agents/tasks/enterprise-ui-redesign/017-datadog-grafana-telemetry-hud-sparklines.md  
> **Benchmark Source**: Datadog Host Telemetry Cards, Grafana Stat Panels  
> **Status**: READY_FOR_DISPATCH  

## 1. Goal:
Standardize HUD metric cards to 88px height with embedded inline SVG micro-sparklines for live resource trends.

## 2. Acceptance Criteria:
- Fixed 88px height; 5-column desktop symmetric grid (epeat(5, 1fr)).
- Inline lightweight SVG sparkline with zero canvas overhead.
- Strict 	abular-nums formatting for CPU cores, memory bytes, and latency.
