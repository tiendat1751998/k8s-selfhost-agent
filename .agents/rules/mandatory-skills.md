# MANDATORY SKILL LOADING — Agent-to-Skill Mapping

> Backup enforcement rule. Every agent MUST read its assigned skills before starting ANY task.
> Main thread (orchestrator) does NOT have a separate agent definition. Its behavior is governed by GEMINI.md, AGENTS.md, and .agents/rules/*.md

## How It Works
Each agent definition in `~/.gemini/config/agents/*.md` contains a `## SKILL LOADING (MANDATORY)` section listing skills to read via `view_file`. This project rule serves as backup documentation and enforcement.

## Universal Rule
ALL agents must call `view_file` on their listed skills BEFORE writing code, running tests, or producing any output. Violation = protocol breach.

## QA-Specific Rule
`qa-test-engineer` MUST use `call_mcp_tool` with `chrome-devtools-mcp` for ALL browser/UI testing. Tools: `navigate_page`, `list_network_requests`, `take_screenshot`, `emulate`. Fabricating browser results is STRICTLY FORBIDDEN.

## Mapping Reference

| Agent | Required Skills |
|-------|----------------|
| backend-coder | verification-before-completion, test-driven-development, systematic-debugging, receiving-code-review, ponytail |
| frontend-coder | verification-before-completion, test-driven-development, systematic-debugging, receiving-code-review, ponytail |
| database-engineer | verification-before-completion, test-driven-development, systematic-debugging, receiving-code-review, ponytail, schema-mapping |
| devops | verification-before-completion, test-driven-development, systematic-debugging, receiving-code-review, ponytail |
| qa-test-engineer | verification-before-completion, systematic-debugging, speckit-checklist + MCP ENFORCEMENT |
| reviewer | verification-before-completion, ponytail-review, requesting-code-review |
| architect | verification-before-completion, writing-plans, brainstorming, speckit-plan, ponytail-audit |
| business-analyst | speckit-specify, speckit-clarify, brainstorming, speckit-checklist |
| governor | speckit-constitution, verification-before-completion |
| planner | speckit-tasks, writing-plans, speckit-analyze |
| product-owner | speckit-taskstoissues, speckit-specify |
| ux-designer | brainstorming |
| release-manager | finishing-a-development-branch, verification-before-completion |
| security-engineer | systematic-debugging, verification-before-completion |
| sre | systematic-debugging, verification-before-completion |
| technical-writer | ponytail-debt |
| fleet-controller | using-git-worktrees |
| loop-operator | executing-plans |
| harness-optimizer | writing-skills |
| performance-engineer | systematic-debugging, verification-before-completion |