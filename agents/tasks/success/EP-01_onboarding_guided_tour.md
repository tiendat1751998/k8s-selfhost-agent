---
title: "Phase 10a: Onboarding Guided Tour (Enterprise Polish)"
status: "pending"
priority: "medium"
assignee: "frontend-agent"
---

# Objective
Enterprise products need a smooth onboarding experience. We need to implement an interactive guided tour for first-time users.

# Tasks
- [ ] **Onboarding Engine**: Implement a guided tour component in JavaScript that displays step-by-step interactive tooltips overlaying key parts of the UI (e.g. Cluster overview, Command Palette, Incidents, Settings).
- [ ] **Progress Tracking**: Store the onboarding completion state in `localStorage` so it only runs for new users or when manually triggered.
- [ ] **Help Tooltip Anchor**: Add a "Start Tour" button in the Help Center / Top bar to allow repeating the tour.

# Acceptance Criteria
- First-time users see a styled tooltip pointing to the main dashboard features.
- Users can skip the tour or click "Next" through all steps.
- Completion state is preserved across reloads.
