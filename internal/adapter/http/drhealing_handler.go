package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/drhealing"
)

// DRHealingHandler provides REST API endpoints for Velero/Kopia repository lock healing,
// edge SQLite database corruption recovery, and KMS key deletion protection auditing.
type DRHealingHandler struct {
	manager drhealing.DRResilienceManager
	logger  *zap.Logger
}

// NewDRHealingHandler instantiates a DRHealingHandler.
func NewDRHealingHandler(manager drhealing.DRResilienceManager, logger *zap.Logger) *DRHealingHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DRHealingHandler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes registers disaster recovery resilience endpoints onto a Chi subrouter.
// When mounted at /api/v1/dr, endpoints become:
//   - GET  /api/v1/dr/backup/locks
//   - POST /api/v1/dr/backup/locks/break
//   - POST /api/v1/dr/edge/repair-db
//   - GET  /api/v1/dr/kms/audit-protection
func (h *DRHealingHandler) RegisterRoutes(r chi.Router) {
	r.Get("/backup/locks", h.InspectBackupLocks)
	r.Post("/backup/locks/break", h.BreakStaleLock)
	r.Post("/edge/repair-db", h.RepairEdgeDatabase)
	r.Get("/kms/audit-protection", h.AuditKMSKeyProtection)
}

// WireToAPIRouter mounts resilience endpoints under /api/v1/dr on a root router.
func (h *DRHealingHandler) WireToAPIRouter(r chi.Router) {
	r.Route("/api/v1/dr", func(sub chi.Router) {
		h.RegisterRoutes(sub)
	})
}

// InspectBackupLocks inspects repository storage locks across backup targets.
// GET /api/v1/dr/backup/locks
func (h *DRHealingHandler) InspectBackupLocks(w http.ResponseWriter, r *http.Request) {
	locks, err := h.manager.InspectBackupLocks(r.Context())
	if err != nil {
		h.respondError(w, err, "failed to inspect backup repository locks")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  locks,
		"count": len(locks),
	})
}

type breakLockRequest struct {
	LocationName string `json:"location_name"`
}

func (r *breakLockRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.LocationName) == "" {
		ve.Add("location_name", "location_name is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// BreakStaleLock clears an orphan repository lock if the controller pod is confirmed dead.
// POST /api/v1/dr/backup/locks/break
func (h *DRHealingHandler) BreakStaleLock(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[breakLockRequest](w, r)
	if !ok {
		return
	}

	err := h.manager.BreakStaleLock(r.Context(), req.LocationName)
	if err != nil {
		h.respondError(w, err, "failed to break stale backup lock")
		return
	}

	// Lock cleared successfully, return RFC 7807 status 200 problem detail confirmation
	writeJSON(w, http.StatusOK, drhealing.ErrStaleBackupLockCleared)
}

type repairDBRequest struct {
	NodeID string `json:"node_id"`
	DBPath string `json:"db_path"`
}

func (r *repairDBRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.NodeID) == "" {
		ve.Add("node_id", "node_id is required")
	}
	if strings.TrimSpace(r.DBPath) == "" {
		ve.Add("db_path", "db_path is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// RepairEdgeDatabase creates a .corrupt snapshot and performs WAL recovery on edge SQLite databases.
// POST /api/v1/dr/edge/repair-db
func (h *DRHealingHandler) RepairEdgeDatabase(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[repairDBRequest](w, r)
	if !ok {
		return
	}

	if err := h.manager.RepairEdgeDatabase(r.Context(), req.NodeID, req.DBPath); err != nil {
		h.respondError(w, err, "failed to repair edge database")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "repaired",
		"node_id": req.NodeID,
		"db_path": req.DBPath,
		"message": "edge SQLite database successfully recovered from WAL",
	})
}

// AuditKMSKeyProtection verifies whether a KMS key has multi-party quorum and deletion grace delay.
// GET /api/v1/dr/kms/audit-protection?key_arn=...
func (h *DRHealingHandler) AuditKMSKeyProtection(w http.ResponseWriter, r *http.Request) {
	keyARN := strings.TrimSpace(r.URL.Query().Get("key_arn"))
	if keyARN == "" {
		ve := NewValidationError("validation failed")
		ve.Add("key_arn", "key_arn query parameter is required")
		writeValidationError(w, ve)
		return
	}

	isProtected, err := h.manager.AuditKMSKeyProtection(r.Context(), keyARN)
	if err != nil {
		h.respondError(w, err, "KMS key deletion protection audit failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"key_arn":   keyARN,
		"protected": isProtected,
		"message":   "KMS key deletion protection verified",
	})
}

// respondError inspects domain RFC 7807 problem details or emits standard structured errors.
func (h *DRHealingHandler) respondError(w http.ResponseWriter, err error, defaultMsg string) {
	var rfcErr *drhealing.RFC7807Error
	if errors.As(err, &rfcErr) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(rfcErr.Status)
		_ = json.NewEncoder(w).Encode(rfcErr)
		return
	}

	h.logger.Error("DR resilience handler error", zap.Error(err), zap.String("context", defaultMsg))
	writeError(w, http.StatusInternalServerError, defaultMsg, err)
}
