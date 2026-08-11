# Task: Consolidated Helpers in sanitizer.js and components.js
ID: TASK-REFACTOR-F01
Status: success

## Objective
Centralize `escapeRegExp` in `Security` (`sanitizer.js`) and `highlightText` in `UIComponents` (`components.js`), expose them globally, and remove dead DOM-based `escapeHtml` code.

## Requirements
- Edit `frontend/core/utils/sanitizer.js` to add `escapeRegExp(string)` to `Security` object.
- Edit `frontend/core/utils/components.js` to add `highlightText(text, term)` to `UIComponents` object.
- In `components.js`, expose `window.highlightText = UIComponents.highlightText`.
- In `components.js`, remove the unused DOM-based `escapeHtml` function.

## Verification
- Code must load successfully without syntax errors in the browser.
