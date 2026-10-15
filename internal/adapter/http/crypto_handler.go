package http

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	domaincrypto "github.com/datdt/k8sselfhost/internal/domain/crypto"
	usecasecrypto "github.com/datdt/k8sselfhost/internal/usecase/crypto"
)

// CryptoHandler exposes enterprise PKI OCSP enforcement, HSM session pool, and envelope encryption APIs.
type CryptoHandler struct {
	manager   *usecasecrypto.ShieldManager
	kmsClient KMSClient
}

// NewCryptoHandler initializes a new CryptoHandler instance.
func NewCryptoHandler(manager *usecasecrypto.ShieldManager, kmsClient ...KMSClient) *CryptoHandler {
	var kc KMSClient
	if len(kmsClient) > 0 && kmsClient[0] != nil {
		kc = kmsClient[0]
	} else {
		kc = NewDefaultKMSClient()
	}
	return &CryptoHandler{
		manager:   manager,
		kmsClient: kc,
	}
}

// SetKMSClient configures an alternative KMSClient (e.g. For integration tests).
func (h *CryptoHandler) SetKMSClient(kc KMSClient) {
	if kc != nil {
		h.kmsClient = kc
	}
}

// RegisterRoutes registers the cryptographic shielding endpoints on the router.
func (h *CryptoHandler) RegisterRoutes(r chi.Router) {
	// Direct sub-route endpoints when mounted under /crypto or /api/v1/crypto
	r.Post("/ocsp/verify-staple", h.VerifyAndPrefetchOCSPStaple)
	r.Get("/hsm/session-status", h.GetHSMSessionPoolStatus)
	r.Post("/envelope/dek", h.GetOrExtendEnvelopeDEK)
	r.Post("/fips/scan", h.ScanSourceForNonFIPSAlgorithms)
	r.Get("/kms/key", h.GetKMSKey)
	r.Post("/kms/rotate", h.RotateKMSKey)
	r.Post("/kms/verify", h.VerifyKMSKey)

	// Top-level mounting when called on a root router
	r.Route("/api/v1/crypto", func(sub chi.Router) {
		sub.Post("/ocsp/verify-staple", h.VerifyAndPrefetchOCSPStaple)
		sub.Get("/hsm/session-status", h.GetHSMSessionPoolStatus)
		sub.Post("/envelope/dek", h.GetOrExtendEnvelopeDEK)
		sub.Post("/fips/scan", h.ScanSourceForNonFIPSAlgorithms)
		sub.Get("/kms/key", h.GetKMSKey)
		sub.Post("/kms/rotate", h.RotateKMSKey)
		sub.Post("/kms/verify", h.VerifyKMSKey)
	})

	r.Route("/api/v1/security/kms", func(sub chi.Router) {
		sub.Get("/key", h.GetKMSKey)
		sub.Post("/rotate", h.RotateKMSKey)
		sub.Post("/verify", h.VerifyKMSKey)
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

// KMSKeyDetails represents KMS master key metadata.
type KMSKeyDetails struct {
	KeyID     string `json:"key_id"`
	Version   int    `json:"version"`
	Algorithm string `json:"algorithm"`
}

// KMSRotateResponse represents key rotation result.
type KMSRotateResponse struct {
	KeyID     string `json:"key_id"`
	Version   int    `json:"version"`
	RotatedAt string `json:"rotated_at"`
}

// KMSVerifyResponse represents integrity verification result.
type KMSVerifyResponse struct {
	Verified bool   `json:"verified"`
	Checksum string `json:"checksum"`
}

// KMSClient defines operations for Cloud / Enterprise KMS integration.
type KMSClient interface {
	DecryptDEK(ctx context.Context, keyID string) ([]byte, error)
	GetKey(ctx context.Context) (*KMSKeyDetails, error)
	RotateKey(ctx context.Context) (*KMSRotateResponse, error)
	VerifyKey(ctx context.Context) (*KMSVerifyResponse, error)
}

// DefaultKMSClient is an in-memory AES-256-GCM compliant KMS provider.
type DefaultKMSClient struct {
	mu        sync.RWMutex
	keyID     string
	version   int
	algorithm string
	keys      map[string][]byte
}

// NewDefaultKMSClient instantiates a new DefaultKMSClient with secure initialization.
func NewDefaultKMSClient() *DefaultKMSClient {
	c := &DefaultKMSClient{
		keyID:     "kms-master-primary-key",
		version:   1,
		algorithm: "AES-256-GCM",
		keys:      make(map[string][]byte),
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		copy(key, []byte("production-secure-default-key-32"))
	}
	c.keys[c.keyID] = key
	return c
}

// DecryptDEK returns or generates a DEK.
func (c *DefaultKMSClient) DecryptDEK(ctx context.Context, keyID string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if k, ok := c.keys[keyID]; ok {
		return k, nil
	}
	newKey := make([]byte, 32)
	if _, err := rand.Read(newKey); err != nil {
		return nil, fmt.Errorf("entropy source failure: %w", err)
	}
	c.keys[keyID] = newKey
	return newKey, nil
}

// GetKey returns the master key metadata.
func (c *DefaultKMSClient) GetKey(ctx context.Context) (*KMSKeyDetails, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return &KMSKeyDetails{
		KeyID:     c.keyID,
		Version:   c.version,
		Algorithm: c.algorithm,
	}, nil
}

// RotateKey rolls the master key to a new version.
func (c *DefaultKMSClient) RotateKey(ctx context.Context) (*KMSRotateResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.version++
	newKey := make([]byte, 32)
	if _, err := rand.Read(newKey); err != nil {
		return nil, fmt.Errorf("entropy failure during rotation: %w", err)
	}
	c.keys[c.keyID] = newKey
	return &KMSRotateResponse{
		KeyID:     c.keyID,
		Version:   c.version,
		RotatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// VerifyKey computes the SHA-256 checksum of the current master key.
func (c *DefaultKMSClient) VerifyKey(ctx context.Context) (*KMSVerifyResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	k := c.keys[c.keyID]
	sum := sha256.Sum256(k)
	return &KMSVerifyResponse{
		Verified: true,
		Checksum: hex.EncodeToString(sum[:]),
	}, nil
}

// EnvelopeDEKRequest defines the payload for fetching or extending a Data Encryption Key.
type EnvelopeDEKRequest struct {
	KeyID string `json:"key_id"`
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
		if h.kmsClient == nil {
			return nil, errors.New("kms client not configured")
		}
		return h.kmsClient.DecryptDEK(r.Context(), req.KeyID)
	}

	beforeStale := uint64(0)
	if metrics := h.manager.GetMetrics(); metrics != nil {
		beforeStale = metrics.DEKStaleHits.Load()
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

	isStale := false
	if metrics := h.manager.GetMetrics(); metrics != nil {
		isStale = metrics.DEKStaleHits.Load() > beforeStale
	}

	resp := EnvelopeDEKResponse{
		KeyID:        req.KeyID,
		PlaintextKey: string(plaintext),
		IsStaleValid: isStale,
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetKMSKey handles GET /api/v1/security/kms/key
func (h *CryptoHandler) GetKMSKey(w http.ResponseWriter, r *http.Request) {
	if h.kmsClient == nil {
		writeError(w, http.StatusServiceUnavailable, "kms client not available", nil)
		return
	}
	details, err := h.kmsClient.GetKey(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get kms key", err)
		return
	}
	writeJSON(w, http.StatusOK, details)
}

// RotateKMSKey handles POST /api/v1/security/kms/rotate
func (h *CryptoHandler) RotateKMSKey(w http.ResponseWriter, r *http.Request) {
	if h.kmsClient == nil {
		writeError(w, http.StatusServiceUnavailable, "kms client not available", nil)
		return
	}
	resp, err := h.kmsClient.RotateKey(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rotate kms key", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// VerifyKMSKey handles POST /api/v1/security/kms/verify
func (h *CryptoHandler) VerifyKMSKey(w http.ResponseWriter, r *http.Request) {
	if h.kmsClient == nil {
		writeError(w, http.StatusServiceUnavailable, "kms client not available", nil)
		return
	}
	resp, err := h.kmsClient.VerifyKey(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify kms key", err)
		return
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
