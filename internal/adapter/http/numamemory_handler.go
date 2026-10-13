// Package http provides HTTP REST adapters and handlers for the k8sselfhost API.
package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/numamemory"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/numamemory"
)

// NUMAMemoryHandler exposes REST endpoints for physical NUMA zone reclaim tuning,
// asynchronous dirty page writeback throttling, buddy allocator HugePage defragmentation,
// and kernel slab unreclaimable monitoring.
type NUMAMemoryHandler struct {
	manager *usecase.NUMAMemoryManager
	logger  *zap.Logger
}

// NewNUMAMemoryHandler constructs a new NUMAMemoryHandler instance.
func NewNUMAMemoryHandler(mgr *usecase.NUMAMemoryManager, loggers ...*zap.Logger) *NUMAMemoryHandler {
	var l *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		l = loggers[0]
	} else {
		l = zap.NewNop()
	}
	return &NUMAMemoryHandler{
		manager: mgr,
		logger:  l,
	}
}

// RegisterRoutes mounts NUMA and memory subsystem optimization routes on the chi router.
func (h *NUMAMemoryHandler) RegisterRoutes(r chi.Router) {
	// Sub-router mounted at /api/v1/kernel
	r.Get("/memory/numa-status", h.GetNUMAStatus)
	r.Post("/memory/zone-reclaim-tune", h.TuneZoneReclaim)
	r.Post("/memory/dirty-limits", h.ApplyDirtyLimits)
	r.Post("/memory/hugepages-check", h.CheckHugePages)

	// Direct routes if mounted at /api/v1/kernel/memory
	r.Get("/numa-status", h.GetNUMAStatus)
	r.Post("/zone-reclaim-tune", h.TuneZoneReclaim)
	r.Post("/dirty-limits", h.ApplyDirtyLimits)
	r.Post("/hugepages-check", h.CheckHugePages)
}

// GetNUMAStatus handles GET /api/v1/kernel/memory/numa-status.
// Audits NUMA zone status, capacity, and detect synchronous reclaim spinlocks.
func (h *NUMAMemoryHandler) GetNUMAStatus(w http.ResponseWriter, r *http.Request) {
	zones, err := h.manager.AuditNUMAZones(r.Context())
	if err != nil {
		if errors.Is(err, domain.ErrNUMAZoneLocked) {
			h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
				"NUMA zone reclaim spinlock detected; zone locked", domain.CodeNUMAZoneLocked, r.URL.Path)
			return
		}
		h.logger.Error("Failed to audit NUMA zones", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"zones":                     zones,
		"count":                     len(zones),
		"spinlock_contention_count": h.manager.GetSpinlockContentionCount(),
	})
}

// TuneZoneReclaimRequest holds parameters for tuning vm.zone_reclaim_mode.
type TuneZoneReclaimRequest struct {
	TargetMode int `json:"target_mode"`
}

// TuneZoneReclaim handles POST /api/v1/kernel/memory/zone-reclaim-tune.
// Configures vm.zone_reclaim_mode (default 0) to allow cross-NUMA allocation and avoid spinlocks.
func (h *NUMAMemoryHandler) TuneZoneReclaim(w http.ResponseWriter, r *http.Request) {
	var req TuneZoneReclaimRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeProblem(w, http.StatusBadRequest, "Bad Request",
				"malformed request body: "+err.Error(), domain.CodeValidationFailed, r.URL.Path)
			return
		}
	}

	if err := h.manager.TuneNUMAZoneReclaim(r.Context(), req.TargetMode); err != nil {
		h.logger.Error("Failed to tune NUMA zone reclaim mode", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":            "applied",
		"zone_reclaim_mode": req.TargetMode,
		"message":           "NUMA zone reclaim mode successfully tuned",
	})
}

// DirtyLimitsRequest specifies dirty background bytes and total dirty bytes limits.
type DirtyLimitsRequest struct {
	DirtyRatio           int   `json:"dirty_ratio,omitempty"`
	DirtyBackgroundBytes int64 `json:"dirty_background_bytes,omitempty"`
	DirtyBytes           int64 `json:"dirty_bytes,omitempty"`
}

// ApplyDirtyLimits handles POST /api/v1/kernel/memory/dirty-limits.
// Converts risky dirty ratios into byte limits (256MB background / 1GB total) to prevent I/O queue freezing.
func (h *NUMAMemoryHandler) ApplyDirtyLimits(w http.ResponseWriter, r *http.Request) {
	var req DirtyLimitsRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeProblem(w, http.StatusBadRequest, "Bad Request",
				"malformed request body: "+err.Error(), domain.CodeValidationFailed, r.URL.Path)
			return
		}
	}

	bgBytes := req.DirtyBackgroundBytes
	totBytes := req.DirtyBytes

	// If dirty ratio was supplied or bytes were not specified, enforce standard 256MB/1GB safe defaults
	if req.DirtyRatio > 0 || (bgBytes <= 0 && totBytes <= 0) {
		bgBytes = usecase.DefaultDirtyBackgroundBytes
		totBytes = usecase.DefaultDirtyBytes
	}

	if err := h.manager.ApplyDirtyPageByteLimits(r.Context(), bgBytes, totBytes); err != nil {
		if errors.Is(err, domain.ErrDirtyPageFlushSaturation) {
			h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable",
				"Dirty page accumulation saturates threshold; synchronous writeback flush active",
				domain.CodeDirtyPageFlushSaturation, r.URL.Path)
			return
		}
		h.logger.Error("Failed to apply dirty page byte limits", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	appliedCfg := h.manager.GetDirtyPageConfig()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":                 "applied",
		"dirty_background_bytes": appliedCfg.DirtyBackgroundBytes,
		"dirty_bytes":            appliedCfg.DirtyBytes,
		"guardrail_enforced":     totBytes < usecase.MinDirtyBytesGuardrail,
	})
}

// HugePagesCheckRequest defines parameters for HugePage pool availability auditing.
type HugePagesCheckRequest struct {
	PageSizeKB          int     `json:"page_size_kb,omitempty"`
	RequestedPages      int     `json:"requested_pages,omitempty"`
	CompactionThreshold float64 `json:"compaction_threshold,omitempty"`
}

// CheckHugePages handles POST /api/v1/kernel/memory/hugepages-check.
// Audits HugePages availability and triggers buddy allocator compaction if fragmentation index > 0.70.
func (h *NUMAMemoryHandler) CheckHugePages(w http.ResponseWriter, r *http.Request) {
	var req HugePagesCheckRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeProblem(w, http.StatusBadRequest, "Bad Request",
				"malformed request body: "+err.Error(), domain.CodeValidationFailed, r.URL.Path)
			return
		}
	}

	if req.PageSizeKB <= 0 {
		req.PageSizeKB = 2048 // 2MiB default
	}
	if req.RequestedPages <= 0 {
		req.RequestedPages = 1
	}

	pool, err := h.manager.CheckHugePageAvailability(r.Context(), req.PageSizeKB, req.RequestedPages)
	if err != nil {
		if errors.Is(err, domain.ErrHugePagesFragmented) {
			h.writeProblem(w, http.StatusInsufficientStorage, "Insufficient Storage",
				"Physical memory buddy allocator fragmented; insufficient contiguous HugePages",
				domain.CodeHugePagesFragmented, r.URL.Path)
			return
		}
		h.logger.Error("Failed to check HugePages availability", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "available",
		"pool":      pool,
		"compacted": pool.FragmentationIdx <= usecase.DefaultCompactionThreshold,
	})
}

func (h *NUMAMemoryHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
	problem := ProblemDetails{
		Type:     "about:blank",
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
		Code:     code,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}
