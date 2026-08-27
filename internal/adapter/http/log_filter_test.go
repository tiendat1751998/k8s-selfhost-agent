package http

import (
	"strings"
	"testing"
)

func TestFilterLogStream_SearchQuery(t *testing.T) {
	rawLogs := `2026-08-25T10:00:00Z [INFO] Server started on port 8080
2026-08-25T10:01:00Z [WARN] Request TIMEOUT after 30s
2026-08-25T10:02:00Z [ERROR] Failed to connect to database
2026-08-25T10:03:00Z [INFO] Health check timeout acknowledged`

	t.Run("Case-insensitive match for lowercase query", func(t *testing.T) {
		got := FilterLogStream(rawLogs, "timeout", "", 0)
		expected := "2026-08-25T10:01:00Z [WARN] Request TIMEOUT after 30s\n2026-08-25T10:03:00Z [INFO] Health check timeout acknowledged"
		if got != expected {
			t.Fatalf("expected:\n%s\ngot:\n%s", expected, got)
		}
	})

	t.Run("Case-insensitive match for UPPERCASE query", func(t *testing.T) {
		got := FilterLogStream(rawLogs, "ERROR", "", 0)
		expected := "2026-08-25T10:02:00Z [ERROR] Failed to connect to database"
		if got != expected {
			t.Fatalf("expected:\n%s\ngot:\n%s", expected, got)
		}
	})

	t.Run("No matching query returns empty string", func(t *testing.T) {
		got := FilterLogStream(rawLogs, "redis", "", 0)
		if got != "" {
			t.Fatalf("expected empty string, got: %s", got)
		}
	})
}

func TestFilterLogStream_LogLevel(t *testing.T) {
	rawLogs := `2026-08-25T10:00:00Z [INFO] Service initialized successfully
2026-08-25T10:01:00Z [DEBUG] Cache lookup trace_id=xyz
2026-08-25T10:02:00Z [WARN] Memory threshold warning reached
2026-08-25T10:03:00Z [WARN] Gateway timeout connecting to upstream
2026-08-25T10:04:00Z [ERROR] Unhandled exception in worker goroutine
2026-08-25T10:05:00Z [FATAL] Critical panic recovered: nil pointer
2026-08-25T10:06:00Z [ERR] Database connection failed`

	t.Run("Level error matches err, fatal, panic, exception, fail", func(t *testing.T) {
		got := FilterLogStream(rawLogs, "", "error", 0)
		lines := strings.Split(got, "\n")
		if len(lines) != 3 {
			t.Fatalf("expected 3 error lines, got %d:\n%s", len(lines), got)
		}
		if !strings.Contains(lines[0], "exception") || !strings.Contains(lines[1], "panic") || !strings.Contains(lines[2], "failed") {
			t.Fatalf("unexpected error lines output:\n%s", got)
		}
	})

	t.Run("Level warn matches warn, warning, timeout", func(t *testing.T) {
		got := FilterLogStream(rawLogs, "", "warn", 0)
		lines := strings.Split(got, "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 warn lines, got %d:\n%s", len(lines), got)
		}
		if !strings.Contains(lines[0], "warning") || !strings.Contains(lines[1], "timeout") {
			t.Fatalf("unexpected warn lines output:\n%s", got)
		}
	})

	t.Run("Level debug matches debug, trace", func(t *testing.T) {
		got := FilterLogStream(rawLogs, "", "debug", 0)
		lines := strings.Split(got, "\n")
		if len(lines) != 1 || !strings.Contains(lines[0], "Cache lookup trace_id=xyz") {
			t.Fatalf("expected 1 debug line, got:\n%s", got)
		}
	})

	t.Run("Level info matches nominal lines", func(t *testing.T) {
		got := FilterLogStream(rawLogs, "", "info", 0)
		lines := strings.Split(got, "\n")
		if len(lines) != 1 || !strings.Contains(lines[0], "Service initialized successfully") {
			t.Fatalf("expected 1 nominal info line, got:\n%s", got)
		}
	})

	t.Run("Level all or empty returns all lines unchanged", func(t *testing.T) {
		gotAll := FilterLogStream(rawLogs, "", "all", 0)
		gotEmpty := FilterLogStream(rawLogs, "", "", 0)
		if gotAll != rawLogs || gotEmpty != rawLogs {
			t.Fatalf("expected full logs, got:\n%s", gotAll)
		}
	})
}

func TestFilterLogStream_Combined(t *testing.T) {
	rawLogs := `2026-08-25T10:00:00Z [INFO] Postgres connected on port 5432
2026-08-25T10:01:00Z [WARN] Postgres query timeout after 5s
2026-08-25T10:02:00Z [ERROR] Postgres failed query: syntax error
2026-08-25T10:03:00Z [ERROR] Redis failed connection refused`

	t.Run("Combined query 'postgres' and level 'error'", func(t *testing.T) {
		got := FilterLogStream(rawLogs, "postgres", "error", 0)
		expected := "2026-08-25T10:02:00Z [ERROR] Postgres failed query: syntax error"
		if got != expected {
			t.Fatalf("expected:\n%s\ngot:\n%s", expected, got)
		}
	})

	t.Run("Combined query 'redis' and level 'warn' returns empty", func(t *testing.T) {
		got := FilterLogStream(rawLogs, "redis", "warn", 0)
		if got != "" {
			t.Fatalf("expected empty string, got: %s", got)
		}
	})
}

func TestFilterLogStream_LimitTruncation(t *testing.T) {
	rawLogs := `line 1
line 2
line 3
line 4
line 5`

	t.Run("Limit caps output lines", func(t *testing.T) {
		got := FilterLogStream(rawLogs, "", "", 2)
		expected := "line 1\nline 2"
		if got != expected {
			t.Fatalf("expected:\n%s\ngot:\n%s", expected, got)
		}
	})

	t.Run("Limit larger than total lines returns all lines", func(t *testing.T) {
		got := FilterLogStream(rawLogs, "", "", 10)
		if got != rawLogs {
			t.Fatalf("expected:\n%s\ngot:\n%s", rawLogs, got)
		}
	})

	t.Run("Limit 0 returns all lines", func(t *testing.T) {
		got := FilterLogStream(rawLogs, "", "", 0)
		if got != rawLogs {
			t.Fatalf("expected:\n%s\ngot:\n%s", rawLogs, got)
		}
	})
}

func TestFilterLogStream_EdgeCases(t *testing.T) {
	t.Run("Empty input", func(t *testing.T) {
		if got := FilterLogStream("", "test", "error", 10); got != "" {
			t.Fatalf("expected empty, got: %s", got)
		}
	})

	t.Run("Single line match", func(t *testing.T) {
		got := FilterLogStream("single error line", "", "error", 0)
		if got != "single error line" {
			t.Fatalf("expected 'single error line', got: %s", got)
		}
	})

	t.Run("Single line mismatch", func(t *testing.T) {
		got := FilterLogStream("single info line", "notfound", "", 0)
		if got != "" {
			t.Fatalf("expected empty string, got: %s", got)
		}
	})

	t.Run("Long line handling within 256KB buffer", func(t *testing.T) {
		longLine := strings.Repeat("A", 100*1024) + " error: something failed"
		got := FilterLogStream(longLine, "error", "error", 0)
		if got != longLine {
			t.Fatalf("expected long line to match, got length %d", len(got))
		}
	})
}
