package crypto_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ocsp"

	domaincrypto "github.com/datdt/k8sselfhost/internal/domain/crypto"
	usecasecrypto "github.com/datdt/k8sselfhost/internal/usecase/crypto"
)

// Helper to generate a test CA certificate and leaf certificate with AIA OCSP URL
func generateTestCertChain(t *testing.T, ocspURL string) (caPEM, leafPEM []byte, caCert *x509.Certificate, caKey *rsa.PrivateKey) {
	t.Helper()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate CA key: %v", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Enterprise Root CA",
			Organization: []string{"Enterprise PKI"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * 365 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create CA cert: %v", err)
	}

	caCert, err = x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("failed to parse CA cert: %v", err)
	}

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate leaf key: %v", err)
	}

	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "vault.internal.enterprise",
		},
		DNSNames:     []string{"vault.internal.enterprise"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(30 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		OCSPServer:   []string{ocspURL},
	}

	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create leaf cert: %v", err)
	}

	leafPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	caPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	return caPEM, leafPEM, caCert, caKey
}

func TestShieldManager_OCSPPrefetchAndGraceRetention(t *testing.T) {
	var ocspStatusCode = http.StatusOK
	var nextUpdateDelta = 2 * time.Hour // within 4h prefetch window

	var responderServer *httptest.Server
	var caCert *x509.Certificate
	var caKey *rsa.PrivateKey

	responderServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ocspStatusCode != http.StatusOK {
			w.WriteHeader(ocspStatusCode)
			return
		}

		respTemplate := ocsp.Response{
			Status:       ocsp.Good,
			SerialNumber: big.NewInt(2),
			ThisUpdate:   time.Now().Add(-1 * time.Hour),
			NextUpdate:   time.Now().Add(nextUpdateDelta),
		}
		respDER, err := ocsp.CreateResponse(caCert, caCert, respTemplate, caKey)
		if err != nil {
			t.Errorf("failed to create OCSP response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/ocsp-response")
		w.WriteHeader(http.StatusOK)
		w.Write(respDER)
	}))
	defer responderServer.Close()

	caPEM, leafPEM, parsedCA, parsedKey := generateTestCertChain(t, responderServer.URL)
	caCert = parsedCA
	caKey = parsedKey

	fullChainPEM := append(leafPEM, caPEM...)

	now := time.Now()
	manager := usecasecrypto.NewShieldManager(usecasecrypto.ShieldManagerOptions{
		HTTPClient:          responderServer.Client(),
		OCSPPrefetchWindow:  4 * time.Hour,
		OCSPGraceWindow:     24 * time.Hour,
		NowFunc:             func() time.Time { return now },
	})

	ctx := context.Background()

	// 1. Initial prefetch succeeds
	status, err := manager.VerifyAndPrefetchOCSPStaple(ctx, fullChainPEM)
	if err != nil {
		t.Fatalf("expected successful staple prefetch, got: %v", err)
	}
	if !status.HasStaple || status.IsExpired {
		t.Fatalf("expected valid staple, got HasStaple=%v, IsExpired=%v", status.HasStaple, status.IsExpired)
	}
	if status.Domain != "vault.internal.enterprise" {
		t.Errorf("expected domain vault.internal.enterprise, got %s", status.Domain)
	}

	// 2. CA server encounters blackout / returns 502
	ocspStatusCode = http.StatusBadGateway

	// Time advances into staple expiration (1 hour after NextUpdate, so within 24h grace window)
	now = now.Add(3 * time.Hour)
	graceStatus, err := manager.VerifyAndPrefetchOCSPStaple(ctx, fullChainPEM)
	if err != nil {
		t.Fatalf("expected staple to be served from 24h grace window during CA blackout, got: %v", err)
	}
	if !graceStatus.IsGraceUsed {
		t.Errorf("expected IsGraceUsed=true during CA responder outage within grace window")
	}

	// 3. Time advances past 24h grace window (26 hours after NextUpdate)
	now = now.Add(25 * time.Hour)
	_, err = manager.VerifyAndPrefetchOCSPStaple(ctx, fullChainPEM)
	if !errors.Is(err, domaincrypto.ErrOCSPStapleFetchFailed) {
		t.Fatalf("expected ErrOCSPStapleFetchFailed after grace window exhaustion, got: %v", err)
	}
}

func TestShieldManager_HSMSessionPool_SaturationFastFail(t *testing.T) {
	const maxSessions = 5 // smaller pool for unit testing
	manager := usecasecrypto.NewShieldManager(usecasecrypto.ShieldManagerOptions{
		MaxHSMSessions:     maxSessions,
		HSMAcquireTimeout: 50 * time.Millisecond,
	})

	ctx := context.Background()

	// Exhaust all sessions
	releases := make([]func(), 0, maxSessions)
	for i := 0; i < maxSessions; i++ {
		sid, release, err := manager.AcquireHSMSession(ctx, 50*time.Millisecond)
		if err != nil {
			t.Fatalf("failed to acquire session %d: %v", i, err)
		}
		if sid == 0 {
			t.Fatalf("expected non-zero session ID")
		}
		releases = append(releases, release)
	}

	poolStatus := manager.GetHSMSessionPoolStatus()
	if !poolStatus.IsSaturated || poolStatus.AvailableSessions != 0 {
		t.Fatalf("expected pool to be saturated, got %+v", poolStatus)
	}

	// Attempting to acquire one more session must fast-fail in 50ms with ErrHSMPoolSaturated
	start := time.Now()
	_, _, err := manager.AcquireHSMSession(ctx, 50*time.Millisecond)
	elapsed := time.Since(start)
	if !errors.Is(err, domaincrypto.ErrHSMPoolSaturated) {
		t.Fatalf("expected ErrHSMPoolSaturated on saturation, got: %v", err)
	}
	if elapsed < 40*time.Millisecond || elapsed > 250*time.Millisecond {
		t.Errorf("expected acquire to fast-fail in ~50ms, took %v", elapsed)
	}

	// Release one session
	releases[0]()
	// Subsequent release call must be idempotent
	releases[0]()

	poolStatus = manager.GetHSMSessionPoolStatus()
	if poolStatus.AvailableSessions != 1 {
		t.Fatalf("expected 1 available session after release, got %d", poolStatus.AvailableSessions)
	}

	// Now acquisition succeeds
	sid, rel, err := manager.AcquireHSMSession(ctx, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("expected successful acquisition after release, got: %v", err)
	}
	defer rel()
	if sid == 0 {
		t.Fatalf("expected valid session ID")
	}

	// Clean up remaining sessions
	for _, r := range releases[1:] {
		r()
	}
}

func TestShieldManager_EnvelopeDEK_StaleWhileRevalidateGrace(t *testing.T) {
	now := time.Now()
	manager := usecasecrypto.NewShieldManager(usecasecrypto.ShieldManagerOptions{
		DEKDefaultTTL: 1 * time.Hour,
		DEKGracePeriod: 4 * time.Hour,
		NowFunc:        func() time.Time { return now },
	})

	ctx := context.Background()
	keyID := "dek-app-payment-01"
	expectedPlaintext := []byte("secret-dek-32-byte-key-payload01")

	kmsCalls := 0
	kmsFallback := func() ([]byte, error) {
		kmsCalls++
		return expectedPlaintext, nil
	}

	// 1. Initial fetch from KMS succeeds
	key, err := manager.GetOrExtendEnvelopeDEK(ctx, keyID, kmsFallback)
	if err != nil {
		t.Fatalf("unexpected error fetching DEK: %v", err)
	}
	if !bytes.Equal(key, expectedPlaintext) {
		t.Fatalf("key mismatch")
	}
	if kmsCalls != 1 {
		t.Fatalf("expected 1 KMS call, got %d", kmsCalls)
	}

	// 2. Fetch within 1h TTL hits in-memory cache directly without calling KMS
	now = now.Add(30 * time.Minute)
	key, err = manager.GetOrExtendEnvelopeDEK(ctx, keyID, kmsFallback)
	if err != nil {
		t.Fatalf("unexpected error fetching cached DEK: %v", err)
	}
	if kmsCalls != 1 {
		t.Fatalf("expected KMS not called on cache hit, got %d calls", kmsCalls)
	}

	// 3. TTL expires (1.5 hours later), and Cloud KMS has an outage returning 503
	now = now.Add(1 * time.Hour) // now is 1.5h after initial fetch
	failingKMSCalls := 0
	failingKMSFallback := func() ([]byte, error) {
		failingKMSCalls++
		return nil, errors.New("kms 503 Service Unavailable: rate limit exceeded")
	}

	// Because 1.5h is within 1h TTL + 4h Grace (up to 5h total), should return stale DEK with IsStaleValid=true
	staleKey, err := manager.GetOrExtendEnvelopeDEK(ctx, keyID, failingKMSFallback)
	if err != nil {
		t.Fatalf("expected stale DEK within 4h grace window during KMS outage, got: %v", err)
	}
	if !bytes.Equal(staleKey, expectedPlaintext) {
		t.Fatalf("stale key payload mismatch")
	}
	if failingKMSCalls != 1 {
		t.Fatalf("expected KMS fallback to be attempted once, got %d", failingKMSCalls)
	}

	// 4. Advance time past grace window (6 hours after initial fetch > 1h TTL + 4h Grace)
	now = now.Add(4 * time.Hour)
	_, err = manager.GetOrExtendEnvelopeDEK(ctx, keyID, failingKMSFallback)
	if !errors.Is(err, domaincrypto.ErrKMSDEKGraceExpired) {
		t.Fatalf("expected ErrKMSDEKGraceExpired when grace window lapsed, got: %v", err)
	}
}

func TestShieldManager_ScanSourceForNonFIPSAlgorithms(t *testing.T) {
	tempDir := t.TempDir()

	// Compliant file
	compliantFile := filepath.Join(tempDir, "safe.go")
	safeContent := []byte(`package safe
import "crypto/sha256"
import "crypto/aes"

func Hash(data []byte) [32]byte {
	return sha256.Sum256(data)
}
`)
	if err := os.WriteFile(compliantFile, safeContent, 0600); err != nil {
		t.Fatalf("failed to write safe file: %v", err)
	}

	manager := usecasecrypto.NewShieldManager(usecasecrypto.ShieldManagerOptions{})
	ctx := context.Background()

	violations, err := manager.ScanSourceForNonFIPSAlgorithms(ctx, tempDir)
	if err != nil {
		t.Fatalf("expected clean scan on compliant source, got: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(violations))
	}

	// Add non-FIPS file with MD5, SHA1, and DES
	badFile := filepath.Join(tempDir, "legacy_crypto.go")
	badContent := []byte(`package legacy
import "crypto/md5"
import "crypto/sha1"
import "crypto/des"

func BadHashes() {
	_ = md5.New()
	_ = sha1.New()
	_, _ = des.NewCipher([]byte("12345678"))
}
`)
	if err := os.WriteFile(badFile, badContent, 0600); err != nil {
		t.Fatalf("failed to write bad file: %v", err)
	}

	violations, err = manager.ScanSourceForNonFIPSAlgorithms(ctx, tempDir)
	if !errors.Is(err, domaincrypto.ErrFIPSNonCompliantHash) {
		t.Fatalf("expected ErrFIPSNonCompliantHash, got: %v", err)
	}
	if len(violations) < 3 {
		t.Fatalf("expected at least 3 violations (MD5, SHA1, DES), got %d: %v", len(violations), violations)
	}
}

func TestShieldManager_HighConcurrencyPaddedCounters(t *testing.T) {
	manager := usecasecrypto.NewShieldManager(usecasecrypto.ShieldManagerOptions{
		MaxHSMSessions: 256,
	})

	ctx := context.Background()
	const workers = 32
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Concurrently acquire and release HSM session
				sid, release, err := manager.AcquireHSMSession(ctx, 100*time.Millisecond)
				if err == nil {
					if sid == 0 {
						t.Errorf("zero session ID returned")
					}
					release()
				}

				// Concurrently access DEK cache
				keyID := "concurrent-dek-test"
				_, _ = manager.GetOrExtendEnvelopeDEK(ctx, keyID, func() ([]byte, error) {
					return []byte("test-dek-key-32-byte-buffer00000"), nil
				})
			}
		}(i)
	}

	wg.Wait()

	metrics := manager.GetMetrics()
	if metrics.HSMAcquisitions.Load() == 0 {
		t.Errorf("expected non-zero HSM acquisitions in metrics")
	}
	if metrics.DEKCacheHits.Load()+metrics.DEKKMSFetches.Load() == 0 {
		t.Errorf("expected non-zero DEK operations in metrics")
	}
}
