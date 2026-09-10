# 🎯 004: ENTERPRISE PRODUCTION CRAFTSMANSHIP, COLLAPSIBLE SIDEBAR & FULL-SYSTEM SECURITY MASTER PLAN

> **Location**: `.agents/tasks/inprocess/004-enterprise-production-craftsmanship-and-security-master.md`  
> **Status**: IN_PROGRESS  
> **Standard**: Production Enterprise Grade (Craftsmanship > Speed, Zero Toy Projects)  
> **Directives**:
> 1. Sidebar Collapsible Rail (64px) for Tablet (768px–1024px) & Desktop with persistent state.
> 2. Full-System Backend & Security Audit across all Go handlers, RBAC, tenant isolation, and parameterized queries.
> 3. Zero-defect screen-by-screen tablet layout and action button verification.

---

## 🏛️ EPIC 1: COLLAPSIBLE SIDEBAR & TABLET AUTO-COLLAPSE (ICON RAIL 64PX)
- **Goal**: Transform the desktop/tablet sidebar into a professional collapsible rail.
- **Requirements**:
  1. Add a sleek collapse toggle button (`«` / `»` or chevron toggle) in the sidebar header or footer.
  2. In collapsed mode:
     - Sidebar width shrinks from 290px/260px down to exactly **64px**.
     - Only category/menu icons are visible, labels are hidden with smooth fade/slide.
     - Hovering an icon displays a floating tooltip with the route title and badge.
     - Active route indicators remain clearly visible on the 64px rail.
  3. **Tablet (768px – 1024px)**:
     - Automatically defaults to collapsed 64px icon rail instead of awkward off-canvas hiding or squishing dashboard content.
     - User can toggle between 64px rail and expanded 260px overlay.
  4. **State Persistence**:
     - User preference stored in `localStorage.getItem('k8s_sidebar_collapsed')`.
  5. **Layout Flow**:
     - Main content wrapper automatically stretches to 100% available space with zero horizontal overflow.

---

## 🛡️ EPIC 2: COMPREHENSIVE BACKEND & SECURITY HARDENING (GO)
- **Goal**: Zero vulnerabilities, zero SQL injection risks, strict tenant isolation, enterprise RBAC.
- **Requirements**:
  1. **Authentication & RBAC**:
     - Verify every single protected route in `internal/adapter/http/` has JWT/Session token auth middleware.
     - Enforce strict RBAC permissions on all destructive/mutation handlers (`POST`, `PUT`, `DELETE`, `PATCH`).
  2. **Database & Tenant Isolation**:
     - Audit all queries in `internal/infrastructure/persistence/` to guarantee parameterized queries (no string concatenation).
     - Ensure all queries enforce tenant isolation (`WHERE tenant_id = $1`).
  3. **HTTP Security Headers & Protection**:
     - Enforce security headers: `Content-Security-Policy`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Strict-Transport-Security`.
     - Rate limiting on authentication, TOTP verification, and API key generation endpoints.
  4. **Concurrency & Memory Safety**:
     - Run `go test -race ./...` across all packages to eliminate any potential data races.
     - Audit mutex locking in agent swarm, cluster telemetry, and web socket event streams.

---

## 📱 EPIC 3: TABLET & DESKTOP SCREEN-BY-SCREEN RIGOROUS RE-AUDIT
- **Goal**: Eradicate all remaining toy-project quirks across all 34 screens.
- **Requirements**:
  1. Audit specifically on **iPad / Tablet (768x1024 / 1024x768)** with 64px collapsed sidebar.
  2. Guarantee balanced 2x2 metric card grids and full data table legibility.
  3. Verify 100% English terminology (zero leftover hardcoded Vietnamese text).
  4. Verify all action buttons trigger functional drawers or live dialogs without exceptions.
