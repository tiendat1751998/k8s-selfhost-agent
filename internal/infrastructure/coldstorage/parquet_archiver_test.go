package coldstorage

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/infrastructure/victoriametrics"
)

func TestRollup1Min_Math(t *testing.T) {
	// Create 100 samples within the same minute bucket (1697274000000 = 00:00:00)
	baseTs := int64(1697274000000)
	samples := make([]victoriametrics.MetricSample, 100)
	for i := 0; i < 100; i++ {
		samples[i] = victoriametrics.MetricSample{
			NodeID:      "worker-1",
			ClusterID:   "us-west",
			TenantID:    "org-acme",
			TimestampMs: baseTs + int64(i*500), // Within same minute (0 to 49.5s)
			CPUPercent:  float64(i + 1),        // 1.0 to 100.0
			MemPercent:  50.0 + float64(i%10),
			DiskPercent: 20.0,
			NetRxBytes:  1000,
			NetTxBytes:  2000,
		}
	}

	rollups := Rollup1Min(samples)
	if len(rollups) != 1 {
		t.Fatalf("expected 1 rollup bucket, got %d", len(rollups))
	}

	r := rollups[0]
	if r.TenantID != "org-acme" || r.ClusterID != "us-west" || r.NodeID != "worker-1" {
		t.Errorf("unexpected identifiers: %+v", r)
	}
	if r.BucketTimestampMs != baseTs {
		t.Errorf("expected bucket %d, got %d", baseTs, r.BucketTimestampMs)
	}
	if r.SampleCount != 100 {
		t.Errorf("expected 100 samples, got %d", r.SampleCount)
	}
	if r.MinCPU != 1.0 {
		t.Errorf("expected min cpu 1.0, got %f", r.MinCPU)
	}
	if r.MaxCPU != 100.0 {
		t.Errorf("expected max cpu 100.0, got %f", r.MaxCPU)
	}
	// Average of 1..100 = 5050 / 100 = 50.5
	if r.AvgCPU != 50.5 {
		t.Errorf("expected avg cpu 50.5, got %f", r.AvgCPU)
	}
	// P95 of 1..100 = 95.0
	if r.P95CPU != 95.0 {
		t.Errorf("expected p95 cpu 95.0, got %f", r.P95CPU)
	}
}

func TestRollup1Min_MultipleBucketsAndNodes(t *testing.T) {
	samples := []victoriametrics.MetricSample{
		// Node 1, Minute 0
		{
			NodeID:      "node-1",
			ClusterID:   "c1",
			TenantID:    "t1",
			TimestampMs: 60000,
			CPUPercent:  10.0,
			MemPercent:  20.0,
		},
		// Node 1, Minute 1
		{
			NodeID:      "node-1",
			ClusterID:   "c1",
			TenantID:    "t1",
			TimestampMs: 120000,
			CPUPercent:  20.0,
			MemPercent:  30.0,
		},
		// Node 2, Minute 0
		{
			NodeID:      "node-2",
			ClusterID:   "c1",
			TenantID:    "t1",
			TimestampMs: 65000,
			CPUPercent:  30.0,
			MemPercent:  40.0,
		},
	}

	rollups := Rollup1Min(samples)
	if len(rollups) != 3 {
		t.Fatalf("expected 3 separate rollups, got %d", len(rollups))
	}
}

func TestSnappyColumnarArchiver_RoundTrip(t *testing.T) {
	archiver := NewSnappyColumnarArchiver()

	records := make([]RollupRecord, 50)
	baseTs := int64(1700000000000)
	for i := 0; i < 50; i++ {
		records[i] = RollupRecord{
			TenantID:          "tenant-x",
			ClusterID:         "cluster-prod",
			NodeID:            "node-alpha",
			BucketTimestampMs: baseTs + int64(i*60000),
			AvgCPU:            25.5 + float64(i),
			MinCPU:            10.0,
			MaxCPU:            80.0,
			P95CPU:            75.0,
			AvgMem:            60.0,
			MaxMem:            70.0,
			AvgDisk:           45.0,
			SampleCount:       12,
		}
	}

	block, err := archiver.EncodeBlock(records)
	if err != nil {
		t.Fatalf("EncodeBlock failed: %v", err)
	}

	// Verify Zone-Map metadata inspection without decompression
	meta, err := archiver.InspectZoneMap(block)
	if err != nil {
		t.Fatalf("InspectZoneMap failed: %v", err)
	}
	if meta.TenantID != "tenant-x" {
		t.Errorf("expected tenant-x, got %s", meta.TenantID)
	}
	if meta.RowCount != 50 {
		t.Errorf("expected 50 rows, got %d", meta.RowCount)
	}
	if meta.MinTimestampMs != baseTs {
		t.Errorf("expected min ts %d, got %d", baseTs, meta.MinTimestampMs)
	}
	expectedMaxTs := baseTs + 49*60000
	if meta.MaxTimestampMs != expectedMaxTs {
		t.Errorf("expected max ts %d, got %d", expectedMaxTs, meta.MaxTimestampMs)
	}

	// Query blocks
	ctx := context.Background()
	results, err := archiver.QueryColdBlocks(ctx, [][]byte{block}, "tenant-x", baseTs, expectedMaxTs)
	if err != nil {
		t.Fatalf("QueryColdBlocks failed: %v", err)
	}

	if len(results) != len(records) {
		t.Fatalf("expected %d results, got %d", len(records), len(results))
	}

	for i := range records {
		if results[i] != records[i] {
			t.Fatalf("record %d mismatch: got %+v, expected %+v", i, results[i], records[i])
		}
	}
}

func TestQueryColdBlocks_ZoneMapPredicatePushdown(t *testing.T) {
	archiver := NewSnappyColumnarArchiver()

	// Block 1: Tenant A, Timestamps 100,000 to 200,000
	b1Records := []RollupRecord{
		{TenantID: "tenant-A", ClusterID: "c1", NodeID: "n1", BucketTimestampMs: 100000, AvgCPU: 10, SampleCount: 1},
		{TenantID: "tenant-A", ClusterID: "c1", NodeID: "n1", BucketTimestampMs: 200000, AvgCPU: 20, SampleCount: 1},
	}
	b1, err := archiver.EncodeBlock(b1Records)
	if err != nil {
		t.Fatal(err)
	}

	// Block 2: Tenant A, Timestamps 500,000 to 600,000 (Time mismatch)
	b2Records := []RollupRecord{
		{TenantID: "tenant-A", ClusterID: "c1", NodeID: "n1", BucketTimestampMs: 500000, AvgCPU: 50, SampleCount: 1},
		{TenantID: "tenant-A", ClusterID: "c1", NodeID: "n1", BucketTimestampMs: 600000, AvgCPU: 60, SampleCount: 1},
	}
	b2, err := archiver.EncodeBlock(b2Records)
	if err != nil {
		t.Fatal(err)
	}

	// Block 3: Tenant B, Timestamps 100,000 to 200,000 (Tenant mismatch)
	b3Records := []RollupRecord{
		{TenantID: "tenant-B", ClusterID: "c1", NodeID: "n1", BucketTimestampMs: 100000, AvgCPU: 99, SampleCount: 1},
	}
	b3, err := archiver.EncodeBlock(b3Records)
	if err != nil {
		t.Fatal(err)
	}

	allBlocks := [][]byte{b1, b2, b3}

	// Query Tenant A in range [50,000, 250,000]
	// Block 1 should match (both records)
	// Block 2 should be skipped by Zone-Map min/max timestamp!
	// Block 3 should be skipped by Zone-Map tenant_id!
	ctx := context.Background()
	matched, err := archiver.QueryColdBlocks(ctx, allBlocks, "tenant-A", 50000, 250000)
	if err != nil {
		t.Fatalf("QueryColdBlocks failed: %v", err)
	}

	if len(matched) != 2 {
		t.Fatalf("expected 2 records, got %d", len(matched))
	}
	if matched[0].BucketTimestampMs != 100000 || matched[1].BucketTimestampMs != 200000 {
		t.Errorf("unexpected matched records: %+v", matched)
	}
}

func TestSnappyColumnarArchiver_ChecksumCorruption(t *testing.T) {
	archiver := NewSnappyColumnarArchiver()

	records := []RollupRecord{
		{TenantID: "tenant-sec", ClusterID: "c1", NodeID: "n1", BucketTimestampMs: 1000, AvgCPU: 10, SampleCount: 1},
	}
	block, err := archiver.EncodeBlock(records)
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt a byte in the payload
	block[len(block)-1] ^= 0xFF

	ctx := context.Background()
	_, err = archiver.QueryColdBlocks(ctx, [][]byte{block}, "tenant-sec", 0, 10000)
	if err == nil {
		t.Fatal("expected checksum error on corrupted block, got nil")
	}
}

func BenchmarkSnappyColumnarArchiver_EncodeBlock(b *testing.B) {
	archiver := NewSnappyColumnarArchiver()
	rng := rand.New(rand.NewSource(99))

	records := make([]RollupRecord, 1000)
	baseTs := time.Now().UnixMilli()
	for i := 0; i < 1000; i++ {
		records[i] = RollupRecord{
			TenantID:          "tenant-benchmark",
			ClusterID:         "cluster-benchmark",
			NodeID:            "node-42",
			BucketTimestampMs: baseTs + int64(i*60000),
			AvgCPU:            rng.Float64() * 100,
			MinCPU:            rng.Float64() * 50,
			MaxCPU:            50 + rng.Float64()*50,
			P95CPU:            80 + rng.Float64()*15,
			AvgMem:            rng.Float64() * 100,
			MaxMem:            rng.Float64() * 100,
			AvgDisk:           50.0,
			SampleCount:       12,
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		block, err := archiver.EncodeBlock(records)
		if err != nil || len(block) == 0 {
			b.Fatal(err)
		}
	}
}

func BenchmarkQueryColdBlocks_WithZoneMapPushdown(b *testing.B) {
	archiver := NewSnappyColumnarArchiver()

	// Create 100 blocks, only 1 matches the query tenant and time range
	blocks := make([][]byte, 100)
	for i := 0; i < 100; i++ {
		tenant := "other-tenant"
		ts := int64(i * 1000000)
		if i == 50 {
			tenant = "target-tenant"
			ts = 5000000
		}
		records := []RollupRecord{
			{TenantID: tenant, ClusterID: "c1", NodeID: "n1", BucketTimestampMs: ts, AvgCPU: 50, SampleCount: 10},
		}
		block, _ := archiver.EncodeBlock(records)
		blocks[i] = block
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		results, err := archiver.QueryColdBlocks(ctx, blocks, "target-tenant", 4900000, 5100000)
		if err != nil {
			b.Fatal(err)
		}
		if len(results) != 1 {
			b.Fatalf("expected 1 result, got %d", len(results))
		}
	}
}
