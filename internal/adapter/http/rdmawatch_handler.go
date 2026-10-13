// Package http provides HTTP REST adapters for RoCEv2 PFC deadlock detection,
// GPU RDMA fabric asymmetry isolation, pod IPC_LOCK admission, and staggered checkpointing.
package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/rdmawatch"
)

// RDMAWatchHandler exposes REST APIs for RoCEv2 and GPU interconnect resilience.
type RDMAWatchHandler struct {
	engine rdmawatch.GPUAIResilienceEngine
	logger *zap.Logger
}

// NewRDMAWatchHandler constructs a new RDMAWatchHandler instance.
func NewRDMAWatchHandler(engine rdmawatch.GPUAIResilienceEngine, loggers ...*zap.Logger) *RDMAWatchHandler {
	var l *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		l = loggers[0]
	} else {
		l = zap.NewNop()
	}
	return &RDMAWatchHandler{
		engine: engine,
		logger: l,
	}
}

// RegisterRoutes registers all RDMA and GPU fabric resilience routes on the provided chi router.
func (h *RDMAWatchHandler) RegisterRoutes(r chi.Router) {
	// Sub-route bindings (mounted under /api/v1/ai/rdma)
	r.Get("/fabric-bandwidth", h.AuditFabricBandwidth)
	r.Post("/pfc-deadlock-check", h.CheckPFCDeadlock)
	r.Post("/pod-preflight", h.ValidatePodPreflight)
	r.Post("/stagger-checkpoint", h.CoordinateStaggerCheckpoint)

	// Direct full path bindings when mounted on root router
	r.Get("/api/v1/ai/rdma/fabric-bandwidth", h.AuditFabricBandwidth)
	r.Post("/api/v1/ai/rdma/pfc-deadlock-check", h.CheckPFCDeadlock)
	r.Post("/api/v1/ai/rdma/pod-preflight", h.ValidatePodPreflight)
	r.Post("/api/v1/ai/rdma/stagger-checkpoint", h.CoordinateStaggerCheckpoint)
}

// AuditFabricBandwidth handles GET /api/v1/ai/rdma/fabric-bandwidth.
func (h *RDMAWatchHandler) AuditFabricBandwidth(w http.ResponseWriter, r *http.Request) {
	links, err := h.engine.AuditGPUFabricBandwidth(r.Context())
	if err != nil {
		if errors.Is(err, rdmawatch.ErrGPUFabricAsymmetric) {
			h.logger.Warn("Asymmetric GPU fabric bandwidth degradation detected",
				zap.Error(err),
				zap.Int("total_links", len(links)),
			)
			h.writeProblem(w, http.StatusServiceUnavailable, "GPU Fabric Asymmetric Degradation",
				err.Error(), rdmawatch.CodeGPUFabricAsymmetric, r.URL.Path)
			return
		}
		h.logger.Error("Failed to audit GPU fabric bandwidth", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "HEALTHY",
		"links":  links,
		"count":  len(links),
	})
}

type pfcDeadlockRequest struct {
	NICName string `json:"nic_name"`
}

// CheckPFCDeadlock handles POST /api/v1/ai/rdma/pfc-deadlock-check.
func (h *RDMAWatchHandler) CheckPFCDeadlock(w http.ResponseWriter, r *http.Request) {
	var req pfcDeadlockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Invalid JSON request body", "BAD_REQUEST", r.URL.Path)
		return
	}

	if strings.TrimSpace(req.NICName) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "nic_name is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	status, err := h.engine.DetectPFCDeadlock(r.Context(), req.NICName)
	if err != nil {
		if errors.Is(err, rdmawatch.ErrRDMAPFCDeadlock) {
			h.logger.Error("RoCEv2 PFC deadlock detected on NIC",
				zap.String("nic", req.NICName),
				zap.Error(err),
			)
			h.writeProblem(w, http.StatusInternalServerError, "RoCEv2 PFC Deadlock Detected",
				err.Error(), rdmawatch.CodeRDMAPFCDeadlock, r.URL.Path)
			return
		}
		h.logger.Error("Failed to execute PFC deadlock check", zap.String("nic", req.NICName), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
}

type podPreflightRequest struct {
	PodYAML string `json:"pod_yaml"`
}

// ValidatePodPreflight handles POST /api/v1/ai/rdma/pod-preflight.
func (h *RDMAWatchHandler) ValidatePodPreflight(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Failed to read request body", "BAD_REQUEST", r.URL.Path)
		return
	}

	yamlContent := bodyBytes
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var req podPreflightRequest
		if err := json.Unmarshal(bodyBytes, &req); err == nil && req.PodYAML != "" {
			yamlContent = []byte(req.PodYAML)
		}
	}

	if len(strings.TrimSpace(string(yamlContent))) == 0 {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "pod yaml body is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	isValid, reason, err := h.engine.ValidatePodRDMAPreflight(r.Context(), yamlContent)
	if err != nil {
		if errors.Is(err, rdmawatch.ErrRDMAMemlockRestricted) || !isValid {
			h.logger.Warn("Pod RDMA preflight rejected",
				zap.String("reason", reason),
				zap.Error(err),
			)
			h.writeProblem(w, http.StatusUnprocessableEntity, "Pod RDMA Preflight Rejected",
				reason, rdmawatch.CodeRDMAMemlockRestricted, r.URL.Path)
			return
		}
		h.logger.Error("Error during pod RDMA preflight check", zap.Error(err))
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", err.Error(), "VALIDATION_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":  isValid,
		"reason": reason,
	})
}

type staggerCheckpointRequest struct {
	RankID     int `json:"rank_id"`
	TotalRanks int `json:"total_ranks"`
	Policy     struct {
		MaxConcurrentWrites int    `json:"max_concurrent_writes"`
		LocalSpoolPath      string `json:"local_spool_path"`
		RemoteStorageTarget string `json:"remote_storage_target"`
		MaxJitterWindowMs   int64  `json:"max_jitter_window_ms"`
	} `json:"policy"`
}

// CoordinateStaggerCheckpoint handles POST /api/v1/ai/rdma/stagger-checkpoint.
func (h *RDMAWatchHandler) CoordinateStaggerCheckpoint(w http.ResponseWriter, r *http.Request) {
	var req staggerCheckpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Invalid JSON request body", "BAD_REQUEST", r.URL.Path)
		return
	}

	if req.TotalRanks <= 0 || req.RankID < 0 || req.RankID >= req.TotalRanks {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"Invalid rank bounds: rank_id must be in [0, total_ranks) and total_ranks > 0", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	window := time.Duration(req.Policy.MaxJitterWindowMs) * time.Millisecond
	if window <= 0 {
		window = 10 * time.Second
	}

	policy := rdmawatch.CheckpointWritePolicy{
		MaxConcurrentWrites: req.Policy.MaxConcurrentWrites,
		LocalSpoolPath:      req.Policy.LocalSpoolPath,
		RemoteStorageTarget: req.Policy.RemoteStorageTarget,
		MaxJitterWindow:     window,
	}

	delay, err := h.engine.CoordinateStaggeredCheckpoint(r.Context(), req.RankID, req.TotalRanks, policy)
	if err != nil {
		h.logger.Error("Failed to coordinate staggered checkpoint", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"rank_id":       req.RankID,
		"total_ranks":   req.TotalRanks,
		"delay_ms":      delay.Milliseconds(),
		"delay_seconds": delay.Seconds(),
	})
}

func (h *RDMAWatchHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
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
