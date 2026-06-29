# TASK: Workflow Automation Engine

## Priority: HIGH

## Objective
Build a visual workflow automation system with event-driven rules.

## Requirements

### Rule Builder
- Visual rule creation form
- Trigger conditions:
  - Pod restart count > threshold
  - Node pressure detected
  - Deployment failure
  - High error rate
  - SLO breach
- Actions:
  - Generate RCA
  - Send notification
  - Auto-rollback
  - Scale deployment
  - Cordon node
  - Create incident

### Rule Management
- List all automation rules
- Enable/disable individual rules
- Edit existing rules
- Delete rules
- Rule execution history

### Execution Log
- Timeline of triggered automations
- Trigger event details
- Action taken
- Result status (success/failure)

## Output
- New section: `#workflow-automation`
- New module: `/modules/automation/workflow-automation.js`
- Sidebar entry under "Operations" group

## Verification
- Navigate to `#workflow-automation`
- Create a new rule with trigger and action
- View rule in rules list
- Toggle rule enable/disable
- View execution history
