package httputil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

var blockedCIDRs = []string{
	"127.0.0.0/8",    // loopback
	"10.0.0.0/8",     // private
	"172.16.0.0/12",  // private
	"192.168.0.0/16", // private
	"169.254.0.0/16", // link-local (AWS metadata)
	"::1/128",        // IPv6 loopback
	"fc00::/7",       // IPv6 private
	"fe80::/10",      // IPv6 link-local
}

var parsedBlockedCIDRs []*net.IPNet

func init() {
	for _, cidr := range blockedCIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			parsedBlockedCIDRs = append(parsedBlockedCIDRs, ipNet)
		}
	}
}

// isBlockedIP checks if an IP is in the blocked CIDRs or is loopback/private/link-local/unspecified.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsPrivate() {
		return true
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		for _, block := range parsedBlockedCIDRs {
			if block.Contains(ipv4) {
				return true
			}
		}
	}
	for _, block := range parsedBlockedCIDRs {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// NewSafeHTTPClient returns an *http.Client with a custom DialContext that blocks connections to private and loopback IPs.
func NewSafeHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address %s: %w", addr, err)
			}

			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, fmt.Errorf("resolving host %s: %w", host, err)
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("no IP addresses resolved for host %s", host)
			}

			for _, ip := range ips {
				if isBlockedIP(ip) {
					return nil, fmt.Errorf("access to IP %s is blocked (SSRF protection)", ip.String())
				}
			}

			targetAddr := net.JoinHostPort(ips[0].String(), port)
			return dialer.DialContext(ctx, network, targetAddr)
		},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

// ValidateExternalURL parses the URL, validates its scheme (http/https), resolves its hostname, and checks resolved IPs against blocked ranges.
func ValidateExternalURL(rawURL string) error {
	if rawURL == "" {
		return errors.New("url cannot be empty")
	}

	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q, only http and https are allowed", parsed.Scheme)
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return errors.New("url hostname is empty")
	}

	if ip := net.ParseIP(hostname); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("url resolves to blocked IP %s", ip.String())
		}
		return nil
	}

	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname %s: %w", hostname, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("no IP addresses resolved for hostname %s", hostname)
	}

	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("url resolves to blocked IP %s", ip.String())
		}
	}

	return nil
}
