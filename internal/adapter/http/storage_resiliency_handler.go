package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/storage"
)

// StorageResiliencyHandler exposes REST endpoints for Ceph BlueStore RocksDB healing,
// MinIO dynamic scanner throttling, and PostgreSQL anti-wipe database safeguards.
type StorageResiliencyHandler struct {
	manager storage.StorageResiliencyManager
}

// NewStorageResiliencyHandler constructs a new StorageResiliencyHandler instance.
func NewStorageResiliencyHandler(manager storage.StorageResiliencyManager) *StorageResiliencyHandler {
	return &StorageResiliencyHandler{
		manager: manager,
	}
}

// RegisterRoutes registers routes on a sub-router scoped to /clusters/{clusterId}/storage.
func (h *StorageResiliencyHandler) RegisterRoutes(r chi.Router) {
	r.Get("/ceph/osds", h.AuditCephCluster)
	r.Post("/ceph/osds/{osdId}/repair", h.TriggerCephOSDRepair)
	r.Post("/minio/autotune", h.AutoTuneMinIOScanner)
	r.Post("/database/safeguard", h.VerifySafeDatabaseTarget)
}

// WireToRouter mounts the full storage resiliency route tree under /clusters/{clusterId}/storage/...
func (h *StorageResiliencyHandler) WireToRouter(r chi.Router) {
	r.Route("/clusters/{clusterId}/storage", func(sub chi.Router) {
		h.RegisterRoutes(sub)
	})
}

func extractClusterID(r *http.Request) string {
	clusterID := chi.URLParam(r, "clusterId")
	if clusterID == "" {
		clusterID = chi.URLParam(r, "cluster")
	}
	if clusterID == "" {
		clusterID = chi.URLParam(r, "cluster_id")
	}
	if clusterID == "" {
		clusterID = r.URL.Query().Get("clusterId")
	}
	if clusterID == "" {
		clusterID = r.URL.Query().Get("cluster")
	}
	if clusterID == "" {
		clusterID = r.URL.Query().Get("cluster_id")
	}
	return strings.TrimSpace(clusterID)
}

// AuditCephCluster handles GET /api/v1/clusters/:clusterId/storage/ceph/osds
func (h *StorageResiliencyHandler) AuditCephCluster(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "storage resiliency manager is not configured", nil)
		return
	}

	clusterID := extractClusterID(r)
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "clusterId parameter is required", nil)
		return
	}

	osds, err := h.manager.AuditCephCluster(r.Context(), clusterID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to audit ceph cluster", err)
		return
	}

	if osds == nil {
		osds = make([]storage.CephOSDStatus, 0)
	}

	writeJSON(w, http.StatusOK, osds)
}

// TriggerCephOSDRepair handles POST /api/v1/clusters/:clusterId/storage/ceph/osds/:osdId/repair
func (h *StorageResiliencyHandler) TriggerCephOSDRepair(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "storage resiliency manager is not configured", nil)
		return
	}

	clusterID := extractClusterID(r)
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "clusterId parameter is required", nil)
		return
	}

	rawOSDID := chi.URLParam(r, "osdId")
	osdID, err := strconv.Atoi(rawOSDID)
	if err != nil || osdID < 0 {
		writeError(w, http.StatusBadRequest, "valid non-negative osdId parameter is required", err)
		return
	}

	if err := h.manager.TriggerCephOSDRepair(r.Context(), clusterID, osdID); err != nil {
		if errors.Is(err, storage.ErrCephOSDNotFound) {
			writeError(w, http.StatusNotFound, "ceph osd not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to repair ceph osd", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "repaired",
		"cluster_id": clusterID,
		"osd_id":     osdID,
		"message":    "ceph-bluestore-tool repair successfully completed with PointInTimeRecovery",
	})
}

// AutoTuneMinIOScanner handles POST /api/v1/clusters/:clusterId/storage/minio/autotune
func (h *StorageResiliencyHandler) AutoTuneMinIOScanner(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "storage resiliency manager is not configured", nil)
		return
	}

	clusterID := extractClusterID(r)
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "clusterId parameter is required", nil)
		return
	}

	tuning, err := h.manager.AutoTuneMinIOScanner(r.Context(), clusterID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to auto-tune minio drive scanner", err)
		return
	}

	writeJSON(w, http.StatusOK, tuning)
}

type verifyDatabaseSafeguardRequest struct {
	DBConnString string `json:"db_conn_string"`
}

// VerifySafeDatabaseTarget handles POST /api/v1/clusters/:clusterId/storage/database/safeguard
// Enforces GitLab 2017 primary wipe prevention: blocks destructive resync if target is primary.
func (h *StorageResiliencyHandler) VerifySafeDatabaseTarget(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "storage resiliency manager is not configured", nil)
		return
	}

	var req verifyDatabaseSafeguardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if strings.TrimSpace(req.DBConnString) == "" {
		writeError(w, http.StatusBadRequest, "db_conn_string parameter is required", nil)
		return
	}

	safe, err := h.manager.VerifySafeDatabaseTarget(r.Context(), req.DBConnString)
	if err != nil {
		if errors.Is(err, storage.ErrPrimaryDatabaseProtected) {
			writeJSON(w, http.StatusForbidden, map[string]interface{}{
				"safe":        false,
				"role":        "primary",
				"in_recovery": false,
				"error":       "destructive operation strictly forbidden: target is primary PostgreSQL node (pg_is_in_recovery = false)",
			})
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to inspect database recovery status", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"safe":        safe,
		"role":        "standby",
		"in_recovery": true,
		"message":     "database target is safe standby replica in recovery",
	})
}
