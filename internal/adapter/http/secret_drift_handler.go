package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/datdt/k8sselfhost/internal/domain/secretguard"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/secretguard"
)

// SecretDriftHandler exposes HTTP REST endpoints for ExternalSecrets drift auditing,
// polling interval mutation, workload rolling restarts, and master key backup verification.
type SecretDriftHandler struct {
	reconciler *usecase.DriftReconciler
	detector   domain.DriftDetector
}

// NewSecretDriftHandler creates a new SecretDriftHandler instance.
func NewSecretDriftHandler(reconciler *usecase.DriftReconciler, detector domain.DriftDetector) *SecretDriftHandler {
	return &SecretDriftHandler{
		reconciler: reconciler,
		detector:   detector,
	}
}

// RegisterRoutes registers the drift endpoints onto a chi.Router.
func (h *SecretDriftHandler) RegisterRoutes(r chi.Router) {
	r.Get("/audit", h.AuditDrift)
	r.Post("/mutate", h.MutateInterval)
	r.Post("/rollout", h.TriggerRollout)
	r.Get("/backup-verify", h.VerifyBackup)
}

// AuditDrift handles GET /api/v1/secrets/drift/audit.
// Inspects ExternalSecrets polling intervals across namespaces and flags violations.
func (h *SecretDriftHandler) AuditDrift(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")

	// If reconciler is available, perform audit & mitigation run; otherwise query detector directly.
	if h.reconciler != nil {
		res, err := h.reconciler.AuditAndMitigatePollingStorms(r.Context(), namespace)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to audit external secrets", err)
			return
		}
		writeJSON(w, http.StatusOK, res)
		return
	}

	if h.detector != nil {
		policies, err := h.detector.AuditExternalSecrets(r.Context(), namespace)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to audit external secrets", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"audited_count": len(policies),
			"policies":      policies,
		})
		return
	}

	writeError(w, http.StatusInternalServerError, "secret drift reconciler unconfigured", nil)
}

type mutateIntervalRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Interval  string `json:"interval"`
}

func (req *mutateIntervalRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(req.Namespace) == "" {
		ve.Add("namespace", "namespace is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		ve.Add("name", "name is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// MutateInterval handles POST /api/v1/secrets/drift/mutate.
// Enforces a minimum safety floor (min 1 hour) on ExternalSecret refresh intervals.
func (h *SecretDriftHandler) MutateInterval(w http.ResponseWriter, r *http.Request) {
	var req mutateIntervalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	targetInterval := domain.MinSafetyInterval
	if strings.TrimSpace(req.Interval) != "" {
		dur, err := time.ParseDuration(req.Interval)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid duration '%s'", req.Interval), err)
			return
		}
		if dur < domain.MinSafetyInterval {
			writeError(w, http.StatusBadRequest, domain.ErrSecretPollingRateTooHigh.Error(), domain.ErrSecretPollingRateTooHigh)
			return
		}
		targetInterval = dur
	}

	if h.detector != nil {
		if err := h.detector.MutateRefreshInterval(r.Context(), req.Namespace, req.Name, targetInterval); err != nil {
			if errors.Is(err, domain.ErrSecretPollingRateTooHigh) {
				writeError(w, http.StatusBadRequest, err.Error(), err)
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to mutate interval", err)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":            "mutated",
		"namespace":         req.Namespace,
		"name":              req.Name,
		"enforced_interval": targetInterval.String(),
	})
}

type triggerRolloutRequest struct {
	Namespace           string `json:"namespace"`
	Name                string `json:"name"`
	Deployment          string `json:"deployment"`
	Hash                string `json:"hash"`
	ReloaderAutoRestart bool   `json:"reloader_auto_restart"`
}

func (req *triggerRolloutRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(req.Deployment) == "" {
		ve.Add("deployment", "deployment is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// TriggerRollout handles POST /api/v1/secrets/drift/rollout.
// Initiates a graceful rolling restart for workloads whose secrets have drifted,
// guarded by a strict 5% error rate safety circuit breaker.
func (h *SecretDriftHandler) TriggerRollout(w http.ResponseWriter, r *http.Request) {
	var req triggerRolloutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	policy := domain.ExternalSecretPolicy{
		Namespace:           req.Namespace,
		Name:                req.Name,
		Provider:            domain.ProviderVault,
		SecretDataHash:      req.Hash,
		ReloaderAutoRestart: req.ReloaderAutoRestart,
	}

	if h.reconciler != nil {
		if err := h.reconciler.ReconcileSecretDataHash(r.Context(), policy, req.Deployment); err != nil {
			if errors.Is(err, domain.ErrRolloutDegraded) {
				writeError(w, http.StatusUnprocessableEntity, err.Error(), err)
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to reconcile secret rollout", err)
			return
		}
	} else if h.detector != nil {
		if err := h.detector.TriggerGracefulRollout(r.Context(), req.Deployment); err != nil {
			if errors.Is(err, domain.ErrRolloutDegraded) {
				writeError(w, http.StatusUnprocessableEntity, err.Error(), err)
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to trigger rollout", err)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "rollout_triggered",
		"deployment": req.Deployment,
	})
}

// VerifyBackup handles GET /api/v1/secrets/drift/backup-verify.
// Asserts that an off-cluster disaster recovery backup exists for the master KMS key URI.
func (h *SecretDriftHandler) VerifyBackup(w http.ResponseWriter, r *http.Request) {
	kmsKeyURI := strings.TrimSpace(r.URL.Query().Get("kms_key_uri"))
	if kmsKeyURI == "" {
		writeError(w, http.StatusBadRequest, "kms_key_uri query parameter is required", nil)
		return
	}

	if h.reconciler != nil {
		verified, err := h.reconciler.VerifyMasterKeyBackupBeforeRotation(r.Context(), kmsKeyURI)
		if err != nil {
			if errors.Is(err, domain.ErrMasterKeyBackupMissing) {
				writeError(w, http.StatusNotFound, err.Error(), err)
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to verify key backup", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"verified":    verified,
			"kms_key_uri": kmsKeyURI,
		})
		return
	}

	if h.detector != nil {
		verified, err := h.detector.VerifySealedKeyBackup(r.Context(), kmsKeyURI)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to verify key backup", err)
			return
		}
		if !verified {
			writeError(w, http.StatusNotFound, domain.ErrMasterKeyBackupMissing.Error(), domain.ErrMasterKeyBackupMissing)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"verified":    true,
			"kms_key_uri": kmsKeyURI,
		})
		return
	}

	writeError(w, http.StatusInternalServerError, "secret drift detector unconfigured", nil)
}
