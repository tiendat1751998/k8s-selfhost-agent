# TASK: Deployment Timeline

## Priority: MEDIUM

## Objective
Build a unified timeline view showing deployments, rollbacks, incidents, and git commits on a single chronological view.

## Requirements

### Timeline View
- Horizontal scrollable timeline
- Event types with distinct icons/colors:
  - 🚀 Deployments (blue)
  - ⏪ Rollbacks (orange)
  - 🔴 Incidents (red)
  - 📝 Git Commits (green)
  - 🔧 Config Changes (purple)
- Time range selector (1h, 6h, 24h, 7d, 30d)
- Zoom in/out on timeline

### Event Details
- Click event to see details panel
- Deployment: image, replicas, namespace, status
- Rollback: from version, to version, reason
- Incident: severity, RCA link, resolution
- Commit: author, message, diff link

### Correlation
- Visual correlation lines between related events
- E.g., deployment → incident → rollback chain
- Highlight correlated events on hover

### Filters
- Filter by cluster
- Filter by namespace
- Filter by event type
- Filter by user/actor

## Output
- New section: `#timeline`
- New module: `/modules/timeline/deployment-timeline.js`
- Sidebar entry under "Deployments" group

## Verification
- Navigate to `#timeline`
- Timeline renders with mock events
- Events are clickable with detail panel
- Time range selector works
- Filters narrow displayed events
