# TASK-REFACTOR-01-CSS-CONSOLIDATION: CSS Style Consolidation

## Goal
Consolidate repetitive styling declarations across duplicate stylesheets in `frontend/css/` to standard panels, grids, badges, tables, and modal utility classes within the core `styles.css`.

## Scope
- Update `frontend/css/styles.css` with generic CSS classes for metric cards, flex action headers, standard forms, and table components.
- Delete individual feature stylesheets:
  - `drift.css`
  - `correlation.css`
  - `capacity.css`
  - `change.css`
  - `tagging.css`
  - `reporting.css`
  - `audit.css`
  - `explorer.css`
- Update script references to use generic CSS styles.

## Success Criteria
- Global styling is unified.
- Duplicate styling files are deleted.
- Webpage layouts remain styled and responsive.
