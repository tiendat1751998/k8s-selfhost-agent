// Package http exposes REST API adapters for GitOps sync-wave DAG analysis,
// commit monotonic ordering guards, and 3-way merge drift reconciliation.
package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
)

// GitOpsSyncWaveHandler handles GitOps sync-wave DAG and webhook resilience endpoints.
type GitOpsSyncWaveHandler struct {
	engine domainGitops.GitOpsResilienceEngine
}

// NewGitOpsSyncWaveHandler creates a new GitOpsSyncWaveHandler instance.
func NewGitOpsSyncWaveHandler(engine domainGitops.GitOpsResilienceEngine) *GitOpsSyncWaveHandler {
	return &GitOpsSyncWaveHandler{
		engine: engine,
	}
}

// RegisterRoutes registers the GitOps resilience endpoints on the provided Chi router.
func (h *GitOpsSyncWaveHandler) RegisterRoutes(r chi.Router) {
	// Sub-router routes (when mounted under /api/v1/gitops)
	r.Post("/webhooks/verify-ordering", h.VerifyCommitOrdering)
	r.Post("/sync-waves/audit-cycle", h.AuditSyncWaveCycle)
	r.Post("/drift/reconcile", h.ReconcileDrift)

	// Direct full paths (when registered on root router)
	r.Post("/api/v1/gitops/webhooks/verify-ordering", h.VerifyCommitOrdering)
	r.Post("/api/v1/gitops/sync-waves/audit-cycle", h.AuditSyncWaveCycle)
	r.Post("/api/v1/gitops/drift/reconcile", h.ReconcileDrift)
}

// VerifyCommitOrderingRequest represents the request body for webhook ordering verification.
type VerifyCommitOrderingRequest struct {
	DeliveryID    string `json:"delivery_id"`
	RepoPath      string `json:"repo_path"`
	IncomingSHA   string `json:"incoming_sha"`
	CurrentSHA    string `json:"current_sha"`
	RepositoryURL string `json:"repository_url"`
	Branch        string `json:"branch"`
}

// VerifyCommitOrdering handles POST /api/v1/gitops/webhooks/verify-ordering.
// Drops incoming webhooks if the commit is an ancestor of the currently running commit (HTTP 409 RFC 7807).
func (h *GitOpsSyncWaveHandler) VerifyCommitOrdering(w http.ResponseWriter, r *http.Request) {
	var req VerifyCommitOrderingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request payload", err)
		return
	}

	if req.IncomingSHA == "" || req.CurrentSHA == "" {
		writeError(w, http.StatusBadRequest, "incoming_sha and current_sha are required", nil)
		return
	}

	isNewer, err := h.engine.VerifyCommitMonotonicOrdering(r.Context(), req.RepoPath, req.IncomingSHA, req.CurrentSHA)
	if err != nil {
		var rfcErr *domainGitops.RFC7807ProblemDetail
		if errors.As(err, &rfcErr) {
			writeRFC7807Problem(w, rfcErr, r.URL.Path)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to verify commit ordering", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "ok",
		"is_newer":     isNewer,
		"incoming_sha": req.IncomingSHA,
		"current_sha":  req.CurrentSHA,
	})
}

// AuditSyncWaveCycleRequest represents the request payload for sync-wave DAG cycle audits.
type AuditSyncWaveCycleRequest struct {
	Manifests []string `json:"manifests"`
}

// AuditSyncWaveCycle handles POST /api/v1/gitops/sync-waves/audit-cycle.
// Returns HTTP 200 if acyclic or HTTP 422 RFC 7807 if circular dependencies or sync-wave inversions exist.
func (h *GitOpsSyncWaveHandler) AuditSyncWaveCycle(w http.ResponseWriter, r *http.Request) {
	var req AuditSyncWaveCycleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request payload", err)
		return
	}

	if len(req.Manifests) == 0 {
		writeError(w, http.StatusBadRequest, "manifests list cannot be empty", nil)
		return
	}

	manifestBytes := make([][]byte, 0, len(req.Manifests))
	for _, m := range req.Manifests {
		manifestBytes = append(manifestBytes, []byte(m))
	}

	hasCycle, details, err := h.engine.AuditSyncWaveCycle(r.Context(), manifestBytes)
	if err != nil {
		var rfcErr *domainGitops.RFC7807ProblemDetail
		if errors.As(err, &rfcErr) {
			writeRFC7807Problem(w, rfcErr, r.URL.Path)
			return
		}
		writeError(w, http.StatusBadRequest, "failed to audit sync-wave cycle", err)
		return
	}

	if hasCycle {
		prob := domainGitops.ErrGitOpsCircularSyncWave.WithDetail(details)
		writeRFC7807Problem(w, prob, r.URL.Path)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":          "ok",
		"has_cycle":       false,
		"manifests_count": len(manifestBytes),
		"message":         "sync-wave DAG validated: no circular dependencies detected",
	})
}

// ReconcileDriftRequest represents the request payload for 3-way merge drift evaluation.
type ReconcileDriftRequest struct {
	ReleaseName string `json:"release_name"`
	Namespace   string `json:"namespace"`
	Manifest    string `json:"manifest"`
}

// ReconcileDrift handles POST /api/v1/gitops/drift/reconcile.
// Detects manual kubectl edit deviations in managedFields and reconciles or flags conflicts (HTTP 409 RFC 7807).
func (h *GitOpsSyncWaveHandler) ReconcileDrift(w http.ResponseWriter, r *http.Request) {
	var req ReconcileDriftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request payload", err)
		return
	}

	if req.ReleaseName == "" || req.Manifest == "" {
		writeError(w, http.StatusBadRequest, "release_name and manifest are required", nil)
		return
	}

	result, err := h.engine.DetectAndReconcileDrift(r.Context(), req.ReleaseName, req.Namespace, []byte(req.Manifest))
	if err != nil {
		var rfcErr *domainGitops.RFC7807ProblemDetail
		if errors.As(err, &rfcErr) {
			writeRFC7807Problem(w, rfcErr, r.URL.Path)
			return
		}
		writeError(w, http.StatusInternalServerError, "drift reconciliation failed", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":            "ok",
		"has_drift":         result.HasDrift,
		"drifted_fields":    result.DriftedFields,
		"conflict_detected": result.ConflictDetected,
		"auto_resolved":     result.AutoResolved,
	})
}

func writeRFC7807Problem(w http.ResponseWriter, prob *domainGitops.RFC7807ProblemDetail, instance string) {
	status := prob.Status
	if status == 0 {
		status = http.StatusInternalServerError
	}

	clone := *prob
	if instance != "" {
		clone.Instance = instance
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(clone)
}
