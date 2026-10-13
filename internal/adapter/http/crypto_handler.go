package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	domaincrypto "github.com/datdt/k8sselfhost/internal/domain/crypto"
	usecasecrypto "github.com/datdt/k8sselfhost/internal/usecase/crypto"
)

// CryptoHandler exposes enterprise PKI OCSP enforcement, HSM session pool, and envelope encryption APIs.
type CryptoHandler struct {
	manager *usecasecrypto.ShieldManager
}

// NewCryptoHandler initializes a new CryptoHandler instance.
func NewCryptoHandler(manager *usecasecrypto.ShieldManager) *CryptoHandler {
	return &CryptoHandler{
		manager: manager,
	}
}

// RegisterRoutes registers the cryptographic shielding endpoints on the router.
func (h *CryptoHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/crypto", func(sub chi.Router) {
		sub.Post("/ocsp/verify-staple", h.VerifyAndPrefetchOCSPStaple)
		sub.Get("/hsm/session-status", h.GetHSMSessionPoolStatus)
		sub.Post("/envelope/dek", h.GetOrExtendEnvelopeDEK)
		sub.Post("/fips/scan", h.ScanSourceForNonFIPSAlgorithms)
	})
}

// VerifyStapleRequest represents the payload for OCSP staple verification and prefetch.
type VerifyStapleRequest struct {
	CertChainPEM string `json:"cert_chain_pem"`
}

// VerifyAndPrefetchOCSPStaple handles POST /api/v1/crypto/ocsp/verify-staple
func (h *CryptoHandler) VerifyAndPrefetchOCSPStaple(w http.ResponseWriter, r *http.Request) {
	var req VerifyStapleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if strings.TrimSpace(req.CertChainPEM) == "" {
		writeError(w, http.StatusBadRequest, "cert_chain_pem is required", nil)
		return
	}

	status, err := h.manager.VerifyAndPrefetchOCSPStaple(r.Context(), []byte(req.CertChainPEM))
	if err != nil {
		if errors.Is(err, domaincrypto.ErrOCSPStapleFetchFailed) {
			h.writeProblem(w, http.StatusBadGateway, "OCSP Staple Fetch Failed", err.Error(), "ERR_OCSP_STAPLE_FETCH_FAILED", r.URL.Path)
			return
		}
		writeError(w, http.StatusBadRequest, "failed to verify ocsp staple", err)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// GetHSMSessionPoolStatus handles GET /api/v1/crypto/hsm/session-status
func (h *CryptoHandler) GetHSMSessionPoolStatus(w http.ResponseWriter, r *http.Request) {
	status := h.manager.GetHSMSessionPoolStatus()
	if status.IsSaturated {
		writeJSON(w, http.StatusServiceUnavailable, status)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// EnvelopeDEKRequest defines the payload for fetching or extending a Data Encryption Key.
type EnvelopeDEKRequest struct {
	KeyID            string `json:"key_id"`
	ForceKMSError    bool   `json:"force_kms_error,omitempty"`
	MockPlaintextKey string `json:"mock_plaintext_key,omitempty"`
}

// EnvelopeDEKResponse defines the JSON response containing the plaintext DEK.
type EnvelopeDEKResponse struct {
	KeyID        string `json:"key_id"`
	PlaintextKey string `json:"plaintext_key"`
	IsStaleValid bool   `json:"is_stale_valid"`
}

// GetOrExtendEnvelopeDEK handles POST /api/v1/crypto/envelope/dek
func (h *CryptoHandler) GetOrExtendEnvelopeDEK(w http.ResponseWriter, r *http.Request) {
	var req EnvelopeDEKRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if strings.TrimSpace(req.KeyID) == "" {
		writeError(w, http.StatusBadRequest, "key_id is required", nil)
		return
	}

	kmsFallback := func() ([]byte, error) {
		if req.ForceKMSError {
			return nil, errors.New("kms 503 service unavailable: rate limit exceeded")
		}
		if req.MockPlaintextKey != "" {
			return []byte(req.MockPlaintextKey), nil
		}
		return nil, errors.New("kms decrypt failed: key not accessible")
	}

	plaintext, err := h.manager.GetOrExtendEnvelopeDEK(r.Context(), req.KeyID, kmsFallback)
	if err != nil {
		if errors.Is(err, domaincrypto.ErrKMSDEKGraceExpired) {
			writeError(w, http.StatusInternalServerError, "kms dek grace period expired", err)
			return
		}
		writeError(w, http.StatusServiceUnavailable, "kms fallback failed", err)
		return
	}

	resp := EnvelopeDEKResponse{
		KeyID:        req.KeyID,
		PlaintextKey: string(plaintext),
		IsStaleValid: req.ForceKMSError,
	}

	writeJSON(w, http.StatusOK, resp)
}

// FIPSScanRequest defines the request body for scanning source directories.
type FIPSScanRequest struct {
	RootDir string `json:"root_dir"`
}

// FIPSScanResponse defines the response structure for FIPS compliance audits.
type FIPSScanResponse struct {
	Compliant  bool     `json:"compliant"`
	Violations []string `json:"violations"`
	Error      string   `json:"error,omitempty"`
}

// ScanSourceForNonFIPSAlgorithms handles POST /api/v1/crypto/fips/scan
func (h *CryptoHandler) ScanSourceForNonFIPSAlgorithms(w http.ResponseWriter, r *http.Request) {
	var req FIPSScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	targetDir := strings.TrimSpace(req.RootDir)
	if targetDir == "" {
		targetDir = "."
	}

	violations, err := h.manager.ScanSourceForNonFIPSAlgorithms(r.Context(), targetDir)
	if err != nil {
		if errors.Is(err, domaincrypto.ErrFIPSNonCompliantHash) {
			writeJSON(w, http.StatusUnprocessableEntity, FIPSScanResponse{
				Compliant:  false,
				Violations: violations,
				Error:      "fips non-compliant cryptographic algorithm detected",
			})
			return
		}
		writeError(w, http.StatusBadRequest, fmt.Sprintf("fips audit failed: %v", err), err)
		return
	}

	writeJSON(w, http.StatusOK, FIPSScanResponse{
		Compliant:  true,
		Violations: []string{},
	})
}

// writeProblem outputs an RFC 7807 compliant error payload with application/problem+json content-type.
func (h *CryptoHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
	prob := ProblemDetails{
		Type:     "https://datatracker.ietf.org/doc/html/rfc7807",
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
		Code:     code,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(prob)
}
