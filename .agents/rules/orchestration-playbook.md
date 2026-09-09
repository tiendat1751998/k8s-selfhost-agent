# MANDATORY ORCHESTRATION PLAYBOOK — Enterprise Pipeline Checklist

> Main agent MUST follow this checklist. References: `.agents/rules/enterprise-pipeline.md`
> Chất lượng > Tốc độ. ALL relevant agents MUST participate.

## STEP 0: TOPOLOGY SELECTION
- [ ] Xác định topology: Micro / Fast-Track / Standard / High-Assurance / Full Enterprise
- [ ] Ghi topology vào task tracker
- [ ] Tham chiếu `.agents/rules/enterprise-pipeline.md` cho chi tiết

## STEP 1: HARNESS (Context Gathering)
- [ ] Dispatch `research` scan codebase, gather context
- [ ] Dispatch `security-engineer` nếu có security impact
- [ ] Dispatch `sre` nếu có production impact
- [ ] KHÔNG tự grep, KHÔNG tự scan — dùng research agent

## STEP 2: GRAPH (Planning & Design)
- [ ] Dispatch `business-analyst` viết user stories & acceptance criteria
- [ ] Dispatch `architect` thiết kế giải pháp (ADR nếu architectural change)
- [ ] Dispatch `ux-designer` nếu có UI change
- [ ] Dispatch `planner` phân rã WBS & sắp xếp task sequence
- [ ] Dispatch `product-owner` validate priority & scope
- [ ] KHÔNG dispatch coder TRƯỚC khi planner hoàn thành

## STEP 3: LOOP (Execute → Review → Test)
Per task từ planner:
- [ ] Dispatch coder với `Workspace: "branch"` (goal + AC + file scope + verify cmd)
- [ ] Dispatch `reviewer` audit code
- [ ] Dispatch `qa-test-engineer` test với MCP (chrome-devtools-mcp)
- [ ] Dispatch `performance-engineer` nếu perf-critical
- [ ] Dispatch `governor` validate compliance
- [ ] Dispatch `security-engineer` review nếu security-sensitive
- [ ] Nếu PASS → Merge. Nếu DEFECT → quay lại coder (max 3 lần)

## STEP 4: RELEASE
- [ ] Dispatch `release-manager` approve & tag
- [ ] Dispatch `technical-writer` update docs
- [ ] Dispatch `sre` post-deploy monitoring

## STEP 5: FLEET MANAGEMENT
- [ ] Dispatch `fleet-controller` cleanup worktrees
- [ ] Kill idle subagents
- [ ] Update task tracker

---

## FORBIDDEN ACTIONS (main thread)
```
❌ view_file trên source code (dùng research)
❌ grep_search / find_by_name (dùng research)
❌ run_command cho build/test (subagent tự verify)
❌ call_mcp_tool chrome-devtools-mcp (dùng qa-test-engineer)
❌ replace_file_content / write_to_file trên source code (dùng coder)
❌ Đọc output > 20 lines (yêu cầu subagent summarize)
❌ Dispatch coder TRƯỚC khi planner hoàn thành WBS
```

## ALLOWED ACTIONS (main thread)
```
✅ git merge / commit / push / checkout / log / status
✅ invoke_subagent / send_message / manage_subagents
✅ write_to_file trên artifacts (plan, task, walkthrough)
✅ write_to_file trên .agents/rules/ và .agents/memory/
```
