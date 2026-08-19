package http

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/audit"
	"github.com/datdt/k8sselfhost/internal/domain/fleet"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
	"github.com/datdt/k8sselfhost/internal/usecase/cluster"
)

// FleetHandler provides HTTP handlers for the multi-cluster fleet API.
type FleetHandler struct {
	repo          fleet.Repository
	auditRepo     audit.Repository
	importUsecase *cluster.ImportUsecase
}

// NewFleetHandler creates a new fleet HTTP handler.
func NewFleetHandler(repo fleet.Repository, auditRepo audit.Repository, importUsecase *cluster.ImportUsecase) *FleetHandler {
	return &FleetHandler{repo: repo, auditRepo: auditRepo, importUsecase: importUsecase}
}

// RegisterRoutes registers fleet routes.
func (h *FleetHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListClusters)
	r.Get("/clusters", h.ListClusters)
	r.Post("/", h.RegisterCluster)
	r.Delete("/{id}", h.RemoveCluster)
	r.Post("/{id}/actions/upgrade", h.UpgradeCluster)
	h.RegisterImportRoutes(r)
}

// ListClusters handles GET /api/v1/fleet
func (h *FleetHandler) ListClusters(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.ListClusters(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list clusters", err)
		return
	}
	for i := range items {
		items[i].EncryptedToken = ""
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

type registerClusterRequest struct {
	fleet.Cluster
}

func (r *registerClusterRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if strings.TrimSpace(r.Region) == "" {
		ve.Add("region", "region is required")
	}
	if strings.TrimSpace(r.Provider) == "" {
		ve.Add("provider", "provider is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// RegisterCluster handles POST /api/v1/fleet
func (h *FleetHandler) RegisterCluster(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[registerClusterRequest](w, r)
	if !ok {
		return
	}
	c := req.Cluster

	err := h.repo.RegisterCluster(r.Context(), &c)
	result := "success"
	var details map[string]interface{}
	if err != nil {
		result = "failure"
		details = map[string]interface{}{"error": err.Error(), "name": c.Name}
	} else {
		details = map[string]interface{}{
			"name":     c.Name,
			"group":    c.Group,
			"region":   c.Region,
			"provider": c.Provider,
		}
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "create", "kubernetes", c.ID, c.Name, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
		logger.Get().Error("failed to record audit action for cluster creation", zap.Error(auditErr))
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to register cluster", err)
		return
	}
	c.EncryptedToken = ""
	writeJSON(w, http.StatusCreated, c)
}

// RemoveCluster handles DELETE /api/v1/fleet/{id}
func (h *FleetHandler) RemoveCluster(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "cluster id is required", err)
		return
	}

	err = h.repo.RemoveCluster(r.Context(), id)
	result := "success"
	var details map[string]interface{}
	if err != nil {
		result = "failure"
		details = map[string]interface{}{"error": err.Error()}
	} else {
		details = map[string]interface{}{}
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "delete", "kubernetes", id, id, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
		logger.Get().Error("failed to record audit action for cluster deletion", zap.Error(auditErr))
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove cluster", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// UpgradeCluster handles POST /api/v1/fleet/{id}/actions/upgrade
func (h *FleetHandler) UpgradeCluster(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "cluster id is required", err)
		return
	}

	cluster, err := h.repo.GetCluster(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get cluster for upgrade", err)
		return
	}
	if cluster == nil {
		writeError(w, http.StatusNotFound, "cluster not found", nil)
		return
	}

	targetVersion := "v1.31.0"
	if cluster.Version != "" {
		parts := strings.Split(cluster.Version, ".")
		if len(parts) >= 2 {
			var major, minor int
			if _, err := fmt.Sscanf(parts[0], "v%d", &major); err == nil {
				if _, err := fmt.Sscanf(parts[1], "%d", &minor); err == nil {
					targetVersion = fmt.Sprintf("v%d.%d.0", major, minor+1)
				}
			}
		}
	}

	err = h.repo.UpdateClusterStatus(r.Context(), id, "upgrading", cluster.Version, cluster.Nodes)
	result := "success"
	var details map[string]interface{}
	if err != nil {
		result = "failure"
		details = map[string]interface{}{"error": err.Error()}
	} else {
		details = map[string]interface{}{"target_version": targetVersion}
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "upgrade", "kubernetes", id, cluster.Name, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
		logger.Get().Warn("failed to record audit action", zap.Error(auditErr))
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start cluster upgrade", err)
		return
	}

	writeError(w, http.StatusNotImplemented, "Cluster upgrade is not implemented", fmt.Errorf("upgrade not implemented"))
}
