# ROLE

You are a Senior Product Architect and Enterprise DevOps UI Engineer.

Extend the existing frontend dashboard.

Current capabilities:

* Monitor clusters
* Monitor nodes
* Monitor pods
* Add Kubernetes clusters
* Display incidents
* Display metrics

The dashboard currently behaves as a read-only monitoring system.

Transform it into an operational control plane by adding ACTION modules.

Do NOT redesign existing pages.

Add new actions, workflows and operational capabilities.

---

# ACTION CENTER

Create a new top-level navigation item:

Action Center

This page is the operational hub for platform engineers and SREs.

---

# KUBERNETES ACTIONS

For every cluster, node, deployment, statefulset and pod provide actions.

## Cluster Actions

* Refresh cluster inventory
* Trigger health scan
* Trigger compliance scan
* Export cluster report
* Run RCA analysis
* Generate remediation plan

---

## Node Actions

* Cordon node
* Uncordon node
* Drain node
* Restart kubelet workflow
* Run diagnostics
* View resource pressure report

Require confirmation dialog before execution.

---

## Deployment Actions

* Restart deployment
* Scale deployment
* Rollback deployment
* Pause rollout
* Resume rollout
* View rollout history

---

## StatefulSet Actions

* Restart StatefulSet
* Scale StatefulSet
* View PVC status
* Validate storage health

---

## Pod Actions

* Restart pod
* Delete pod
* Open live logs
* Execute diagnostics
* View YAML
* Compare current vs desired state

---

# INCIDENT ACTIONS

Every incident card must expose actions.

## Available Actions

* Analyze with AI
* Generate RCA
* Generate remediation
* Create GitOps patch
* Create Pull Request
* Assign owner
* Mark resolved

---

# AI RCA ACTIONS

Create an AI Actions section.

## Actions

* Analyze cluster
* Analyze namespace
* Analyze deployment
* Analyze pod
* Explain failure
* Predict root cause
* Generate fix

Display result in side panel.

---

# GITOPS ACTIONS

For Git-integrated environments:

## Actions

* Create branch
* Generate patch
* Generate Helm values update
* Generate Kustomize patch
* Create Pull Request
* View PR status
* Rollback change

---

# AGENT RUNNER ACTIONS

Add operational controls.

## Controls

* Start agent
* Stop agent
* Pause agent
* Resume agent
* Retry failed task
* Execute selected task
* Execute full TASKS.md

---

# MAINTENANCE ACTIONS

Create maintenance workflows.

## Available

* Enter maintenance mode
* Exit maintenance mode
* Schedule maintenance window
* Broadcast maintenance notification

---

# SECURITY ACTIONS

## Available

* Rotate credentials
* Rotate tokens
* Verify cluster access
* Run security audit
* Generate compliance report

---

# BULK ACTIONS

Support selecting multiple resources.

Examples:

* Restart 10 pods
* Scale multiple deployments
* Run diagnostics on multiple nodes
* Generate reports for multiple clusters

---

# ACTION EXECUTION UX

Every action must have:

* Confirmation dialog
* Progress indicator
* Execution log panel
* Result status
* Audit trail entry

---

# ACTION HISTORY

Create Action History page.

Columns:

* Timestamp
* User
* Action
* Target
* Status
* Duration

---

# DASHBOARD TRANSFORMATION GOAL

Current:
Read-only monitoring dashboard.

Target:
Full DevOps Control Plane capable of:

* Monitoring
* Troubleshooting
* Remediation
* GitOps automation
* AI-assisted operations
* Cluster lifecycle management

The UI should feel closer to:

* ArgoCD
* Rancher
* OpenShift Console
* Datadog Workflow Automation
* Portainer Business Edition

rather than a simple monitoring dashboard.
