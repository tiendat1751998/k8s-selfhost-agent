# MANDATORY ORCHESTRATION PLAYBOOK — Step-by-Step Checklist

> Main agent PHẢI tuân thủ từng bước. Không bỏ bước. Không làm tắt.

## PER-TASK EXECUTION CHECKLIST

### Step 0: TOPOLOGY (fleet-controller logic)
- [ ] Xác định topology: Fast-Track / Standard / High-Assurance / Emergency
- [ ] Ghi topology vào task tracker

### Step 1: DISPATCH (Graph layer)
- [ ] Dispatch `backend-coder` / `frontend-coder` / `database-engineer` / `devops` với `Workspace: "branch"`
- [ ] Prompt PHẢI có: goal, acceptance criteria, file scope, verify command
- [ ] KHÔNG tự code, KHÔNG tự grep, KHÔNG tự scan

### Step 2: RECEIVE & INSPECT (Graph layer)
- [ ] Đọc report summary từ coder (< 50 lines)
- [ ] Kiểm tra line counts, build result, file list
- [ ] KHÔNG đọc build log dài, KHÔNG đọc file content

### Step 3: REVIEW (Loop:Reflect) — Skip nếu Fast-Track
- [ ] Dispatch `reviewer` với diff summary
- [ ] Đợi verdict: APPROVE hoặc REJECT
- [ ] Nếu REJECT → send feedback về coder (Step 1), count retry
- [ ] Nếu retry >= 3 → escalate `architect`

### Step 4: MERGE (Graph layer)
- [ ] `git -C <worktree> add -A; git -C <worktree> commit -m "..."`
- [ ] `git merge <branch> --no-edit`
- [ ] Rebuild backend nếu Go changes: `go build ./cmd/standalone/...`

### Step 5: QA AUDIT (Loop:Check)
- [ ] Dispatch `qa-test-engineer` với danh sách pages cần check
- [ ] QA dùng `chrome-devtools-mcp`: navigate, list_network_requests, take_screenshot
- [ ] QA report: PASS hoặc DEFECT
- [ ] Nếu DEFECT → dispatch coder fix (Step 1), count retry
- [ ] Nếu retry >= 3 → escalate `architect`

### Step 6: COMPLETE
- [ ] Update task tracker (task.md)
- [ ] Kill idle subagents
- [ ] Proceed to next task

---

## FORBIDDEN ACTIONS (main thread)
```
❌ view_file trên source code (dùng research)
❌ grep_search / find_by_name (dùng research)
❌ run_command cho build/test (subagent tự verify)
❌ call_mcp_tool chrome-devtools-mcp (dùng qa-test-engineer)
❌ replace_file_content / write_to_file trên source code (dùng coder)
❌ Đọc output > 20 lines (yêu cầu subagent summarize)
```

## ALLOWED ACTIONS (main thread)
```
✅ git merge / commit / push / checkout / log / status
✅ invoke_subagent / send_message / manage_subagents
✅ write_to_file trên artifacts (plan, task, walkthrough)
✅ write_to_file trên .agents/rules/ và .agents/memory/
```
