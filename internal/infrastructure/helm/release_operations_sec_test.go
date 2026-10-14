package helm

import (
	"context"
	"strings"
	"testing"
)

func TestReleaseOperations_SecurityValidation(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewReleaseManager(nil, nil, tempDir)
	ctx := context.Background()

	t.Run("InstallRelease rejects path traversal in Chart", func(t *testing.T) {
		badCharts := []string{
			"../../etc/shadow",
			"/etc/passwd",
			`\Windows\System32`,
			"foo/../../bar",
		}
		for _, bc := range badCharts {
			req := InstallRequest{
				ReleaseName: "my-rel",
				Chart:       bc,
			}
			_, err := rm.InstallRelease(ctx, "default", req)
			if err == nil || !strings.Contains(err.Error(), "invalid chart") {
				t.Fatalf("expected error for bad chart %q, got: %v", bc, err)
			}
		}
	})

	t.Run("InstallRelease rejects SSRF and bad schemes in Repo", func(t *testing.T) {
		badRepos := []string{
			"http://169.254.169.254/charts",
			"http://10.254.0.1/charts",
			"ftp://example.com/charts",
			"file:///etc/passwd",
		}
		for _, br := range badRepos {
			req := InstallRequest{
				ReleaseName: "my-rel",
				Chart:       "nginx",
				Repo:        br,
			}
			_, err := rm.InstallRelease(ctx, "default", req)
			if err == nil || !strings.Contains(err.Error(), "repository") {
				t.Fatalf("expected repository validation error for bad repo %q, got: %v", br, err)
			}
		}
	})

	t.Run("UpgradeRelease rejects path traversal in Chart", func(t *testing.T) {
		badCharts := []string{
			"../../etc/shadow",
			"/etc/passwd",
			`\Windows\System32`,
		}
		for _, bc := range badCharts {
			req := UpgradeRequest{
				ReleaseName: "my-rel",
				Chart:       bc,
			}
			_, err := rm.UpgradeRelease(ctx, "default", req)
			if err == nil || !strings.Contains(err.Error(), "invalid chart") {
				t.Fatalf("expected error for bad chart %q, got: %v", bc, err)
			}
		}
	})

	t.Run("UpgradeRelease rejects SSRF and bad schemes in Repo", func(t *testing.T) {
		badRepos := []string{
			"http://169.254.169.254/charts",
			"http://10.254.0.1/charts",
			"ftp://example.com/charts",
		}
		for _, br := range badRepos {
			req := UpgradeRequest{
				ReleaseName: "my-rel",
				Chart:       "nginx",
				Repo:        br,
			}
			_, err := rm.UpgradeRelease(ctx, "default", req)
			if err == nil || !strings.Contains(err.Error(), "repository") {
				t.Fatalf("expected repository validation error for bad repo %q, got: %v", br, err)
			}
		}
	})
}
