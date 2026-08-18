package iac_test

import (
	"bytes"
	"context"
	"strings"
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
	if err != nil {
		t.Fatalf("Plan execution failed: %v", err)
	}

	if !planResult.Success {
		t.Errorf("expected success plan result")
	}

	applyResult, err := runner.Apply(ctx, opts, &streamOutput)
	if err != nil {
		t.Fatalf("Apply execution failed: %v", err)
	}

	if !applyResult.Success {
		t.Errorf("expected success apply result")
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
	if err != nil {
		t.Fatalf("Playbook run failed: %v", err)
	}

	if !result.Success {
		t.Errorf("expected successful playbook execution")
	}

	if !strings.Contains(result.Output, "PLAY RECAP") {
		t.Errorf("expected output to contain play recap, got: %s", result.Output)
	}
}
