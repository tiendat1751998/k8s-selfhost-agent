# TASK-REFACTOR-05-ROUTING-AND-STATE: Routing and AI Providers Tab Integration

## Goal
Integrate the AI Copilot console into the AI Providers view as a sub-tab, remove redundant separate copilot routing, and register the top 20 providers.

## Scope
- Implement the sub-tab switching handlers inside `ai-providers.js` to toggle between registry and copilot panels.
- Toggle the visibility of the "+ Add Provider" button when on the copilot console tab.
- Remove the separate menu item for `ai-copilot` from `sidebar.js` and routing definitions from `router/index.js`.
- Verify the top 20 providers in the standalone mock database are fetched and shown correctly.

## Success Criteria
- The "AI Copilot" sidebar navigation item is removed.
- Switching tabs inside AI Providers functions cleanly.
- 20 AI providers render correctly in the Model Registry table.
