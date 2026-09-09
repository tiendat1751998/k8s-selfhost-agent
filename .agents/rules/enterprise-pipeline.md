# ENTERPRISE PIPELINE — Mandatory Agent Orchestration Flow

> Binding rule. Main thread (orchestrator) MUST follow this pipeline for EVERY task.
> Quality > Speed. ALL relevant agents MUST participate. No shortcuts.

## 3-Layer Architecture

- **Harness**: Environment setup, context gathering, security scanning
- **Graph**: Planning, design, prioritization, task decomposition  
- **Loop**: Implementation, review, testing, compliance (iterative)

## Pipeline Phases

### PHASE 1: HARNESS (Context Gathering)
| Step | Agent | Action | Skip Condition |
|------|-------|--------|----------------|
| 1.1 | `research` | Scan codebase, gather context, identify affected files | Never skip |
| 1.2 | `security-engineer` | Threat model, vulnerability scan | Skip if pure UI/docs change |
| 1.3 | `sre` | SLO/reliability impact assessment | Skip if non-production change |

### PHASE 2: GRAPH (Planning & Design)
| Step | Agent | Action | Skip Condition |
|------|-------|--------|----------------|
| 2.1 | `business-analyst` | Write user stories & acceptance criteria | Skip if AC already provided by user |
| 2.2 | `architect` | System design, ADR, component boundaries | Skip if < 50 lines, no architectural impact |
| 2.3 | `ux-designer` | UI/UX flow, design tokens, accessibility | Skip if backend-only change |
| 2.4 | `planner` | WBS decomposition, task sequencing, dependencies | Never skip (even for simple tasks, planner sequences) |
| 2.5 | `product-owner` | Priority validation, scope check | Skip if task is pre-approved by user |

### PHASE 3: LOOP (Execute → Review → Test)
For EACH task from planner's WBS:

```
LOOP (max 3 iterations):
  1. CODER implements (Workspace: branch)
     → backend-coder / frontend-coder / database-engineer / devops
  2. REVIEWER audits code quality & SOLID compliance
  3. QA tests with chrome-devtools-mcp (MANDATORY for UI changes)
  4. PERFORMANCE benchmarks (if perf-critical)
  5. GOVERNOR validates compliance & anti-fabrication
  6. SECURITY reviews (if security-sensitive)
  
  └─ ALL PASS → Merge to main branch
  └─ ANY DEFECT → Back to step 1 (count retry)
  └─ 3 failures → Escalate to architect for redesign
```

### PHASE 4: RELEASE
| Step | Agent | Action | Skip Condition |
|------|-------|--------|----------------|
| 4.1 | `release-manager` | Approve release, tag version | Skip if not releasing |
| 4.2 | `technical-writer` | Update docs, changelog, API reference | Skip if no public API change |
| 4.3 | `sre` | Post-deploy monitoring plan | Skip if not deploying |

### PHASE 5: FLEET MANAGEMENT
| Step | Agent | Action | Skip Condition |
|------|-------|--------|----------------|
| 5.1 | `fleet-controller` | Cleanup worktrees, kill idle agents | Never skip |

## Topology Selection

Orchestrator MUST select topology BEFORE dispatching any agent:

| Topology | When | Phases Used |
|:---|:---|:---|
| **Micro** | Typo, 1-line fix, comment | Phase 3 only (coder → merge) |
| **Fast-Track** | < 50 lines, simple fix | Phase 1.1 + Phase 3 (coder → reviewer → QA → merge) |
| **Standard** | Features & bug fixes | Phase 1 + Phase 2 (planner) + Phase 3 + Phase 5 |
| **High-Assurance** | DB/DDD/multi-module | Phase 1 + Phase 2 (all) + Phase 3 (all gates) + Phase 4 + Phase 5 |
| **Full Enterprise** | New feature, architectural change | ALL phases, ALL agents |

## Diagnostic: When Things Go Wrong

| Symptom | Layer to Check |
|---------|---------------|
| Missing tools, permissions, context | HARNESS — research/harness-optimizer |
| Confused context, stale data | HARNESS — research re-scan |
| Wrong approach, poor estimation | GRAPH — planner/architect redesign |
| Weak review, poor test coverage | LOOP — reviewer/qa escalation |
| Merge conflicts, bad sequencing | GRAPH — planner re-sequence |
| Performance regression | LOOP — performance-engineer |
| Security vulnerability | HARNESS + LOOP — security-engineer |
