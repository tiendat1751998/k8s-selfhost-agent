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

## Enterprise Pipeline Enforcement

> Added 2026-09-09. References `.agents/rules/enterprise-pipeline.md`

### MANDATORY: Planning Before Coding
- Orchestrator MUST dispatch `planner` BEFORE dispatching any coder agent
- Coder agents receive tasks from planner's WBS, NOT directly from user request
- Exception: Micro topology (typo fix, 1-line change)

### MANDATORY: Full Review Chain
- Every code change MUST go through: Coder → Reviewer → QA (MCP) → Merge
- High-Assurance adds: Performance → Governor → Security
- Exception: Micro topology

### ALL 20 Agents Must Be Used
- No agent should be permanently idle
- Each topology defines which agents participate
- Orchestrator selects topology based on task complexity

## Anti-Stupidity & Anti-Lazy Hard Enforcement

> References `.agents/rules/anti-stupid-guardrails.md`

### 1. Mandatory Dispatch Contract
Every subagent dispatch MUST explicitly mandate:
1. `view_file` on listed skills + target source code as Step 0.
2. Exact file whitelist (no touching out-of-scope files).
3. Explicit verifiable acceptance criteria.
4. Negative constraints (no stubs, no fake data, no `_ = err`, strictly < 500 lines/file).
5. Exact verification evidence required.

### 2. Zero-Tolerance Rejection (Fail-Closed)
Orchestrator MUST reject handoffs immediately if:
- Coder did not run verification commands or provide verbatim output.
- QA claimed tests passed without calling `chrome-devtools-mcp` (`call_mcp_tool`) and capturing real screenshots.
- Any modified file exceeds 500 lines.
- Any mock/stub data or discarded error (`_ :=`) was introduced.
