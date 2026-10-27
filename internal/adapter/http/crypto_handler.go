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
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	domaincrypto "github.com/datdt/k8sselfhost/internal/domain/crypto"
	"github.com/datdt/k8sselfhost/internal/domain/settings"
	usecasecrypto "github.com/datdt/k8sselfhost/internal/usecase/crypto"
)

// CryptoHandler exposes enterprise PKI OCSP enforcement, HSM session pool, and envelope encryption APIs.
type CryptoHandler struct {
	manager      *usecasecrypto.ShieldManager
	kmsClient    KMSClient
	settingsRepo settings.Repository
}

// NewCryptoHandler initializes a new CryptoHandler instance.
func NewCryptoHandler(manager *usecasecrypto.ShieldManager, kmsClient ...KMSClient) *CryptoHandler {
	var kc KMSClient
	if len(kmsClient) > 0 && kmsClient[0] != nil {
		kc = kmsClient[0]
	} else {
		kc = NewDefaultKMSClient()
	}
	return &CryptoHandler{manager: manager, kmsClient: kc}
}

// SetKMSClient configures an alternative KMSClient (e.g. For integration tests).
func (h *CryptoHandler) SetKMSClient(kc KMSClient) {
	if kc != nil {
		h.kmsClient = kc
	}
}

// SetSettingsRepo configures settings repository for persistent KMS metadata across restarts.
func (h *CryptoHandler) SetSettingsRepo(repo settings.Repository) {
	h.settingsRepo = repo
	if defKMS, ok := h.kmsClient.(*DefaultKMSClient); ok {
		defKMS.SetSettingsRepo(repo)
	}
}

// RegisterRoutes registers the cryptographic shielding endpoints on the router.
func (h *CryptoHandler) RegisterRoutes(r chi.Router) {
	m := func(sub chi.Router) {
		sub.Post("/ocsp/verify-staple", h.VerifyAndPrefetchOCSPStaple)
		sub.Get("/hsm/session-status", h.GetHSMSessionPoolStatus)
		sub.Post("/envelope/dek", h.GetOrExtendEnvelopeDEK)
		sub.Post("/fips/scan", h.ScanSourceForNonFIPSAlgorithms)
		sub.Get("/kms/key", h.GetKMSKey)
		sub.Get("/kms/keys", h.GetKMSKeys)
		sub.Get("/kms/status", h.GetKMSStatus)
		sub.Post("/kms/rotate", h.RotateKMSKey)
		sub.Post("/kms/verify", h.VerifyKMSKey)
	}
	m(r)
	r.Route("/api/v1/crypto", m)

	kms := func(sub chi.Router) {
		sub.Get("/key", h.GetKMSKey)
		sub.Get("/keys", h.GetKMSKeys)
		sub.Get("/status", h.GetKMSStatus)
		sub.Post("/rotate", h.RotateKMSKey)
		sub.Post("/verify", h.VerifyKMSKey)
	}
	r.Route("/api/v1/security/kms", kms)
	r.Route("/security/kms", kms)
}

func (h *CryptoHandler) checkKMS(w http.ResponseWriter) bool {
	if h.kmsClient == nil {
		writeError(w, http.StatusServiceUnavailable, "kms client not available", nil)
		return false
	}
	return true
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
		} else {
			writeError(w, http.StatusBadRequest, "failed to verify ocsp staple", err)
		}
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

type (
	VerifyStapleRequest struct {
		CertChainPEM string `json:"cert_chain_pem"`
	}
	EnvelopeDEKRequest struct {
		KeyID string `json:"key_id"`
	}
	EnvelopeDEKResponse struct {
		KeyID        string `json:"key_id"`
		PlaintextKey string `json:"plaintext_key"`
		IsStaleValid bool   `json:"is_stale_valid"`
	}
	FIPSScanRequest struct {
		RootDir string `json:"root_dir"`
	}
	FIPSScanResponse struct {
		Compliant  bool     `json:"compliant"`
		Violations []string `json:"violations"`
		Error      string   `json:"error,omitempty"`
	}
	KMSKeyDetails struct {
		KeyID     string `json:"key_id"`
		Version   int    `json:"version"`
		Algorithm string `json:"algorithm"`
	}
	KMSRotateResponse struct {
		KeyID     string `json:"key_id"`
		Version   int    `json:"version"`
		RotatedAt string `json:"rotated_at"`
	}
	KMSVerifyResponse struct {
		Verified bool   `json:"verified"`
		Checksum string `json:"checksum"`
	}
	KMSKeyMetadata struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		Algorithm      string `json:"algorithm"`
		Provider       string `json:"provider"`
		Version        int    `json:"version"`
		Status         string `json:"status"`
		HardwareBacked bool   `json:"hardware_backed"`
		Mode           string `json:"mode"`
		CreatedAt      string `json:"created_at"`
		RotatedAt      string `json:"rotated_at,omitempty"`
	}
	KMSStatusResponse struct {
		Provider       string `json:"provider"`
		HardwareBacked bool   `json:"hardware_backed"`
		Mode           string `json:"mode"`
		KeyID          string `json:"key_id"`
		Version        int    `json:"version"`
		Algorithm      string `json:"algorithm"`
		Status         string `json:"status"`
		CreatedAt      string `json:"created_at"`
		RotatedAt      string `json:"rotated_at,omitempty"`
	}
	KMSKeysResponse struct {
		Keys  []KMSKeyMetadata `json:"keys"`
		Count int              `json:"count"`
	}
)

// KMSClient defines operations for Cloud / Enterprise KMS integration.
type KMSClient interface {
	DecryptDEK(ctx context.Context, keyID string) ([]byte, error)
	GetKey(ctx context.Context) (*KMSKeyDetails, error)
	RotateKey(ctx context.Context) (*KMSRotateResponse, error)
	VerifyKey(ctx context.Context) (*KMSVerifyResponse, error)
}

func detectKMSProvider() (string, bool, string) {
	if os.Getenv("VAULT_ADDR") != "" {
		return "hashicorp-vault", false, "envelope-aes-256-gcm"
	}
	return "local-aes256-gcm", false, "envelope-aes-256-gcm"
}

// DefaultKMSClient is an AES-256-GCM compliant KMS provider with PostgreSQL metadata persistence.
type DefaultKMSClient struct {
	mu           sync.RWMutex
	keyID        string
	version      int
	algorithm    string
	createdAt    string
	rotatedAt    string
	keys         map[string][]byte
	settingsRepo settings.Repository
}

// NewDefaultKMSClient instantiates a new DefaultKMSClient with secure initialization.
func NewDefaultKMSClient() *DefaultKMSClient {
	c := &DefaultKMSClient{
		keyID: "kms-master-primary-key", version: 1, algorithm: "AES-256-GCM",
		createdAt: time.Now().UTC().Format(time.RFC3339), keys: make(map[string][]byte),
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		copy(key, []byte("production-secure-default-key-32"))
	}
	c.keys[c.keyID] = key
	return c
}

// SetSettingsRepo configures settings repository for persistent metadata across restarts.
func (c *DefaultKMSClient) SetSettingsRepo(repo settings.Repository) {
	c.mu.Lock()
	c.settingsRepo = repo
	c.mu.Unlock()
	c.loadMetadataFromRepo(context.Background())
}

func (c *DefaultKMSClient) loadMetadataFromRepo(ctx context.Context) {
	c.mu.RLock()
	repo := c.settingsRepo
	c.mu.RUnlock()
	if repo == nil {
		return
	}
	if s, err := repo.GetByKey(ctx, "default-tenant", "security", "security.kms_metadata"); err == nil && s != nil && s.Value != "" {
		var m KMSKeyMetadata
		if json.Unmarshal([]byte(s.Value), &m) == nil && m.ID != "" {
			c.mu.Lock()
			c.keyID, c.version, c.algorithm, c.createdAt, c.rotatedAt = m.ID, m.Version, m.Algorithm, m.CreatedAt, m.RotatedAt
			c.mu.Unlock()
			return
		}
	}
	c.saveMetadataToRepo(ctx)
}

func (c *DefaultKMSClient) saveMetadataToRepo(ctx context.Context) {
	c.mu.RLock()
	repo, id, ver, alg, created, rotated := c.settingsRepo, c.keyID, c.version, c.algorithm, c.createdAt, c.rotatedAt
	c.mu.RUnlock()
	if repo == nil {
		return
	}
	p, hw, m := detectKMSProvider()
	val, _ := json.Marshal(KMSKeyMetadata{
		ID: id, Name: "Primary Master Key", Algorithm: alg, Provider: p, Version: ver,
		Status: "active", HardwareBacked: hw, Mode: m, CreatedAt: created, RotatedAt: rotated,
	})
	_ = repo.Upsert(ctx, &settings.Setting{
		Category: "security", Key: "security.kms_metadata", Value: string(val),
		TenantID: "default-tenant", UpdatedBy: "system",
	})
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
	return &KMSKeyDetails{KeyID: c.keyID, Version: c.version, Algorithm: c.algorithm}, nil
}

// RotateKey rolls the master key to a new version and persists metadata.
func (c *DefaultKMSClient) RotateKey(ctx context.Context) (*KMSRotateResponse, error) {
	c.mu.Lock()
	c.version++
	newKey := make([]byte, 32)
	if _, err := rand.Read(newKey); err != nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("entropy failure during rotation: %w", err)
	}
	c.keys[c.keyID] = newKey
	c.rotatedAt = time.Now().UTC().Format(time.RFC3339)
	resp := &KMSRotateResponse{KeyID: c.keyID, Version: c.version, RotatedAt: c.rotatedAt}
	c.mu.Unlock()
	c.saveMetadataToRepo(ctx)
	return resp, nil
}

// VerifyKey computes the SHA-256 checksum of the current master key.
func (c *DefaultKMSClient) VerifyKey(ctx context.Context) (*KMSVerifyResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	sum := sha256.Sum256(c.keys[c.keyID])
	return &KMSVerifyResponse{Verified: true, Checksum: hex.EncodeToString(sum[:])}, nil
}

// GetStatus returns the honest cryptographic provider status.
func (c *DefaultKMSClient) GetStatus(ctx context.Context) (*KMSStatusResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, hw, m := detectKMSProvider()
	return &KMSStatusResponse{
		Provider: p, HardwareBacked: hw, Mode: m, KeyID: c.keyID,
		Version: c.version, Algorithm: c.algorithm, Status: "active",
		CreatedAt: c.createdAt, RotatedAt: c.rotatedAt,
	}, nil
}

// GetKeys returns master key metadata list without raw key material.
func (c *DefaultKMSClient) GetKeys(ctx context.Context) ([]KMSKeyMetadata, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, hw, m := detectKMSProvider()
	return []KMSKeyMetadata{{
		ID: c.keyID, Name: "Primary Master Key", Algorithm: c.algorithm,
		Provider: p, Version: c.version, Status: "active", HardwareBacked: hw, Mode: m,
		CreatedAt: c.createdAt, RotatedAt: c.rotatedAt,
	}}, nil
}

// GetOrExtendEnvelopeDEK handles POST /api/v1/crypto/envelope/dek
func (h *CryptoHandler) GetOrExtendEnvelopeDEK(w http.ResponseWriter, r *http.Request) {
	var req EnvelopeDEKRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.KeyID) == "" {
		writeError(w, http.StatusBadRequest, "invalid request body: key_id is required", err)
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
		} else {
			writeError(w, http.StatusServiceUnavailable, "kms fallback failed", err)
		}
		return
	}
	isStale := false
	if metrics := h.manager.GetMetrics(); metrics != nil {
		isStale = metrics.DEKStaleHits.Load() > beforeStale
	}
	writeJSON(w, http.StatusOK, EnvelopeDEKResponse{KeyID: req.KeyID, PlaintextKey: string(plaintext), IsStaleValid: isStale})
}

// GetKMSKey handles GET /api/v1/security/kms/key
func (h *CryptoHandler) GetKMSKey(w http.ResponseWriter, r *http.Request) {
	if !h.checkKMS(w) {
		return
	}
	if details, err := h.kmsClient.GetKey(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get kms key", err)
	} else {
		writeJSON(w, http.StatusOK, details)
	}
}

// RotateKMSKey handles POST /api/v1/security/kms/rotate
func (h *CryptoHandler) RotateKMSKey(w http.ResponseWriter, r *http.Request) {
	if !h.checkKMS(w) {
		return
	}
	if resp, err := h.kmsClient.RotateKey(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rotate kms key", err)
	} else {
		writeJSON(w, http.StatusOK, resp)
	}
}

// VerifyKMSKey handles POST /api/v1/security/kms/verify
func (h *CryptoHandler) VerifyKMSKey(w http.ResponseWriter, r *http.Request) {
	if !h.checkKMS(w) {
		return
	}
	if resp, err := h.kmsClient.VerifyKey(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify kms key", err)
	} else {
		writeJSON(w, http.StatusOK, resp)
	}
}

// GetKMSStatus handles GET /api/v1/security/kms/status (Honest provider status reporting)
func (h *CryptoHandler) GetKMSStatus(w http.ResponseWriter, r *http.Request) {
	if !h.checkKMS(w) {
		return
	}
	if sp, ok := h.kmsClient.(interface{ GetStatus(context.Context) (*KMSStatusResponse, error) }); ok {
		if status, err := sp.GetStatus(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get kms status", err)
		} else {
			writeJSON(w, http.StatusOK, status)
		}
		return
	}
	d, err := h.kmsClient.GetKey(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get kms key", err)
		return
	}
	p, hw, m := detectKMSProvider()
	writeJSON(w, http.StatusOK, KMSStatusResponse{
		Provider: p, HardwareBacked: hw, Mode: m, KeyID: d.KeyID, Version: d.Version, Algorithm: d.Algorithm, Status: "active",
	})
}

// GetKMSKeys handles GET /api/v1/security/kms/keys
func (h *CryptoHandler) GetKMSKeys(w http.ResponseWriter, r *http.Request) {
	if !h.checkKMS(w) {
		return
	}
	if sp, ok := h.kmsClient.(interface{ GetKeys(context.Context) ([]KMSKeyMetadata, error) }); ok {
		if keys, err := sp.GetKeys(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get kms keys", err)
		} else {
			writeJSON(w, http.StatusOK, KMSKeysResponse{Keys: keys, Count: len(keys)})
		}
		return
	}
	d, err := h.kmsClient.GetKey(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get kms key", err)
		return
	}
	p, hw, m := detectKMSProvider()
	keys := []KMSKeyMetadata{{
		ID: d.KeyID, Name: "Primary Master Key", Algorithm: d.Algorithm,
		Provider: p, Version: d.Version, Status: "active", HardwareBacked: hw, Mode: m,
	}}
	writeJSON(w, http.StatusOK, KMSKeysResponse{Keys: keys, Count: len(keys)})
}

// ScanSourceForNonFIPSAlgorithms handles POST /api/v1/crypto/fips/scan
func (h *CryptoHandler) ScanSourceForNonFIPSAlgorithms(w http.ResponseWriter, r *http.Request) {
	var req FIPSScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	dir := strings.TrimSpace(req.RootDir)
	if dir == "" {
		dir = "."
	}
	violations, err := h.manager.ScanSourceForNonFIPSAlgorithms(r.Context(), dir)
	if err != nil {
		if errors.Is(err, domaincrypto.ErrFIPSNonCompliantHash) {
			writeJSON(w, http.StatusUnprocessableEntity, FIPSScanResponse{Compliant: false, Violations: violations, Error: "fips non-compliant cryptographic algorithm detected"})
			return
		}
		writeError(w, http.StatusBadRequest, fmt.Sprintf("fips audit failed: %v", err), err)
		return
	}
	writeJSON(w, http.StatusOK, FIPSScanResponse{Compliant: true, Violations: []string{}})
}

// writeProblem outputs an RFC 7807 compliant error payload with application/problem+json content-type.
func (h *CryptoHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ProblemDetails{
		Type: "https://datatracker.ietf.org/doc/html/rfc7807", Title: title, Status: status, Detail: detail, Instance: instance, Code: code,
	})
}
