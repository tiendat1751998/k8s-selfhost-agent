# BINDING RULE: 3-Layer Agent Orchestration (Harness → Graph → Loop)

> ADR-024 — 2026-09-09. Binding for all orchestrated tasks.

## Main Agent = Orchestrator (Graph Layer) — ZERO EXCEPTIONS

### MUST DO (via subagents only)
- **Code** → `backend-coder` / `frontend-coder` / `database-engineer` / `devops` (Loop layer)
- **MCP audit** → `qa-test-engineer` (Loop layer)
- **Code review** → `reviewer` (Loop layer)
- **Research/scan** → `research` (Harness layer)
- **Architecture** → `architect` (Graph layer)
- **Fleet health** → `fleet-controller` (Supervisor)

### MUST NOT DO (directly in main thread)
- ❌ Write/edit any source code
- ❌ Run `chrome-devtools-mcp` tools
- ❌ Run grep/scan/find to inspect codebase
- ❌ Run build/test verification commands
- ❌ Consume tokens on long outputs

### ALLOWED (orchestrator duties)
- ✅ `git merge`, `git commit`, `git push`, `git checkout`
- ✅ `manage_subagents`, `invoke_subagent`, `send_message`
- ✅ Create/update artifacts
- ✅ Brief `git status`, `git log` (< 5 lines)

---

## Circular Review Pipeline (per task)

```
CODER (Loop:Do) → REVIEWER (Loop:Reflect) → QA+MCP (Loop:Check) → MERGE
       ◀── REJECT ──┘                ◀── DEFECT ──┘
       (max 3 rounds, then escalate to architect)
```

## Topology Selection

| Topology | When | Pipeline |
|:---|:---|:---|
| **Fast-Track** | < 50 lines, pure split | Coder → QA → Merge |
| **Standard** | Features & fixes | Coder ↔ Reviewer ↔ QA (3 rounds) |
| **High-Assurance** | DB/DDD/multi-module | Coder → Rev → Perf → QA → Sec → PO |
| **Emergency** | Production incident | SRE → Coder → QA → Release Manager |

## 3-Layer Agent Mapping

**Harness** (6): harness-optimizer, security-engineer, sre, research, integrity-protocol, technical-writer
**Graph** (7): orchestrator, planner, architect, product-owner, business-analyst, ux-designer, release-manager
**Loop** (9): backend-coder, frontend-coder, database-engineer, devops, qa-test-engineer, reviewer, performance-engineer, governor, loop-operator
**Supervisor** (1): fleet-controller

## Token Conservation
1. Orchestrator NEVER reads long outputs
2. Subagents summarize results in < 50 lines
3. Kill idle subagents after task completion
4. Reuse existing subagent conversations via `send_message`
