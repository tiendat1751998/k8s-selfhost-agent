package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/storage"
	usecaseStorage "github.com/datdt/k8sselfhost/internal/usecase/storage"
)

// StorageHandler provides HTTP endpoints for distributed storage and volume snapshots.
type StorageHandler struct {
	usecase usecaseStorage.VolumeService
}

// NewStorageHandler creates a new StorageHandler.
func NewStorageHandler(usecase usecaseStorage.VolumeService) *StorageHandler {
	return &StorageHandler{
		usecase: usecase,
	}
}

// RegisterRoutes registers the distributed storage routes onto a Chi sub-router.
func (h *StorageHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListVolumes)
	r.Post("/{name}/expand", h.ExpandVolume)
	r.Post("/{name}/snapshot", h.CreateSnapshot)
}

// ListVolumes handles GET /api/v1/k8s/{cluster}/storage/volumes
func (h *StorageHandler) ListVolumes(w http.ResponseWriter, r *http.Request) {
	if h.usecase == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	if cluster == "" {
		cluster = r.URL.Query().Get("cluster")
	}

	volumes, err := h.usecase.ListVolumes(r.Context(), cluster)
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailableWithErr(w, err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list volumes", err)
		return
	}

	if volumes == nil {
		volumes = make([]storage.DistributedVolume, 0)
	}

	writeJSON(w, http.StatusOK, volumes)
}

type expandVolumeRequest struct {
	NewSizeBytes int64  `json:"new_size_bytes"`
	Namespace    string `json:"namespace,omitempty"`
}

// ExpandVolume handles POST /api/v1/k8s/{cluster}/storage/volumes/{name}/expand
func (h *StorageHandler) ExpandVolume(w http.ResponseWriter, r *http.Request) {
	if h.usecase == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	if cluster == "" {
		cluster = r.URL.Query().Get("cluster")
	}

	name := chi.URLParam(r, "name")
	if strings.TrimSpace(name) == "" {
		writeError(w, http.StatusBadRequest, "volume name cannot be empty", nil)
		return
	}

	var req expandVolumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if req.NewSizeBytes <= 0 {
		writeError(w, http.StatusBadRequest, "new_size_bytes must be greater than zero", nil)
		return
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = r.URL.Query().Get("namespace")
	}

	err := h.usecase.ExpandVolume(r.Context(), cluster, namespace, name, req.NewSizeBytes)
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailableWithErr(w, err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to expand volume", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":         "ok",
		"message":        "volume expanded successfully",
		"volume":         name,
		"new_size_bytes": req.NewSizeBytes,
	})
}

type createSnapshotRequest struct {
	SnapshotName string            `json:"snapshot_name,omitempty"`
	Namespace    string            `json:"namespace,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// CreateSnapshot handles POST /api/v1/k8s/{cluster}/storage/volumes/{name}/snapshot
func (h *StorageHandler) CreateSnapshot(w http.ResponseWriter, r *http.Request) {
	if h.usecase == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	if cluster == "" {
		cluster = r.URL.Query().Get("cluster")
	}

	name := chi.URLParam(r, "name")
	if strings.TrimSpace(name) == "" {
		writeError(w, http.StatusBadRequest, "volume name cannot be empty", nil)
		return
	}

	var req createSnapshotRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body", err)
			return
		}
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = r.URL.Query().Get("namespace")
	}

	result, err := h.usecase.CreateSnapshot(r.Context(), cluster, namespace, name, storage.VolumeSnapshotRequest{
		SnapshotName: req.SnapshotName,
		Labels:       req.Labels,
	})
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailableWithErr(w, err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create volume snapshot", err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}
