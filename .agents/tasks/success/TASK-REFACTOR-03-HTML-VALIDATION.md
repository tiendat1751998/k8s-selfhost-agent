# TASK-REFACTOR-03-HTML-VALIDATION: HTML and Structure Cleanup

## Goal
Optimize and audit `frontend/index.html` to eliminate invalid markup and duplicate section elements.

## Scope
- Resolve the duplicate `id="section-enterprise"` sections in `index.html` by separating tenant management structures and RBAC roles cleanly.
- Ensure unique IDs exist for all standard input elements and tabs.
- Validate that the HTML structure is syntactically valid and free of duplicate ID attributes.

## Success Criteria
- DOM queries do not retrieve wrong elements due to duplicate IDs.
- HTML builds/renders cleanly.
