package http

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	"github.com/datdt/k8sselfhost/internal/usecase/gitops"
)

// GitOpsHandler provides HTTP endpoints for GitOps workflows, pull requests, and webhooks.
type GitOpsHandler struct {
	controller    *gitops.Controller
	verifier      middleware.WebhookHMACVerifier
	webhookSecret []byte
}

// GitOpsHandlerOption configures GitOpsHandler.
type GitOpsHandlerOption func(*GitOpsHandler)

// WithWebhookSecret configures the webhook HMAC secret key.
func WithWebhookSecret(secret []byte) GitOpsHandlerOption {
	return func(h *GitOpsHandler) {
		h.webhookSecret = secret
	}
}

// WithWebhookVerifier overrides the default webhook HMAC verifier.
func WithWebhookVerifier(verifier middleware.WebhookHMACVerifier) GitOpsHandlerOption {
	return func(h *GitOpsHandler) {
		h.verifier = verifier
	}
}

// NewGitOpsHandler creates a new GitOpsHandler.
func NewGitOpsHandler(controller *gitops.Controller, opts ...GitOpsHandlerOption) *GitOpsHandler {
	h := &GitOpsHandler{
		controller: controller,
		verifier:   middleware.NewWebhookVerifier(),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// RegisterRoutes registers GitOps endpoints on the given Chi router.
func (h *GitOpsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/prs", h.ListPRs)
	r.Get("/prs/{id}", h.GetPR)

	r.With(middleware.RequireRolesForMutations("platform_admin", "tenant_admin")).Group(func(mut chi.Router) {
		mut.Post("/prs", h.CreatePR)
		mut.Post("/prs/{id}/merge", h.MergePR)
		mut.Post("/prs/{id}/close", h.ClosePR)
	})

	r.Post("/webhook", h.HandleWebhook)
}

// ListPRs handles GET /api/v1/gitops/prs
func (h *GitOpsHandler) ListPRs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pull_requests": []interface{}{},
		"total":         0,
	})
}

// GetPR handles GET /api/v1/gitops/prs/{id}
func (h *GitOpsHandler) GetPR(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "PR ID is required", nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"id":     id,
		"status": string(domainGitops.PRStatusPending),
	})
}

// CreatePR handles POST /api/v1/gitops/prs
func (h *GitOpsHandler) CreatePR(w http.ResponseWriter, r *http.Request) {
	var req gitops.CreatePRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if h.controller == nil {
		writeError(w, http.StatusServiceUnavailable, "GitOps controller is not configured", nil)
		return
	}

	pr, err := h.controller.CreatePR(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create PR", err)
		return
	}

	writeJSON(w, http.StatusCreated, pr)
}

// MergePR handles POST /api/v1/gitops/prs/{id}/merge
func (h *GitOpsHandler) MergePR(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "PR ID is required", nil)
		return
	}

	if h.controller == nil {
		writeError(w, http.StatusServiceUnavailable, "GitOps controller is not configured", nil)
		return
	}

	pr, err := h.controller.MergePR(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to merge PR", err)
		return
	}

	writeJSON(w, http.StatusOK, pr)
}

// ClosePR handles POST /api/v1/gitops/prs/{id}/close
func (h *GitOpsHandler) ClosePR(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "PR ID is required", nil)
		return
	}

	if h.controller == nil {
		writeError(w, http.StatusServiceUnavailable, "GitOps controller is not configured", nil)
		return
	}

	pr, err := h.controller.ClosePR(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to close PR", err)
		return
	}

	writeJSON(w, http.StatusOK, pr)
}

// HandleWebhook handles POST /api/v1/gitops/webhook
func (h *GitOpsHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if len(h.webhookSecret) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "webhook secret is not configured",
			"code":  "ERR_WEBHOOK_SECRET_UNSET",
		})
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read webhook payload", err)
		return
	}

	sigHeader := r.Header.Get("X-Hub-Signature-256")
	gitlabToken := r.Header.Get("X-Gitlab-Token")

	if strings.TrimSpace(sigHeader) == "" && strings.TrimSpace(gitlabToken) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Missing webhook signature or token header.",
			"code":  "ERR_WEBHOOK_HMAC_INVALID",
		})
		return
	}

	var valid bool
	if sigHeader != "" {
		valid = h.verifier.VerifyGitHubSignature(bodyBytes, sigHeader, h.webhookSecret)
	} else if gitlabToken != "" {
		valid = h.verifier.VerifyGitLabToken(gitlabToken, string(h.webhookSecret))
	}

	if !valid {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid X-Hub-Signature-256 HMAC signature.",
			"code":  "ERR_WEBHOOK_HMAC_INVALID",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "received",
	})
}
