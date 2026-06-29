# TASK: Cost Management Dashboard

## Priority: HIGH

## Objective
Build a Cost Management module that provides visibility into cluster, namespace, and deployment costs.

## Requirements

### Cluster Cost View
- Total cost per cluster (monthly/daily)
- Cost breakdown by namespace
- Cost trend charts (7d, 30d, 90d)

### Namespace Cost View
- Resource consumption per namespace
- CPU/Memory cost allocation
- Storage cost tracking

### Deployment Cost View
- Per-deployment resource cost
- Idle resource detection
- Resource waste alerts

### Resource Waste Detection
- Underutilized pods (CPU < 10%, Memory < 20%)
- Orphaned PVCs
- Unused ConfigMaps/Secrets
- Over-provisioned deployments

## Output
- New section: `#cost-management`
- New module: `/modules/cost/cost-management.js`
- New CSS: `/css/cost.css`
- Sidebar entry under "Platform" group

## Verification
- Navigate to `#cost-management`
- Verify cost cards render with mock data
- Verify waste detection table renders
- Verify cost trend charts display
