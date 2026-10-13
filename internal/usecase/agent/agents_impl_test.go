package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/agent"
)

func TestQAAgent_Execute_BuildFailure(t *testing.T) {
	tmpDir := t.TempDir()
	goModContent := "module testbuildfail\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}
	badGoContent := "package bad\nfunc invalid() {"
	if err := os.WriteFile(filepath.Join(tmpDir, "bad.go"), []byte(badGoContent), 0644); err != nil {
		t.Fatalf("failed to write bad.go: %v", err)
	}

	t.Setenv("PROJECT_DIR", tmpDir)
	t.Setenv("QA_AGENT_TEST", "1")

	qa := NewAgent(agent.QAEngineer, nil)
	task := &agent.Task{
		ID:    "task-test-1",
		Title: "Test QA Build Fail",
	}

	out, err := qa.Execute(context.Background(), task, "")
	if err == nil {
		t.Fatalf("expected error on build failure, got nil (out: %s)", out)
	}
	if !strings.Contains(err.Error(), "qa build verification failed") {
		t.Errorf("expected error message to contain 'qa build verification failed', got: %v", err)
	}
}

func TestQAAgent_Execute_TestFailure(t *testing.T) {
	tmpDir := t.TempDir()
	goModContent := "module testtestfail\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}
	validGoContent := "package testfail\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "valid.go"), []byte(validGoContent), 0644); err != nil {
		t.Fatalf("failed to write valid.go: %v", err)
	}
	failTestContent := "package testfail\nimport \"testing\"\nfunc TestShouldFail(t *testing.T) { t.Fatal(\"intentional failure\") }\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "valid_test.go"), []byte(failTestContent), 0644); err != nil {
		t.Fatalf("failed to write valid_test.go: %v", err)
	}

	t.Setenv("PROJECT_DIR", tmpDir)
	t.Setenv("QA_AGENT_TEST", "1")

	qa := NewAgent(agent.QAEngineer, nil)
	task := &agent.Task{
		ID:    "task-test-2",
		Title: "Test QA Test Fail",
	}

	out, err := qa.Execute(context.Background(), task, "")
	if err == nil {
		t.Fatalf("expected error on test failure, got nil (out: %s)", out)
	}
	if !strings.Contains(err.Error(), "qa test verification failed") {
		t.Errorf("expected error message to contain 'qa test verification failed', got: %v", err)
	}
}

func TestQAAgent_Execute_Success(t *testing.T) {
	tmpDir := t.TempDir()
	goModContent := "module testpass\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}
	validGoContent := "package testpass\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "valid.go"), []byte(validGoContent), 0644); err != nil {
		t.Fatalf("failed to write valid.go: %v", err)
	}
	passTestContent := "package testpass\nimport \"testing\"\nfunc TestShouldPass(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "valid_test.go"), []byte(passTestContent), 0644); err != nil {
		t.Fatalf("failed to write valid_test.go: %v", err)
	}

	t.Setenv("PROJECT_DIR", tmpDir)
	t.Setenv("QA_AGENT_TEST", "1")

	qa := NewAgent(agent.QAEngineer, nil)
	task := &agent.Task{
		ID:    "task-test-3",
		Title: "Test QA Success",
	}

	out, err := qa.Execute(context.Background(), task, "")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !strings.Contains(out, "QA SUCCESS") {
		t.Errorf("expected output to contain 'QA SUCCESS', got: %s", out)
	}
}

func TestQAAgent_Execute_TestV_Bypass(t *testing.T) {
	qa := NewAgent(agent.QAEngineer, nil)
	task := &agent.Task{
		ID:    "task-test-4",
		Title: "Test QA Bypass",
	}

	out, err := qa.Execute(context.Background(), task, "")
	if err != nil {
		t.Fatalf("expected nil error on bypass, got: %v", err)
	}
	if !strings.Contains(out, "Bypassed actual compiler and test runs") {
		t.Errorf("expected bypass message, got: %s", out)
	}
}
