---
title: "Phase 8a: Personalization & User Preferences"
status: "pending"
priority: "medium"
assignee: "frontend-agent"
---

# Objective
Implement personal dashboard widget customizations, pinning favorite resources, tracking recently viewed logs/clusters/incidents, and saving user settings (timezone, language, notification preferences).

# Scope
Frontend SPA (HTML/JS/CSS).

# Tasks
- [ ] **Personalization Dashboard**: Add customizable widget toggle cards on the Overview dashboard.
- [ ] **Favorites pinning**: Add a "Pin to Favorites" star icon next to cluster names in the Clusters table and store them in localStorage.
- [ ] **Recently Viewed**: Maintain a "Recently Viewed" list of clusters and incidents in localStorage, rendering them in a sidebar widget or Overview panel.
- [ ] **User Settings**: Implement dropdown inputs for timezone, date format, and checkbox preferences for notifications in the Settings panel.

# Acceptance Criteria
- Star icons toggle cluster favorites instantly and persist on page reload.
- Recently viewed list updates dynamically upon navigating to different sections.
- Settings are saved to localStorage and applied globally to date formatting and UI layouts.
