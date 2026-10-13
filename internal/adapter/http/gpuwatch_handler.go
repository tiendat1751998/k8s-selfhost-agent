// Package http provides HTTP REST adapters for GPU AI workload watchdog,
// XID hardware fault containment, and PagedAttention KV-cache protection.
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/gpuwatch"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/gpuwatch"
)

// GPUWatchHandler exposes REST APIs for NVIDIA GPU cluster telemetry,
// hardware fault remediation, NCCL AllReduce deadlock eviction, and KV-cache guarding.
type GPUWatchHandler struct {
	service *usecase.GPUManagerService
	logger  *zap.Logger
}

// NewGPUWatchHandler constructs a new GPUWatchHandler instance.
func NewGPUWatchHandler(service *usecase.GPUManagerService, loggers ...*zap.Logger) *GPUWatchHandler {
	var l *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		l = loggers[0]
	} else {
		l = zap.NewNop()
	}
	return &GPUWatchHandler{
		service: service,
		logger:  l,
	}
}

// RegisterRoutes registers GPU watchdog endpoints on the provided chi router.
func (h *GPUWatchHandler) RegisterRoutes(r chi.Router) {
	r.Get("/nodes/{node}/telemetry", h.GetNodeTelemetry)
	r.Post("/nodes/{node}/taint-faulty", h.TaintFaultyNode)
	r.Post("/jobs/{jobId}/nccl-progress", h.CheckNCCLProgress)
	r.Post("/inference/kv-cache-check", h.CheckKVCache)
}

// GetNodeTelemetry handles GET /api/v1/gpu/nodes/:node/telemetry.
func (h *GPUWatchHandler) GetNodeTelemetry(w http.ResponseWriter, r *http.Request) {
	node := chi.URLParam(r, "node")
	if node == "" {
		node = chi.URLParam(r, "node_name")
	}
	if strings.TrimSpace(node) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "node path parameter is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	cards, err := h.service.GetNodeTelemetry(r.Context(), node)
	if err != nil {
		h.logger.Error("Failed to query node GPU telemetry", zap.String("node", node), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"node":  node,
		"cards": cards,
		"count": len(cards),
	})
}

// taintFaultyNodeRequest models payload for node hardware fault containment.
type taintFaultyNodeRequest struct {
	DmesgLog     string                 `json:"dmesg_log"`
	AffectedPods []usecase.PodReference `json:"affected_pods"`
}

// TaintFaultyNode handles POST /api/v1/gpu/nodes/:node/taint-faulty.
func (h *GPUWatchHandler) TaintFaultyNode(w http.ResponseWriter, r *http.Request) {
	node := chi.URLParam(r, "node")
	if node == "" {
		node = chi.URLParam(r, "node_name")
	}
	if strings.TrimSpace(node) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "node path parameter is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	var req taintFaultyNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Invalid JSON request body", "BAD_REQUEST", r.URL.Path)
		return
	}

	result, err := h.service.ProcessNodeXIDLogs(r.Context(), node, req.DmesgLog, req.AffectedPods)
	if err != nil {
		if errors.Is(err, gpuwatch.ErrGPUXIDHardwareFault) {
			h.logger.Warn("Critical NVIDIA XID hardware fault isolated",
				zap.String("node", node),
				zap.Int("critical_faults", len(result.CriticalFaults)),
				zap.Strings("evicted_pods", result.EvictedPods),
			)
			h.writeProblem(w, http.StatusInternalServerError, "GPU Hardware Fault",
				err.Error(), gpuwatch.CodeGPUXIDHardwareFault, r.URL.Path)
			return
		}
		h.logger.Error("Failed to process node XID logs", zap.String("node", node), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

// kvCacheCheckRequest models payload for vLLM/Triton KV-cache monitoring.
type kvCacheCheckRequest struct {
	WorkloadID  string  `json:"workload_id"`
	UsagePct    float64 `json:"usage_pct"`
	TotalBlocks int     `json:"total_blocks"`
}

// CheckKVCache handles POST /api/v1/gpu/inference/kv-cache-check.
func (h *GPUWatchHandler) CheckKVCache(w http.ResponseWriter, r *http.Request) {
	var req kvCacheCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Invalid JSON request body", "BAD_REQUEST", r.URL.Path)
		return
	}

	if req.WorkloadID == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "workload_id is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	result, err := h.service.EvaluateAndPreemptKVCache(r.Context(), req.WorkloadID, req.UsagePct, req.TotalBlocks)
	if err != nil {
		if errors.Is(err, gpuwatch.ErrKVCacheOOMRisk) {
			h.logger.Warn("vLLM/Triton KV-cache pressure triggered proactive offload",
				zap.String("workload", req.WorkloadID),
				zap.Float64("usage_pct", req.UsagePct),
				zap.Int("blocks_offloaded", result.BlocksOffloaded),
			)
			h.writeProblem(w, http.StatusTooManyRequests, "KV-Cache OOM Risk",
				err.Error(), gpuwatch.CodeKVCacheOOMRisk, r.URL.Path)
			return
		}
		h.logger.Error("Failed to evaluate KV-cache pressure", zap.String("workload", req.WorkloadID), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

// ncclProgressRequest models payload for distributed NCCL training watchdog.
type ncclProgressRequest struct {
	IdleDurationSec int64  `json:"idle_duration_sec"`
	Namespace       string `json:"namespace"`
	PodName         string `json:"pod_name"`
}

// CheckNCCLProgress handles POST /api/v1/gpu/jobs/:jobId/nccl-progress.
func (h *GPUWatchHandler) CheckNCCLProgress(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	if jobID == "" {
		jobID = chi.URLParam(r, "job_id")
	}
	if strings.TrimSpace(jobID) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "jobId path parameter is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	var req ncclProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Invalid JSON request body", "BAD_REQUEST", r.URL.Path)
		return
	}

	idleDuration := time.Duration(req.IdleDurationSec) * time.Second
	status, err := h.service.EvaluateNCCLProgress(r.Context(), jobID, idleDuration, req.Namespace, req.PodName)
	if err != nil {
		if errors.Is(err, gpuwatch.ErrNCCLHangDetected) {
			h.logger.Warn("NCCL AllReduce distributed training stall evicted",
				zap.String("job_id", jobID),
				zap.Duration("idle_duration", idleDuration),
				zap.String("action", status.ActionTaken),
			)
			h.writeProblem(w, http.StatusGatewayTimeout, "NCCL AllReduce Deadlock",
				err.Error(), gpuwatch.CodeNCCLHangDetected, r.URL.Path)
			return
		}
		h.logger.Error("Failed to inspect NCCL training progress", zap.String("job_id", jobID), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
}

func (h *GPUWatchHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
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
