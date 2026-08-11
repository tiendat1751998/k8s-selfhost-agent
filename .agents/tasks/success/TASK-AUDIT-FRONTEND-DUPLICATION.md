# Task: Resolve Frontend Code Duplication & Redundant Styling

Refactor frontend files to eliminate duplicate JavaScript code builders and CSS styling repetitions.

## Scope of Work

### 1. Extract Advanced Rules Builder UI/Engine
- **Files**: `frontend/modules/deployments/deployment-catalog.js` (lines 326–515) and `frontend/modules/search/search-adv-builder.js` (lines 7–210)
- **Fix**: Consolidate the rules builder engine into a shared class/function in `frontend/core/utils/rules-builder.js` and instantiate it with custom configurations in both modules.

### 2. Consolidate Tab Buttons Styling
- **Files**: `frontend/css/cost.css` (lines 119–136) and `frontend/css/enterprise.css` (lines 676–678, 1096–1108, 1251–1265)
- **Fix**: Replace repeated module tab styles with a global `.tab-btn` class inside `frontend/css/styles.css`. Remove duplicate blocks from module CSS files.
