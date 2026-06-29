# TASK: AI Copilot — Natural Language Command Interface

## Priority: HIGH

## Objective
Build an AI Copilot that accepts natural language commands and executes platform operations.

## Requirements

### Command Input
- Chat-style command bar (fixed bottom or modal)
- Command history
- Autocomplete suggestions
- Keyboard shortcut (Ctrl+Shift+K)

### Natural Language Processing
- Parse intent from commands like:
  - "Deploy nginx to production"
  - "Scale payment-api to 20 replicas"
  - "Show failed deployments today"
  - "Find all OOMKilled pods"
  - "What is the health of cluster-prod?"
- Show parsed intent confirmation before execution
- Display execution results inline

### Action Execution
- Map parsed intents to platform actions
- Show confirmation dialog for destructive operations
- Display execution logs in real-time
- Show success/failure status

### Context Awareness
- Current cluster context
- Current namespace context
- Recent command context for follow-ups

## Output
- New floating component: AI Copilot bar
- New module: `/modules/ai/ai-copilot.js`
- Accessible from any page via keyboard shortcut
- Sidebar entry under "AI Ops" group

## Verification
- Press Ctrl+Shift+K to open copilot
- Type a command and see parsed intent
- Confirm execution and see results
- Command history persists
