# Git Branching Strategy & Zero Direct Master Push

> **Mandatory Policy**: Applies to all sessions and subagents in `k8sseflhost`.

## 1. Branching Invariant
- **NEVER push directly to `master`**.
- Every task, bugfix, or feature MUST be developed on a dedicated branch:
  - `feat/<short-description>` for new features
  - `fix/<short-description>` for bug fixes
  - `refactor/<short-description>` for refactoring
- Checkout the new branch before writing or committing code.

## 2. Commit & Push Protocol
- Use Conventional Commits format: `type(scope): description`.
- Push ONLY to the remote feature branch: `git push -u origin <branch-name>`.
- The `master` branch is protected and reserved for stable, verified code.

## 3. Merging
- After full automated verification (tests, builds, lints) and live QA verification (via Chrome DevTools MCP where applicable), report the branch name and verification summary to the user.
- Merge into `master` only after confirmation.

## 4. Post-Merge Branch Cleanup (MANDATORY)
Immediately after a branch is merged into `master` (local merge or GitHub PR) and post-merge verification passes, the orchestrator MUST delete that branch everywhere:
```powershell
git fetch origin --prune
git merge-base --is-ancestor <branch> origin/master   # exit 0 = merged; otherwise STOP
git branch -d <branch>                                 # safe delete only
git push origin --delete <branch>                      # only if it was pushed
git worktree prune                                     # drop stale subagent worktrees
```
- Applies to `feat/*`, `fix/*`, `refactor/*` and `subagent-*` worktree branches after their merge.
- Use `git branch -d` ONLY. `git branch -D` (force) on an unmerged branch requires explicit user approval.
- Never delete `master`, the currently checked-out branch, or a branch whose PR is still open.
- Report deleted branches in the merge summary.

## 5. Never Merge Rewritten History
- If `master` and a branch have divergent rewritten histories (e.g. after a secret purge), do NOT run `git merge master` / `git pull origin master` — it re-imports purged commits and produces mass add/add conflicts. Stop and escalate to the user.
