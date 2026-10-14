package iac_test

import (
	"context"
	"strings"
	"testing"

	"github.com/datdt/k8sselfhost/internal/infrastructure/iac"
)

func TestAnsibleRunner_SEC12_FlagInjectionAndPathTraversalValidation(t *testing.T) {
	runner := iac.NewAnsibleRunner("echo")
	ctx := context.Background()

	t.Run("rejects path traversal in playbook file", func(t *testing.T) {
		opts := iac.AnsiblePlaybookOptions{
			PlaybookFile: "../../etc/ansible/playbook.yaml",
		}
		_, err := runner.RunPlaybook(ctx, opts, nil)
		if err == nil || !strings.Contains(err.Error(), "playbook") {
			t.Fatalf("expected error for path traversal in playbook file, got: %v", err)
		}
	})

	t.Run("rejects flag injection in playbook file", func(t *testing.T) {
		opts := iac.AnsiblePlaybookOptions{
			PlaybookFile: "--version",
		}
		_, err := runner.RunPlaybook(ctx, opts, nil)
		if err == nil || !strings.Contains(err.Error(), "playbook") {
			t.Fatalf("expected error for flag injection in playbook file, got: %v", err)
		}
	})

	t.Run("rejects flag injection in limit", func(t *testing.T) {
		opts := iac.AnsiblePlaybookOptions{
			PlaybookFile: "playbook.yaml",
			Limit:        "--extra-vars=malicious=true",
		}
		_, err := runner.RunPlaybook(ctx, opts, nil)
		if err == nil || !strings.Contains(err.Error(), "limit") {
			t.Fatalf("expected error for flag injection in limit, got: %v", err)
		}
	})

	t.Run("rejects flag injection in user", func(t *testing.T) {
		opts := iac.AnsiblePlaybookOptions{
			PlaybookFile: "playbook.yaml",
			User:         "--become-user=root",
		}
		_, err := runner.RunPlaybook(ctx, opts, nil)
		if err == nil || !strings.Contains(err.Error(), "user") {
			t.Fatalf("expected error for flag injection in user, got: %v", err)
		}
	})
}
