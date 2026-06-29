# TASK: Observability — SLO Dashboard, Traces & Events

## Priority: HIGH

## Objective
Build an Observability module covering SLO dashboards, error budgets, distributed traces, and Kubernetes events.

## Requirements

### SLO Dashboard
- Define SLO targets (99.9%, 99.95%, etc.)
- SLO compliance gauge per service
- Burn rate indicator
- SLO history (30d rolling)

### Error Budgets
- Remaining error budget percentage
- Budget consumption rate
- Budget alerts (50%, 75%, 90% thresholds)

### Traces View
- Distributed trace list with latency/status
- Trace detail waterfall view
- Span breakdown per service
- Filter by service, status, duration

### Events Stream
- Kubernetes event feed (Warning, Normal)
- Event filtering by namespace, type, reason
- Event correlation with incidents

## Output
- New section: `#observability`
- New module: `/modules/observability/observability.js`
- New CSS styles for SLO gauges and trace waterfalls
- Sidebar entry under "Operations" group

## Verification
- Navigate to `#observability`
- SLO gauges render with mock targets
- Error budget bars display
- Trace waterfall renders correctly
- Events stream populates
