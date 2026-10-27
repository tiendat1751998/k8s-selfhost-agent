package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestCert(t *testing.T, dir string) (string, string) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	notBefore := time.Now()
	notAfter := notBefore.Add(time.Hour)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test Agent"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	require.NoError(t, err)

	certPath := filepath.Join(dir, "agent.crt")
	certOut, err := os.Create(certPath)
	require.NoError(t, err)
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	keyPath := filepath.Join(dir, "agent.key")
	keyOut, err := os.Create(keyPath)
	require.NoError(t, err)
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	keyOut.Close()

	return certPath, keyPath
}

func TestGetAgentTLSConfig(t *testing.T) {
	origCert := os.Getenv("AGENT_TLS_CERT")
	origKey := os.Getenv("AGENT_TLS_KEY")
	defer func() {
		os.Setenv("AGENT_TLS_CERT", origCert)
		os.Setenv("AGENT_TLS_KEY", origKey)
	}()

	t.Run("Disabled when env vars empty", func(t *testing.T) {
		os.Unsetenv("AGENT_TLS_CERT")
		os.Unsetenv("AGENT_TLS_KEY")
		cfg := GetAgentTLSConfig()
		assert.False(t, cfg.Enabled)
		assert.Empty(t, cfg.CertFile)
		assert.Empty(t, cfg.KeyFile)
	})

	t.Run("Enabled when both cert and key are provided", func(t *testing.T) {
		os.Setenv("AGENT_TLS_CERT", "/etc/agent/cert.pem")
		os.Setenv("AGENT_TLS_KEY", "/etc/agent/key.pem")
		cfg := GetAgentTLSConfig()
		assert.True(t, cfg.Enabled)
		assert.Equal(t, "/etc/agent/cert.pem", cfg.CertFile)
		assert.Equal(t, "/etc/agent/key.pem", cfg.KeyFile)
	})
}

func TestServeWithOptionalTLS_MinTLSVersion(t *testing.T) {
	tempDir := t.TempDir()
	certFile, keyFile := generateTestCert(t, tempDir)

	tlsCfg := &AgentTLSConfig{
		CertFile: certFile,
		KeyFile:  keyFile,
		Enabled:  true,
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	listener.Close()

	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}

	go func() {
		_ = ServeWithOptionalTLS(srv, tlsCfg)
	}()
	defer srv.Close()

	// Wait briefly for server to bind
	time.Sleep(50 * time.Millisecond)

	assert.NotNil(t, srv.TLSConfig)
	assert.Equal(t, uint16(tls.VersionTLS12), srv.TLSConfig.MinVersion)
}
