package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	clusterDomain "github.com/datdt/k8sselfhost/internal/domain/cluster"
	usecaseCluster "github.com/datdt/k8sselfhost/internal/usecase/cluster"
)

// K8sBootstrapHandler provides HTTP handlers for cluster essentials and metrics bootstrap operations.
type K8sBootstrapHandler struct {
	usecase *usecaseCluster.BootstrapUsecase
}

// NewK8sBootstrapHandler creates a new K8sBootstrapHandler.
func NewK8sBootstrapHandler(usecase *usecaseCluster.BootstrapUsecase) *K8sBootstrapHandler {
	return &K8sBootstrapHandler{
		usecase: usecase,
	}
}

// GetEssentialsStatus handles GET /api/v1/k8s/{cluster}/essentials
func (h *K8sBootstrapHandler) GetEssentialsStatus(w http.ResponseWriter, r *http.Request) {
	if h.usecase == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	if cluster == "" {
		writeError(w, http.StatusBadRequest, "cluster parameter is required", nil)
		return
	}

	status, err := h.usecase.GetEssentialsStatus(r.Context(), cluster)
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailableWithErr(w, err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get cluster essentials status", err)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// ExecuteBootstrap handles POST /api/v1/k8s/{cluster}/bootstrap
func (h *K8sBootstrapHandler) ExecuteBootstrap(w http.ResponseWriter, r *http.Request) {
	if h.usecase == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	if cluster == "" {
		writeError(w, http.StatusBadRequest, "cluster parameter is required", nil)
		return
	}

	var req clusterDomain.BootstrapRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	res, err := h.usecase.ExecuteBootstrap(r.Context(), cluster, req)
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailableWithErr(w, err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to execute cluster bootstrap", err)
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// GetPodMetrics handles GET /api/v1/k8s/{cluster}/metrics/pods
func (h *K8sBootstrapHandler) GetPodMetrics(w http.ResponseWriter, r *http.Request) {
	if h.usecase == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	if cluster == "" {
		writeError(w, http.StatusBadRequest, "cluster parameter is required", nil)
		return
	}

	data, err := h.usecase.GetPodMetrics(r.Context(), cluster)
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailableWithErr(w, err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get pod metrics", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
