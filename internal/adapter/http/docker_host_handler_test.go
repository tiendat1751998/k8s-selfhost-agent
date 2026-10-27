package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

func TestIsSensitiveHostLabelKey(t *testing.T) {
	testCases := []struct {
		key      string
		expected bool
	}{
		{"auth_token", true},
		{"AUTH_TOKEN", true},
		{"token", true},
		{"Token", true},
		{"secret", true},
		{"client_secret", true},
		{"password", true},
		{"db_password", true},
		{"api_key", true},
		{"custom_apikey", true},
		{"user_credential", true},
		{"hostname", false},
		{"os", false},
		{"arch", false},
		{"env", false},
		{"", false},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expected, IsSensitiveHostLabelKey(tc.key), "key: %s", tc.key)
	}
}

func TestSanitizeHostLabels(t *testing.T) {
	orig := map[string]string{
		"auth_token": "super-secret-1",
		"password":   "p@ssword",
		"hostname":   "node-1",
		"env":        "production",
	}

	sanitized := SanitizeHostLabels(orig)
	require.NotNil(t, sanitized)

	assert.Equal(t, "***REDACTED***", sanitized["auth_token"])
	assert.Equal(t, "***REDACTED***", sanitized["password"])
	assert.Equal(t, "node-1", sanitized["hostname"])
	assert.Equal(t, "production", sanitized["env"])

	// Original map must remain unchanged
	assert.Equal(t, "super-secret-1", orig["auth_token"])
	assert.Equal(t, "p@ssword", orig["password"])
}

func TestSanitizeComputeHost(t *testing.T) {
	host := &docker.ComputeHost{
		ID:       "host-1",
		Name:     "worker-1",
		Endpoint: "http://10.0.0.1:9100",
		TLSKey:   "private-key-material",
		Labels: map[string]string{
			"api_key": "raw-key-value",
			"region":  "us-west",
		},
	}

	sanitized := SanitizeComputeHost(host)
	require.NotNil(t, sanitized)

	assert.Equal(t, "", sanitized.TLSKey, "TLSKey must be cleared")
	assert.Equal(t, "***REDACTED***", sanitized.Labels["api_key"])
	assert.Equal(t, "us-west", sanitized.Labels["region"])

	// Original host must remain intact
	assert.Equal(t, "private-key-material", host.TLSKey)
	assert.Equal(t, "raw-key-value", host.Labels["api_key"])
}
