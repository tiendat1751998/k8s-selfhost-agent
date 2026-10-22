package http

import (
	"errors"
	"net/http/httptest"
	"testing"
)

func TestIsK8sUnavailable_ClientManagerErrors(t *testing.T) {
	testCases := []struct {
		errText  string
		expected bool
	}{
		{"clusterID cannot be empty", true},
		{"error decrypting cluster kubeconfig token", true},
		{"failed parsing kubeconfig yaml", true},
		{"insecure tls rejected for endpoint", true},
		{"host is not kubernetes api server", true},
		{"invalid configuration for cluster context", true},
		{"fleet repository not initialized", true},
		{"kubernetes cluster \"default\" is not connected or configured", true},
		{"some random database query error", false},
	}

	for _, tc := range testCases {
		t.Run(tc.errText, func(t *testing.T) {
			got := isK8sUnavailable(errors.New(tc.errText))
			if got != tc.expected {
				t.Errorf("isK8sUnavailable(%q) = %v, expected %v", tc.errText, got, tc.expected)
			}
		})
	}
}

func TestHelmHandler_ListReleases_FallbackCluster(t *testing.T) {
	// When releaseManager is nil, should return 503
	hNil := NewHelmHandler(nil, nil)
	wNil := httptest.NewRecorder()
	reqNil := httptest.NewRequest("GET", "/releases", nil)
	hNil.ListReleases(wNil, reqNil)
	if wNil.Code != 503 {
		t.Fatalf("expected 503 for nil release manager, got %d", wNil.Code)
	}
}

