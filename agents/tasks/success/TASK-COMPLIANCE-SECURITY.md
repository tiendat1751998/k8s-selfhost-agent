# TASK: Compliance & Security Posture Dashboard

## Priority: HIGH

## Objective
Build a compliance dashboard showing security posture, policy violations, and audit compliance.

## Requirements

### Compliance Dashboard
- Overall compliance score (0-100%)
- Compliance by framework (CIS, SOC2, PCI-DSS, HIPAA)
- Compliance trend over time
- Non-compliant resources list

### Security Posture
- Container image vulnerability summary
- Privileged container detection
- Network policy coverage
- Pod security standards compliance
- RBAC over-permission detection

### Policy Violations
- Active policy violations list
- Violation severity (Critical, High, Medium, Low)
- Affected resources
- Remediation suggestions
- Violation trend chart

### Secret Management Overview
- Secrets inventory per namespace
- Secret rotation status
- Expired/expiring secrets alerts

## Output
- New section: `#compliance`
- New module: `/modules/compliance/compliance.js`
- Sidebar entry under "Platform" group

## Verification
- Navigate to `#compliance`
- Compliance score gauge renders
- Policy violations table displays
- Security posture cards render
- Framework compliance breakdown shows
