---
name: vbc-verification
description: Verification-Before-Completion (VBC) and Zero-Hallucination rule.
activation: always_on
---

# Verification-Before-Completion (VBC) Protocol

1. **Evidence-Based Completion**: A task is never marked as complete without running relevant verification commands (tests, builds, linter) and inspecting exact outputs.
2. **Anti-Hallucination Rule**: Never guess package paths, struct fields, or API endpoints. Read real source files and cite `file:line`.
3. **No Stub Progress**: Stubs, TODO mocks, or simulated passing tests are treated as bugs.
4. **Recovery Limit**: Max 3 distinct attempts when fixing an issue. If still failing, stop and escalate with verbatim logs.
