// Package http provides HTTP adapters and handlers for the k8sselfhost API.
package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/nodemetrics"
)

// CGroupHandler exposes HTTP REST endpoints for Linux cgroup v2
// memory throttling event auditing, eBPF connection tracking map inspection/scaling,
// and VFS slab cache pressure tuning. It delegates to CGroupScalerService.
type CGroupHandler struct {
	scaler *usecase.CGroupScalerService
	logger *zap.Logger
}

// NewCGroupHandler constructs a CGroupHandler instance with the usecase service.
func NewCGroupHandler(scaler *usecase.CGroupScalerService, loggers ...*zap.Logger) *CGroupHandler {
	var l *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		l = loggers[0]
	} else {
		l = zap.NewNop()
	}
	return &CGroupHandler{
		scaler: scaler,
		logger: l,
	}
}

// RegisterRoutes mounts cgroup interceptor and eBPF auto-scaling routes on the provided chi router.
func (h *CGroupHandler) RegisterRoutes(r chi.Router) {
	r.Get("/nodes/{node_name}/cgroup-events", h.GetNodeCGroupEvents)
	r.Get("/clusters/{cluster_id}/ebpf-maps", h.GetClusterEBPFMaps)
	r.Post("/clusters/{cluster_id}/ebpf-maps/resize", h.ResizeClusterEBPFMap)
	r.Post("/nodes/{node_name}/slab-tune", h.EnforceNodeSlabTune)
}

// GetNodeCGroupEvents handles GET /api/v1/kernel/nodes/:node_name/cgroup-events.
func (h *CGroupHandler) GetNodeCGroupEvents(w http.ResponseWriter, r *http.Request) {
	nodeName := chi.URLParam(r, "node_name")
	if nodeName == "" {
		nodeName = chi.URLParam(r, "nodeName")
	}
	if strings.TrimSpace(nodeName) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "node_name path parameter is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	events, err := h.scaler.AuditCGroupV2Events(r.Context(), nodeName)
	if err != nil {
		if errors.Is(err, domain.ErrCGroupV2MemoryThrottled) {
			h.writeProblem(w, http.StatusTooManyRequests, "Too Many Requests",
				fmt.Sprintf("Container is experiencing kernel-level memory.high throttling: %v", err),
				domain.CodeCGroupV2MemoryThrottled, r.URL.Path)
			return
		}
		h.logger.Error("Failed to audit cgroup v2 events", zap.String("node", nodeName), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"node_name": nodeName,
		"events":    events,
		"count":     len(events),
	})
}

// GetClusterEBPFMaps handles GET /api/v1/kernel/clusters/:cluster_id/ebpf-maps.
func (h *CGroupHandler) GetClusterEBPFMaps(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "cluster_id")
	if clusterID == "" {
		clusterID = chi.URLParam(r, "clusterId")
	}
	if strings.TrimSpace(clusterID) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "cluster_id path parameter is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	maps, err := h.scaler.InspectEBPFMaps(r.Context(), clusterID)
	if err != nil {
		if errors.Is(err, domain.ErrEBPFMapSaturated) {
			h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable",
				fmt.Sprintf("eBPF map utilization exceeded critical threshold (80%%). Silent packet drop imminent: %v", err),
				domain.CodeEBPFMapSaturated, r.URL.Path)
			return
		}
		h.logger.Error("Failed to inspect eBPF maps", zap.String("cluster_id", clusterID), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"cluster_id": clusterID,
		"maps":       maps,
		"count":      len(maps),
	})
}

type resizeCGroupEBPFMapRequest struct {
	NewCapacity int64 `json:"new_capacity"`
}

// ResizeClusterEBPFMap handles POST /api/v1/kernel/clusters/:cluster_id/ebpf-maps/resize.
func (h *CGroupHandler) ResizeClusterEBPFMap(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "cluster_id")
	if clusterID == "" {
		clusterID = chi.URLParam(r, "clusterId")
	}
	if strings.TrimSpace(clusterID) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "cluster_id path parameter is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	var req resizeCGroupEBPFMapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Invalid request body JSON payload", "BAD_REQUEST", r.URL.Path)
		return
	}

	if req.NewCapacity <= 0 {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "new_capacity must be greater than zero", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	if err := h.scaler.AutoResizeEBPFConntrackMap(r.Context(), clusterID, req.NewCapacity); err != nil {
		if errors.Is(err, domain.ErrEBPFMemoryLimitExceeded) {
			h.writeProblem(w, http.StatusUnprocessableEntity, "Unprocessable Entity",
				err.Error(), domain.CodeEBPFMemoryLimitExceeded, r.URL.Path)
			return
		}
		h.logger.Error("Failed to resize eBPF conntrack map", zap.String("cluster_id", clusterID), zap.Error(err))
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", err.Error(), "BAD_REQUEST", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "resized",
		"cluster_id":   clusterID,
		"new_capacity": req.NewCapacity,
	})
}

// EnforceNodeSlabTune handles POST /api/v1/kernel/nodes/:node_name/slab-tune.
func (h *CGroupHandler) EnforceNodeSlabTune(w http.ResponseWriter, r *http.Request) {
	nodeName := chi.URLParam(r, "node_name")
	if nodeName == "" {
		nodeName = chi.URLParam(r, "nodeName")
	}
	if strings.TrimSpace(nodeName) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "node_name path parameter is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	if err := h.scaler.EnforceNodeSlabTuning(r.Context(), nodeName); err != nil {
		h.logger.Error("Failed to enforce slab tuning", zap.String("node", nodeName), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), domain.CodeSlabTuningFailed, r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "applied",
		"node_name": nodeName,
		"sysctl": map[string]int{
			domain.SysctlVFSNodeCachePressure: domain.DefaultVFSNodeCachePressure,
		},
	})
}

func (h *CGroupHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
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
