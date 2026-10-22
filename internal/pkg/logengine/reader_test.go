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

func TestReader_TimeRange_ReturnsNewestLimitInWindow(t *testing.T) {
	tmpDir := t.TempDir()
	dict := NewLabelDictionary()

	writer, err := NewWriter(tmpDir, dict, WithMaxRowsPerBlock(4), WithFlushInterval(50*time.Millisecond))
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	baseTime := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	// Write 30 entries across multiple parts
	for i := 0; i < 30; i++ {
		ts := baseTime.Add(time.Duration(i) * time.Minute)
		err := writer.Write(Entry{
			Timestamp: ts,
			Service:   "api-server",
			Level:     "info",
			Message:   fmt.Sprintf("log message number %02d", i),
		})
		if err != nil {
			t.Fatalf("write failed: %v", err)
		}
		if (i+1)%5 == 0 {
			if err := writer.Flush(); err != nil {
				t.Fatalf("flush failed: %v", err)
			}
			writer.SealActivePart()
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	reader, err := NewReader(tmpDir, dict)
	if err != nil {
		t.Fatalf("reader creation failed: %v", err)
	}
	defer reader.Close()

	// Query window [minute 5 .. minute 25], limit 5
	// Inside [5..25], logs are 05..25 (21 logs total).
	// Limit 5 must return the NEWEST 5 in that window: 21, 22, 23, 24, 25 in chronological order.
	since := baseTime.Add(5 * time.Minute)
	until := baseTime.Add(25 * time.Minute)
	results, err := reader.Search(context.Background(), QueryParams{
		Since: since,
		Until: until,
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	assert.Len(t, results, 5)
	expectedMsgs := []string{
		"log message number 21",
		"log message number 22",
		"log message number 23",
		"log message number 24",
		"log message number 25",
	}
	for i, r := range results {
		assert.Equal(t, expectedMsgs[i], r.Message)
	}
}

func TestReader_SummarizeErrors_BlindDiscovery(t *testing.T) {
	tmpDir := t.TempDir()
	dict := NewLabelDictionary()

	writer, err := NewWriter(tmpDir, dict, WithMaxRowsPerBlock(100), WithFlushInterval(50*time.Millisecond))
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// 1. Write 2000 normal INFO logs (should not be in error groups)
	for i := 0; i < 2000; i++ {
		_ = writer.Write(Entry{
			Timestamp: baseTime.Add(time.Duration(i) * time.Millisecond),
			Service:   "web-frontend",
			Level:     "info",
			Message:   fmt.Sprintf("HTTP 200 GET /items/%d", i),
		})
	}
	_ = writer.Flush()
	writer.SealActivePart()

	// 2. Write 50 repeated connection timeout errors with varying latency & IPs
	for i := 0; i < 50; i++ {
		_ = writer.Write(Entry{
			Timestamp: baseTime.Add(time.Duration(2000+i) * time.Millisecond),
			Service:   "payment-svc",
			Level:     "error",
			Message:   fmt.Sprintf("connection timeout after %dms to 10.0.0.%d:5432", 100+i, 10+i),
		})
	}
	_ = writer.Flush()
	writer.SealActivePart()

	// 3. Write 1 rare DEADLOCK error
	_ = writer.Write(Entry{
		Timestamp: baseTime.Add(3000 * time.Millisecond),
		Service:   "billing-db",
		Level:     "fatal",
		Message:   "CRITICAL: DEADLOCK_TX_999999 detected in isolation level SERIALIZABLE transaction_id=999999",
	})
	_ = writer.Flush()
	if err := writer.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	reader, err := NewReader(tmpDir, dict)
	if err != nil {
		t.Fatalf("reader creation failed: %v", err)
	}
	defer reader.Close()

	// Call SummarizeErrors with NO keyword query
	groups, err := reader.SummarizeErrors(context.Background(), QueryParams{})
	if err != nil {
		t.Fatalf("SummarizeErrors failed: %v", err)
	}

	assert.Len(t, groups, 2, "must discover exactly 2 error groups across all parts")

	// Group 1: 50 connection timeouts
	assert.Equal(t, "payment-svc", groups[0].Service)
	assert.Equal(t, "error", groups[0].Level)
	assert.Equal(t, 50, groups[0].Count)
	assert.Contains(t, groups[0].Fingerprint, "connection timeout after * to *.*.*.*:*")

	// Group 2: 1 rare deadlock
	assert.Equal(t, "billing-db", groups[1].Service)
	assert.Equal(t, "fatal", groups[1].Level)
	assert.Equal(t, 1, groups[1].Count)
	assert.Contains(t, groups[1].Fingerprint, "DEADLOCK_TX_*")
}
