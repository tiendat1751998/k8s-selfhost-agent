package deltastream

import (
	"math/rand"
	"testing"
	"time"
)

func TestEncodeDecodeFrame_RoundTrip(t *testing.T) {
	ts := time.Now().UnixMilli()
	records := []NodeDeltaRecord{
		{
			NodeHash:      HashNodeID("node-us-east-1a-001"),
			CPUCentipct:   PercentToCentipct(45.25),
			MemCentipct:   PercentToCentipct(72.10),
			DiskCentipct:  PercentToCentipct(88.90),
			StatusFlags:   FlagStatusOnline,
			LatencyMicros: 1250,
			PodCount:      42,
			Reserved:      0,
		},
		{
			NodeHash:      HashNodeID("node-us-east-1a-002"),
			CPUCentipct:   PercentToCentipct(98.50),
			MemCentipct:   PercentToCentipct(91.00),
			DiskCentipct:  PercentToCentipct(65.40),
			StatusFlags:   FlagStatusDegraded | FlagStatusDraining,
			LatencyMicros: 85400,
			PodCount:      110,
			Reserved:      0,
		},
	}

	raw, err := EncodeFrame(ts, records)
	if err != nil {
		t.Fatalf("EncodeFrame failed: %v", err)
	}

	// 24-byte header + 2 * 24-byte records = 72 bytes
	if len(raw) != 72 {
		t.Errorf("expected 72 bytes, got %d", len(raw))
	}

	decodedTs, decodedRecords, err := DecodeFrame(raw)
	if err != nil {
		t.Fatalf("DecodeFrame failed: %v", err)
	}

	if decodedTs != ts {
		t.Errorf("expected ts %d, got %d", ts, decodedTs)
	}
	if len(decodedRecords) != len(records) {
		t.Fatalf("expected %d records, got %d", len(records), len(decodedRecords))
	}

	for i := range records {
		if decodedRecords[i] != records[i] {
			t.Errorf("record %d mismatch: got %+v, expected %+v", i, decodedRecords[i], records[i])
		}
	}
}

func TestEncodeDecodeFrameZstd_RoundTrip(t *testing.T) {
	ts := int64(1700000000000)
	records := make([]NodeDeltaRecord, 50)
	for i := 0; i < 50; i++ {
		records[i] = NodeDeltaRecord{
			NodeHash:      uint64(i + 1),
			CPUCentipct:   uint16(i * 100),
			MemCentipct:   uint16(5000 + i*50),
			DiskCentipct:  4500,
			StatusFlags:   FlagStatusOnline,
			LatencyMicros: uint32(500 + i*10),
			PodCount:      uint16(20 + i),
			Reserved:      0,
		}
	}

	compressed, err := EncodeFrameZstd(ts, records)
	if err != nil {
		t.Fatalf("EncodeFrameZstd failed: %v", err)
	}

	decodedTs, decodedRecords, err := DecodeFrameZstd(compressed)
	if err != nil {
		t.Fatalf("DecodeFrameZstd failed: %v", err)
	}

	if decodedTs != ts {
		t.Errorf("expected ts %d, got %d", ts, decodedTs)
	}
	if len(decodedRecords) != len(records) {
		t.Fatalf("expected %d records, got %d", len(records), len(decodedRecords))
	}

	for i := range records {
		if decodedRecords[i] != records[i] {
			t.Fatalf("record %d mismatch: got %+v, expected %+v", i, decodedRecords[i], records[i])
		}
	}
}

func TestDecodeFrame_HeaderErrors(t *testing.T) {
	// Frame too short
	_, _, err := DecodeFrame([]byte{0x4B, 0x38})
	if err == nil {
		t.Error("expected error on truncated header, got nil")
	}

	// Corrupted magic
	badMagic := make([]byte, 24)
	badMagic[0] = 0xFF
	_, _, err = DecodeFrame(badMagic)
	if err == nil {
		t.Error("expected error on bad magic, got nil")
	}

	// Corrupted version
	badVersion := make([]byte, 24)
	badVersion[0] = 0x4B
	badVersion[1] = 0x38
	badVersion[2] = 0x53
	badVersion[3] = 0x44
	badVersion[4] = 99 // unsupported version
	_, _, err = DecodeFrame(badVersion)
	if err == nil {
		t.Error("expected error on bad version, got nil")
	}
}

func TestEncodeFrameZstd_2000Nodes_SizeConstraint(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	records := make([]NodeDeltaRecord, 2000)

	for i := 0; i < 2000; i++ {
		records[i] = NodeDeltaRecord{
			NodeHash:      uint64(100000 + i),
			CPUCentipct:   uint16(rng.Intn(10000)),
			MemCentipct:   uint16(4000 + rng.Intn(4000)),
			DiskCentipct:  uint16(2000 + rng.Intn(6000)),
			StatusFlags:   FlagStatusOnline,
			LatencyMicros: uint32(500 + rng.Intn(5000)),
			PodCount:      uint16(10 + rng.Intn(50)),
			Reserved:      0,
		}
	}

	raw, err := EncodeFrame(1700000000000, records)
	if err != nil {
		t.Fatalf("EncodeFrame failed: %v", err)
	}

	// Raw size: 24 header + 2000 * 24 = 48024 bytes (~46.9 KB)
	if len(raw) != 48024 {
		t.Errorf("expected 48024 bytes raw, got %d", len(raw))
	}

	compressed, err := EncodeFrameZstd(1700000000000, records)
	if err != nil {
		t.Fatalf("EncodeFrameZstd failed: %v", err)
	}

	t.Logf("2,000 nodes frame: raw=%d bytes, zstd=%d bytes (ratio: %.2fx)",
		len(raw), len(compressed), float64(len(raw))/float64(len(compressed)))

	// Assert compressed size < 25 KB requirement (< 25600 bytes) and < 50 KB requirement
	if len(compressed) >= 25*1024 {
		t.Errorf("compressed size %d bytes exceeded 25 KB target", len(compressed))
	}
}

func BenchmarkEncodeFrameZstd_2000Nodes(b *testing.B) {
	rng := rand.New(rand.NewSource(12345))
	records := make([]NodeDeltaRecord, 2000)
	for i := 0; i < 2000; i++ {
		records[i] = NodeDeltaRecord{
			NodeHash:      uint64(100000 + i),
			CPUCentipct:   uint16(rng.Intn(10000)),
			MemCentipct:   uint16(rng.Intn(10000)),
			DiskCentipct:  uint16(rng.Intn(10000)),
			StatusFlags:   FlagStatusOnline,
			LatencyMicros: uint32(rng.Intn(10000)),
			PodCount:      uint16(rng.Intn(100)),
			Reserved:      0,
		}
	}

	ts := time.Now().UnixMilli()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		compressed, err := EncodeFrameZstd(ts, records)
		if err != nil {
			b.Fatalf("EncodeFrameZstd failed: %v", err)
		}
		if len(compressed) >= 50*1024 {
			b.Fatalf("size %d exceeds 50 KB", len(compressed))
		}
	}
}

func BenchmarkDecodeFrameZstd_2000Nodes(b *testing.B) {
	rng := rand.New(rand.NewSource(12345))
	records := make([]NodeDeltaRecord, 2000)
	for i := 0; i < 2000; i++ {
		records[i] = NodeDeltaRecord{
			NodeHash:      uint64(100000 + i),
			CPUCentipct:   uint16(rng.Intn(10000)),
			MemCentipct:   uint16(rng.Intn(10000)),
			DiskCentipct:  uint16(rng.Intn(10000)),
			StatusFlags:   FlagStatusOnline,
			LatencyMicros: uint32(rng.Intn(10000)),
			PodCount:      uint16(rng.Intn(100)),
			Reserved:      0,
		}
	}

	ts := time.Now().UnixMilli()
	compressed, err := EncodeFrameZstd(ts, records)
	if err != nil {
		b.Fatalf("setup EncodeFrameZstd failed: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		decTs, decRecords, err := DecodeFrameZstd(compressed)
		if err != nil {
			b.Fatalf("DecodeFrameZstd failed: %v", err)
		}
		if decTs != ts || len(decRecords) != 2000 {
			b.Fatalf("invalid decoded output")
		}
	}
}
