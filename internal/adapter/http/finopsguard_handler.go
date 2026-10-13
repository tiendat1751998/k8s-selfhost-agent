package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/finopsguard"
)

// FinOpsGuardHandler exposes REST API endpoints for FinOps optimization and resource reaping.
type FinOpsGuardHandler struct {
	engine finopsguard.FinOpsAutomationEngine
}

// NewFinOpsGuardHandler instantiates a new FinOpsGuardHandler with injected automation engine.
func NewFinOpsGuardHandler(engine finopsguard.FinOpsAutomationEngine) *FinOpsGuardHandler {
	return &FinOpsGuardHandler{engine: engine}
}

// RegisterRoutes registers the FinOps endpoints on a chi.Router.
func (h *FinOpsGuardHandler) RegisterRoutes(r chi.Router) {
	// Standard sub-router mounts (when mounted under /api/v1/finops)
	r.Get("/services/topology-audit", h.AuditTopologyHints)
	r.Post("/services/{ns}/{name}/inject-hints", h.InjectTopologyAwareHints)
	r.Get("/resources/orphans", h.ScanOrphanResources)
	r.Post("/resources/{id}/reap", h.ReapResource)

	// Direct paths when registered at root router
	r.Get("/api/v1/finops/services/topology-audit", h.AuditTopologyHints)
	r.Post("/api/v1/finops/services/{ns}/{name}/inject-hints", h.InjectTopologyAwareHints)
	r.Get("/api/v1/finops/resources/orphans", h.ScanOrphanResources)
	r.Post("/api/v1/finops/resources/{id}/reap", h.ReapResource)
}

// AuditTopologyHints handles GET /api/v1/finops/services/topology-audit
// Audits multi-AZ Kubernetes services lacking topology-aware hints.
// Returns HTTP 409 with RFC 7807 problem details if cross-AZ egress waste is detected.
func (h *FinOpsGuardHandler) AuditTopologyHints(w http.ResponseWriter, r *http.Request) {
	unhintedCount, err := h.engine.AuditTopologyHints(r.Context())
	if err != nil {
		if errors.Is(err, finopsguard.ErrFinOpsCrossAZWasteDetected) {
			writeRFC7807(w, finopsguard.ErrFinOpsCrossAZWasteDetected, r.URL.Path)
			return
		}
		h.respondError(w, r, err, "failed to audit service topology hints")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"unhinted_services_count": unhintedCount,
		"status":                  "optimal",
		"message":                 "all multi-AZ services have topology hints configured",
	})
}

// InjectTopologyAwareHints handles POST /api/v1/finops/services/:ns/:name/inject-hints
// Injects 'service.kubernetes.io/topology-mode: Auto' annotation or triggers traffic skew fallback.
func (h *FinOpsGuardHandler) InjectTopologyAwareHints(w http.ResponseWriter, r *http.Request) {
	namespace := getURLParam(r, "ns")
	serviceName := getURLParam(r, "name")

	if namespace == "" || serviceName == "" {
		writeError(w, http.StatusBadRequest, "namespace and service name parameters are required", nil)
		return
	}

	if err := h.engine.InjectTopologyAwareHints(r.Context(), namespace, serviceName); err != nil {
		h.respondError(w, r, err, "failed to inject topology hints")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "success",
		"namespace": namespace,
		"service":   serviceName,
		"message":   "topology hints or traffic skew fallback applied successfully",
	})
}

// ScanOrphanResources handles GET /api/v1/finops/resources/orphans
// Scans and lists orphan cloud resources idle >= 48 hours.
func (h *FinOpsGuardHandler) ScanOrphanResources(w http.ResponseWriter, r *http.Request) {
	orphans, err := h.engine.ScanOrphanCloudResources(r.Context())
	if err != nil {
		h.respondError(w, r, err, "failed to scan orphan cloud resources")
		return
	}

	if orphans == nil {
		orphans = []finopsguard.OrphanCloudResource{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  orphans,
		"count": len(orphans),
	})
}

// ReapResource handles POST /api/v1/finops/resources/:id/reap
// Reaps approved orphan resources with mandatory ephemeral snapshot generation for EBS volumes.
func (h *FinOpsGuardHandler) ReapResource(w http.ResponseWriter, r *http.Request) {
	resourceID := getURLParam(r, "id")
	if resourceID == "" {
		writeError(w, http.StatusBadRequest, "resource id parameter is required", nil)
		return
	}

	if err := h.engine.ReapApprovedResource(r.Context(), resourceID); err != nil {
		h.respondError(w, r, err, "failed to reap approved orphan resource")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "reaped",
		"resource_id": resourceID,
		"message":     "orphan cloud resource safely destroyed with snapshot verification",
	})
}

// respondError sends RFC 7807 problem details if matching domain error, otherwise standard structured error.
func (h *FinOpsGuardHandler) respondError(w http.ResponseWriter, r *http.Request, err error, defaultMsg string) {
	var rfcErr *finopsguard.RFC7807ProblemDetail
	if errors.As(err, &rfcErr) {
		writeRFC7807(w, rfcErr, r.URL.Path)
		return
	}
	writeError(w, http.StatusInternalServerError, defaultMsg, err)
}

// writeRFC7807 serializes RFC 7807 problem details to the HTTP response.
func writeRFC7807(w http.ResponseWriter, prob *finopsguard.RFC7807ProblemDetail, instance string) {
	status := prob.Status
	if status == 0 {
		status = http.StatusInternalServerError
	}

	pCopy := *prob
	if instance != "" {
		pCopy.Instance = instance
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(pCopy)
}

// getURLParam retrieves URL parameter supporting both Chi patterns and fallback query parameters.
func getURLParam(r *http.Request, key string) string {
	if val := chi.URLParam(r, key); val != "" {
		return val
	}
	if val := chi.URLParam(r, ":"+key); val != "" {
		return val
	}
	return r.URL.Query().Get(key)
}
