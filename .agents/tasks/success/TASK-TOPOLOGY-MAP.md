# TASK: Topology Map & Service Dependency Graph

## Priority: HIGH

## Objective
Build a visual topology map showing cluster-to-pod hierarchy and a service dependency graph.

## Requirements

### Topology Map
- Interactive visual map showing:
  - Cluster → Namespace → Deployment → Pod → Service → Ingress
- Node status coloring (healthy=green, warning=yellow, critical=red)
- Click-to-drill navigation
- Zoom and pan controls
- Namespace filter dropdown

### Service Dependency Graph
- Service-to-service dependency visualization
- Request flow direction arrows
- Latency annotations on edges
- Error rate highlighting
- Dependency health indicators

## Output
- New section: `#topology`
- New module: `/modules/topology/topology.js`
- Canvas or SVG-based rendering
- Sidebar entry under "Infrastructure" group

## Verification
- Navigate to `#topology`
- Topology tree renders with mock cluster data
- Nodes are clickable and drill down
- Dependency graph shows service connections
- Color coding reflects health status
