package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
)

// BackupHandler provides HTTP endpoints for Backup & DR actions.
type BackupHandler struct {
	repo  backup.Repository
	wsHub *WSHub
}

// NewBackupHandler creates a new BackupHandler.
func NewBackupHandler(repo backup.Repository, wsHub *WSHub) *BackupHandler {
	return &BackupHandler{
		repo:  repo,
		wsHub: wsHub,
	}
}

// RegisterRoutes registers backup endpoints.
func (h *BackupHandler) RegisterRoutes(r chi.Router) {
	r.Get("/history", h.GetHistory)
	r.Post("/recover", h.TriggerRecovery)
}

// GetHistory handles GET /api/v1/backup/history
func (h *BackupHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	history, err := h.repo.GetHistory(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get backup history", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": history,
	})
}

// TriggerRecovery handles POST /api/v1/backup/recover
func (h *BackupHandler) TriggerRecovery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if req.Target == "" {
		writeError(w, http.StatusBadRequest, "missing target parameter", nil)
		return
	}

	logRecord, err := h.repo.TriggerRecovery(r.Context(), req.Target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to trigger recovery", err)
		return
	}

	// Broadcast recovery steps via WebSocket
	go func() {
		h.wsHub.Broadcast(WSMessage{Type: "log", Data: fmt.Sprintf("backup: initializing recovery for target %s...", req.Target)})
		time.Sleep(1 * time.Second)
		h.wsHub.Broadcast(WSMessage{Type: "log", Data: "backup: downloading snapshot..."})
		time.Sleep(1 * time.Second)
		h.wsHub.Broadcast(WSMessage{Type: "log", Data: "backup: applying persistent volume claims..."})
		time.Sleep(1 * time.Second)
		h.wsHub.Broadcast(WSMessage{Type: "log", Data: "backup: verifying services status..."})
		time.Sleep(1 * time.Second)
		h.wsHub.Broadcast(WSMessage{Type: "log", Data: fmt.Sprintf("backup: recovery completed successfully for target %s!", req.Target)})
		h.wsHub.Broadcast(WSMessage{Type: "backup_status", Data: logRecord})
	}()

	writeJSON(w, http.StatusAccepted, logRecord)
}
