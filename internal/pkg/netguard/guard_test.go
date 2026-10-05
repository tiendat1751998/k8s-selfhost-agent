package netguard_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/netguard"
)

func TestGuard_ValidateIP(t *testing.T) {
	guard := netguard.New()

	blockedIPs := []string{
		"127.0.0.1",
		"127.0.0.2",
		"::1",
		"10.0.0.1",
		"10.254.1.1",
		"172.16.0.1",
		"172.31.255.255",
		"192.168.0.1",
		"192.168.1.254",
		"169.254.169.254",
		"169.254.1.1",
		"fe80::1",
		"fc00::1",
		"fd12:3456:789a::1",
		"0.0.0.0",
		"::",
		"224.0.0.1",
		"ff02::1",
		"::ffff:127.0.0.1",
		"::ffff:10.0.0.1",
		"::ffff:169.254.169.254",
	}

	for _, ipStr := range blockedIPs {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			t.Fatalf("failed to parse test ip: %s", ipStr)
		}
		if err := guard.ValidateIP(ip); err == nil {
			t.Errorf("expected IP %s to be blocked, but was allowed", ipStr)
		}
	}

	allowedIPs := []string{
		"8.8.8.8",
		"1.1.1.1",
		"93.184.216.34",
		"2606:4700:4700::1111",
	}

	for _, ipStr := range allowedIPs {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			t.Fatalf("failed to parse test ip: %s", ipStr)
		}
		if err := guard.ValidateIP(ip); err != nil {
			t.Errorf("expected public IP %s to be allowed, got error: %v", ipStr, err)
		}
	}
}

func TestGuard_Allowlist(t *testing.T) {
	guard := netguard.New(
		netguard.WithAllowedCIDRStrings("10.0.0.0/8", "192.168.10.0/24"),
		netguard.WithAllowedHosts("custom-internal-host.local"),
	)

	// Allowed by CIDR
	if err := guard.ValidateIP(net.ParseIP("10.1.2.3")); err != nil {
		t.Errorf("expected 10.1.2.3 to be allowed by CIDR, got: %v", err)
	}
	if err := guard.ValidateIP(net.ParseIP("192.168.10.42")); err != nil {
		t.Errorf("expected 192.168.10.42 to be allowed by CIDR, got: %v", err)
	}

	// Still blocked
	if err := guard.ValidateIP(net.ParseIP("192.168.1.1")); err == nil {
		t.Errorf("expected 192.168.1.1 to be blocked")
	}
	if err := guard.ValidateIP(net.ParseIP("127.0.0.1")); err == nil {
		t.Errorf("expected 127.0.0.1 to be blocked")
	}

	// Host check
	if !guard.IsHostAllowed("custom-internal-host.local") {
		t.Errorf("expected custom-internal-host.local to be allowed")
	}
	if guard.IsHostAllowed("other.local") {
		t.Errorf("expected other.local not to be allowed")
	}
}

func TestGuard_ValidateURL(t *testing.T) {
	guard := netguard.New()

	// Scheme check
	validURLs := []string{
		"http://example.com/api",
		"https://example.com/webhook",
	}
	for _, u := range validURLs {
		if _, err := guard.ValidateURL(u); err != nil {
			t.Errorf("expected URL %s to be valid, got: %v", u, err)
		}
	}

	invalidURLs := []string{
		"ftp://example.com/file",
		"file:///etc/passwd",
		"gopher://example.com",
		"javascript:alert(1)",
		"://missing-scheme",
		"",
	}
	for _, u := range invalidURLs {
		if _, err := guard.ValidateURL(u); err == nil {
			t.Errorf("expected URL %q to fail validation", u)
		}
	}

	// HTTPS only requirement
	if _, err := guard.ValidateURL("http://example.com", "https"); err == nil {
		t.Errorf("expected http:// to fail when https is required")
	}
	if _, err := guard.ValidateURL("https://example.com", "https"); err != nil {
		t.Errorf("expected https:// to pass when https is required: %v", err)
	}
}

func TestGuard_Dialer_BlocksPrivateAndLoopback(t *testing.T) {
	guard := netguard.New()
	dialer := guard.NewDialer()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Direct dial to loopback
	_, err := dialer.DialContext(ctx, "tcp", "127.0.0.1:80")
	if err == nil {
		t.Errorf("expected dial to 127.0.0.1 to fail at dial time")
	}

	// Direct dial to link-local
	_, err = dialer.DialContext(ctx, "tcp", "169.254.169.254:80")
	if err == nil {
		t.Errorf("expected dial to 169.254.169.254 to fail at dial time")
	}

	// Dial localhost hostname (resolves to loopback)
	_, err = dialer.DialContext(ctx, "tcp", "localhost:80")
	if err == nil {
		t.Errorf("expected dial to localhost to fail via DNS resolution + dial-time check")
	}
}

func TestGuard_Dialer_AllowsAllowlistedCIDR(t *testing.T) {
	// Start local test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	// Guard with 127.0.0.0/8 allowlisted
	guard := netguard.New(netguard.WithAllowedCIDRStrings("127.0.0.0/8", "::1/128"))
	client := guard.NewClient(2 * time.Second)

	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("expected request to allowlisted loopback to succeed, got: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got: %d", resp.StatusCode)
	}
}

func TestGuard_FromEnv(t *testing.T) {
	t.Setenv("EGRESS_ALLOW_PRIVATE_CIDRS", "10.0.0.0/8, 172.20.0.0/16, host.internal")

	guard := netguard.New(netguard.WithFromEnv())

	if err := guard.ValidateIP(net.ParseIP("10.5.5.5")); err != nil {
		t.Errorf("expected 10.5.5.5 to be allowed via env CIDR, got: %v", err)
	}
	if err := guard.ValidateIP(net.ParseIP("172.20.1.1")); err != nil {
		t.Errorf("expected 172.20.1.1 to be allowed via env CIDR, got: %v", err)
	}
	if !guard.IsHostAllowed("host.internal") {
		t.Errorf("expected host.internal to be allowed via env host")
	}
	if err := guard.ValidateIP(net.ParseIP("192.168.1.1")); err == nil {
		t.Errorf("expected 192.168.1.1 to be blocked")
	}
}
