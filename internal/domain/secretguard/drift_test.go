package secretguard_test

import (
	"testing"
	"time"
	"unsafe"

	"github.com/datdt/k8sselfhost/internal/domain/secretguard"
)

func TestExternalSecretPolicy_ValidationAndViolation(t *testing.T) {
	t.Parallel()

	now := time.Now()
	policy := secretguard.ExternalSecretPolicy{
		Namespace:           "production",
		Name:                "db-credentials",
		Provider:            secretguard.ProviderVault,
		ConfiguredInterval:  10 * time.Second,
		EnforcedInterval:    secretguard.MinSafetyInterval,
		ReloaderAutoRestart: true,
		SecretDataHash:      "sha256:abc12345",
		LastSyncedAt:        now,
	}

	if !policy.IsViolation() {
		t.Fatalf("expected 10s interval to be flagged as violation, got false")
	}

	if err := policy.Validate(); err != nil {
		t.Fatalf("expected valid policy struct, got error: %v", err)
	}

	// Test safe policy
	safePolicy := secretguard.ExternalSecretPolicy{
		Namespace:           "production",
		Name:                "db-credentials",
		Provider:            secretguard.ProviderAWS,
		ConfiguredInterval:  2 * time.Hour,
		EnforcedInterval:    2 * time.Hour,
		ReloaderAutoRestart: false,
		SecretDataHash:      "sha256:def67890",
		LastSyncedAt:        now,
	}

	if safePolicy.IsViolation() {
		t.Fatalf("expected 2h interval not to be flagged as violation, got true")
	}
}

func TestExternalSecretPolicy_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		policy  secretguard.ExternalSecretPolicy
		wantErr bool
	}{
		{
			name: "missing namespace",
			policy: secretguard.ExternalSecretPolicy{
				Namespace:          "",
				Name:               "secret1",
				Provider:           secretguard.ProviderVault,
				ConfiguredInterval: time.Hour,
			},
			wantErr: true,
		},
		{
			name: "missing name",
			policy: secretguard.ExternalSecretPolicy{
				Namespace:          "default",
				Name:               "",
				Provider:           secretguard.ProviderVault,
				ConfiguredInterval: time.Hour,
			},
			wantErr: true,
		},
		{
			name: "invalid provider",
			policy: secretguard.ExternalSecretPolicy{
				Namespace:          "default",
				Name:               "sec",
				Provider:           "unknown-provider",
				ConfiguredInterval: time.Hour,
			},
			wantErr: true,
		},
		{
			name: "valid in-cluster provider",
			policy: secretguard.ExternalSecretPolicy{
				Namespace:          "default",
				Name:               "sec",
				Provider:           secretguard.ProviderInCluster,
				ConfiguredInterval: time.Hour,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.policy.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestReconcilerCounters_CacheLinePadding(t *testing.T) {
	t.Parallel()

	// Verify that each counter block is spaced by at least 64 bytes to eliminate false sharing.
	counters := secretguard.ReconcilerCounters{}

	addrAudit := uintptr(unsafe.Pointer(&counters.AuditRuns))
	addrMutations := uintptr(unsafe.Pointer(&counters.MutationsDone))
	addrRollouts := uintptr(unsafe.Pointer(&counters.RolloutsDone))
	addrFailed := uintptr(unsafe.Pointer(&counters.RolloutsFailed))
	addrDrift := uintptr(unsafe.Pointer(&counters.DriftDetected))

	if addrMutations-addrAudit < 64 {
		t.Fatalf("AuditRuns and MutationsDone offset difference is %d < 64 bytes", addrMutations-addrAudit)
	}
	if addrRollouts-addrMutations < 64 {
		t.Fatalf("MutationsDone and RolloutsDone offset difference is %d < 64 bytes", addrRollouts-addrMutations)
	}
	if addrFailed-addrRollouts < 64 {
		t.Fatalf("RolloutsDone and RolloutsFailed offset difference is %d < 64 bytes", addrFailed-addrRollouts)
	}
	if addrDrift-addrFailed < 64 {
		t.Fatalf("RolloutsFailed and DriftDetected offset difference is %d < 64 bytes", addrDrift-addrFailed)
	}
}
