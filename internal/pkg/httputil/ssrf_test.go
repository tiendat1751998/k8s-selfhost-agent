package httputil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestValidateExternalURL_BlocksLoopback(t *testing.T) {
	cases := []string{
		"http://127.0.0.1",
		"http://127.0.0.1:8080/test",
		"http://127.0.1.1",
		"http://localhost",
		"http://localhost:3000",
		"http://[::1]",
		"http://[::1]:8080",
	}

	for _, url := range cases {
		err := ValidateExternalURL(url)
		if err == nil {
			t.Errorf("expected error for loopback URL %s, got nil", url)
		}
	}
}

func TestValidateExternalURL_BlocksPrivateRanges(t *testing.T) {
	cases := []string{
		"http://10.0.0.1",
		"http://10.255.255.255:8080",
		"http://172.16.0.1",
		"http://172.31.255.255/path",
		"http://192.168.0.1",
		"http://192.168.1.100:9000",
	}

	for _, url := range cases {
		err := ValidateExternalURL(url)
		if err == nil {
			t.Errorf("expected error for private range URL %s, got nil", url)
		}
	}
}

func TestValidateExternalURL_BlocksMetadata169(t *testing.T) {
	cases := []string{
		"http://169.254.169.254/latest/meta-data",
		"http://169.254.0.1",
		"http://169.254.1.1:8080",
	}

	for _, url := range cases {
		err := ValidateExternalURL(url)
		if err == nil {
			t.Errorf("expected error for link-local metadata URL %s, got nil", url)
		}
	}
}

func TestValidateExternalURL_AllowsPublicIP(t *testing.T) {
	cases := []string{
		"http://8.8.8.8",
		"https://1.1.1.1",
		"https://9.9.9.9/dns-query",
	}

	for _, url := range cases {
		err := ValidateExternalURL(url)
		if err != nil {
			t.Errorf("expected public URL %s to be allowed, got error: %v", url, err)
		}
	}
}

func TestValidateExternalURL_BlocksNonHTTPScheme(t *testing.T) {
	cases := []string{
		"ftp://8.8.8.8/file",
		"file:///etc/passwd",
		"gopher://127.0.0.1",
		"javascript:alert(1)",
		"ssh://git@github.com",
		"",
	}

	for _, url := range cases {
		err := ValidateExternalURL(url)
		if err == nil {
			t.Errorf("expected error for non-http(s) or empty URL %s, got nil", url)
		}
	}
}

func TestSafeHTTPClient_BlocksLoopbackAndPrivate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewSafeHTTPClient(2 * time.Second)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	_, err = client.Do(req)
	if err == nil {
		t.Fatal("expected request to local test server to be blocked by SSRF protection, but it succeeded")
	}
}
