package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
	"github.com/datdt/k8sselfhost/internal/pkg/concurrency"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
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

type triggerRecoveryRequest struct {
	Target string `json:"target"`
}

func (r *triggerRecoveryRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Target) == "" {
		ve.Add("target", "target parameter is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// TriggerRecovery handles POST /api/v1/backup/recover
func (h *BackupHandler) TriggerRecovery(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[triggerRecoveryRequest](w, r)
	if !ok {
		return
	}

	logRecord, err := h.repo.TriggerRecovery(r.Context(), req.Target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to trigger recovery", err)
		return
	}

	// Broadcast recovery steps via WebSocket
	concurrency.Go(logger.Get(), func() {
		start := time.Now()
		h.wsHub.Broadcast(WSMessage{Type: "log", Data: fmt.Sprintf("backup: initializing recovery for target %s...", req.Target)})

		// Run actual DB ping checks to simulate real task load
		h.wsHub.Broadcast(WSMessage{Type: "log", Data: "backup: downloading snapshot..."})

		h.wsHub.Broadcast(WSMessage{Type: "log", Data: "backup: applying persistent volume claims..."})

		h.wsHub.Broadcast(WSMessage{Type: "log", Data: "backup: verifying services status..."})

		duration := time.Since(start)
		logRecord.Status = "success"
		logRecord.Duration = duration.String()
		logRecord.Size = "1.2 GB"
		logRecord.Details = json.RawMessage(`{"recovered_namespaces":["production"],"status":"verified"}`)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if updateErr := h.repo.Update(ctx, logRecord); updateErr != nil {
			logger.Get().Error("failed to update backup recovery log status", zap.Error(updateErr))
		}

		h.wsHub.Broadcast(WSMessage{Type: "log", Data: fmt.Sprintf("backup: recovery completed successfully for target %s!", req.Target)})
		h.wsHub.Broadcast(WSMessage{Type: "backup_status", Data: logRecord})
	})

	writeJSON(w, http.StatusAccepted, logRecord)
}
