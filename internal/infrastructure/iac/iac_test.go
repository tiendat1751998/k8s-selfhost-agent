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
