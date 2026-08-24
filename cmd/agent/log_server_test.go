package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogServer_GetLogs_PlainTextAndJSON(t *testing.T) {
	memSource := NewMemoryLogSource()
	now := time.Now().UTC().Truncate(time.Second)

	memSource.AddEntry("tiki_traefik", now.Add(-5*time.Minute), "info", "Starting proxy router")
	memSource.AddEntry("tiki_traefik", now.Add(-3*time.Minute), "warn", "Upstream server slow response")
	memSource.AddEntry("tiki_traefik", now.Add(-1*time.Minute), "error", "Failed to connect to backend 502")
	memSource.AddEntry("nats", now.Add(-2*time.Minute), "info", "Client connected")

	logServer := NewLogServer(WithLogSource(memSource))
	collector := NewSystemCollector("", "", nil)
	handler := setupHandler(collector, "", logServer)

	t.Run("GET /logs default plain text", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs?app=tiki_traefik", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, "Starting proxy router") {
			t.Errorf("expected logs to contain 'Starting proxy router', got %s", body)
		}
		if !strings.Contains(body, "Failed to connect to backend 502") {
			t.Errorf("expected logs to contain 'Failed to connect to backend 502', got %s", body)
		}
		if strings.Contains(body, "Client connected") {
			t.Errorf("expected logs NOT to contain nats message when filtering app=tiki_traefik")
		}
	})

	t.Run("GET /logs format=json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs?app=tiki_traefik&format=json", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		var resp LogsResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode LogsResponse: %v", err)
		}
		if resp.Count != 3 {
			t.Errorf("expected 3 log lines, got %d", resp.Count)
		}
		if len(resp.Lines) != 3 {
			t.Errorf("expected len(Lines) == 3, got %d", len(resp.Lines))
		}
	})

	t.Run("GET /logs level filter=error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs?app=tiki_traefik&level=error&format=json", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		var resp LogsResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode LogsResponse: %v", err)
		}
		if resp.Count != 1 {
			t.Errorf("expected 1 error log line, got %d", resp.Count)
		}
		if len(resp.Lines) > 0 && !strings.Contains(resp.Lines[0], "502") {
			t.Errorf("expected 502 error line, got %s", resp.Lines[0])
		}
	})

	t.Run("GET /logs query filter q=slow", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs?app=tiki_traefik&q=slow&format=json", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		var resp LogsResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode LogsResponse: %v", err)
		}
		if resp.Count != 1 {
			t.Errorf("expected 1 matching line, got %d", resp.Count)
		}
	})

	t.Run("GET /logs tail limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs?app=tiki_traefik&tail=2&format=json", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		var resp LogsResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode LogsResponse: %v", err)
		}
		if resp.Count != 2 {
			t.Errorf("expected tail 2 results, got %d", resp.Count)
		}
	})

	t.Run("GET /logs time window since/until", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs?app=tiki_traefik&since=4m&format=json", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		var resp LogsResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode LogsResponse: %v", err)
		}
		if resp.Count != 2 {
			t.Errorf("expected 2 lines in past 4m, got %d", resp.Count)
		}
	})
}

func TestLogServer_GetServices(t *testing.T) {
	memSource := NewMemoryLogSource()
	memSource.AddEntry("tiki_traefik", time.Now(), "info", "msg1")
	memSource.AddEntry("nats", time.Now(), "info", "msg2")
	memSource.AddEntry("postgres_db", time.Now(), "info", "msg3")

	logServer := NewLogServer(WithLogSource(memSource))
	collector := NewSystemCollector("", "", nil)
	handler := setupHandler(collector, "", logServer)

	req := httptest.NewRequest(http.MethodGet, "/logs/services", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp LogServicesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode LogServicesResponse: %v", err)
	}

	if len(resp.Services) != 3 {
		t.Fatalf("expected 3 services, got %d (%v)", len(resp.Services), resp.Services)
	}
	expected := []string{"nats", "postgres_db", "tiki_traefik"}
	for i, s := range expected {
		if resp.Services[i] != s {
			t.Errorf("expected service[%d] == %s, got %s", i, s, resp.Services[i])
		}
	}
}

func TestLogServer_SearchLogs_POST(t *testing.T) {
	memSource := NewMemoryLogSource()
	now := time.Now().UTC()

	memSource.AddEntry("tiki_traefik", now.Add(-10*time.Minute), "error", "tls handshake failure")
	memSource.AddEntry("tiki_traefik", now.Add(-5*time.Minute), "info", "reloaded certificate")
	memSource.AddEntry("nats", now.Add(-2*time.Minute), "error", "cluster split-brain detected")

	logServer := NewLogServer(WithLogSource(memSource))
	collector := NewSystemCollector("", "", nil)
	handler := setupHandler(collector, "", logServer)

	t.Run("Search all error logs across services", func(t *testing.T) {
		body := LogSearchRequest{
			Level: "error",
			Limit: 10,
		}
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/logs/search", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp LogSearchResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode LogSearchResponse: %v", err)
		}
		if resp.Total != 2 {
			t.Errorf("expected 2 error logs across all services, got %d", resp.Total)
		}
	})

	t.Run("Search query text filter", func(t *testing.T) {
		body := LogSearchRequest{
			Query: "handshake",
			Limit: 10,
		}
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/logs/search", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var resp LogSearchResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode LogSearchResponse: %v", err)
		}
		if resp.Total != 1 {
			t.Fatalf("expected 1 result, got %d", resp.Total)
		}
		if resp.Results[0].Service != "tiki_traefik" {
			t.Errorf("expected service tiki_traefik, got %s", resp.Results[0].Service)
		}
		if !strings.Contains(resp.Results[0].Message, "tls handshake") {
			t.Errorf("expected message containing tls handshake, got %s", resp.Results[0].Message)
		}
	})
}

func TestLogServer_Authentication(t *testing.T) {
	memSource := NewMemoryLogSource()
	memSource.AddEntry("test", time.Now(), "info", "hello world")
	logServer := NewLogServer(WithLogSource(memSource))
	collector := NewSystemCollector("", "", nil)

	token := "secure-token-xyz"
	handler := setupHandler(collector, token, logServer)

	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/metrics", ""},
		{http.MethodGet, "/logs?app=test", ""},
		{http.MethodGet, "/logs/services", ""},
		{http.MethodPost, "/logs/search", `{"query":"hello"}`},
	}

	for _, ep := range endpoints {
		t.Run("Unauthorized "+ep.method+" "+ep.path, func(t *testing.T) {
			var r *http.Request
			if ep.body != "" {
				r = httptest.NewRequest(ep.method, ep.path, strings.NewReader(ep.body))
			} else {
				r = httptest.NewRequest(ep.method, ep.path, nil)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, r)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 Unauthorized for %s %s without token, got %d", ep.method, ep.path, rec.Code)
			}
		})

		t.Run("Authorized "+ep.method+" "+ep.path, func(t *testing.T) {
			var r *http.Request
			if ep.body != "" {
				r = httptest.NewRequest(ep.method, ep.path, strings.NewReader(ep.body))
			} else {
				r = httptest.NewRequest(ep.method, ep.path, nil)
			}
			r.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, r)

			if rec.Code != http.StatusOK {
				t.Errorf("expected 200 OK for %s %s with valid token, got %d", ep.method, ep.path, rec.Code)
			}
		})
	}
}

func TestFileLogSource_ReadLogs(t *testing.T) {
	tempDir := t.TempDir()
	logContent := "2026-08-24T12:00:00Z [INFO] Service initialized\n" +
		"2026-08-24T12:05:00Z [WARN] Memory high\n" +
		"2026-08-24T12:10:00Z [ERROR] Fatal disk error\n"

	err := os.WriteFile(filepath.Join(tempDir, "myservice.log"), []byte(logContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test log file: %v", err)
	}

	fileSrc := &FileLogSource{logDir: tempDir}
	services, err := fileSrc.ListServices(context.Background())
	if err != nil {
		t.Fatalf("ListServices error: %v", err)
	}
	if len(services) != 1 || services[0] != "myservice" {
		t.Errorf("expected ['myservice'], got %v", services)
	}

	entries, err := fileSrc.GetLogs(context.Background(), "myservice", 10, nil, nil, "", "error")
	if err != nil {
		t.Fatalf("GetLogs error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 error log entry, got %d", len(entries))
	}
	if !strings.Contains(entries[0].Message, "disk error") {
		t.Errorf("expected disk error message, got %s", entries[0].Message)
	}
}
