package http

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/compliance"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	usecaseCompliance "github.com/datdt/k8sselfhost/internal/usecase/compliance"
)

// ComplianceOption configures ComplianceHandler.
type ComplianceOption func(*ComplianceHandler)

// WithComplianceScanner sets custom compliance scanner.
func WithComplianceScanner(s *usecaseCompliance.FleetComplianceScanner) ComplianceOption {
	return func(h *ComplianceHandler) {
		h.scanner = s
	}
}

// WithBootScan configures whether to run a background scan on initialization.
func WithBootScan(enabled bool) ComplianceOption {
	return func(h *ComplianceHandler) {
		h.bootScan = enabled
	}
}

// ComplianceHandler provides HTTP handlers for the compliance API.
type ComplianceHandler struct {
	repo     compliance.Repository
	scanner  *usecaseCompliance.FleetComplianceScanner
	bootScan bool
}

// NewComplianceHandler creates a new compliance HTTP handler.
func NewComplianceHandler(repo compliance.Repository, opts ...ComplianceOption) *ComplianceHandler {
	h := &ComplianceHandler{
		repo:     repo,
		bootScan: true,
	}
	for _, opt := range opts {
		opt(h)
	}

	// Auto-wire scanner from DB provider if not explicitly injected
	if h.scanner == nil && repo != nil {
		if dbp, ok := repo.(interface{ DB() postgres.DBTX }); ok && dbp.DB() != nil {
			store := postgres.NewComplianceScannerStore(dbp.DB())
			h.scanner = usecaseCompliance.NewScanner(usecaseCompliance.WithDBStore(store))
		}
	}

	// Trigger asynchronous initial scan once on handler initialization so stale migration-062 seed rows are replaced on server boot
	if h.bootScan && h.scanner != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_, _ = h.scanner.Scan(ctx)
		}()
	}

	return h
}

// RegisterRoutes registers compliance routes.
func (h *ComplianceHandler) RegisterRoutes(r chi.Router) {
	r.Get("/frameworks", h.ListFrameworks)
	r.Get("/violations", h.ListViolations)
	r.Post("/scan", h.ScanNow)
}

// ScanNow handles POST /api/v1/compliance/scan
func (h *ComplianceHandler) ScanNow(w http.ResponseWriter, r *http.Request) {
	if h.scanner == nil {
		writeError(w, http.StatusInternalServerError, "compliance scanner not initialized", nil)
		return
	}

	res, err := h.scanner.Scan(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to execute compliance scan", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":           "completed",
		"frameworks":       res.Frameworks,
		"violations_count": len(res.Violations),
		"scanned_at":       res.ScannedAt,
	})
}

// ListFrameworks handles GET /api/v1/compliance/frameworks
func (h *ComplianceHandler) ListFrameworks(w http.ResponseWriter, r *http.Request) {
	frameworks, err := h.repo.ListFrameworks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list frameworks", err)
		return
	}
	if frameworks == nil {
		frameworks = make([]compliance.Framework, 0)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": frameworks})
}

// ListViolations handles GET /api/v1/compliance/violations
func (h *ComplianceHandler) ListViolations(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)
	var sev *compliance.ViolationSeverity
	if s := r.URL.Query().Get("severity"); s != "" {
		sv := compliance.ViolationSeverity(s)
		sev = &sv
	}
	items, total, err := h.repo.ListViolations(r.Context(), sev, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list violations", err)
		return
	}
	if items == nil {
		items = make([]compliance.Violation, 0)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}
