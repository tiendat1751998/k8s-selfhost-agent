package http

import (
	"encoding/json"
	"errors"
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
	r.Get("/nodes/{node}/fencing/preflight", h.PreflightFencing)

	r.With(middleware.RBACMiddleware("platform_admin", "tenant_admin")).Group(func(mut chi.Router) {
		mut.Post("/etcd/snapshot", h.TriggerEtcdSnapshot)
		mut.Post("/etcd/restore", h.RestoreEtcdSnapshot)
		mut.Post("/backups", h.CreateBackup)
		mut.Post("/remediation/{node}", h.TriggerRemediation)
		mut.Post("/nodes/{node}/fencing/execute", h.ExecuteFencing)
		mut.Post("/nodes/{node}/rescue", h.ExecuteRescue)
	})
}

// RegisterClusterFencingRoutes registers /clusters/:clusterId/nodes/:nodeName/fencing routes.
func (h *DRHandler) RegisterClusterFencingRoutes(r chi.Router) {
	r.Get("/preflight", h.PreflightFencing)
	r.With(middleware.RBACMiddleware("platform_admin", "tenant_admin")).Post("/execute", h.ExecuteFencing)
	r.With(middleware.RBACMiddleware("platform_admin", "tenant_admin")).Post("/rescue", h.ExecuteRescue)
}

// WireToRouter mounts cluster fencing endpoints onto a parent router.
func (h *DRHandler) WireToRouter(r chi.Router) {
	r.Get("/clusters/{clusterId}/nodes/{nodeName}/fencing/preflight", h.PreflightFencing)
	r.With(middleware.RBACMiddleware("platform_admin", "tenant_admin")).Post("/clusters/{clusterId}/nodes/{nodeName}/fencing/execute", h.ExecuteFencing)
	r.With(middleware.RBACMiddleware("platform_admin", "tenant_admin")).Post("/clusters/{clusterId}/nodes/{nodeName}/rescue", h.ExecuteRescue)
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

// PreflightFencing performs pre-flight scan and RCA analysis for node storage fencing.
// GET /api/v1/clusters/:clusterId/nodes/:nodeName/fencing/preflight
func (h *DRHandler) PreflightFencing(w http.ResponseWriter, r *http.Request) {
	if h.remediation == nil {
		writeError(w, http.StatusServiceUnavailable, "Remediation controller is not configured", nil)
		return
	}

	clusterID := chi.URLParam(r, "clusterId")
	if clusterID == "" {
		clusterID = chi.URLParam(r, "cluster")
	}
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "clusterId parameter is required", nil)
		return
	}

	nodeName := chi.URLParam(r, "nodeName")
	if nodeName == "" {
		nodeName = chi.URLParam(r, "node")
	}
	if nodeName == "" {
		writeError(w, http.StatusBadRequest, "nodeName parameter is required", nil)
		return
	}

	result, err := h.remediation.PreflightScan(r.Context(), clusterID, nodeName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to perform fencing preflight scan", err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ExecuteFencing triggers controlled storage fencing and workload failover.
// POST /api/v1/clusters/:clusterId/nodes/:nodeName/fencing/execute
func (h *DRHandler) ExecuteFencing(w http.ResponseWriter, r *http.Request) {
	if h.remediation == nil {
		writeError(w, http.StatusServiceUnavailable, "Remediation controller is not configured", nil)
		return
	}

	clusterID := chi.URLParam(r, "clusterId")
	if clusterID == "" {
		clusterID = chi.URLParam(r, "cluster")
	}
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "clusterId parameter is required", nil)
		return
	}

	nodeName := chi.URLParam(r, "nodeName")
	if nodeName == "" {
		nodeName = chi.URLParam(r, "node")
	}
	if nodeName == "" {
		writeError(w, http.StatusBadRequest, "nodeName parameter is required", nil)
		return
	}

	var opts sre.FencingExecutionOptions
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body", err)
			return
		}
	}

	result, err := h.remediation.ExecuteFencing(r.Context(), clusterID, nodeName, opts)
	if err != nil {
		if errors.Is(err, sre.ErrPowerIsolationRequired) {
			writeError(w, http.StatusBadRequest, err.Error(), err)
			return
		}
		if errors.Is(err, sre.ErrCircuitBreakerTripped) {
			writeError(w, http.StatusConflict, err.Error(), err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to execute storage fencing", err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

type serviceRescueRequest struct {
	Services []string `json:"services"`
}

// ExecuteRescue triggers soft-death service restart rescue.
// POST /api/v1/clusters/:clusterId/nodes/:nodeName/rescue
func (h *DRHandler) ExecuteRescue(w http.ResponseWriter, r *http.Request) {
	if h.remediation == nil {
		writeError(w, http.StatusServiceUnavailable, "Remediation controller is not configured", nil)
		return
	}

	clusterID := chi.URLParam(r, "clusterId")
	if clusterID == "" {
		clusterID = chi.URLParam(r, "cluster")
	}
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "clusterId parameter is required", nil)
		return
	}

	nodeName := chi.URLParam(r, "nodeName")
	if nodeName == "" {
		nodeName = chi.URLParam(r, "node")
	}
	if nodeName == "" {
		writeError(w, http.StatusBadRequest, "nodeName parameter is required", nil)
		return
	}

	var req serviceRescueRequest
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	result, err := h.remediation.ExecuteRescue(r.Context(), clusterID, nodeName, req.Services)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to execute service rescue", err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}