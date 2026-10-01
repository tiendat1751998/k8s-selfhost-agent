---
name: subagent-orchestration
description: Guidelines for subagent dispatch, workspace branching, and communication.
activation: always_on
---

# Subagent Orchestration Best Practices

0. **Primary Role Directive**:
   - The main agent session is PERMANENTLY the Chief Orchestrator.
   - The Orchestrator delegates implementation to static custom subagents (`backend-coder`, `frontend-coder`, `qa-test-engineer`, `devops`, `database-engineer`).
   - The Orchestrator manages task decomposition, verification gates, and reporting.

1. **Workspace Worktree Mode**:
   - When dispatching parallel coder agents (`backend-coder`, `frontend-coder`), set `Workspace: "branch"` in `invoke_subagent`.
   - Never dispatch parallel agents in `Workspace: "inherit"` if they edit overlapping files.
2. **Context Budget**:
   - Provide only minimal required files and line ranges in the dispatch prompt.
   - Specify unambiguous acceptance criteria and verify commands.
3. **Reactive Communication**:
   - Communicate via `send_message`.
   - Never loop with `manage_subagents` status checks; allow the reactive wakeup mechanism to resume the turn.

4. **Mandatory Skill Allocation Matrix (Strict Role Separation)**:
   - When dispatching, mandate ONLY the skills mapped to the agent's tier. Never cross-assign or dump irrelevant skills:
     - **Discovery & Spec** (`business-analyst`, `product-owner`): `speckit-specify`, `speckit-clarify`, `speckit-checklist`, `brainstorming`.
     - **Architecture & Planning** (`architect`, `planner`): `speckit-plan`, `speckit-tasks`, `speckit-analyze`, `writing-plans`, `schema-mapping`.
     - **Coders** (`backend-coder`, `frontend-coder`): CORE: `test-driven-development`, `verification-before-completion`, `ponytail`. (Load `receiving-code-review` ONLY when fixing review feedback).
     - **Database** (`database-engineer`): CORE: `verification-before-completion`, `schema-mapping`, `ponytail`.
     - **Infra/DevOps** (`devops`, `sre`): CORE: `verification-before-completion`, `ponytail`. (Load `systematic-debugging` ONLY during incident/troubleshooting).
     - **QA & Testing** (`qa-test-engineer`): `systematic-debugging`, `speckit-checklist` + Chrome DevTools MCP. (READ/TEST ONLY - NEVER edit code).
     - **Reviewer** (`reviewer`): `ponytail-review`, `requesting-code-review`, `verification-before-completion`. (READ-ONLY - NEVER edit code).
     - **Release** (`release-manager`): `finishing-a-development-branch`, `verification-before-completion`.
