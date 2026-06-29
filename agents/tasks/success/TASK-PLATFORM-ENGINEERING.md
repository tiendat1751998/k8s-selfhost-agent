# TASK: Platform Engineering — Golden Templates & Developer Experience

## Priority: HIGH

## Objective
Build a platform engineering module with golden templates, deployment blueprints, and self-service developer tools.

## Requirements

### Service Catalog
- Pre-built service templates (Web App, API Service, Worker, CronJob, StatefulSet)
- Template parameters (name, image, replicas, resources)
- One-click deploy from template
- Template versioning

### Golden Templates
- Organization-approved deployment templates
- Enforced resource limits
- Pre-configured health checks
- Built-in security policies

### Environment Templates
- Dev / Staging / Production environment presets
- Auto-configured networking
- Auto-configured resource quotas
- Environment cloning

### Deployment Blueprints
- Multi-service deployment blueprints
- Blueprint editor (YAML preview)
- Blueprint library with categories
- Import/export blueprints

### Developer Self-Service
- Self-service namespace creation
- Self-service deployment
- Deployment preview environments
- Deployment diff viewer (before/after YAML comparison)

## Output
- New section: `#platform-engineering`
- New module: `/modules/platform-eng/platform-engineering.js`
- Sidebar entry under "Platform" group

## Verification
- Navigate to `#platform-engineering`
- Service catalog lists templates
- Deploy from template works
- Environment template selector renders
- Blueprint editor shows YAML
- Diff viewer shows comparison
