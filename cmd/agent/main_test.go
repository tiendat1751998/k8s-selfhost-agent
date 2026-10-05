package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
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
	t.Setenv("AGENT_BIND_ADDR", "127.0.0.1")
	t.Setenv("AGENT_LOG_DIR", "/custom/logs")
	t.Setenv("AGENT_ENGINE_DIR", "/custom/engine")
	t.Setenv("AGENT_AUTH_TOKEN", "secret-token")

	port, bindAddr, logDir, engineDir, authToken := applyEnvOverrides(9100, "", "/var/log", "", "")

	if port != 9200 {
		t.Errorf("expected port 9200, got %d", port)
	}
	if bindAddr != "127.0.0.1" {
		t.Errorf("expected bindAddr '127.0.0.1', got %q", bindAddr)
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
	t.Setenv("AGENT_BIND_ADDR", "127.0.0.1")
	t.Setenv("AGENT_LOG_DIR", "/custom/logs")
	t.Setenv("AGENT_ENGINE_DIR", "/custom/engine")
	t.Setenv("AGENT_AUTH_TOKEN", "secret-token")

	port, bindAddr, logDir, engineDir, authToken := applyEnvOverrides(8080, "0.0.0.0", "/my/logs", "/my/engine", "flag-token")

	if port != 8080 {
		t.Errorf("expected port 8080 preserved, got %d", port)
	}
	if bindAddr != "0.0.0.0" {
		t.Errorf("expected bindAddr '0.0.0.0' preserved, got %q", bindAddr)
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
	port, _, _, _, _ := applyEnvOverrides(9100, "", "/var/log", "", "")
	if port != 9100 {
		t.Errorf("expected port 9100 to be preserved on invalid env, got %d", port)
	}

	t.Setenv("AGENT_PORT", "-5")
	port, _, _, _, _ = applyEnvOverrides(9100, "", "/var/log", "", "")
	if port != 9100 {
		t.Errorf("expected port 9100 to be preserved on negative env, got %d", port)
	}
}

func TestResolveListenAddr(t *testing.T) {
	tests := []struct {
		name     string
		bindAddr string
		port     int
		expected string
	}{
		{
			name:     "empty bind defaults to :port",
			bindAddr: "",
			port:     9100,
			expected: ":9100",
		},
		{
			name:     "whitespace bind defaults to :port",
			bindAddr: "   ",
			port:     9100,
			expected: ":9100",
		},
		{
			name:     "host only joins with port",
			bindAddr: "127.0.0.1",
			port:     9100,
			expected: "127.0.0.1:9100",
		},
		{
			name:     "host with port used directly",
			bindAddr: "0.0.0.0:8080",
			port:     9100,
			expected: "0.0.0.0:8080",
		},
		{
			name:     "port only used directly",
			bindAddr: ":9200",
			port:     9100,
			expected: ":9200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveListenAddr(tt.bindAddr, tt.port)
			if got != tt.expected {
				t.Errorf("resolveListenAddr(%q, %d) = %q, expected %q", tt.bindAddr, tt.port, got, tt.expected)
			}
		})
	}
}

func TestSystemCollector_Stop_Concurrent_NoPanic(t *testing.T) {
	for i := 0; i < 50; i++ {
		collector := NewSystemCollector("", "", nil)
		var wg sync.WaitGroup
		const concurrency = 20
		wg.Add(concurrency)
		for j := 0; j < concurrency; j++ {
			go func() {
				defer wg.Done()
				collector.Stop()
			}()
		}
		wg.Wait()
	}
}

func TestValidateConfig_EmptyTokenFails(t *testing.T) {
	tests := []struct {
		name      string
		cfg       AgentConfig
		wantError bool
	}{
		{
			name: "empty auth token fails",
			cfg: AgentConfig{
				Port:      9100,
				AuthToken: "",
			},
			wantError: true,
		},
		{
			name: "whitespace auth token fails",
			cfg: AgentConfig{
				Port:      9100,
				AuthToken: "   ",
			},
			wantError: true,
		},
		{
			name: "valid token passes",
			cfg: AgentConfig{
				Port:      9100,
				AuthToken: "super-secure-token",
			},
			wantError: false,
		},
		{
			name: "invalid port zero fails",
			cfg: AgentConfig{
				Port:      0,
				AuthToken: "valid-token",
			},
			wantError: true,
		},
		{
			name: "invalid negative port fails",
			cfg: AgentConfig{
				Port:      -1,
				AuthToken: "valid-token",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.cfg)
			if tt.wantError && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("unexpected error for valid config: %v", err)
			}
		})
	}
}

func TestSetupHandler_EmptyTokenRejectsAll(t *testing.T) {
	collector := NewSystemCollector("", "", nil)
	// Empty authToken should never allow access to protected endpoints
	handler := setupHandler(collector, "")

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for empty authToken on /metrics, got %d", rec.Code)
	}
}

func TestSetupHandler_AuthEnforcement(t *testing.T) {
	collector := NewSystemCollector("", "", nil)
	token := "secure-token-xyz"
	handler := setupHandler(collector, token)

	// 1. Request without token -> 401
	{
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 without auth header, got %d", rec.Code)
		}
	}

	// 2. Request with wrong token -> 401
	{
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", "Bearer wrong-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 with wrong bearer token, got %d", rec.Code)
		}
	}

	// 3. Request with correct token -> 200
	{
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 with correct bearer token, got %d", rec.Code)
		}
	}

	// 4. Liveness /health without token -> 200
	{
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for unauthenticated /health, got %d", rec.Code)
		}
	}
}
