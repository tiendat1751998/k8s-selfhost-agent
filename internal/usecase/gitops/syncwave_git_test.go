package gitops_test

import (
	"context"
	"testing"

	usecaseGitops "github.com/datdt/k8sselfhost/internal/usecase/gitops"
)

func TestValidateCommitSHA(t *testing.T) {
	validSHAs := []string{
		"a1b2c3d",
		"1234567",
		"deadbeef1234567890abcdef1234567890abcdef",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}

	for _, sha := range validSHAs {
		if err := usecaseGitops.ValidateCommitSHA(sha); err != nil {
			t.Errorf("expected valid SHA %q, got: %v", sha, err)
		}
	}

	invalidSHAs := []string{
		"",
		"   ",
		"-a1b2c3d",
		"--all",
		"-f",
		"short", // < 7 chars
		"deadbeef; rm -rf /",
		"deadbeef`whoami`",
		"deadbeef$(id)",
		"invalid-non-hex-characters",
	}

	for _, sha := range invalidSHAs {
		if err := usecaseGitops.ValidateCommitSHA(sha); err == nil {
			t.Errorf("expected rejection for invalid SHA %q, got nil error", sha)
		}
	}
}

func TestDefaultGitCommander_FlagInjectionRejected(t *testing.T) {
	cmd := &usecaseGitops.DefaultGitCommander{}
	ctx := context.Background()

	maliciousSHAs := []string{"--all", "-f", "-j DROP", "not-a-hex-sha"}
	for _, badSHA := range maliciousSHAs {
		_, err := cmd.IsAncestor(ctx, ".", badSHA, "deadbeef1234567")
		if err == nil {
			t.Errorf("expected IsAncestor to reject bad ancestorSHA %q", badSHA)
		}

		_, err = cmd.IsAncestor(ctx, ".", "deadbeef1234567", badSHA)
		if err == nil {
			t.Errorf("expected IsAncestor to reject bad descendantSHA %q", badSHA)
		}
	}
}

func TestVerifyCommitMonotonicOrdering_RejectsHyphen(t *testing.T) {
	mgr := usecaseGitops.NewSyncWaveManager()
	ctx := context.Background()

	_, err := mgr.VerifyCommitMonotonicOrdering(ctx, ".", "-incoming", "current")
	if err == nil {
		t.Errorf("expected VerifyCommitMonotonicOrdering to reject leading hyphen in incomingSHA")
	}

	_, err = mgr.VerifyCommitMonotonicOrdering(ctx, ".", "incoming", "-current")
	if err == nil {
		t.Errorf("expected VerifyCommitMonotonicOrdering to reject leading hyphen in currentSHA")
	}
}
