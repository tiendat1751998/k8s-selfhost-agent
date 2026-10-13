package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"

	domain "github.com/datdt/k8sselfhost/internal/domain/secdefense"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
	"go.uber.org/zap"
)

// SecDefenseHandler handles HTTP requests for DevSecOps supply chain firewalls,
// dual IPv4/IPv6 SSRF / IMDSv2 protection, and native sidecar validation.
type SecDefenseHandler struct {
	engine domain.SecurityDefenseEngine
}

// NewSecDefenseHandler creates a new SecDefenseHandler instance.
func NewSecDefenseHandler(engine domain.SecurityDefenseEngine) *SecDefenseHandler {
	return &SecDefenseHandler{
		engine: engine,
	}
}

// Mount registers the routes onto a chi router under /api/v1/security.
func (h *SecDefenseHandler) Mount(r chi.Router) {
	r.Route("/api/v1/security", func(sub chi.Router) {
		sub.Post("/ssrf/validate-url", h.ValidateSSRF)
		sub.Post("/supplychain/audit-deps", h.AuditDependencies)
		sub.Post("/sidecar/validate-order", h.ValidateSidecarOrder)
		sub.Post("/imdsv2/enforce", h.EnforceIMDSv2)
	})
}

// RegisterRoutes registers routes directly on a sub-router.
func (h *SecDefenseHandler) RegisterRoutes(r chi.Router) {
	r.Post("/ssrf/validate-url", h.ValidateSSRF)
	r.Post("/supplychain/audit-deps", h.AuditDependencies)
	r.Post("/sidecar/validate-order", h.ValidateSidecarOrder)
	r.Post("/imdsv2/enforce", h.EnforceIMDSv2)
}

type ssrfValidateRequest struct {
	URL string `json:"url"`
}

// ValidateSSRF handles POST /api/v1/security/ssrf/validate-url.
func (h *SecDefenseHandler) ValidateSSRF(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Missing or empty request body", "READ_ERROR", r.URL.Path)
		return
	}
	defer r.Body.Close()

	var req ssrfValidateRequest
	if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.URL) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Valid 'url' field is required in request body", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	result, err := h.engine.ValidateURLAgainstSSRF(r.Context(), req.URL)
	if errors.Is(err, domain.ErrSecSSRFMetadataAttempt) {
		h.writeProblem(w, http.StatusForbidden, "SSRF Cloud Metadata Forbidden",
			"Target URL attempts to access cloud metadata endpoint (169.254.169.254 or fd00:ec2::254)",
			"ERR_SEC_SSRF_METADATA_ATTEMPT", r.URL.Path)
		return
	}
	if err != nil {
		logger.Get().Warn("SSRF URL validation failed with error", zap.Error(err))
		h.writeProblem(w, http.StatusBadRequest, "Validation Error", err.Error(), "VALIDATION_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

type auditDepsRequest struct {
	LockfilePath    string   `json:"lockfile_path"`
	LockfileContent string   `json:"lockfile_content"`
	PrivatePrefixes []string `json:"private_prefixes"`
}

// AuditDependencies handles POST /api/v1/security/supplychain/audit-deps.
func (h *SecDefenseHandler) AuditDependencies(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Missing request body", "READ_ERROR", r.URL.Path)
		return
	}
	defer r.Body.Close()

	var req auditDepsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Malformed JSON request body", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	targetPath := req.LockfilePath
	if targetPath == "" && req.LockfileContent != "" {
		tmpFile, err := os.CreateTemp("", "package-lock-*.json")
		if err != nil {
			h.writeProblem(w, http.StatusInternalServerError, "Internal Error", "Failed to create temp lockfile", "IO_ERROR", r.URL.Path)
			return
		}
		defer os.Remove(tmpFile.Name())
		if _, err := tmpFile.WriteString(req.LockfileContent); err != nil {
			_ = tmpFile.Close()
			h.writeProblem(w, http.StatusInternalServerError, "Internal Error", "Failed to write temp lockfile", "IO_ERROR", r.URL.Path)
			return
		}
		_ = tmpFile.Close()
		targetPath = tmpFile.Name()
	}

	if targetPath == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Either lockfile_path or lockfile_content must be provided", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	report, err := h.engine.AuditDependencies(r.Context(), targetPath, req.PrivatePrefixes)
	if errors.Is(err, domain.ErrSecDependencyConfusion) {
		h.writeProblem(w, http.StatusUnprocessableEntity, "Dependency Confusion Detected",
			"Internal namespace packages resolved to public registry or abnormal spoofed version spike",
			"ERR_SEC_DEPENDENCY_CONFUSION", r.URL.Path)
		return
	}
	if err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Audit Failed", err.Error(), "AUDIT_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(report)
}

type validateSidecarRequest struct {
	PodSpecYAML string `json:"pod_spec_yaml"`
}

// ValidateSidecarOrder handles POST /api/v1/security/sidecar/validate-order.
func (h *SecDefenseHandler) ValidateSidecarOrder(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Missing request body", "READ_ERROR", r.URL.Path)
		return
	}
	defer r.Body.Close()

	var req validateSidecarRequest
	if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.PodSpecYAML) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Valid 'pod_spec_yaml' field is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	compliant, err := h.engine.ValidateSidecarStartupOrdering(r.Context(), []byte(req.PodSpecYAML))
	if errors.Is(err, domain.ErrInvalidSidecarOrder) {
		h.writeProblem(w, http.StatusUnprocessableEntity, "Invalid Sidecar Startup Ordering",
			"Sidecar proxies must run as native initContainers with restartPolicy: Always to prevent unencrypted plaintext egress",
			"ERR_INVALID_SIDECAR_ORDER", r.URL.Path)
		return
	}
	if err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Validation Error", err.Error(), "VALIDATION_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"compliant": compliant,
	})
}

type enforceIMDSv2Request struct {
	InstanceID    string `json:"instance_id"`
	HopLimit      int    `json:"hop_limit"`
	RequireTokens bool   `json:"require_tokens"`
}

// EnforceIMDSv2 handles POST /api/v1/security/imdsv2/enforce.
func (h *SecDefenseHandler) EnforceIMDSv2(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Missing request body", "READ_ERROR", r.URL.Path)
		return
	}
	defer r.Body.Close()

	var req enforceIMDSv2Request
	if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.InstanceID) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Valid 'instance_id' is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	if err := h.engine.EnforceIMDSv2(r.Context(), req.InstanceID); err != nil {
		h.writeProblem(w, http.StatusInternalServerError, "IMDSv2 Enforcement Failed", err.Error(), "ENFORCEMENT_ERROR", r.URL.Path)
		return
	}

	hopLimit := req.HopLimit
	if hopLimit <= 0 {
		hopLimit = 1
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "ENFORCED",
		"instance_id":    req.InstanceID,
		"imds_version":   "v2",
		"hop_limit":      hopLimit,
		"require_tokens": true,
	})
}

func (h *SecDefenseHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
	problem := ProblemDetails{
		Type:     "about:blank",
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
		Code:     code,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}
