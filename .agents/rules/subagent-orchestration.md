---
name: subagent-orchestration
description: Guidelines for subagent dispatch, workspace branching, and communication.
activation: always_on
---

# Subagent Orchestration Best Practices

1. **Workspace Worktree Mode**:
   - When dispatching parallel coder agents (`backend-coder`, `frontend-coder`), set `Workspace: "branch"` in `invoke_subagent`.
   - Never dispatch parallel agents in `Workspace: "inherit"` if they edit overlapping files.
2. **Context Budget**:
   - Provide only minimal required files and line ranges in the dispatch prompt.
   - Specify unambiguous acceptance criteria and verify commands.
3. **Reactive Communication**:
   - Communicate via `send_message`.
   - Never loop with `manage_subagents` status checks; allow the reactive wakeup mechanism to resume the turn.
