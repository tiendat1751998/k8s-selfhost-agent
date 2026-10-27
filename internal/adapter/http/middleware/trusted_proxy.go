package middleware

import (
	"net"
	"net/http"
	"os"
	"strings"
)

// defaultTrustedCIDRs returns loopback CIDRs.
var defaultTrustedCIDRs = []string{"127.0.0.1/32", "::1/128"}

// ParseTrustedCIDRs parses a comma-separated list of CIDRs or IP addresses into []*net.IPNet.
func ParseTrustedCIDRs(raw string) []*net.IPNet {
	var list []string
	if strings.TrimSpace(raw) == "" {
		list = defaultTrustedCIDRs
	} else {
		list = strings.Split(raw, ",")
	}

	var parsed []*net.IPNet
	for _, item := range list {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if !strings.Contains(trimmed, "/") {
			ip := net.ParseIP(trimmed)
			if ip == nil {
				continue
			}
			if ip.To4() != nil {
				trimmed += "/32"
			} else {
				trimmed += "/128"
			}
		}
		_, ipNet, err := net.ParseCIDR(trimmed)
		if err == nil && ipNet != nil {
			parsed = append(parsed, ipNet)
		}
	}
	if len(parsed) == 0 {
		for _, def := range defaultTrustedCIDRs {
			_, ipNet, _ := net.ParseCIDR(def)
			if ipNet != nil {
				parsed = append(parsed, ipNet)
			}
		}
	}
	return parsed
}

// IsTrustedProxy checks whether the given IP is within any trusted CIDR.
func IsTrustedProxy(ip net.IP, trusted []*net.IPNet) bool {
	if ip == nil {
		return false
	}
	for _, cidr := range trusted {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// TrustedProxyRealIP creates a middleware that replaces r.RemoteAddr with the verified
// client IP only when the immediate TCP peer is in TRUSTED_PROXIES (default: 127.0.0.1/32, ::1/128).
// When immediate peer is untrusted, X-Forwarded-For and X-Real-IP headers are ignored.
func TrustedProxyRealIP(explicitCIDRs ...string) func(http.Handler) http.Handler {
	var raw string
	if len(explicitCIDRs) > 0 && strings.TrimSpace(strings.Join(explicitCIDRs, ",")) != "" {
		raw = strings.Join(explicitCIDRs, ",")
	}
	if raw == "" {
		raw = os.Getenv("TRUSTED_PROXIES")
	}
	trustedCIDRs := ParseTrustedCIDRs(raw)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peerHost, peerPort, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				peerHost = r.RemoteAddr
				peerPort = "0"
			}

			peerIP := net.ParseIP(peerHost)
			if peerIP != nil && IsTrustedProxy(peerIP, trustedCIDRs) {
				realIP := extractForwardedIP(r, trustedCIDRs)
				if realIP != "" {
					r.RemoteAddr = net.JoinHostPort(realIP, peerPort)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractForwardedIP extracts the true client IP from X-Forwarded-For or X-Real-IP.
func extractForwardedIP(r *http.Request, trustedCIDRs []*net.IPNet) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		// Walk backwards from right to left; the first untrusted IP is the client
		for i := len(ips) - 1; i >= 0; i-- {
			candidate := strings.TrimSpace(ips[i])
			if candidate == "" {
				continue
			}
			parsed := net.ParseIP(candidate)
			if parsed != nil {
				if !IsTrustedProxy(parsed, trustedCIDRs) {
					return candidate
				}
			}
		}
		// If all were trusted, pick the leftmost candidate
		for _, candidate := range ips {
			candidate = strings.TrimSpace(candidate)
			if net.ParseIP(candidate) != nil {
				return candidate
			}
		}
	}

	xri := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if xri != "" && net.ParseIP(xri) != nil {
		return xri
	}

	return ""
}
