
package http

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Extending FleetHandler
func (h *FleetHandler) RegisterImportRoutes(r chi.Router) {
	r.Post("/import", h.ImportCluster)
	r.Get("/{id}/health", h.GetClusterHealth)
	r.Post("/{id}/discover", h.DiscoverClusterResources)
}

func (h *FleetHandler) ImportCluster(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form", err)
		return
	}

	id := r.FormValue("id")
	name := r.FormValue("name")
	group := r.FormValue("group")
	region := r.FormValue("region")
	provider := r.FormValue("provider")

	file, _, err := r.FormFile("kubeconfig")
	if err != nil {
		writeError(w, http.StatusBadRequest, "kubeconfig file is required", err)
		return
	}
	defer file.Close()

	kubeconfig, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read kubeconfig", err)
		return
	}

	c, err := h.importUsecase.ImportCluster(r.Context(), id, name, group, region, provider, kubeconfig)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to import cluster", err)
		return
	}

	if c != nil {
		c.EncryptedToken = ""
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *FleetHandler) GetClusterHealth(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "cluster id is required", err)
		return
	}
	c, err := h.repo.GetCluster(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get cluster", err)
		return
	}
	if c == nil {
		writeError(w, http.StatusNotFound, "cluster not found", nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"health_status": c.HealthStatus,
		"last_health_check": c.LastHealthCheck,
	})
}

func (h *FleetHandler) DiscoverClusterResources(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "cluster id is required", err)
		return
	}
	c, err := h.repo.GetCluster(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get cluster", err)
		return
	}
	if c == nil {
		writeError(w, http.StatusNotFound, "cluster not found", nil)
		return
	}

	// Would call DiscoveryPort here, but doing simple for now
	writeJSON(w, http.StatusOK, c.DiscoveredResources)
}

