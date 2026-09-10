# TASK 019: Netdata-Style High-Performance Canvas Telemetry & Synchronized Scrubbing

> **Location**: .agents/tasks/enterprise-ui-redesign/019-netdata-style-uplot-telemetry-and-synchronized-scrubbing.md  
> **Benchmark Source**: Netdata (uPlot Canvas Engine), Grafana TimeSync, Datadog DRUIDS  
> **Status**: READY_FOR_DISPATCH  

## 1. Problem Statement
The current telemetry charts across `/capacity`, `/hosts`, and `/slo` either rely on heavy SVG elements or basic DOM updates. When streaming realtime metrics (1s–5s resolution) or monitoring clusters with dozens of nodes, DOM churn can cause:
1. High CPU usage in the browser tab.
2. Stuttering during live updates (frame drops below 60fps).
3. Lack of time correlation: Hovering over a CPU spike does not highlight what was happening in Memory, Network, or Logs at that exact second.

## 2. Industry Standard Pattern (Netdata & uPlot)
- **Canvas-based Rendering (`uPlot`)**: Uses HTML5 Canvas for time-series charts instead of DOM nodes. Renders tens of thousands of data points with negligible memory (<100KB) and instant startup time.
- **Synchronized Time-Scrubbing**: Moving the mouse over any chart moves a synchronized vertical crosshair guide across all other charts on the page at the exact same timestamp.
- **Micro-Spike Visibility**: Real-time 1-second to 5-second polling catching transient load bursts that 30-second Prometheus scrapes miss.
- **Anomaly Overlay**: Subtle colored background tinting when metrics breach adaptive thresholds.

## 3. Action Items
1. **Lightweight Canvas Chart Primitive (`frontend-vue/src/components/telemetry/CanvasSparkline.vue` / `CanvasTimeSeries.vue`)**:
   - Create a Vue 3 component integrating a Canvas-based time-series renderer (or lightweight `uPlot` wrapper).
   - Props: `data: [timestamps, values]`, `color: string`, `height: number`, `unit: string`, `syncGroup?: string`.
   - Emits: `syncTime(timestamp: number | null)`.
2. **Synchronized Crosshair Event Bus (`src/composables/useTimeSync.ts`)**:
   - Provide shared reactive timestamp `activeHoverTime` across all sparklines in the view.
   - When hovering over a metric point on Node CPU, automatically position tooltips on Node Memory, Disk I/O, and Network.
3. **Log & Telemetry Correlation**:
   - Integrate with Task 020 (ClickHouse Log Histogram): Scrubbing the log volume bar highlights the telemetry crosshair at that exact time.
4. **Verification**:
   - `npm.cmd run build` passes with 0 errors.
   - Chrome DevTools MCP performance trace verifies smooth 60fps animation during realtime updates without memory leaks.
