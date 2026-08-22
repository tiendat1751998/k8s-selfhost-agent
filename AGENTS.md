# AGENTS.md — k8s-selfhost-agent Constitution

> Binding project constitution for `k8sseflhost`. Applies to all agents and subagents operating in this workspace.

## 1. Core Directives & Verification

- **Evidence First (Research Gate)**: Read the actual source code with `view_file` or `grep_search` and cite `file:line` before proposing or writing code.
- **Verification-Before-Completion (VBC)**:
  - **Go Backend**: Verify changes using `go test ./...` or `go vet ./...` (or target packages).
  - **Frontend**: Verify builds/lints with `npm run build` or `npm run type-check` where available.
  - **K8s & Infra**: Validate manifests with `kubectl dry-run` or terraform validate where applicable.
  - **Zero Tolerance for Stubs**: No hardcoded mocks, empty TODO handlers, or fake progress.

## 2. Multi-Agent & Subagent Guidelines

- **Isolated Workspaces for Coding**: When dispatching parallel coder subagents, use `Workspace: "branch"` (Git worktree isolation) to prevent concurrent file overwrite conflicts.
- **Context Starvation Diet**: Pass surgical context (`file:line`, specific contract types), never dump entire directories into prompts.
- **Event-Driven Handoff**: Rely on reactive wake-ups via `send_message`. Do not poll `manage_subagents status` in loops.
- **Collusion Defense**: The Orchestrator MUST independently inspect git diffs and command outputs before accepting handoffs.

## 3. Rules and Memory

- Rules reside in `.agents/rules/` (including `git-branching.md`, `subagent-orchestration.md`, `vbc-verification.md`).
- Architectural decisions and task progress must be logged in `.agents/memory/decision_log.jsonl` and `.agents/tasks/`.
- **Project State Memory**: Every session MUST load `.agents/memory/project_state.md` to restore full architectural context, completed modules, and active tasks.

## 4. Git Branching & Protected Master (Universal Invariant)

- **Zero Direct Master Pushes**: NEVER commit or push directly to `master`.
- **Branch Naming**: Always create and checkout `feat/<name>` or `fix/<name>` before coding.
- **Push & Merge Protocol**: Push only to the remote branch (`git push -u origin <branch>`), verify thoroughly, and wait for confirmation before merging to `master`.


