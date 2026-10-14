package iac_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/datdt/k8sselfhost/internal/infrastructure/iac"
)

func TestTerraformRunner_PlanAndApply(t *testing.T) {
	runner := iac.NewTerraformRunner("terraform-test-bin")
	ctx := context.Background()

	opts := iac.TerraformRunOptions{
		WorkingDir: t.TempDir(),
		Vars: map[string]string{
			"environment": "production",
			"node_count":  "3",
		},
		AutoApprove: true,
	}

	var streamOutput bytes.Buffer
	planResult, err := runner.Plan(ctx, opts, &streamOutput)
	if err == nil {
		t.Fatalf("expected error when terraform binary is not found, got nil")
	}

	if planResult != nil && planResult.Success {
		t.Errorf("expected failed plan result when binary is not found")
	}

	applyResult, err := runner.Apply(ctx, opts, &streamOutput)
	if err == nil {
		t.Fatalf("expected error when terraform binary is not found, got nil")
	}

	if applyResult != nil && applyResult.Success {
		t.Errorf("expected failed apply result when binary is not found")
	}
}

func TestAnsibleRunner_RunPlaybook(t *testing.T) {
	runner := iac.NewAnsibleRunner("ansible-test-bin")
	ctx := context.Background()

	opts := iac.AnsiblePlaybookOptions{
		PlaybookFile: "deploy/ansible/hardening.yaml",
		Inventory:    "127.0.0.1,",
		Become:       true,
	}

	var streamOutput bytes.Buffer
	result, err := runner.RunPlaybook(ctx, opts, &streamOutput)
	if err == nil {
		t.Fatalf("expected error when ansible binary is not found, got nil")
	}

	if result != nil && result.Success {
		t.Errorf("expected failed playbook execution when binary is not found")
	}
}

func TestNormalizeWorkDir_RejectsForbiddenSystemPathsAndTraversal(t *testing.T) {
	forbiddenPaths := []string{
		"/etc",
		"/root",
		"/proc",
		"/sys",
		"/boot",
		"/dev",
		"/var/run",
		"/usr",
		"/bin",
		"/sbin",
		"../../etc",
		"/etc/shadow",
		"/root/.ssh",
		`C:\Windows`,
		`C:\Windows\System32`,
		`c:/windows/system32`,
		`C:\System32`,
	}

	for _, path := range forbiddenPaths {
		_, err := iac.NormalizeWorkDir(path)
		if err == nil {
			t.Errorf("expected NormalizeWorkDir to reject forbidden path %q, got nil", path)
		}
	}
}

func TestNormalizeWorkDir_BaseDirContainment(t *testing.T) {
	baseDir := t.TempDir()
	subDir := baseDir + "/terraform/prod"

	// Valid sub directory within baseDir
	normalized, err := iac.NormalizeWorkDir(subDir, baseDir)
	if err != nil {
		t.Fatalf("expected valid subDir to succeed, got: %v", err)
	}
	if normalized == "" {
		t.Errorf("expected non-empty normalized path")
	}

	// Escaping directory outside baseDir
	outsideDir := t.TempDir()
	_, err = iac.NormalizeWorkDir(outsideDir, baseDir)
	if err == nil {
		t.Errorf("expected outsideDir %q to be rejected when baseDir is %q", outsideDir, baseDir)
	}
}

func TestTerraformRunner_RejectsForbiddenWorkingDir(t *testing.T) {
	runner := iac.NewTerraformRunner("terraform-bin")
	ctx := context.Background()

	opts := iac.TerraformRunOptions{
		WorkingDir: "/etc",
	}
	_, err := runner.Plan(ctx, opts, nil)
	if err == nil {
		t.Errorf("expected runner.Plan to reject WorkingDir '/etc'")
	}
}

func TestAnsibleRunner_RejectsForbiddenWorkingDir(t *testing.T) {
	runner := iac.NewAnsibleRunner("ansible-bin")
	ctx := context.Background()

	opts := iac.AnsiblePlaybookOptions{
		WorkingDir: "/root",
	}
	_, err := runner.RunPlaybook(ctx, opts, nil)
	if err == nil {
		t.Errorf("expected runner.RunPlaybook to reject WorkingDir '/root'")
	}
}
