// Package netguard provides SSRF mitigation utilities including URL validation,
// IP range checks (blocking loopback, RFC1918, link-local, multicast, etc.),
// and dial-time socket interception to prevent DNS rebinding.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	// ErrBlockedAddress is returned when a destination IP address is blocked.
	ErrBlockedAddress = errors.New("netguard: destination address is blocked")
	// ErrInvalidScheme is returned when a URL scheme is not allowed.
	ErrInvalidScheme = errors.New("netguard: invalid or disallowed URL scheme")
	// ErrMissingHost is returned when a URL has no host.
	ErrMissingHost = errors.New("netguard: URL host cannot be empty")
)

var (
	// Reserved cloud metadata and special CIDRs
	metadataCIDR = mustParseCIDR("169.254.169.254/32")
	cgnatCIDR    = mustParseCIDR("100.64.0.0/10")
	zeroCIDR     = mustParseCIDR("0.0.0.0/8")
)

func mustParseCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(fmt.Sprintf("netguard: invalid cidr %s: %v", s, err))
	}
	return n
}

// Guard provides URL validation and network dial protection against SSRF.
type Guard struct {
	mu           sync.RWMutex
	allowedCIDRs []*net.IPNet
	allowedHosts map[string]struct{}
	allowPrivate bool
}

// Option configures a Guard.
type Option func(*Guard)

// WithAllowedCIDRs sets allowed IP networks.
func WithAllowedCIDRs(cidrs ...*net.IPNet) Option {
	return func(g *Guard) {
		g.allowedCIDRs = append(g.allowedCIDRs, cidrs...)
	}
}

// WithAllowedCIDRStrings parses CIDR strings and adds them to allowed networks.
func WithAllowedCIDRStrings(cidrStrs ...string) Option {
	return func(g *Guard) {
		for _, s := range cidrStrs {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if !strings.Contains(s, "/") {
				// Single IP, treat as /32 or /128
				if ip := net.ParseIP(s); ip != nil {
					if ip.To4() != nil {
						s += "/32"
					} else {
						s += "/128"
					}
				}
			}
			_, ipNet, err := net.ParseCIDR(s)
			if err == nil && ipNet != nil {
				g.allowedCIDRs = append(g.allowedCIDRs, ipNet)
			}
		}
	}
}

// WithAllowedHosts adds explicit allowed hostnames.
func WithAllowedHosts(hosts ...string) Option {
	return func(g *Guard) {
		for _, h := range hosts {
			h = strings.ToLower(strings.TrimSpace(h))
			if h != "" {
				g.allowedHosts[h] = struct{}{}
			}
		}
	}
}

// WithAllowPrivate configures whether private RFC1918 IPs are permitted.
func WithAllowPrivate(allow bool) Option {
	return func(g *Guard) {
		g.allowPrivate = allow
	}
}

// WithFromEnv loads allowed CIDRs and hosts from the EGRESS_ALLOW_PRIVATE_CIDRS environment variable.
func WithFromEnv() Option {
	return func(g *Guard) {
		val := os.Getenv("EGRESS_ALLOW_PRIVATE_CIDRS")
		if val == "" {
			return
		}
		parts := strings.Split(val, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if strings.Contains(part, "/") || net.ParseIP(part) != nil {
				WithAllowedCIDRStrings(part)(g)
			} else {
				WithAllowedHosts(part)(g)
			}
		}
	}
}

// New creates a new Guard with optional settings.
func New(opts ...Option) *Guard {
	g := &Guard{
		allowedHosts: make(map[string]struct{}),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(g)
		}
	}
	return g
}

// NewDefault returns a Guard initialized with environment variables (EGRESS_ALLOW_PRIVATE_CIDRS).
func NewDefault() *Guard {
	return New(WithFromEnv())
}

// IsHostAllowed returns true if the host is explicitly allowlisted.
func (g *Guard) IsHostAllowed(host string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	h := strings.ToLower(strings.TrimSpace(host))
	if idx := strings.IndexByte(h, ':'); idx != -1 {
		h = h[:idx]
	}
	_, ok := g.allowedHosts[h]
	return ok
}

// ValidateIP returns nil if ip is allowed, or ErrBlockedAddress if it is restricted.
func (g *Guard) ValidateIP(ip net.IP) error {
	if ip == nil {
		return ErrBlockedAddress
	}

	// 1. Check against explicit allowlist first
	g.mu.RLock()
	for _, netRange := range g.allowedCIDRs {
		if netRange.Contains(ip) {
			g.mu.RUnlock()
			return nil
		}
	}
	allowPrivate := g.allowPrivate
	g.mu.RUnlock()

	// Normalize IPv4-mapped IPv6 addresses (e.g., ::ffff:127.0.0.1)
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}

	// Always block loopback (127.0.0.0/8, ::1)
	if ip.IsLoopback() {
		return fmt.Errorf("%w: loopback address %s", ErrBlockedAddress, ip)
	}

	// Always block link-local unicast (169.254.0.0/16, fe80::/10)
	if ip.IsLinkLocalUnicast() || metadataCIDR.Contains(ip) {
		return fmt.Errorf("%w: link-local address %s", ErrBlockedAddress, ip)
	}

	// Always block link-local multicast & multicast (224.0.0.0/4, ff00::/8)
	if ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return fmt.Errorf("%w: multicast address %s", ErrBlockedAddress, ip)
	}

	// Always block unspecified addresses (0.0.0.0, ::) and 0.0.0.0/8
	if ip.IsUnspecified() || zeroCIDR.Contains(ip) {
		return fmt.Errorf("%w: unspecified address %s", ErrBlockedAddress, ip)
	}

	// Block CGNAT (100.64.0.0/10)
	if cgnatCIDR.Contains(ip) {
		return fmt.Errorf("%w: shared address space %s", ErrBlockedAddress, ip)
	}

	// Block private networks unless explicitly allowed
	if !allowPrivate {
		if ip.IsPrivate() {
			return fmt.Errorf("%w: private network address %s", ErrBlockedAddress, ip)
		}
		// Additional safety for unique local IPv6 (fc00::/7) in case stdlib IsPrivate behaves differently
		if len(ip) == net.IPv6len && (ip[0]&0xfe == 0xfc) {
			return fmt.Errorf("%w: unique local IPv6 address %s", ErrBlockedAddress, ip)
		}
	}

	return nil
}

// ValidateURL validates rawURL against allowed schemes (default: http, https).
func (g *Guard) ValidateURL(rawURL string, allowedSchemes ...string) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errors.New("empty URL")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parsing URL: %w", err)
	}

	if u.Scheme == "" {
		return nil, fmt.Errorf("%w: missing scheme", ErrInvalidScheme)
	}

	schemes := allowedSchemes
	if len(schemes) == 0 {
		schemes = []string{"http", "https"}
	}

	schemeAllowed := false
	for _, s := range schemes {
		if strings.EqualFold(u.Scheme, s) {
			schemeAllowed = true
			break
		}
	}
	if !schemeAllowed {
		return nil, fmt.Errorf("%w: %s (allowed: %v)", ErrInvalidScheme, u.Scheme, schemes)
	}

	if u.Hostname() == "" {
		return nil, ErrMissingHost
	}

	return u, nil
}

// Control returns a net.Dialer Control function hook that inspects dialed IPs.
func (g *Guard) Control() func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			host = address
		}
		ip := net.ParseIP(host)
		if ip != nil {
			return g.ValidateIP(ip)
		}
		return nil
	}
}

// DialContext validates resolved addresses and dials using a secured Dialer.
func (g *Guard) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("splitting host:port %s: %w", address, err)
	}

	// 1. If host is explicitly allowlisted, dial directly
	if g.IsHostAllowed(host) {
		d := &net.Dialer{Timeout: 10 * time.Second}
		return d.DialContext(ctx, network, address)
	}

	// 2. If host is an IP literal, validate immediately
	if ip := net.ParseIP(host); ip != nil {
		if err := g.ValidateIP(ip); err != nil {
			return nil, err
		}
	} else {
		// 3. Host is a hostname: resolve IPs ahead of dial to check for loopback/private
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("resolving host %s: %w", host, err)
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("no IP addresses found for host %s", host)
		}
		for _, ip := range ips {
			if err := g.ValidateIP(ip); err != nil {
				return nil, fmt.Errorf("host %s resolves to blocked address %s: %w", host, ip, err)
			}
		}
	}

	// 4. Dial with Control hook for socket-level dial-time enforcement (prevents DNS rebinding)
	dialer := g.NewDialer()
	return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
}

// NewDialer creates a net.Dialer configured with the Control hook.
func (g *Guard) NewDialer() *net.Dialer {
	return &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   g.Control(),
	}
}

// NewTransport creates an http.Transport configured with SSRF protection.
func (g *Guard) NewTransport() *http.Transport {
	return &http.Transport{
		DialContext:           g.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// NewClient creates an http.Client with SSRF-protected Transport and redirect validation.
func (g *Guard) NewClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: g.NewTransport(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			if _, err := g.ValidateURL(req.URL.String()); err != nil {
				return fmt.Errorf("redirect URL blocked: %w", err)
			}
			return nil
		},
	}
}

// ValidateURL is a package-level helper using the default guard.
func ValidateURL(rawURL string, allowedSchemes ...string) (*url.URL, error) {
	return NewDefault().ValidateURL(rawURL, allowedSchemes...)
}

// NewClient returns an http.Client configured with the given options.
func NewClient(timeout time.Duration, opts ...Option) *http.Client {
	return New(opts...).NewClient(timeout)
}

// NewDialer returns a net.Dialer configured with the given options.
func NewDialer(opts ...Option) *net.Dialer {
	return New(opts...).NewDialer()
}

// NewTransport returns an http.Transport configured with the given options.
func NewTransport(opts ...Option) *http.Transport {
	return New(opts...).NewTransport()
}
