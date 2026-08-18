# Workspace Rules

Antigravity reads rules from `.agents/rules/` (global rules live in
`~/.gemini/GEMINI.md`). Each rule is a Markdown file, max 12,000 chars,
registered with YAML frontmatter declaring its **activation mode**:

## Activation modes

| Mode | Frontmatter | When it applies |
|---|---|---|
| Always On | `activation: always_on` | Injected into every session in this workspace |
| Manual | `activation: manual` | Only when mentioned by name (`@rule-name`) |
| Model Decision | `activation: model_decision` + `description:` | Model decides from the description |
| Glob | `activation: glob` + `glob: "**/*.ts"` | Applied when files match the pattern |

## Example

```markdown
---
name: db-safety
activation: always_on
---

All migrations are additive-only. Destructive changes (DROP, TRUNCATE)
require explicit human approval.
```

## Usage

- Copy `example-rule.md` below into a named file, or delete it.
- Keep rules small and specific — prefer the global GEMINI.md for universal
  invariants, workspace rules for project-specific ones.
