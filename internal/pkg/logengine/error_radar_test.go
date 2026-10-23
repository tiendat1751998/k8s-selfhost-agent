package logengine

import (
	"testing"
)

func TestIsNoiseService(t *testing.T) {
	tests := []struct {
		service  string
		expected bool
	}{
		{"init.scope", true},
		{"session-1.scope", true},
		{"user-1000.slice", true},
		{"system.slice", true},
		{"user@1000.service", true},
		{"user@.service", true},
		{"user-runtime-dir@1000.service", true},
		{"-- no entries --", true},
		{"", true},
		{"   ", true},
		{"auth-service", false},
		{"payment-gateway", false},
		{"nginx", false},
		{"k8s-agent", false},
	}

	for _, tc := range tests {
		t.Run(tc.service, func(t *testing.T) {
			got := IsNoiseService(tc.service)
			if got != tc.expected {
				t.Errorf("IsNoiseService(%q) = %v, expected %v", tc.service, got, tc.expected)
			}
		})
	}
}

func TestFingerprintError(t *testing.T) {
	msg1 := []byte("dial tcp 10.0.0.1:5432: i/o timeout")
	msg2 := []byte("dial tcp 10.0.0.2:5432: i/o timeout")
	fp1 := FingerprintError(msg1)
	fp2 := FingerprintError(msg2)
	if fp1 != fp2 {
		t.Errorf("expected same fingerprint for matching error pattern, got %q vs %q", fp1, fp2)
	}

	msg3 := []byte("record with id 98124 not found")
	msg4 := []byte("record with id 12345 not found")
	if FingerprintError(msg3) != FingerprintError(msg4) {
		t.Errorf("expected numbers normalized to wildcard")
	}
}
