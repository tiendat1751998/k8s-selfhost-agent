package logengine

import (
	"strings"
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

func TestFingerprintError_KlogPrefixStripping(t *testing.T) {
	msg1 := []byte(`E1105 12:41:03.123456   1842 kubelet_node_status.go:106] "Unable to register node" err="dial tcp 10.10.10.60:6443: connect: no route to host"`)
	msg2 := []byte(`E1105 12:42:15.987654  99999 kubelet_node_status.go:106] "Unable to register node" err="dial tcp 10.10.10.60:6443: connect: no route to host"`)
	fp1 := FingerprintError(msg1)
	fp2 := FingerprintError(msg2)
	if fp1 != fp2 {
		t.Fatalf("expected identical fingerprints for recurring klog errors with different timestamps and PIDs, got %q vs %q", fp1, fp2)
	}
	if !strings.HasPrefix(fp1, "kubelet_node_status.go:*]") {
		t.Errorf("expected volatile klog header stripped so fingerprint starts with file.go:*], got %q", fp1)
	}
}
