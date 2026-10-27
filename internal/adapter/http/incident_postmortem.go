package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
	usecaseIncident "github.com/datdt/k8sselfhost/internal/usecase/incident"
)

type acknowledgeIncidentRequest struct {
	Notes string `json:"notes"`
}

// AcknowledgeIncident handles POST /api/v1/incidents/{id}/acknowledge
func (h *Handler) AcknowledgeIncident(w http.ResponseWriter, r *http.Request) {
	var req acknowledgeIncidentRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			writeError(w, http.StatusBadRequest, "invalid request body", err)
			return
		}
	}

	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "incident ID is required", errors.NewValidation("id", "must not be empty"))
		return
	}

	tenantID := resolveTenant(r.Context(), r)
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	ctx := tenancy.WithTenantID(r.Context(), tenantID)

	operator := tenancy.UserIDFromContext(r.Context())
	if operator == "" {
		operator = tenancy.UserRoleFromContext(r.Context())
	}
	if operator == "" {
		operator = "operator"
	}

	uc := h.getIncidentUsecase()
	if uc == nil {
		writeError(w, http.StatusInternalServerError, "incident usecase not configured", errors.ErrInternal)
		return
	}

	cmd := usecaseIncident.AcknowledgeCommand{
		Operator: operator,
		Notes:    strings.TrimSpace(req.Notes),
	}

	inc, err := uc.AcknowledgeIncident(ctx, id, cmd)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "incident not found", err)
			return
		}
		if errors.Is(err, errors.ErrConflict) {
			writeError(w, http.StatusConflict, err.Error(), err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to acknowledge incident", err)
		return
	}

	logger.Get().Info("audit: incident acknowledged",
		zap.String("action", "incident.acknowledge"),
		zap.String("incident_id", inc.ID),
		zap.String("tenant_id", tenantID),
		zap.String("actor", operator),
		zap.String("status", string(inc.Status)),
	)

	writeJSON(w, http.StatusOK, inc)
}

// GetPostmortem handles GET /api/v1/incidents/{id}/postmortem
func (h *Handler) GetPostmortem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "incident ID is required", errors.NewValidation("id", "must not be empty"))
		return
	}

	tenantID := resolveTenant(r.Context(), r)
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	ctx := tenancy.WithTenantID(r.Context(), tenantID)

	if h.incidentRepo == nil {
		writeError(w, http.StatusInternalServerError, "incident repository not configured", errors.ErrInternal)
		return
	}

	inc, err := h.incidentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "incident not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get incident", err)
		return
	}

	markdown := buildPostmortemMarkdown(inc)

	if strings.Contains(r.Header.Get("Accept"), "text/markdown") || strings.Contains(r.Header.Get("Accept"), "text/plain") {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(markdown))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"incident_id": inc.ID,
		"title":       inc.Title,
		"severity":    inc.Severity,
		"status":      inc.Status,
		"markdown":    markdown,
		"content":     markdown,
	})
}

// buildPostmortemMarkdown constructs a structured postmortem report.
func buildPostmortemMarkdown(inc *incident.Incident) string {
	title := inc.Title
	if title == "" {
		title = inc.Message
	}
	if title == "" {
		title = fmt.Sprintf("Incident %s", inc.ID)
	}

	durationStr := "Ongoing / Unresolved"
	if inc.ResolvedAt != nil && !inc.ResolvedAt.IsZero() {
		dur := inc.ResolvedAt.Sub(inc.CreatedAt).Round(time.Second)
		durationStr = dur.String()
	}

	resolvedAtStr := "N/A"
	if inc.ResolvedAt != nil && !inc.ResolvedAt.IsZero() {
		resolvedAtStr = inc.ResolvedAt.Format(time.RFC3339)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Incident Postmortem: %s\n\n", title))
	sb.WriteString("## Executive Summary\n")
	sb.WriteString(fmt.Sprintf("- **Incident ID**: `%s`\n", inc.ID))
	sb.WriteString(fmt.Sprintf("- **Severity**: `%s`\n", inc.Severity))
	sb.WriteString(fmt.Sprintf("- **Current Status**: `%s`\n", inc.Status))
	sb.WriteString(fmt.Sprintf("- **Classification Type**: `%s`\n", inc.Type))
	sb.WriteString(fmt.Sprintf("- **Cluster**: `%s`\n", inc.ClusterName))
	sb.WriteString(fmt.Sprintf("- **Namespace**: `%s`\n", inc.Namespace))
	sb.WriteString(fmt.Sprintf("- **Impacted Resource**: `%s`\n", inc.PodName))
	sb.WriteString(fmt.Sprintf("- **Detected At**: %s\n", inc.CreatedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- **Resolved At**: %s\n", resolvedAtStr))
	sb.WriteString(fmt.Sprintf("- **Total Duration**: %s\n\n", durationStr))

	sb.WriteString("## Incident Description & Impact\n")
	desc := inc.Description
	if desc == "" {
		desc = inc.Message
	}
	sb.WriteString(fmt.Sprintf("%s\n\n", desc))

	sb.WriteString("## Root Cause Analysis\n")
	sb.WriteString(fmt.Sprintf("- **Trigger Event**: %s failure detected on `%s`.\n", inc.Type, inc.PodName))
	sb.WriteString(fmt.Sprintf("- **Failure Detail**: %s\n", inc.Message))
	if len(inc.RawData) > 0 {
		sb.WriteString("- **Diagnostics Captured**:\n")
		for k, v := range inc.RawData {
			if !strings.HasSuffix(k, "_notes") && !strings.HasSuffix(k, "_by") && !strings.HasSuffix(k, "_at") {
				sb.WriteString(fmt.Sprintf("  - `%s`: %s\n", k, v))
			}
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## Remediation Timeline\n")
	sb.WriteString(fmt.Sprintf("- **%s**: Incident detected by telemetry monitor.\n", inc.CreatedAt.Format(time.RFC3339)))
	if ackAt, ok := inc.RawData["acknowledged_at"]; ok && ackAt != "" {
		ackBy := inc.RawData["acknowledged_by"]
		ackNotes := inc.RawData["acknowledgement_notes"]
		sb.WriteString(fmt.Sprintf("- **%s**: Acknowledged by `%s`. Notes: %s\n", ackAt, ackBy, ackNotes))
	}
	if mitAt, ok := inc.RawData["mitigated_at"]; ok && mitAt != "" {
		mitBy := inc.RawData["mitigated_by"]
		mitNotes := inc.RawData["mitigation_notes"]
		sb.WriteString(fmt.Sprintf("- **%s**: Mitigation applied by `%s`. Notes: %s\n", mitAt, mitBy, mitNotes))
	}
	if resAt, ok := inc.RawData["resolved_at"]; ok && resAt != "" {
		resBy := inc.RawData["resolved_by"]
		resNotes := inc.RawData["resolution_notes"]
		sb.WriteString(fmt.Sprintf("- **%s**: Resolved by `%s`. Notes: %s\n", resAt, resBy, resNotes))
	}
	sb.WriteString("\n")

	sb.WriteString("## Action Items & Preventative Measures\n")
	sb.WriteString("1. Verify cluster capacity limits and autoscaling policies.\n")
	sb.WriteString("2. Update alerting thresholds to prevent false positives and catch degradation early.\n")
	sb.WriteString("3. Document root cause patterns in operational runbooks.\n")

	return sb.String()
}
