// Package http provides HTTP adapters and handlers for the k8sselfhost API.
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/vfswatchdog"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// VFSWatchdogHandler exposes HTTP REST endpoints for VFS inode exhaustion monitoring,
// uninterruptible sleep (D-state) process breaking, and hung storage lazy unmounting.
type VFSWatchdogHandler struct {
	watchdog domain.VFSWatchdog
	logger   *zap.Logger
}

// NewVFSWatchdogHandler constructs a VFSWatchdogHandler instance.
func NewVFSWatchdogHandler(watchdog domain.VFSWatchdog, loggers ...*zap.Logger) *VFSWatchdogHandler {
	var l *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		l = loggers[0]
	} else {
		l = zap.NewNop()
	}

	return &VFSWatchdogHandler{
		watchdog: watchdog,
		logger:   l,
	}
}

// RegisterRoutes mounts VFS watchdog sub-routes onto the provided chi router.
func (h *VFSWatchdogHandler) RegisterRoutes(r chi.Router) {
	r.Get("/inodes/report", h.GetInodeReport)
	r.Get("/inodes/top-culprits", h.GetTopInodeCulprits)
	r.Get("/processes/d-state", h.GetDStateProcesses)
	r.Post("/mounts/lazy-unmount", h.LazyUnmount)
	r.Post("/pids/enforce", h.EnforcePIDLimits)
}

// RegisterAPIRoutes mounts the VFS watchdog endpoints under /api/v1/vfs.
func (h *VFSWatchdogHandler) RegisterAPIRoutes(r chi.Router) {
	r.Route("/api/v1/vfs", h.RegisterRoutes)
}

// GetInodeReport handles GET /api/v1/vfs/inodes/report.
func (h *VFSWatchdogHandler) GetInodeReport(w http.ResponseWriter, r *http.Request) {
	reports, err := h.watchdog.ScanInodes(r.Context())
	if err != nil {
		if errors.Is(err, domain.ErrVFSInodeExhaustion) {
			h.logger.Warn("VFS inode exhaustion detected", zap.Error(err))
			h.writeProblem(w, http.StatusInsufficientStorage, "VFS Inode Exhaustion",
				err.Error(), domain.CodeVFSInodeExhaustion, r.URL.Path)
			return
		}

		h.logger.Error("Failed to scan filesystem inodes", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), domainerrors.CodeInternal, r.URL.Path)
			return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"count":   len(reports),
		"reports": reports,
	})
}

// GetTopInodeCulprits handles GET /api/v1/vfs/inodes/top-culprits?mount=/&limit=10.
func (h *VFSWatchdogHandler) GetTopInodeCulprits(w http.ResponseWriter, r *http.Request) {
	mountPoint := r.URL.Query().Get("mount")
	if mountPoint == "" {
		mountPoint = r.URL.Query().Get("mount_point")
	}

	if strings.TrimSpace(mountPoint) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"mount query parameter is required", domainerrors.CodeValidation, r.URL.Path)
		return
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit <= 0 {
			h.writeProblem(w, http.StatusBadRequest, "Bad Request",
				"limit must be a positive integer", domainerrors.CodeValidation, r.URL.Path)
			return
		}
		limit = parsedLimit
	}

	culprits, err := h.watchdog.FindTopInodeConsumers(r.Context(), mountPoint, limit)
	if err != nil {
		var dErr *domainerrors.DomainError
		if errors.As(err, &dErr) && dErr.Code == domainerrors.CodeValidation {
			h.writeProblem(w, http.StatusBadRequest, "Validation Error",
				err.Error(), domainerrors.CodeValidation, r.URL.Path)
			return
		}

		h.logger.Error("Failed to find top inode consumers", zap.String("mount", mountPoint), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), domainerrors.CodeInternal, r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"mount_point":  mountPoint,
		"count":        len(culprits),
		"culprit_dirs": culprits,
	})
}

// GetDStateProcesses handles GET /api/v1/vfs/processes/d-state?max_duration=5m.
func (h *VFSWatchdogHandler) GetDStateProcesses(w http.ResponseWriter, r *http.Request) {
	duration := domain.DefaultDStateMaxDuration

	if durationStr := r.URL.Query().Get("max_duration"); durationStr != "" {
		parsed, err := time.ParseDuration(durationStr)
		if err != nil || parsed <= 0 {
			h.writeProblem(w, http.StatusBadRequest, "Bad Request",
				"max_duration must be a valid duration string (e.g., 5m, 300s)", domainerrors.CodeValidation, r.URL.Path)
			return
		}
		duration = parsed
	}

	processes, err := h.watchdog.DetectDStateProcesses(r.Context(), duration)
	if err != nil {
		if errors.Is(err, domain.ErrVFSDStateDeadlock) {
			h.logger.Warn("Uninterruptible sleep (D-state) deadlock detected", zap.Error(err))
			h.writeProblem(w, http.StatusInternalServerError, "D-State Process Deadlock",
				err.Error(), domain.CodeVFSDStateDeadlock, r.URL.Path)
			return
		}

		h.logger.Error("Failed to scan D-state processes", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), domainerrors.CodeInternal, r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"count":     len(processes),
		"processes": processes,
	})
}

// LazyUnmountRequest defines payload for forced lazy unmount of a hung storage mount.
type LazyUnmountRequest struct {
	MountPoint string `json:"mount_point"`
}

// LazyUnmount handles POST /api/v1/vfs/mounts/lazy-unmount.
func (h *VFSWatchdogHandler) LazyUnmount(w http.ResponseWriter, r *http.Request) {
	var req LazyUnmountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"malformed json payload", domainerrors.CodeValidation, r.URL.Path)
		return
	}

	if strings.TrimSpace(req.MountPoint) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"mount_point is required in payload", domainerrors.CodeValidation, r.URL.Path)
		return
	}

	if err := h.watchdog.RemountOrUnmountHungNFS(r.Context(), req.MountPoint); err != nil {
		if errors.Is(err, domain.ErrNFSUnmountFailed) {
			h.logger.Error("Lazy unmount execution failed", zap.String("mount", req.MountPoint), zap.Error(err))
			h.writeProblem(w, http.StatusInternalServerError, "NFS Unmount Failed",
				err.Error(), domain.CodeNFSUnmountFailed, r.URL.Path)
			return
		}

		var dErr *domainerrors.DomainError
		if errors.As(err, &dErr) && dErr.Code == domainerrors.CodeValidation {
			h.writeProblem(w, http.StatusBadRequest, "Validation Error",
				err.Error(), domainerrors.CodeValidation, r.URL.Path)
			return
		}

		h.logger.Error("Unexpected error during unmount", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), domainerrors.CodeInternal, r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "unmounted",
		"mount_point": req.MountPoint,
		"message":     "forced lazy unmount executed successfully",
	})
}

// EnforcePIDLimitsRequest defines payload for pod PID limit enforcement.
type EnforcePIDLimitsRequest struct {
	MaxPIDsPerPod int `json:"max_pids_per_pod"`
}

// EnforcePIDLimits handles POST /api/v1/vfs/pids/enforce.
func (h *VFSWatchdogHandler) EnforcePIDLimits(w http.ResponseWriter, r *http.Request) {
	var req EnforcePIDLimitsRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeProblem(w, http.StatusBadRequest, "Bad Request",
				"malformed json payload", domainerrors.CodeValidation, r.URL.Path)
			return
		}
	}

	if req.MaxPIDsPerPod <= 0 {
		req.MaxPIDsPerPod = 1024
	}

	if err := h.watchdog.EnforcePIDLimits(r.Context(), req.MaxPIDsPerPod); err != nil {
		if errors.Is(err, domain.ErrPIDQuotaExceeded) {
			h.logger.Warn("PID quota or zombie accumulation threshold exceeded", zap.Error(err))
			h.writeProblem(w, http.StatusTooManyRequests, "PID Quota Exceeded",
				err.Error(), domain.CodePIDQuotaExceeded, r.URL.Path)
			return
		}

		h.logger.Error("Failed to enforce PID limits", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), domainerrors.CodeInternal, r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "enforced",
		"max_pids_per_pod": req.MaxPIDsPerPod,
	})
}

func (h *VFSWatchdogHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
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
