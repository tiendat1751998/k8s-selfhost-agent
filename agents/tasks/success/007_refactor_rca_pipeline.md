# TASK: Refactor internal/usecase/rca/pipeline.go
## Target File Path: internal/usecase/rca/pipeline.go
## Current Status: SUCCESS

## Critical Issues Found
- **Missing error propagation**: If updating the incident status (`incRepo.Update`) fails in the error recovery blocks, the error is ignored.
- **Brittle JSON Extraction**: `extractJSON` implements a naive bracket-counting algorithm to parse markdown-wrapped JSON, which can easily fail if string literals contain brackets.

## Action Plan & Refactoring Rules
1. Log ignored errors during fallback state updates (`inc.MarkFailed()`).
2. Replace `extractJSON` with a robust regex (e.g., matching ```json ... ``` blocks) or use standard library tokenization to extract JSON safely.
