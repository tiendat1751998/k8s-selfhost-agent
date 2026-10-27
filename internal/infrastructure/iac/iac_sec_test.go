package iac_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/infrastructure/iac"
)

func TestTerraformRunner_EmptyBaseDir_DefaultsToSandbox_FailsClosed(t *testing.T) {
	// Runner created with empty baseDir
	runner := iac.NewTerraformRunner("echo")
	ctx := context.Background()

	// Default baseDir must not be empty and must point to sandbox directory
	assert.NotEmpty(t, runner.BaseDir(), "baseDir must default to dedicated sandbox directory")

	// Attempting to set empty baseDir must NOT bypass containment
	runner.SetBaseDir("")
	assert.NotEmpty(t, runner.BaseDir(), "setting empty baseDir must reset to sandbox directory")

	// Outside path must fail-closed
	outsideDir := filepath.Join(os.TempDir(), "arbitrary-outside-directory-12345")
	opts := iac.TerraformRunOptions{
		WorkingDir: outsideDir,
	}

	_, err := runner.Plan(ctx, opts, nil)
	require.Error(t, err, "expected outside directory to be rejected by containment")
	assert.True(t, strings.Contains(err.Error(), "outside allowed base directory") || strings.Contains(err.Error(), "rejected"),
		"unexpected error message: %v", err)
}

func TestAnsibleRunner_InventoryValidation(t *testing.T) {
	runner := iac.NewAnsibleRunner("echo")
	ctx := context.Background()

	t.Run("rejects flag injection in inventory", func(t *testing.T) {
		opts := iac.AnsiblePlaybookOptions{
			PlaybookFile: "playbook.yaml",
			Inventory:    "--extra-vars=malicious=true",
		}
		_, err := runner.RunPlaybook(ctx, opts, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "inventory")
	})

	t.Run("rejects null bytes in inventory", func(t *testing.T) {
		opts := iac.AnsiblePlaybookOptions{
			PlaybookFile: "playbook.yaml",
			Inventory:    "hosts.ini\x00malicious",
		}
		_, err := runner.RunPlaybook(ctx, opts, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "inventory")
	})

	t.Run("rejects inventory path escaping baseDir", func(t *testing.T) {
		opts := iac.AnsiblePlaybookOptions{
			PlaybookFile: "playbook.yaml",
			Inventory:    "../../../../etc/passwd",
		}
		_, err := runner.RunPlaybook(ctx, opts, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "inventory")
	})
}
