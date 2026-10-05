package logengine

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMatchFilter_ExactAndBoundary(t *testing.T) {
	tests := []struct {
		name     string
		actual   string
		pattern  string
		expected bool
	}{
		{
			name:     "exact match same case",
			actual:   "postgres",
			pattern:  "postgres",
			expected: true,
		},
		{
			name:     "exact match different case",
			actual:   "Postgres_DB",
			pattern:  "postgres_db",
			expected: true,
		},
		{
			name:     "short pattern <= 3 exact match allowed",
			actual:   "db",
			pattern:  "db",
			expected: true,
		},
		{
			name:     "short pattern <= 3 substring in dbus.service MUST NOT MATCH",
			actual:   "dbus.service",
			pattern:  "db",
			expected: false,
		},
		{
			name:     "short pattern <= 3 substring in postgres_db MUST NOT MATCH",
			actual:   "postgres_db",
			pattern:  "db",
			expected: false,
		},
		{
			name:     "short pattern <= 3 substring in systemd-udevd MUST NOT MATCH",
			actual:   "systemd-udevd",
			pattern:  "dev",
			expected: false,
		},
		{
			name:     "prefix boundary with underscore",
			actual:   "postgres_db",
			pattern:  "postgres",
			expected: true,
		},
		{
			name:     "prefix boundary with hyphen",
			actual:   "postgres-main",
			pattern:  "postgres",
			expected: true,
		},
		{
			name:     "prefix boundary with dot",
			actual:   "dbus.service",
			pattern:  "dbus",
			expected: true,
		},
		{
			name:     "suffix boundary with hyphen",
			actual:   "prod-postgres",
			pattern:  "postgres",
			expected: true,
		},
		{
			name:     "arbitrary substring inside service name without boundary MUST NOT MATCH",
			actual:   "postgresql",
			pattern:  "postgres",
			expected: false,
		},
		{
			name:     "arbitrary substring in middle MUST NOT MATCH",
			actual:   "unauthorized-worker",
			pattern:  "auth",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchFilter(tc.actual, tc.pattern)
			assert.Equal(t, tc.expected, got, "matchFilter(%q, %q)", tc.actual, tc.pattern)
		})
	}
}

func TestReader_StartsAtTargetedTimeRangeNotOldestChunkParts(t *testing.T) {
	InitEngineRuntime()
	tmpDir, err := os.MkdirTemp("", "logengine-reader-range-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dict := NewLabelDictionary()
	writer, err := NewWriter(tmpDir, dict, WithMaxRowsPerBlock(100), WithFlushInterval(1*time.Hour))
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	t10 := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	t11 := time.Date(2026, 9, 16, 11, 0, 0, 0, time.UTC)
	t12 := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)

	// Part 1: 10:00 logs
	for i := 0; i < 5; i++ {
		_ = writer.Write(Entry{Timestamp: t10.Add(time.Duration(i) * time.Second), Service: "app", Message: fmt.Sprintf("10:00 log %d", i)})
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush part 1: %v", err)
	}

	// Part 2: 11:00 logs
	for i := 0; i < 5; i++ {
		_ = writer.Write(Entry{Timestamp: t11.Add(time.Duration(i) * time.Second), Service: "app", Message: fmt.Sprintf("11:00 log %d", i)})
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush part 2: %v", err)
	}

	// Part 3: 12:00 logs
	for i := 0; i < 5; i++ {
		_ = writer.Write(Entry{Timestamp: t12.Add(time.Duration(i) * time.Second), Service: "app", Message: fmt.Sprintf("12:00 log %d", i)})
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush part 3: %v", err)
	}
	_ = writer.Close()

	reader, err := NewReader(tmpDir, dict)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}
	defer reader.Close()

	ctx := context.Background()

	// 1. When querying latest logs with limit (no Since), must return the NEWEST logs (12:00), NOT oldest (10:00)
	resLatest, err := reader.Search(ctx, QueryParams{Limit: 3})
	if err != nil {
		t.Fatalf("Search latest error: %v", err)
	}
	if len(resLatest) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(resLatest))
	}
	for _, e := range resLatest {
		if e.Timestamp.Before(t12) {
			t.Fatalf("expected search without Since to start at newest chunk parts (>= 12:00), but got entry at %v (%s)", e.Timestamp, e.Message)
		}
	}
}
