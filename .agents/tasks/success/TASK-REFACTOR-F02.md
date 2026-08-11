# Task: Remove duplicate helpers in search and backup modules
ID: TASK-REFACTOR-F02
Status: success

## Objective
Remove redundant local duplicate functions `esc`, `escapeRegExp`, and `highlightText` in `enterprise-backup.js`, `enterprise-search.js`, and `search-autocomplete.js`.

## Requirements
- Edit `frontend/modules/platform/enterprise-backup.js` and delete the local `esc` function.
- Edit `frontend/modules/search/enterprise-search.js` and delete local `escapeRegExp` and `highlightText` functions. Reference `UIComponents.highlightText` in the exports block.
- Edit `frontend/modules/search/search-autocomplete.js` and delete local `escapeRegExp` and `highlightText` functions.

## Verification
- Code must load successfully without syntax errors in the browser.
