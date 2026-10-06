package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/usecase/dr"
	"github.com/datdt/k8sselfhost/internal/usecase/sre"
)

// DRHandler handles Disaster Recovery and Node Remediation endpoints.
type DRHandler struct {
	drUsecase   *dr.DRUsecase
	remediation *sre.RemediationController
	logger      *zap.Logger
}

// NewDRHandler constructs a new DRHandler instance.
func NewDRHandler(drUsecase *dr.DRUsecase, remediation *sre.RemediationController, logger *zap.Logger) *DRHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DRHandler{
		drUsecase:   drUsecase,
		remediation: remediation,
		logger:      logger,
	}
}

// RegisterRoutes registers the DR routes onto the Chi router.
// Read endpoints are available to authenticated users; mutating endpoints require platform_admin or tenant_admin.
func (h *DRHandler) RegisterRoutes(r chi.Router) {
	r.Get("/backups", h.ListBackups)

	r.With(middleware.RBACMiddleware("platform_admin", "tenant_admin")).Group(func(mut chi.Router) {
		mut.Post("/etcd/snapshot", h.TriggerEtcdSnapshot)
		mut.Post("/etcd/restore", h.RestoreEtcdSnapshot)
		mut.Post("/backups", h.CreateBackup)
		mut.Post("/remediation/{node}", h.TriggerRemediation)
	})
}

// TriggerEtcdSnapshot triggers an immediate etcd snapshot on the cluster.
// POST /api/v1/k8s/{cluster}/dr/etcd/snapshot
func (h *DRHandler) TriggerEtcdSnapshot(w http.ResponseWriter, r *http.Request) {
	if h.drUsecase == nil {
		writeError(w, http.StatusServiceUnavailable, "DR usecase is not configured", nil)
		return
	}

	clusterID := chi.URLParam(r, "cluster")
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "cluster parameter is required", nil)
		return
	}

	result, err := h.drUsecase.TriggerEtcdSnapshot(r.Context(), clusterID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to trigger etcd snapshot", err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

type restoreEtcdRequest struct {
	SnapshotID string `json:"snapshot_id"`
}

// RestoreEtcdSnapshot restores an etcd snapshot to the cluster.
// POST /api/v1/k8s/{cluster}/dr/etcd/restore
func (h *DRHandler) RestoreEtcdSnapshot(w http.ResponseWriter, r *http.Request) {
	if h.drUsecase == nil {
		writeError(w, http.StatusServiceUnavailable, "DR usecase is not configured", nil)
		return
	}

	clusterID := chi.URLParam(r, "cluster")
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "cluster parameter is required", nil)
		return
	}

	var req restoreEtcdRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if strings.TrimSpace(req.SnapshotID) == "" {
		writeError(w, http.StatusBadRequest, "snapshot_id is required", nil)
		return
	}

	if err := h.drUsecase.RestoreEtcdSnapshot(r.Context(), clusterID, req.SnapshotID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to restore etcd snapshot", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":      "Restored",
		"snapshot_id": req.SnapshotID,
		"cluster_id":  clusterID,
	})
}

// ListBackups returns all cluster backups.
// GET /api/v1/k8s/{cluster}/dr/backups
func (h *DRHandler) ListBackups(w http.ResponseWriter, r *http.Request) {
	if h.drUsecase == nil {
		writeError(w, http.StatusServiceUnavailable, "DR usecase is not configured", nil)
		return
	}

	clusterID := chi.URLParam(r, "cluster")
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "cluster parameter is required", nil)
		return
	}

	backups, err := h.drUsecase.ListClusterBackups(r.Context(), clusterID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list cluster backups", err)
		return
	}

	writeJSON(w, http.StatusOK, backups)
}

type createBackupRequest struct {
	BackupName string `json:"backup_name"`
}

// CreateBackup triggers a cluster-level backup (Velero-compatible).
// POST /api/v1/k8s/{cluster}/dr/backups
func (h *DRHandler) CreateBackup(w http.ResponseWriter, r *http.Request) {
	if h.drUsecase == nil {
		writeError(w, http.StatusServiceUnavailable, "DR usecase is not configured", nil)
		return
	}

	clusterID := chi.URLParam(r, "cluster")
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "cluster parameter is required", nil)
		return
	}

	var backupName string
	if r.Body != nil && r.ContentLength > 0 {
		var req createBackupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			backupName = strings.TrimSpace(req.BackupName)
		}
	}

	result, err := h.drUsecase.TriggerClusterBackup(r.Context(), clusterID, backupName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to trigger cluster backup", err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// TriggerRemediation triggers automated fast-failover remediation on an offline/crashing node.
// POST /api/v1/k8s/{cluster}/dr/remediation/{node}
func (h *DRHandler) TriggerRemediation(w http.ResponseWriter, r *http.Request) {
	if h.remediation == nil {
		writeError(w, http.StatusServiceUnavailable, "Remediation controller is not configured", nil)
		return
	}

	clusterID := chi.URLParam(r, "cluster")
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "cluster parameter is required", nil)
		return
	}

	nodeName := chi.URLParam(r, "node")
	if nodeName == "" {
		writeError(w, http.StatusBadRequest, "node parameter is required", nil)
		return
	}

	result, err := h.remediation.HandleNodeOffline(r.Context(), clusterID, nodeName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remediate node", err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}