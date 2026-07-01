# Business Rules

## Incident Lifecycle

```
detected → investigating → identified → mitigating → resolved
                                                        ↓
                                                    closed
```

### Rules:
- Incidents are created with status `detected` and severity `medium` by default.
- RCA can only be triggered on incidents with status `detected` or `investigating`.
- Resolving an incident requires a `resolved_at` timestamp.
- Severity levels: `critical`, `high`, `medium`, `low`, `info`.
- Each incident belongs to a specific `cluster_name`, `namespace`, and `pod_name`.

---

## Fleet Management

### Cluster Registration
- Clusters are registered with encrypted credentials (AES-256-GCM).
- Each cluster has a `provider` type: `kubernetes` or `docker_swarm`.
- Cluster connectivity is verified via `POST /fleet/clusters/{id}/test`.
- Multi-tenant: clusters are scoped by `tenant_id`.

### Resource Discovery
- Resources (Pods, Services, Deployments) are queried live from cluster APIs.
- No resource state is cached permanently — always live queries with context deadlines.

---

## Drift Detection

### Rules:
- Drift is detected by comparing live cluster state against Git repository baseline.
- Drift types: `replica_mismatch`, `image_version`, `config_change`, `missing_resource`.
- Reconciliation applies Git state to live cluster.
- Drift scans can be triggered manually or on schedule.

---

## Change Management

### Approval Flow:
```
draft → pending_approval → approved → executing → completed
                         → rejected
```

### Rules:
- Changes require at least one approver.
- Approved changes can be executed within a configurable time window.
- Rejected changes must include a rejection reason.

---

## Deployment Promotion

### Environment Chain:
```
dev → staging → production
```

### Rules:
- Promotions copy configuration from source to target environment.
- Each promotion is tracked as a transactional operation.
- Rollback is available for failed promotions.

---

## Runbook Automation

### Rules:
- Runbooks define repeatable operational procedures.
- Each runbook has steps that can be executed sequentially.
- Execution results are logged for audit purposes.
- Runbooks can be triggered manually or by automation rules.

---

## Notification Rules

### Channels:
- WebSocket (real-time push to dashboard).
- NATS JetStream (async event bus).

### Rules:
- Notifications are created for: incident detection, RCA completion, drift detection, deployment status changes.
- Users can mark notifications as read.
- Notifications are tenant-scoped.

---

## Multi-Tenancy

### Rules:
- Every data entity is scoped by `tenant_id`.
- JWT token contains tenant claim.
- Cross-tenant data access is forbidden.
- Platform admins can access all tenants.
- Tenant admins can only access their own tenant.
- Viewers have read-only access within their tenant.

---

## AI Agent System (ADK)

### 10 Specialist Agents:
1. **Orchestrator** — Routes tasks to specialist agents
2. **Backend Engineer** — Go service implementation
3. **Frontend Engineer** — Dashboard UI
4. **DBA** — Database schema and queries
5. **DevOps** — Container orchestration and CI/CD
6. **Security** — Security auditing
7. **QA** — Testing and quality assurance
8. **Architect** — System design documentation
9. **Code Reviewer** — Code review and standards
10. **SRE** — Monitoring and incident response

### Rules:
- Orchestrator never writes code directly — always delegates.
- Each agent reads context files before starting work.
- Agent outputs follow the ORCHESTRATOR SUMMARY format.
- Max 3-4 concurrent agents to avoid rate limits.
