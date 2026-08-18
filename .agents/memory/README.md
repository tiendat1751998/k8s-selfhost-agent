# Decision Log

Non-trivial engineering decisions are recorded here (see GEMINI.md §6 and
AGENTS.md). Format: one JSON object per line (JSONL). Newest entries last.

## Schema

```json
{
  "ts": "2026-08-14T10:00:00Z",
  "decision": "short title",
  "context": "what prompted the decision",
  "options": ["option A", "option B"],
  "choice": "option B",
  "reason": "why this option won",
  "consequences": ["what it costs / enables"],
  "author": "agent or person"
}
```

## Requirements

- Append-only: never edit or delete past entries — supersede instead.
- One line per decision; valid JSON only (no trailing commas, no comments).
- Record when: choosing a framework/DB/architecture, changing public API,
  adding dependencies, or any decision GEMINI.md §5 (scope) flagged as risky.
