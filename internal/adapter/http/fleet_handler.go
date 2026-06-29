package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/audit"
	"github.com/datdt/k8sselfhost/internal/domain/fleet"
)

// FleetHandler provides HTTP handlers for the multi-cluster fleet API.
type FleetHandler struct {
	repo      fleet.Repository
	auditRepo audit.Repository
}

// NewFleetHandler creates a new fleet HTTP handler.
func NewFleetHandler(repo fleet.Repository, auditRepo audit.Repository) *FleetHandler {
	return &FleetHandler{repo: repo, auditRepo: auditRepo}
}

// RegisterRoutes registers fleet routes.
func (h *FleetHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListClusters)
	r.Post("/", h.RegisterCluster)
	r.Delete("/{id}", h.RemoveCluster)
	r.Post("/{id}/actions/upgrade", h.UpgradeCluster)
}

// ListClusters handles GET /api/v1/fleet
func (h *FleetHandler) ListClusters(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.ListClusters(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list clusters", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

// RegisterCluster handles POST /api/v1/fleet
func (h *FleetHandler) RegisterCluster(w http.ResponseWriter, r *http.Request) {
	var c fleet.Cluster
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

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
	_ = h.auditRepo.RecordAction(r.Context(), userID, "create", "kubernetes", c.ID, c.Name, result, details, r.RemoteAddr, r.Header.Get("User-Agent"))

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to register cluster", err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

// RemoveCluster handles DELETE /api/v1/fleet/{id}
func (h *FleetHandler) RemoveCluster(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	
	err := h.repo.RemoveCluster(r.Context(), id)
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
	_ = h.auditRepo.RecordAction(r.Context(), userID, "delete", "kubernetes", id, id, result, details, r.RemoteAddr, r.Header.Get("User-Agent"))

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove cluster", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// UpgradeCluster handles POST /api/v1/fleet/{id}/actions/upgrade
func (h *FleetHandler) UpgradeCluster(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

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
	_ = h.auditRepo.RecordAction(r.Context(), userID, "upgrade", "kubernetes", id, cluster.Name, result, details, r.RemoteAddr, r.Header.Get("User-Agent"))

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start cluster upgrade", err)
		return
	}

	// Simulate background upgrade process
	go func() {
		time.Sleep(5 * time.Second)
		ctx := context.Background()
		_ = h.repo.UpdateClusterStatus(ctx, id, "active", targetVersion, cluster.Nodes)
	}()

	writeJSON(w, http.StatusOK, map[string]string{
		"status":         "upgrade_started",
		"target_version": targetVersion,
	})
}
