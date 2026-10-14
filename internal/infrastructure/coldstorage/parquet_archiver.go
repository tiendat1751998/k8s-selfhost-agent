// Package coldstorage implements a Three-Tier Time-Series Lifecycle Rollup engine
// and Snappy-compressed Columnar Cold Storage Archiver with Zone-Map predicate pushdown.
package coldstorage

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/cespare/xxhash/v2"
	"github.com/datdt/k8sselfhost/internal/infrastructure/victoriametrics"
	"github.com/klauspost/compress/snappy"
)

const (
	// MagicBlock identifies cold storage blocks: "COLD" (0x434F4C44).
	MagicBlock uint32 = 0x434F4C44
	// BlockVersion represents the columnar format version.
	BlockVersion uint16 = 1
	// FixedHeaderSize is the size of the header excluding variable-length tenant string.
	FixedHeaderSize = 44
)

var (
	// ErrInvalidBlockHeader is returned when the block magic or header is malformed.
	ErrInvalidBlockHeader = errors.New("coldstorage: invalid block header")
	// ErrUnsupportedBlockVersion is returned when the block version is not supported.
	ErrUnsupportedBlockVersion = errors.New("coldstorage: unsupported block version")
	// ErrCorruptedBlockChecksum is returned when the block checksum fails verification.
	ErrCorruptedBlockChecksum = errors.New("coldstorage: corrupted block checksum")
)

// MetricSample aliases the TSDB metric sample for unified cross-tier telemetry.
type MetricSample = victoriametrics.MetricSample

// RollupRecord represents downsampled 1-minute historical telemetry for a compute node.
type RollupRecord struct {
	TenantID          string  `json:"tenant_id"`
	ClusterID         string  `json:"cluster_id"`
	NodeID            string  `json:"node_id"`
	BucketTimestampMs int64   `json:"bucket_timestamp_ms"`
	AvgCPU            float64 `json:"avg_cpu"`
	MinCPU            float64 `json:"min_cpu"`
	MaxCPU            float64 `json:"max_cpu"`
	P95CPU            float64 `json:"p95_cpu"`
	AvgMem            float64 `json:"avg_mem"`
	MaxMem            float64 `json:"max_mem"`
	AvgDisk           float64 `json:"avg_disk"`
	SampleCount       int     `json:"sample_count"`
}

// ZoneMapMetadata stores block-level index bounds enabling O(1) predicate pushdown.
type ZoneMapMetadata struct {
	TenantID         string
	MinTimestampMs   int64
	MaxTimestampMs   int64
	RowCount         uint32
	UncompressedSize uint32
	CompressedSize   uint32
	XXHash64Checksum uint64
	HeaderLength     int
}

// SnappyColumnarArchiver manages encoding, decoding, and querying of cold partition blocks.
type SnappyColumnarArchiver struct{}

// NewSnappyColumnarArchiver creates a new archiver instance.
func NewSnappyColumnarArchiver() *SnappyColumnarArchiver {
	return &SnappyColumnarArchiver{}
}

type rollupGroup struct {
	tenantID string
	clusterID string
	nodeID    string
	bucketMs  int64
	cpuVals   []float64
	memVals   []float64
	diskVals  []float64
}

// Rollup1Min computes 1-minute downsampled rollups (avg, min, max, p95) per (tenant, cluster, node, minute).
func Rollup1Min(samples []MetricSample) []RollupRecord {
	if len(samples) == 0 {
		return nil
	}

	groups := make(map[string]*rollupGroup)
	var keys []string

	for i := range samples {
		s := &samples[i]
		bucketMs := (s.TimestampMs / 60000) * 60000
		key := s.TenantID + "\x00" + s.ClusterID + "\x00" + s.NodeID + "\x00" + strconv.FormatInt(bucketMs, 10)

		g, exists := groups[key]
		if !exists {
			g = &rollupGroup{
				tenantID: s.TenantID,
				clusterID: s.ClusterID,
				nodeID:    s.NodeID,
				bucketMs:  bucketMs,
				cpuVals:   make([]float64, 0, 12),
				memVals:   make([]float64, 0, 12),
				diskVals:  make([]float64, 0, 12),
			}
			groups[key] = g
			keys = append(keys, key)
		}
		g.cpuVals = append(g.cpuVals, s.CPUPercent)
		g.memVals = append(g.memVals, s.MemPercent)
		g.diskVals = append(g.diskVals, s.DiskPercent)
	}

	out := make([]RollupRecord, 0, len(keys))
	for _, key := range keys {
		g := groups[key]
		n := len(g.cpuVals)
		if n == 0 {
			continue
		}

		var sumCPU, minCPU, maxCPU float64
		var sumMem, maxMem float64
		var sumDisk float64

		minCPU = g.cpuVals[0]
		maxCPU = g.cpuVals[0]
		maxMem = g.memVals[0]

		for i := 0; i < n; i++ {
			c := g.cpuVals[i]
			m := g.memVals[i]
			d := g.diskVals[i]

			sumCPU += c
			sumMem += m
			sumDisk += d

			if c < minCPU {
				minCPU = c
			}
			if c > maxCPU {
				maxCPU = c
			}
			if m > maxMem {
				maxMem = m
			}
		}

		sort.Float64s(g.cpuVals)
		p95Idx := int(math.Ceil(0.95*float64(n))) - 1
		if p95Idx < 0 {
			p95Idx = 0
		} else if p95Idx >= n {
			p95Idx = n - 1
		}
		p95CPU := g.cpuVals[p95Idx]

		out = append(out, RollupRecord{
			TenantID:          g.tenantID,
			ClusterID:         g.clusterID,
			NodeID:            g.nodeID,
			BucketTimestampMs: g.bucketMs,
			AvgCPU:            sumCPU / float64(n),
			MinCPU:            minCPU,
			MaxCPU:            maxCPU,
			P95CPU:            p95CPU,
			AvgMem:            sumMem / float64(n),
			MaxMem:            maxMem,
			AvgDisk:           sumDisk / float64(n),
			SampleCount:       n,
		})
	}

	return out
}

// EncodeBlock serializes rollup records into Snappy-compressed columnar format with Zone-Map header.
func (a *SnappyColumnarArchiver) EncodeBlock(records []RollupRecord) ([]byte, error) {
	if len(records) == 0 {
		return nil, nil
	}

	tenantID := records[0].TenantID
	minTs := records[0].BucketTimestampMs
	maxTs := records[0].BucketTimestampMs

	for i := 1; i < len(records); i++ {
		ts := records[i].BucketTimestampMs
		if ts < minTs {
			minTs = ts
		}
		if ts > maxTs {
			maxTs = ts
		}
	}

	var colBuf bytes.Buffer
	// Reserve estimated capacity: ~80 bytes per row
	colBuf.Grow(len(records) * 80)

	// Column 1: Identifiers (ClusterID, NodeID)
	for i := range records {
		r := &records[i]
		writeLenString(&colBuf, r.ClusterID)
		writeLenString(&colBuf, r.NodeID)
	}

	// Column 2: BucketTimestampMs (int64)
	for i := range records {
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], uint64(records[i].BucketTimestampMs))
		colBuf.Write(b[:])
	}

	// Column 3..9: float64 metrics
	writeFloats(&colBuf, records, func(r *RollupRecord) float64 { return r.AvgCPU })
	writeFloats(&colBuf, records, func(r *RollupRecord) float64 { return r.MinCPU })
	writeFloats(&colBuf, records, func(r *RollupRecord) float64 { return r.MaxCPU })
	writeFloats(&colBuf, records, func(r *RollupRecord) float64 { return r.P95CPU })
	writeFloats(&colBuf, records, func(r *RollupRecord) float64 { return r.AvgMem })
	writeFloats(&colBuf, records, func(r *RollupRecord) float64 { return r.MaxMem })
	writeFloats(&colBuf, records, func(r *RollupRecord) float64 { return r.AvgDisk })

	// Column 10: SampleCount (uint32)
	for i := range records {
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], uint32(records[i].SampleCount))
		colBuf.Write(b[:])
	}

	uncompressed := colBuf.Bytes()
	compressed := snappy.Encode(nil, uncompressed)
	checksum := xxhash.Sum64(compressed)

	tenantBytes := []byte(tenantID)
	headerLen := FixedHeaderSize + len(tenantBytes)
	out := make([]byte, headerLen+len(compressed))

	// Layout:
	// [0:4]   Magic (0x434F4C44)
	// [4:6]   Version (1)
	// [6:8]   TenantLen (uint16)
	// [8 : 8+len(tenant)] TenantID string
	// [...]   MinTimestampMs (int64)
	// [...]   MaxTimestampMs (int64)
	// [...]   RowCount (uint32)
	// [...]   UncompressedSize (uint32)
	// [...]   CompressedSize (uint32)
	// [...]   XXHash64Checksum (uint64)
	binary.BigEndian.PutUint32(out[0:4], MagicBlock)
	binary.BigEndian.PutUint16(out[4:6], BlockVersion)
	binary.BigEndian.PutUint16(out[6:8], uint16(len(tenantBytes)))
	copy(out[8:8+len(tenantBytes)], tenantBytes)

	pos := 8 + len(tenantBytes)
	binary.BigEndian.PutUint64(out[pos:pos+8], uint64(minTs))
	pos += 8
	binary.BigEndian.PutUint64(out[pos:pos+8], uint64(maxTs))
	pos += 8
	binary.BigEndian.PutUint32(out[pos:pos+4], uint32(len(records)))
	pos += 4
	binary.BigEndian.PutUint32(out[pos:pos+4], uint32(len(uncompressed)))
	pos += 4
	binary.BigEndian.PutUint32(out[pos:pos+4], uint32(len(compressed)))
	pos += 4
	binary.BigEndian.PutUint64(out[pos:pos+8], checksum)
	pos += 8

	copy(out[pos:], compressed)

	return out, nil
}

// InspectZoneMap extracts block metadata in O(1) time without decompressing the payload.
func (a *SnappyColumnarArchiver) InspectZoneMap(block []byte) (*ZoneMapMetadata, error) {
	if len(block) < FixedHeaderSize {
		return nil, ErrInvalidBlockHeader
	}

	magic := binary.BigEndian.Uint32(block[0:4])
	if magic != MagicBlock {
		return nil, ErrInvalidBlockHeader
	}

	version := binary.BigEndian.Uint16(block[4:6])
	if version != BlockVersion {
		return nil, fmt.Errorf("%w: got %d", ErrUnsupportedBlockVersion, version)
	}

	tenantLen := int(binary.BigEndian.Uint16(block[6:8]))
	expectedHeaderLen := FixedHeaderSize + tenantLen
	if len(block) < expectedHeaderLen {
		return nil, ErrInvalidBlockHeader
	}

	tenantID := string(block[8 : 8+tenantLen])
	pos := 8 + tenantLen

	minTs := int64(binary.BigEndian.Uint64(block[pos : pos+8]))
	pos += 8
	maxTs := int64(binary.BigEndian.Uint64(block[pos : pos+8]))
	pos += 8
	rowCount := binary.BigEndian.Uint32(block[pos : pos+4])
	pos += 4
	uncompressedSize := binary.BigEndian.Uint32(block[pos : pos+4])
	pos += 4
	compressedSize := binary.BigEndian.Uint32(block[pos : pos+4])
	pos += 4
	checksum := binary.BigEndian.Uint64(block[pos : pos+8])
	pos += 8

	return &ZoneMapMetadata{
		TenantID:         tenantID,
		MinTimestampMs:   minTs,
		MaxTimestampMs:   maxTs,
		RowCount:         rowCount,
		UncompressedSize: uncompressedSize,
		CompressedSize:   compressedSize,
		XXHash64Checksum: checksum,
		HeaderLength:     expectedHeaderLen,
	}, nil
}

// DecodeBlock validates the checksum, decompresses Snappy columnar data, and reconstructs records.
func (a *SnappyColumnarArchiver) DecodeBlock(block []byte) ([]RollupRecord, error) {
	meta, err := a.InspectZoneMap(block)
	if err != nil {
		return nil, err
	}

	compressedPayload := block[meta.HeaderLength:]
	if uint32(len(compressedPayload)) != meta.CompressedSize {
		return nil, ErrInvalidBlockHeader
	}

	actualChecksum := xxhash.Sum64(compressedPayload)
	if actualChecksum != meta.XXHash64Checksum {
		return nil, fmt.Errorf("%w: expected %x, got %x", ErrCorruptedBlockChecksum, meta.XXHash64Checksum, actualChecksum)
	}

	uncompressed, err := snappy.Decode(nil, compressedPayload)
	if err != nil {
		return nil, fmt.Errorf("snappy decompression error: %w", err)
	}

	count := int(meta.RowCount)
	records := make([]RollupRecord, count)
	for i := 0; i < count; i++ {
		records[i].TenantID = meta.TenantID
	}

	pos := 0
	// Column 1: Identifiers
	for i := 0; i < count; i++ {
		clusterID, n1 := readLenString(uncompressed[pos:])
		pos += n1
		nodeID, n2 := readLenString(uncompressed[pos:])
		pos += n2
		records[i].ClusterID = clusterID
		records[i].NodeID = nodeID
	}

	// Column 2: Timestamps
	for i := 0; i < count; i++ {
		records[i].BucketTimestampMs = int64(binary.BigEndian.Uint64(uncompressed[pos : pos+8]))
		pos += 8
	}

	// Column 3..9: float64 metrics
	readFloats(uncompressed, &pos, records, func(r *RollupRecord, val float64) { r.AvgCPU = val })
	readFloats(uncompressed, &pos, records, func(r *RollupRecord, val float64) { r.MinCPU = val })
	readFloats(uncompressed, &pos, records, func(r *RollupRecord, val float64) { r.MaxCPU = val })
	readFloats(uncompressed, &pos, records, func(r *RollupRecord, val float64) { r.P95CPU = val })
	readFloats(uncompressed, &pos, records, func(r *RollupRecord, val float64) { r.AvgMem = val })
	readFloats(uncompressed, &pos, records, func(r *RollupRecord, val float64) { r.MaxMem = val })
	readFloats(uncompressed, &pos, records, func(r *RollupRecord, val float64) { r.AvgDisk = val })

	// Column 10: SampleCount
	for i := 0; i < count; i++ {
		records[i].SampleCount = int(binary.BigEndian.Uint32(uncompressed[pos : pos+4]))
		pos += 4
	}

	return records, nil
}

// QueryColdBlocks applies Zone-Map pushdown to skip non-matching blocks in O(1) without decompression.
func (a *SnappyColumnarArchiver) QueryColdBlocks(ctx context.Context, blocks [][]byte, tenantID string, minTs, maxTs int64) ([]RollupRecord, error) {
	if len(blocks) == 0 {
		return nil, nil
	}

	var results []RollupRecord

	for _, block := range blocks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		meta, err := a.InspectZoneMap(block)
		if err != nil {
			return nil, err
		}

		// Predicate 1: Tenant Isolation Pushdown (O(1))
		if meta.TenantID != tenantID {
			continue
		}

		// Predicate 2: Time-Series Range Overlap Pushdown (O(1))
		if meta.MaxTimestampMs < minTs || meta.MinTimestampMs > maxTs {
			continue
		}

		// Matching block: decompress and extract records
		records, err := a.DecodeBlock(block)
		if err != nil {
			return nil, err
		}

		for i := range records {
			r := &records[i]
			if r.BucketTimestampMs >= minTs && r.BucketTimestampMs <= maxTs {
				results = append(results, *r)
			}
		}
	}

	return results, nil
}

func writeLenString(buf *bytes.Buffer, s string) {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], uint16(len(s)))
	buf.Write(b[:])
	buf.WriteString(s)
}

func readLenString(src []byte) (string, int) {
	strLen := int(binary.BigEndian.Uint16(src[0:2]))
	return string(src[2 : 2+strLen]), 2 + strLen
}

func writeFloats(buf *bytes.Buffer, records []RollupRecord, extract func(*RollupRecord) float64) {
	var b [8]byte
	for i := range records {
		val := extract(&records[i])
		binary.BigEndian.PutUint64(b[:], math.Float64bits(val))
		buf.Write(b[:])
	}
}

func readFloats(src []byte, pos *int, records []RollupRecord, assign func(*RollupRecord, float64)) {
	p := *pos
	for i := range records {
		bits := binary.BigEndian.Uint64(src[p : p+8])
		assign(&records[i], math.Float64frombits(bits))
		p += 8
	}
	*pos = p
}
