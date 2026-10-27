package http

import (
	"strings"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/usecase/metrics"
)

// IsSensitiveHostLabelKey checks if a label key corresponds to sensitive credential material.
// Matches auth_token, token, secret, password, api_key (case-insensitive) or any key containing them.
func IsSensitiveHostLabelKey(key string) bool {
	lk := strings.ToLower(strings.TrimSpace(key))
	if lk == "" {
		return false
	}
	sensitiveSubstrings := []string{
		"auth_token", "token", "secret", "password", "api_key", "apikey", "credential",
	}
	for _, s := range sensitiveSubstrings {
		if strings.Contains(lk, s) {
			return true
		}
	}
	return false
}

// SanitizeHostLabels returns a cloned label map where all sensitive keys are masked as "***REDACTED***".
func SanitizeHostLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return nil
	}
	sanitized := make(map[string]string, len(labels))
	for k, v := range labels {
		if IsSensitiveHostLabelKey(k) && strings.TrimSpace(v) != "" {
			sanitized[k] = "***REDACTED***"
		} else {
			sanitized[k] = v
		}
	}
	return sanitized
}

// SanitizeComputeHost returns a clone of the host with sensitive labels masked and TLSKey cleared.
// The original object is not modified.
func SanitizeComputeHost(h *docker.ComputeHost) *docker.ComputeHost {
	if h == nil {
		return nil
	}
	clone := *h
	clone.TLSKey = ""
	clone.Labels = SanitizeHostLabels(h.Labels)
	cleanIP := metrics.ExtractCleanHostIP(clone.IP, clone.IPAddress, clone.Address, clone.Endpoint)
	if cleanIP != "" {
		if clone.IP == "" {
			clone.IP = cleanIP
		}
		if clone.IPAddress == "" {
			clone.IPAddress = cleanIP
		}
		if clone.Address == "" || strings.HasPrefix(clone.Address, "http://") || strings.HasPrefix(clone.Address, "https://") || strings.Contains(clone.Address, ":") {
			clone.Address = cleanIP
		}
	}
	return &clone
}

// SanitizeComputeHosts returns a slice of sanitized ComputeHost copies.
func SanitizeComputeHosts(hosts []docker.ComputeHost) []docker.ComputeHost {
	if hosts == nil {
		return nil
	}
	sanitized := make([]docker.ComputeHost, len(hosts))
	for i := range hosts {
		h := SanitizeComputeHost(&hosts[i])
		sanitized[i] = *h
	}
	return sanitized
}
