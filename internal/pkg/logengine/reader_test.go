package logengine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

func TestReader_BlockCount_Over300Parts(t *testing.T) {
	tmpDir := t.TempDir()
	partsDir := filepath.Join(tmpDir, "parts")
	if err := os.MkdirAll(partsDir, 0755); err != nil {
		t.Fatalf("failed to create parts dir: %v", err)
	}

	// Create 305 fake part directories, each with a primary.idx containing 1 block (16 bytes)
	entryBytes := make([]byte, 16)
	for i := 1; i <= 305; i++ {
		pDir := filepath.Join(partsDir, fmt.Sprintf("part_%016d", i))
		if err := os.MkdirAll(pDir, 0755); err != nil {
			t.Fatalf("failed to create part dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(pDir, "primary.idx"), entryBytes, 0644); err != nil {
			t.Fatalf("failed to write primary.idx: %v", err)
		}
	}

	reader, err := NewReader(tmpDir, nil)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}
	defer reader.Close()

	count := reader.BlockCount()
	assert.Equal(t, 305, count, "BlockCount should not be capped at 256 parts and accurately count all 305 parts")
}

func TestReader_BloomPruning_SubTokenUnderscore(t *testing.T) {
	InitEngineRuntime()
	tmpDir := t.TempDir()
	dict := NewLabelDictionary()
	writer, err := NewWriter(tmpDir, dict, WithMaxRowsPerBlock(10))
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	now := time.Now().UTC()
	err = writer.Write(Entry{
		Timestamp: now,
		Service:   "payment-svc",
		Level:     "error",
		Message:   "Transaction CRITICAL_PAYMENT_FAILURE_XYZ_999 encountered database timeout",
	})
	if err != nil {
		t.Fatalf("failed to write entry: %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("failed to flush: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	reader, err := NewReader(tmpDir, dict)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}
	defer reader.Close()

	// 1. Exact match with sub-token "PAYMENT_FAILURE"
	results, err := reader.Search(context.Background(), QueryParams{
		Query:      "PAYMENT_FAILURE",
		ExactMatch: true,
	})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	assert.Len(t, results, 1, "sub-token 'PAYMENT_FAILURE' should be indexed in bloom and found via ExactMatch")

	// 2. Exact match with sub-token "CRITICAL"
	results2, err := reader.Search(context.Background(), QueryParams{
		Query:      "CRITICAL",
		ExactMatch: true,
	})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	assert.Len(t, results2, 1, "sub-token 'CRITICAL' should be indexed in bloom and found via ExactMatch")

	// 3. Substring match with interior token bounded by spaces
	results3, err := reader.Search(context.Background(), QueryParams{
		Query:      "Transaction CRITICAL_PAYMENT_FAILURE_XYZ_999 encountered",
		ExactMatch: false,
	})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	assert.Len(t, results3, 1, "interior bounded token should survive bloom stage and match")
}
