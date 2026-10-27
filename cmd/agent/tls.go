package main

import (
	"crypto/tls"
	"net/http"
	"os"
	"strings"
)

// AgentTLSConfig holds TLS certificate and key paths for the agent HTTP server.
type AgentTLSConfig struct {
	CertFile string
	KeyFile  string
	Enabled  bool
}

// GetAgentTLSConfig reads AGENT_TLS_CERT and AGENT_TLS_KEY environment variables.
func GetAgentTLSConfig() *AgentTLSConfig {
	cert := strings.TrimSpace(os.Getenv("AGENT_TLS_CERT"))
	key := strings.TrimSpace(os.Getenv("AGENT_TLS_KEY"))
	return &AgentTLSConfig{
		CertFile: cert,
		KeyFile:  key,
		Enabled:  cert != "" && key != "",
	}
}

// ServeWithOptionalTLS runs srv.ListenAndServeTLS if tlsCfg is configured with cert and key,
// enforcing tls.VersionTLS12 as the minimum TLS version. Otherwise runs srv.ListenAndServe().
func ServeWithOptionalTLS(srv *http.Server, tlsCfg *AgentTLSConfig) error {
	if srv == nil {
		return http.ErrServerClosed
	}
	if tlsCfg != nil && tlsCfg.Enabled {
		if srv.TLSConfig == nil {
			srv.TLSConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
		} else if srv.TLSConfig.MinVersion < tls.VersionTLS12 {
			srv.TLSConfig.MinVersion = tls.VersionTLS12
		}
		return srv.ListenAndServeTLS(tlsCfg.CertFile, tlsCfg.KeyFile)
	}
	return srv.ListenAndServe()
}
