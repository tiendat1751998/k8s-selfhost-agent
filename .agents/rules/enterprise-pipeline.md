# ENTERPRISE PIPELINE — COMPREHENSIVE 12-SCENARIO DYNAMIC ROUTING PROTOCOL

> Binding framework for the Main Agent (Orchestrator).
> Principle: Identify scenario first -> Print Scenario Tag -> Dispatch ONLY designated agents -> Enforce decoupled I/O (RECEIVE -> DO -> RETURN).

---

## 🗺️ THE 12 ENTERPRISE SCENARIOS (EXHAUSTIVE ROUTING MATRIX)

### [CORE CODING & DEFECTS]
1. **SCENARIO 1: Micro / Cosmetic UI Fix (Typo, Button Style, CSS Tweaks)**
   - **Pipeline**: `frontend-coder` -> `qa-test-engineer` (MCP) -> `reviewer` -> MERGE
   - **Bypassed**: BA, Architect, Planner, SRE, DB, Security, PO

2. **SCENARIO 2: Logic Bugfix / API Defect (404/500, Nil Pointer, Input Validation)**
   - **Pipeline**: `research` -> `planner` -> `backend-coder` -> `reviewer` -> `qa-test-engineer` -> MERGE
   - **Bypassed**: UX, PO, BA, SRE

3. **SCENARIO 3: Security Patching (RBAC Bypass, SQLi, Auth, SSRF, Crypto Leak)**
   - **Pipeline**: `security-engineer` (PoC/Audit) -> `planner` -> `backend-coder` / `database-engineer` -> `reviewer` -> `security-engineer` (Re-test/Verify) -> `qa-test-engineer` -> MERGE
   - **Bypassed**: UX, PO, SRE

### [DATABASE, SYSTEM & PERFORMANCE]
4. **SCENARIO 4: Performance Degradation & Production Incident (Slow DB, High CPU/RAM, N+1)**
   - **Pipeline**: `sre` (Telemetry/Bottleneck) -> `performance-engineer` (Benchmark/Profile) -> `database-engineer` / `backend-coder` -> `qa-test-engineer` -> `reviewer` -> MERGE
   - **Bypassed**: UX, PO, BA

5. **SCENARIO 5: Database Evolution & Migrations (New Tables, Schema DDL, Indexing)**
   - **Pipeline**: `architect` (Data model) -> `database-engineer` (DDL/DML Up/Down) -> `backend-coder` (Repo/Entity) -> `reviewer` -> `qa-test-engineer` -> MERGE
   - **Bypassed**: UX, SRE, PO

### [INFRASTRUCTURE & ARCHITECTURE]
6. **SCENARIO 6: Infrastructure, K8s, Docker & CI/CD (Helm, Manifests, Traefik, Dockerfile)**
   - **Pipeline**: `devops` -> `security-engineer` (Least privilege/Secret audit) -> `qa-test-engineer` (Dry-run/Cluster validation) -> `sre` -> MERGE
   - **Bypassed**: UX, PO, BA, Frontend

7. **SCENARIO 7: Codebase Refactoring & Technical Debt (Monolith Shredding, Clean Arch)**
   - **Pipeline**: `architect` (Component boundaries) -> `planner` (Topological WBS) -> `backend-coder` / `frontend-coder` -> `reviewer` (`ponytail-review`) -> `qa-test-engineer` -> MERGE
   - **Bypassed**: BA, PO, SRE

### [DOCUMENTATION, GOVERNANCE & FLEET]
8. **SCENARIO 8: Documentation, OpenAPI Specs & Runbooks (API Docs, Mermaid, Playbooks)**
   - **Pipeline**: `business-analyst` (Scope/Flows) -> `technical-writer` (Markdown/OpenAPI) -> `reviewer` -> MERGE
   - **Bypassed**: Coders, DB, SRE, QA

9. **SCENARIO 9: Compliance, Governance & Audit (SOC2, Multi-Tenant Isolation Audit)**
   - **Pipeline**: `governor` (Rules/Boundary audit) -> `security-engineer` (Pen-test/SAST) -> `business-analyst` -> `planner` -> REMEDIATION PIPELINE
   - **Bypassed**: UI, Marketing

10. **SCENARIO 10: Agent Harness & Meta-Optimization (Rules, MCP Configs, Skills, Prompts)**
    - **Pipeline**: `harness-optimizer` (Audit harness/tokens) -> `governor` (Rules check) -> `loop-operator` (Loop validation) -> MERGE
    - **Bypassed**: Coders, QA

### [FEATURES & EMERGENCIES]
11. **SCENARIO 11: Complex Major Feature (New Payment System, AI Assistant Hub, New Module)**
    - **Pipeline (Full Corporate Lifecycle)**:
      1. `business-analyst` (User stories & Gherkin AC)
      2. `product-owner` (Scope boundaries & prioritization)
      3. `planner` (Topological WBS task tree)
      4. Parallel Design: [`architect` (ADR) + `ux-designer` (Layout/Tokens) + `database-engineer` (Schema)]
      5. Parallel Implementation: [`backend-coder` (APIs) + `frontend-coder` (UI)]
      6. Quality Gates: `qa-test-engineer` (MCP) -> [`reviewer` (Code quality) + `security-engineer` (Vulnerability audit)]
      7. Governance & Rollout: `release-manager` -> `governor` (Anti-collusion) -> RELEASE

12. **SCENARIO 12: Emergency Production Hotfix (Server Down, CrashLoopBackOff, Data Loss)**
    - **Pipeline**: `sre` (Incident commander) -> `backend-coder` / `devops` (Minimal hotfix) -> `qa-test-engineer` (Sanity check) -> `release-manager` (Emergency deploy)
    - **Bypassed**: All non-essential agents (Fastest route to restore service)

---

## 🔒 HOW TO ENFORCE 100% COMPLIANCE (THE 4 ENFORCEMENT GATES)

### GATE 1: Zero-Shot Scenario Tagging (Main Agent Turn 1)
Every response by the Orchestrator MUST start with an explicit scenario declaration:
`[ROUTING CLASSIFICATION: SCENARIO <ID> - <NAME>]`
If a task does not fit, the Orchestrator maps it to the nearest topology. It is forbidden to dispatch subagents without declaring the scenario.

### GATE 2: Git Worktree Physical Sandboxing
Every coder agent is dispatched with `Workspace: "branch"`. Subagents have NO physical write access to the active branch or master. They cannot merge their own code. Only the Orchestrator inspects and merges the worktree upon passing all quality gates.

### GATE 3: Pre-Flight Transcript & Evidence Inspection
Before accepting any subagent report, the Orchestrator inspects the subagent's transcript:
- Did Coder execute `view_file` on `project_state.md` and required skills at Step 0?
- Did QA call actual `call_mcp_tool` (`chrome-devtools-mcp`) and output screenshot paths?
- Did Reviewer cite exact lines of code under the 7-Point Audit Checklist?
If NO -> Orchestrator automatically REJECTS the handoff and triggers a redo.

### GATE 4: The Decoupled I/O Contract (RECEIVE -> DO -> RETURN)
- Subagents NEVER chat peer-to-peer.
- Every dispatch payload strictly defines:
  `I RECEIVE`: Whitelist of allowed files, exact AC, upstream design artifacts.
  `I DO`: Execution in isolated worktree with native verification commands.
  `I RETURN`: Structured summary (< 50 lines), verbatim terminal exit codes, screenshot URIs.
