// Package http provides HTTP adapters and handlers for the k8sselfhost API.
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/ebpfwatch"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/ebpfwatch"
)

// EBPFWatchHandler exposes HTTP REST endpoints for eBPF ring buffer telemetry auditing,
// dynamic buffer page expansion, and kernel probe / BTF safety validation.
type EBPFWatchHandler struct {
	validator domain.KernelSafetyValidator
	manager   *usecase.WatchdogManager
	logger    *zap.Logger
}

// NewEBPFWatchHandler constructs an EBPFWatchHandler instance.
func NewEBPFWatchHandler(validator domain.KernelSafetyValidator, loggers ...*zap.Logger) *EBPFWatchHandler {
	var l *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		l = loggers[0]
	} else {
		l = zap.NewNop()
	}

	var mgr *usecase.WatchdogManager
	if m, ok := validator.(*usecase.WatchdogManager); ok {
		mgr = m
	}

	return &EBPFWatchHandler{
		validator: validator,
		manager:   mgr,
		logger:    l,
	}
}

// RegisterRoutes mounts eBPF watchdog routes on the provided chi router.
func (h *EBPFWatchHandler) RegisterRoutes(r chi.Router) {
	r.Get("/probes/pressure", h.GetRingBufferPressure)
	r.Post("/probes/{probe}/expand", h.ExpandRingBuffer)
	r.Post("/hooks/validate", h.ValidateHook)
}

// GetRingBufferPressure handles GET /api/v1/ebpf/probes/pressure.
func (h *EBPFWatchHandler) GetRingBufferPressure(w http.ResponseWriter, r *http.Request) {
	telemetries, err := h.validator.AuditRingBufferPressure(r.Context())
	if err != nil {
		if errors.Is(err, domain.ErrRingBufferOverflow) {
			h.logger.Error("Ring buffer overflow detected", zap.Error(err))
			h.writeProblem(w, http.StatusInternalServerError, "RingBuffer Overflow",
				err.Error(), domain.CodeRingBufferOverflow, r.URL.Path)
			return
		}
		h.logger.Error("Failed to audit ring buffer pressure", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"count":       len(telemetries),
		"telemetries": telemetries,
	})
}

// ExpandRingBufferRequest defines payload for ring buffer expansion.
type ExpandRingBufferRequest struct {
	Pages int `json:"pages"`
}

// ExpandRingBuffer handles POST /api/v1/ebpf/probes/:probe/expand.
func (h *EBPFWatchHandler) ExpandRingBuffer(w http.ResponseWriter, r *http.Request) {
	probe := chi.URLParam(r, "probe")
	if strings.TrimSpace(probe) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"probe path parameter is required", domainerrors.CodeValidation, r.URL.Path)
		return
	}

	var req ExpandRingBufferRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeProblem(w, http.StatusBadRequest, "Bad Request",
				"malformed json payload", domainerrors.CodeValidation, r.URL.Path)
			return
		}
	}

	if err := h.validator.AutoExpandRingBuffer(r.Context(), probe, req.Pages); err != nil {
		var dErr *domainerrors.DomainError
		if errors.As(err, &dErr) && dErr.Code == domainerrors.CodeNotFound {
			h.writeProblem(w, http.StatusNotFound, "Not Found",
				err.Error(), domainerrors.CodeNotFound, r.URL.Path)
			return
		}
		h.logger.Error("Failed to expand ring buffer", zap.String("probe", probe), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	var currentPages int
	if h.manager != nil {
		currentPages, _ = h.manager.GetProbePages(probe)
	}
	if currentPages <= 0 && req.Pages > 0 {
		currentPages = req.Pages
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "expanded",
		"probe":  probe,
		"pages":  currentPages,
	})
}

// ValidateHookRequest defines payload for probe hook safety validation.
type ValidateHookRequest struct {
	HookType domain.ProbeHookType `json:"hook_type"`
	Symbol   string               `json:"symbol"`
}

// ValidateHook handles POST /api/v1/ebpf/hooks/validate.
func (h *EBPFWatchHandler) ValidateHook(w http.ResponseWriter, r *http.Request) {
	var req ValidateHookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"malformed json payload", domainerrors.CodeValidation, r.URL.Path)
		return
	}

	if strings.TrimSpace(string(req.HookType)) == "" || strings.TrimSpace(req.Symbol) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"hook_type and symbol fields are required", domainerrors.CodeValidation, r.URL.Path)
		return
	}

	if err := h.validator.ValidateHookSafety(req.HookType, req.Symbol); err != nil {
		if errors.Is(err, domain.ErrUnstableKprobeRejected) {
			h.writeProblem(w, http.StatusBadRequest, "Unstable Kprobe Rejected",
				err.Error(), domain.CodeUnstableKprobeRejected, r.URL.Path)
			return
		}
		if errors.Is(err, domain.ErrBTFMissing) {
			h.writeProblem(w, http.StatusUnprocessableEntity, "Kernel BTF Missing",
				err.Error(), domain.CodeBTFMissing, r.URL.Path)
			return
		}
		var dErr *domainerrors.DomainError
		if errors.As(err, &dErr) && dErr.Code == domainerrors.CodeValidation {
			h.writeProblem(w, http.StatusBadRequest, "Validation Error",
				err.Error(), domainerrors.CodeValidation, r.URL.Path)
			return
		}

		h.logger.Error("Failed to validate hook safety", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":     true,
		"hook_type": req.HookType,
		"symbol":    req.Symbol,
	})
}

func (h *EBPFWatchHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
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
