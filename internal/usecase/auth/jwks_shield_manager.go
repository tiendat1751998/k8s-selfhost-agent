package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	randv2 "math/rand/v2"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	domainauth "github.com/datdt/k8sselfhost/internal/domain/auth"
)

// Default constants for JWKS keyset caching and stale window protection.
const (
	DefaultJWKSTTL       = 15 * time.Minute
	DefaultMaxStaleWindow = 24 * time.Hour
	DefaultFetchTimeout   = 2 * time.Second
)

// JWKSFetcher defines the port for fetching remote JWKS documents over HTTPS.
type JWKSFetcher interface {
	FetchJWKS(ctx context.Context, providerURL string) ([]*domainauth.JWKSKey, error)
}

// VaultUnsealExecutor defines the port for triggering automated TPM2/KMS unseal sequence.
type VaultUnsealExecutor interface {
	ExecuteUnseal(ctx context.Context) (domainauth.VaultUnsealStatus, error)
}

// HTTPJWKSFetcher provides a production-grade JWKS fetcher over HTTP/HTTPS.
type HTTPJWKSFetcher struct {
	client *http.Client
}

// NewHTTPJWKSFetcher creates a new HTTP fetcher with custom or default HTTP client.
func NewHTTPJWKSFetcher(client *http.Client) *HTTPJWKSFetcher {
	if client == nil {
		client = &http.Client{Timeout: DefaultFetchTimeout}
	}
	return &HTTPJWKSFetcher{client: client}
}

// jwkKey represents a single JWK key in an RFC 7517 JWKS document.
type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// jwksDocument represents the top-level keys collection in RFC 7517.
type jwksDocument struct {
	Keys []jwkKey `json:"keys"`
}

// FetchJWKS fetches and decodes the JSON Web Key Set from the specified provider URL.
func (f *HTTPJWKSFetcher) FetchJWKS(ctx context.Context, providerURL string) ([]*domainauth.JWKSKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, providerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating jwks request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching jwks endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("jwks endpoint returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decoding jwks response json: %w", err)
	}

	now := time.Now()
	var keys []*domainauth.JWKSKey
	for _, k := range doc.Keys {
		if k.Kty != "RSA" && k.Kty != "" {
			continue
		}
		pubKey, err := ParseRSAPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		alg := k.Alg
		if alg == "" {
			alg = "RS256"
		}
		keys = append(keys, &domainauth.JWKSKey{
			KID:       k.Kid,
			PublicKey: pubKey,
			Algorithm: alg,
			FetchedAt: now,
			ExpiresAt: now.Add(DefaultJWKSTTL),
		})
	}

	return keys, nil
}

// ParseRSAPublicKey converts base64url-encoded modulus (N) and exponent (E) to *rsa.PublicKey.
func ParseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := decodeBase64URL(nStr)
	if err != nil {
		return nil, fmt.Errorf("decoding modulus: %w", err)
	}
	eBytes, err := decodeBase64URL(eStr)
	if err != nil {
		return nil, fmt.Errorf("decoding exponent: %w", err)
	}

	var eVal int
	for _, b := range eBytes {
		eVal = (eVal << 8) | int(b)
	}
	if eVal == 0 {
		return nil, errors.New("invalid RSA exponent (evaluates to zero)")
	}

	modulus := new(big.Int).SetBytes(nBytes)
	return &rsa.PublicKey{
		N: modulus,
		E: eVal,
	}, nil
}

func decodeBase64URL(s string) ([]byte, error) {
	if data, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	return base64.URLEncoding.DecodeString(s)
}

// JWKSOption configures JWKSShieldManager.
type JWKSOption func(*JWKSShieldManager)

// WithProviderURL sets the remote OIDC JWKS endpoint URL.
func WithProviderURL(url string) JWKSOption {
	return func(m *JWKSShieldManager) {
		m.providerURL = url
	}
}

// WithDefaultTTL configures standard key expiry window.
func WithDefaultTTL(ttl time.Duration) JWKSOption {
	return func(m *JWKSShieldManager) {
		m.defaultTTL = ttl
	}
}

// WithMaxStaleWindow configures the maximum grace period after which stale keys are rejected.
func WithMaxStaleWindow(w time.Duration) JWKSOption {
	return func(m *JWKSShieldManager) {
		m.maxStaleWindow = w
	}
}

// WithFetcher injects a custom JWKS fetcher implementation.
func WithFetcher(f JWKSFetcher) JWKSOption {
	return func(m *JWKSShieldManager) {
		m.fetcher = f
	}
}

// WithVaultUnsealExecutor injects an unseal executor.
func WithVaultUnsealExecutor(e VaultUnsealExecutor) JWKSOption {
	return func(m *JWKSShieldManager) {
		m.vaultExecutor = e
	}
}

// WithClock overrides the system clock for deterministic testing.
func WithClock(clock func() time.Time) JWKSOption {
	return func(m *JWKSShieldManager) {
		m.clock = clock
	}
}

// WithStaticBackupKeys preloads static fallback keys from K8s secrets/configmaps.
func WithStaticBackupKeys(keys map[string]*domainauth.JWKSKey) JWKSOption {
	return func(m *JWKSShieldManager) {
		m.SetStaticBackupKeys(keys)
	}
}

// JWKSShieldManager manages high-frequency JWKS validation keys with Stale-While-Revalidate caching.
type JWKSShieldManager struct {
	mu                sync.RWMutex
	keys              map[string]*domainauth.JWKSKey
	staticBackupKeys  map[string]*domainauth.JWKSKey
	providerURL       string
	defaultTTL        time.Duration
	maxStaleWindow    time.Duration
	fetcher           JWKSFetcher
	vaultExecutor     VaultUnsealExecutor
	clock             func() time.Time
	metrics           domainauth.JWKSMetrics
	lastFetchError    string
	lastFetchTime     time.Time
	refreshInProgress int32
	refreshGroup      singleflight.Group
}

// NewJWKSShieldManager creates an initialized JWKSShieldManager.
func NewJWKSShieldManager(opts ...JWKSOption) *JWKSShieldManager {
	m := &JWKSShieldManager{
		keys:             make(map[string]*domainauth.JWKSKey),
		staticBackupKeys: make(map[string]*domainauth.JWKSKey),
		defaultTTL:       DefaultJWKSTTL,
		maxStaleWindow:   DefaultMaxStaleWindow,
		clock:            time.Now,
		fetcher:          NewHTTPJWKSFetcher(nil),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *JWKSShieldManager) now() time.Time {
	if m.clock != nil {
		return m.clock()
	}
	return time.Now()
}

// InjectKey inserts a key into RAM cache directly.
func (m *JWKSShieldManager) InjectKey(key *domainauth.JWKSKey) {
	if key == nil || key.KID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys[key.KID] = key
}

// SetStaticBackupKeys registers backup keys to serve when IdP is completely unreachable.
func (m *JWKSShieldManager) SetStaticBackupKeys(keys map[string]*domainauth.JWKSKey) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.staticBackupKeys = make(map[string]*domainauth.JWKSKey, len(keys))
	for k, v := range keys {
		m.staticBackupKeys[k] = v
	}
}

// GetValidationKey retrieves the RSA public key for signature verification with Stale-While-Revalidate semantics.
func (m *JWKSShieldManager) GetValidationKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	now := m.now()

	m.mu.RLock()
	key, found := m.keys[kid]
	m.mu.RUnlock()

	// Fast Path: key found in RAM and not expired
	if found && !key.IsExpired(now) {
		m.metrics.Hits.Inc()
		return key.PublicKey, nil
	}

	// Stale path: key found but expired
	if found && key.IsExpired(now) {
		// Asynchronously trigger refresh with 2s timeout
		m.triggerBackgroundRefresh()

		// Enforce 24h maximum stale window guardrail
		if key.IsStaleExhausted(now, m.maxStaleWindow) {
			// Check if static backup key exists for this kid
			if backupKey := m.lookupStaticBackup(kid, now); backupKey != nil {
				return backupKey.PublicKey, nil
			}
			return nil, domainauth.ErrJWKSStaleKeyExhausted
		}

		// Valid within 24h stale window -> serve stale key with zero downtime
		m.metrics.StaleHits.Inc()
		return key.PublicKey, nil
	}

	// Cache Miss: key not found in memory
	m.metrics.Misses.Inc()

	// Check static backup first
	if backupKey := m.lookupStaticBackup(kid, now); backupKey != nil {
		return backupKey.PublicKey, nil
	}

	// Synchronous on-demand refresh attempt with 2s timeout
	refreshCtx, cancel := context.WithTimeout(ctx, DefaultFetchTimeout)
	_ = m.RefreshKeys(refreshCtx)
	cancel()

	// Check RAM cache again post-refresh
	m.mu.RLock()
	postKey, postFound := m.keys[kid]
	m.mu.RUnlock()

	if postFound && !postKey.IsStaleExhausted(now, m.maxStaleWindow) {
		return postKey.PublicKey, nil
	}

	// Final check on static backup
	if backupKey := m.lookupStaticBackup(kid, now); backupKey != nil {
		return backupKey.PublicKey, nil
	}

	return nil, domainauth.ErrKeyNotFound
}

func (m *JWKSShieldManager) lookupStaticBackup(kid string, now time.Time) *domainauth.JWKSKey {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if b, ok := m.staticBackupKeys[kid]; ok {
		if !b.IsExpired(now) {
			return b
		}
	}
	return nil
}

func (m *JWKSShieldManager) triggerBackgroundRefresh() {
	if atomic.CompareAndSwapInt32(&m.refreshInProgress, 0, 1) {
		go func() {
			defer atomic.StoreInt32(&m.refreshInProgress, 0)
			bgCtx, cancel := context.WithTimeout(context.Background(), DefaultFetchTimeout)
			defer cancel()
			_ = m.RefreshKeys(bgCtx)
		}()
	}
}

// RefreshKeys synchronously fetches new keys from the IdP provider URL and updates the cache.
func (m *JWKSShieldManager) RefreshKeys(ctx context.Context) error {
	if m.fetcher == nil || m.providerURL == "" {
		return nil
	}

	_, err, _ := m.refreshGroup.Do(m.providerURL, func() (any, error) {
		keys, err := m.fetcher.FetchJWKS(ctx, m.providerURL)
		now := m.now()

		m.mu.Lock()
		defer m.mu.Unlock()

		if err != nil {
			m.metrics.RefreshErrors.Inc()
			m.lastFetchError = err.Error()
			return nil, err
		}

		// Update active cache without purging existing unexpired keys
		for _, k := range keys {
			m.keys[k.KID] = k
		}
		m.lastFetchError = ""
		m.lastFetchTime = now
		return nil, nil
	})
	return err
}

// GetCacheStatus reports cache status, stale key age, and atomic counters.
func (m *JWKSShieldManager) GetCacheStatus() domainauth.JWKSCacheStatus {
	now := m.now()

	m.mu.RLock()
	defer m.mu.RUnlock()

	totalKeys := len(m.keys)
	var isUsingStaleKey bool
	var maxStaleAge time.Duration

	for _, k := range m.keys {
		if k.IsExpired(now) {
			isUsingStaleKey = true
			age := now.Sub(k.ExpiresAt)
			if age > maxStaleAge {
				maxStaleAge = age
			}
		}
	}

	var humanAge string
	if isUsingStaleKey {
		humanAge = maxStaleAge.Round(time.Second).String()
	}

	return domainauth.JWKSCacheStatus{
		TotalKeys:        totalKeys,
		IsUsingStaleKey:  isUsingStaleKey,
		StaleKeyAge:      maxStaleAge,
		StaleKeyAgeHuman: humanAge,
		LastFetchError:   m.lastFetchError,
		ProviderURL:      m.providerURL,
		Hits:             m.metrics.Hits.Load(),
		Misses:           m.metrics.Misses.Load(),
		StaleHits:        m.metrics.StaleHits.Load(),
	}
}

// CalculateLeaseRenewal calculates renewal time using jitter formula:
// RenewTime = TTL * (0.60 + 0.30 * rand())
// Spreads renewals evenly across 60% to 90% TTL to eliminate thundering herd against Vault storage.
func (m *JWKSShieldManager) CalculateLeaseRenewal(ttl time.Duration) time.Duration {
	jitter := 0.60 + 0.30*randv2.Float64()
	return time.Duration(float64(ttl) * jitter)
}

// EvaluateVaultSealStatus checks seal duration and triggers TPM2/KMS unseal sequence if sealed > 60s.
func (m *JWKSShieldManager) EvaluateVaultSealStatus(
	ctx context.Context,
	status domainauth.VaultUnsealStatus,
	sealedDuration time.Duration,
) (domainauth.VaultUnsealStatus, error) {
	if !status.IsSealed {
		return status, nil
	}

	if sealedDuration > 60*time.Second {
		if m.vaultExecutor != nil {
			recovered, err := m.vaultExecutor.ExecuteUnseal(ctx)
			if err == nil && !recovered.IsSealed {
				return recovered, nil
			}
		}
		return status, domainauth.ErrVaultSealedDeadlock
	}

	return status, nil
}
