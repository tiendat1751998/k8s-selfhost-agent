package crypto

import (
	"bufio"
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ocsp"

	domaincrypto "github.com/datdt/k8sselfhost/internal/domain/crypto"
)

// ShieldManagerOptions holds configuration parameters for the Enterprise PKI & HSM Guard.
type ShieldManagerOptions struct {
	MaxHSMSessions     int
	HSMAcquireTimeout  time.Duration
	DEKDefaultTTL      time.Duration
	DEKGracePeriod     time.Duration
	OCSPPrefetchWindow time.Duration
	OCSPGraceWindow    time.Duration
	HTTPClient         *http.Client
	NowFunc            func() time.Time
}

// ShieldManager implements the EnterpriseCryptoShield domain interface.
type ShieldManager struct {
	opts ShieldManagerOptions

	// Cache-line aligned metrics to prevent false sharing under high concurrency
	metrics domaincrypto.CryptoMetrics

	// OCSP stapling cache
	ocspMu      sync.RWMutex
	ocspStaples map[string]*domaincrypto.OCSPStapleStatus

	// HSM bounded session pool
	hsmSem          chan struct{}
	hsmSessionSeq   uint64
	activeSessions  int64
	maxSessions     int
	hsmMu           sync.Mutex
	lastAcquireWait time.Duration

	// Envelope DEK Stale-While-Revalidate cache
	dekMu    sync.RWMutex
	dekCache map[string]*domaincrypto.EnvelopeCacheEntry
}

// NewShieldManager initializes the cryptographic shield manager with default or provided options.
func NewShieldManager(opts ShieldManagerOptions) *ShieldManager {
	if opts.MaxHSMSessions <= 0 {
		opts.MaxHSMSessions = 256
	}
	if opts.HSMAcquireTimeout <= 0 {
		opts.HSMAcquireTimeout = 500 * time.Millisecond
	}
	if opts.DEKDefaultTTL <= 0 {
		opts.DEKDefaultTTL = 1 * time.Hour
	}
	if opts.DEKGracePeriod <= 0 {
		opts.DEKGracePeriod = 4 * time.Hour
	}
	if opts.OCSPPrefetchWindow <= 0 {
		opts.OCSPPrefetchWindow = 4 * time.Hour
	}
	if opts.OCSPGraceWindow <= 0 {
		opts.OCSPGraceWindow = 24 * time.Hour
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	if opts.NowFunc == nil {
		opts.NowFunc = time.Now
	}

	return &ShieldManager{
		opts:        opts,
		ocspStaples: make(map[string]*domaincrypto.OCSPStapleStatus),
		hsmSem:      make(chan struct{}, opts.MaxHSMSessions),
		maxSessions: opts.MaxHSMSessions,
		dekCache:    make(map[string]*domaincrypto.EnvelopeCacheEntry),
	}
}

func (m *ShieldManager) now() time.Time {
	return m.opts.NowFunc()
}

// VerifyAndPrefetchOCSPStaple parses cert chain, verifies revocation, and prefetches signed OCSP staples.
func (m *ShieldManager) VerifyAndPrefetchOCSPStaple(ctx context.Context, certChainPEM []byte) (*domaincrypto.OCSPStapleStatus, error) {
	leafCert, issuerCert, err := parseCertChain(certChainPEM)
	if err != nil {
		return nil, fmt.Errorf("%w: malformed certificate chain PEM: %v", domaincrypto.ErrOCSPStapleFetchFailed, err)
	}

	domain := leafCert.Subject.CommonName
	if domain == "" && len(leafCert.DNSNames) > 0 {
		domain = leafCert.DNSNames[0]
	}

	now := m.now()

	// Check existing cached staple
	m.ocspMu.RLock()
	cached, hasCached := m.ocspStaples[domain]
	m.ocspMu.RUnlock()

	// If cached staple is still fresh and does not yet require prefetching, return immediately
	if hasCached && cached != nil && !cached.NeedsPrefetch(now, m.opts.OCSPPrefetchWindow) {
		return cached, nil
	}

	// Attempt OCSP fetch from CA responder
	if len(leafCert.OCSPServer) > 0 {
		responderURL := leafCert.OCSPServer[0]
		fetchedStatus, fetchErr := m.fetchOCSPFromResponder(ctx, leafCert, issuerCert, responderURL)
		if fetchErr == nil {
			m.metrics.OCSPPrefetches.Inc()
			m.ocspMu.Lock()
			m.ocspStaples[domain] = fetchedStatus
			m.ocspMu.Unlock()
			return fetchedStatus, nil
		}
	}

	// Fetch failed or no responder URL available: evaluate 24-hour grace cache retention
	if hasCached && cached != nil && cached.IsWithinGrace(now, m.opts.OCSPGraceWindow) {
		m.metrics.OCSPGraceHits.Inc()
		graceCopy := *cached
		graceCopy.IsGraceUsed = true
		graceCopy.IsExpired = now.After(cached.NextUpdate)
		return &graceCopy, nil
	}

	m.metrics.OCSPFailures.Inc()
	return nil, domaincrypto.ErrOCSPStapleFetchFailed
}

func (m *ShieldManager) fetchOCSPFromResponder(ctx context.Context, leaf, issuer *x509.Certificate, responderURL string) (*domaincrypto.OCSPStapleStatus, error) {
	reqDER, err := ocsp.CreateRequest(leaf, issuer, nil)
	if err != nil {
		return nil, fmt.Errorf("create ocsp request der: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, responderURL, bytes.NewReader(reqDER))
	if err != nil {
		return nil, fmt.Errorf("build http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/ocsp-request")
	httpReq.Header.Set("Accept", "application/ocsp-response")

	resp, err := m.opts.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ocsp http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ocsp responder returned non-200 status: %d", resp.StatusCode)
	}

	respDER, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ocsp response: %w", err)
	}

	ocspResp, err := ocsp.ParseResponse(respDER, issuer)
	if err != nil {
		return nil, fmt.Errorf("parse ocsp response: %w", err)
	}

	now := m.now()
	domain := leaf.Subject.CommonName
	if domain == "" && len(leaf.DNSNames) > 0 {
		domain = leaf.DNSNames[0]
	}

	var revStatus string
	switch ocspResp.Status {
	case ocsp.Good:
		revStatus = "good"
	case ocsp.Revoked:
		revStatus = "revoked"
	default:
		revStatus = "unknown"
	}

	status := &domaincrypto.OCSPStapleStatus{
		Domain:           domain,
		HasStaple:        true,
		NextUpdate:       ocspResp.NextUpdate,
		IsExpired:        now.After(ocspResp.NextUpdate),
		ResponderURL:     responderURL,
		RevocationStatus: revStatus,
		ProducedAt:       ocspResp.ProducedAt,
		GraceValidUntil:  ocspResp.NextUpdate.Add(m.opts.OCSPGraceWindow),
		IsGraceUsed:      false,
	}

	return status, nil
}

// AcquireHSMSession reserves a hardware session from bounded pool with fast-fail timeout.
func (m *ShieldManager) AcquireHSMSession(ctx context.Context, timeout time.Duration) (uint64, func(), error) {
	if timeout <= 0 {
		timeout = m.opts.HSMAcquireTimeout
	}

	start := m.now()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case m.hsmSem <- struct{}{}:
		waitDur := m.now().Sub(start)
		m.hsmMu.Lock()
		m.lastAcquireWait = waitDur
		m.hsmMu.Unlock()

		atomic.AddInt64(&m.activeSessions, 1)
		m.metrics.HSMAcquisitions.Inc()
		sid := atomic.AddUint64(&m.hsmSessionSeq, 1)

		var once sync.Once
		release := func() {
			once.Do(func() {
				select {
				case <-m.hsmSem:
					atomic.AddInt64(&m.activeSessions, -1)
					m.metrics.HSMReleases.Inc()
				default:
				}
			})
		}
		return sid, release, nil

	case <-timer.C:
		m.metrics.HSMSaturations.Inc()
		return 0, nil, domaincrypto.ErrHSMPoolSaturated

	case <-ctx.Done():
		return 0, nil, ctx.Err()
	}
}

// GetHSMSessionPoolStatus audits current saturation metrics of the HSM session pool.
func (m *ShieldManager) GetHSMSessionPoolStatus() domaincrypto.HSMSessionPoolStatus {
	active := int(atomic.LoadInt64(&m.activeSessions))
	if active < 0 {
		active = 0
	}
	avail := m.maxSessions - active
	if avail < 0 {
		avail = 0
	}

	m.hsmMu.Lock()
	wait := m.lastAcquireWait
	m.hsmMu.Unlock()

	return domaincrypto.HSMSessionPoolStatus{
		MaxSessions:       m.maxSessions,
		ActiveSessions:    active,
		AvailableSessions: avail,
		AcquireWaitTime:   wait,
		IsSaturated:       active >= m.maxSessions,
	}
}

// GetOrExtendEnvelopeDEK retrieves or extends a DEK with stale-while-revalidate 4-hour grace.
func (m *ShieldManager) GetOrExtendEnvelopeDEK(ctx context.Context, keyID string, kmsFallbackFunc func() ([]byte, error)) ([]byte, error) {
	now := m.now()

	// 1. Fast read from local in-memory cache
	m.dekMu.RLock()
	cached, exists := m.dekCache[keyID]
	m.dekMu.RUnlock()

	if exists && cached != nil && cached.IsFresh(now) {
		m.metrics.DEKCacheHits.Inc()
		return cached.PlaintextKey, nil
	}

	// 2. Cache miss or expired: attempt Cloud KMS refresh
	if kmsFallbackFunc != nil {
		newKey, kmsErr := kmsFallbackFunc()
		if kmsErr == nil && len(newKey) > 0 {
			m.metrics.DEKKMSFetches.Inc()
			entry := &domaincrypto.EnvelopeCacheEntry{
				KeyID:        keyID,
				PlaintextKey: newKey,
				CachedAt:     now,
				TTL:          m.opts.DEKDefaultTTL,
				GracePeriod:  m.opts.DEKGracePeriod,
				IsStaleValid: false,
			}
			m.dekMu.Lock()
			m.dekCache[keyID] = entry
			m.dekMu.Unlock()
			return newKey, nil
		}

		// Cloud KMS returned error: inspect 4-hour stale grace window
		if exists && cached != nil {
			if cached.IsWithinGrace(now) {
				m.metrics.DEKStaleHits.Inc()
				return cached.PlaintextKey, nil
			}
			// Lapsed past 4-hour grace window
			m.metrics.DEKGraceExpired.Inc()
			return nil, domaincrypto.ErrKMSDEKGraceExpired
		}

		// No cached entry exists and KMS failed
		return nil, kmsErr
	}

	if exists && cached != nil && cached.IsWithinGrace(now) {
		m.metrics.DEKStaleHits.Inc()
		return cached.PlaintextKey, nil
	}

	return nil, domaincrypto.ErrKMSDEKGraceExpired
}

// ScanSourceForNonFIPSAlgorithms walks source directory and detects banned cryptographic algorithms.
func (m *ShieldManager) ScanSourceForNonFIPSAlgorithms(ctx context.Context, rootDir string) ([]string, error) {
	var violations []string

	bannedPatterns := []struct {
		algo    string
		pattern string
	}{
		{algo: "MD5", pattern: "crypto/md5"},
		{algo: "MD5", pattern: "md5.New"},
		{algo: "MD5", pattern: "md5.Sum"},
		{algo: "SHA-1", pattern: "crypto/sha1"},
		{algo: "SHA-1", pattern: "sha1.New"},
		{algo: "SHA-1", pattern: "sha1.Sum"},
		{algo: "DES", pattern: "crypto/des"},
		{algo: "DES", pattern: "des.NewCipher"},
		{algo: "DES", pattern: "des.NewTripleDESCipher"},
		{algo: "RC4", pattern: "crypto/rc4"},
	}

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".go" && ext != ".c" && ext != ".cpp" && ext != ".py" && ext != ".js" && ext != ".ts" {
			return nil
		}

		file, openErr := os.Open(path)
		if openErr != nil {
			return openErr
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			// Skip comments mentioning FIPS rules
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
				continue
			}

			for _, bp := range bannedPatterns {
				if strings.Contains(line, bp.pattern) {
					relPath, relErr := filepath.Rel(rootDir, path)
					if relErr != nil {
						relPath = path
					}
					violation := fmt.Sprintf("%s:%d: Non-FIPS algorithm '%s' detected (%s)", relPath, lineNum, bp.algo, bp.pattern)
					violations = append(violations, violation)
				}
			}
		}
		return scanner.Err()
	})

	if err != nil {
		return violations, err
	}

	if len(violations) > 0 {
		m.metrics.FIPSViolations.Add(uint64(len(violations)))
		return violations, domaincrypto.ErrFIPSNonCompliantHash
	}

	return violations, nil
}

// GetMetrics returns a pointer to the cache-line isolated metrics struct.
func (m *ShieldManager) GetMetrics() *domaincrypto.CryptoMetrics {
	return &m.metrics
}

// parseCertChain extracts the leaf certificate and issuer certificate from PEM encoded bytes.
func parseCertChain(certChainPEM []byte) (*x509.Certificate, *x509.Certificate, error) {
	var certs []*x509.Certificate
	rest := certChainPEM

	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("parse certificate: %w", err)
		}
		certs = append(certs, cert)
	}

	if len(certs) == 0 {
		return nil, nil, fmt.Errorf("no certificate blocks found in PEM")
	}

	leaf := certs[0]
	var issuer *x509.Certificate
	if len(certs) > 1 {
		issuer = certs[1]
	} else {
		// Self-signed or single certificate in chain
		issuer = leaf
	}

	return leaf, issuer, nil
}
