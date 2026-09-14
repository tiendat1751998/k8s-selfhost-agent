package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveEngineDir_Candidate1_ExplicitRequested(t *testing.T) {
	reqDir := filepath.Join(t.TempDir(), "custom-engine")
	logDir := t.TempDir()

	resolved := resolveEngineDir(logDir, reqDir, nil)
	if resolved != reqDir {
		t.Fatalf("expected resolved path %q, got %q", reqDir, resolved)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		t.Fatalf("expected resolved directory to exist, got error: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected resolved path to be a directory")
	}
}

func TestResolveEngineDir_Candidate2_LogDirSubdir(t *testing.T) {
	logDir := t.TempDir()
	expectedDir := filepath.Join(logDir, ".logengine")

	resolved := resolveEngineDir(logDir, "", nil)
	if resolved != expectedDir {
		t.Fatalf("expected resolved path %q, got %q", expectedDir, resolved)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		t.Fatalf("expected resolved directory to exist, got error: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected resolved path to be a directory")
	}
}

func TestResolveEngineDir_InvalidRequested_FallsBackToLogDir(t *testing.T) {
	// Create a regular file to serve as an unwritable/invalid engine directory candidate
	invalidDir := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(invalidDir, []byte("file content"), 0644); err != nil {
		t.Fatalf("failed to create dummy file: %v", err)
	}

	logDir := t.TempDir()
	expectedDir := filepath.Join(logDir, ".logengine")

	resolved := resolveEngineDir(logDir, invalidDir, nil)
	if resolved != expectedDir {
		t.Fatalf("expected fallback to candidate 2 %q, got %q", expectedDir, resolved)
	}
}

func TestResolveEngineDir_InvalidLogDir_FallsBack(t *testing.T) {
	// Create a regular file to make filepath.Join(logDir, ".logengine") fail MkdirAll
	invalidLogDir := filepath.Join(t.TempDir(), "file-as-logdir")
	if err := os.WriteFile(invalidLogDir, []byte("file"), 0644); err != nil {
		t.Fatalf("failed to create dummy file: %v", err)
	}

	resolved := resolveEngineDir(invalidLogDir, "", nil)
	if resolved == "" {
		t.Fatal("expected non-empty resolved directory")
	}

	// Verify that the resolved fallback path is indeed writable
	probe := filepath.Join(resolved, ".test-probe")
	if err := os.WriteFile(probe, []byte("probe"), 0644); err != nil {
		t.Fatalf("resolved directory %q is not writable: %v", resolved, err)
	}
	_ = os.Remove(probe)
}

func TestResolveEngineDir_WithLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	logDir := t.TempDir()
	resolved := resolveEngineDir(logDir, "", logger)
	if resolved == "" {
		t.Fatal("expected non-empty resolved directory")
	}

	logOutput := buf.String()
	if len(logOutput) == 0 {
		t.Fatal("expected logger to produce log output when resolving directory")
	}
}

func TestApplyEnvOverrides_DefaultValues(t *testing.T) {
	t.Setenv("AGENT_PORT", "9200")
	t.Setenv("AGENT_LOG_DIR", "/custom/logs")
	t.Setenv("AGENT_ENGINE_DIR", "/custom/engine")
	t.Setenv("AGENT_AUTH_TOKEN", "secret-token")

	port, logDir, engineDir, authToken := applyEnvOverrides(9100, "/var/log", "", "")

	if port != 9200 {
		t.Errorf("expected port 9200, got %d", port)
	}
	if logDir != "/custom/logs" {
		t.Errorf("expected logDir '/custom/logs', got %q", logDir)
	}
	if engineDir != "/custom/engine" {
		t.Errorf("expected engineDir '/custom/engine', got %q", engineDir)
	}
	if authToken != "secret-token" {
		t.Errorf("expected authToken 'secret-token', got %q", authToken)
	}
}

func TestApplyEnvOverrides_NonDefaultPreserved(t *testing.T) {
	t.Setenv("AGENT_PORT", "9200")
	t.Setenv("AGENT_LOG_DIR", "/custom/logs")
	t.Setenv("AGENT_ENGINE_DIR", "/custom/engine")
	t.Setenv("AGENT_AUTH_TOKEN", "secret-token")

	port, logDir, engineDir, authToken := applyEnvOverrides(8080, "/my/logs", "/my/engine", "flag-token")

	if port != 8080 {
		t.Errorf("expected port 8080 preserved, got %d", port)
	}
	if logDir != "/my/logs" {
		t.Errorf("expected logDir '/my/logs' preserved, got %q", logDir)
	}
	if engineDir != "/my/engine" {
		t.Errorf("expected engineDir '/my/engine' preserved, got %q", engineDir)
	}
	if authToken != "flag-token" {
		t.Errorf("expected authToken 'flag-token' preserved, got %q", authToken)
	}
}

func TestApplyEnvOverrides_InvalidPortPreservesDefault(t *testing.T) {
	t.Setenv("AGENT_PORT", "invalid-port")
	port, _, _, _ := applyEnvOverrides(9100, "/var/log", "", "")
	if port != 9100 {
		t.Errorf("expected port 9100 to be preserved on invalid env, got %d", port)
	}

	t.Setenv("AGENT_PORT", "-5")
	port, _, _, _ = applyEnvOverrides(9100, "/var/log", "", "")
	if port != 9100 {
		t.Errorf("expected port 9100 to be preserved on negative env, got %d", port)
	}
}
