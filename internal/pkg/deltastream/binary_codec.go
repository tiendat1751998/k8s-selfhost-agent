// Package deltastream implements a compact binary WebSocket codec for streaming
// node telemetry updates. It utilizes a 24-byte binary record format combined with
// fast ZSTD compression to shrink multi-thousand node updates to < 25 KB.
package deltastream

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/cespare/xxhash/v2"
	"github.com/klauspost/compress/zstd"
)

const (
	// MagicHeader identifies the K8s Selfhost Delta frame: "K8SD" (0x4B385344).
	MagicHeader uint32 = 0x4B385344
	// CodecVersion represents the wire format schema version.
	CodecVersion uint8 = 1
	// HeaderSize is the fixed size of the frame header in bytes.
	HeaderSize = 24
	// RecordSize is the fixed size of each node delta record in bytes.
	RecordSize = 24
)

// Status flag bitmasks for NodeDeltaRecord.
const (
	FlagStatusOnline   uint16 = 1 << 0 // Node is healthy and responding
	FlagStatusDegraded uint16 = 1 << 1 // High latency or metric warnings
	FlagStatusOffline  uint16 = 1 << 2 // Node unreachable
	FlagStatusCordoned uint16 = 1 << 3 // Node scheduling disabled
	FlagStatusDraining uint16 = 1 << 4 // Node workloads being evicted
)

var (
	// ErrCorruptFrame is returned when the frame is truncated or invalid.
	ErrCorruptFrame = errors.New("deltastream: corrupt or truncated frame")
	// ErrInvalidMagic is returned when the frame does not start with MagicHeader.
	ErrInvalidMagic = errors.New("deltastream: invalid magic header")
	// ErrUnsupportedVersion is returned when the wire version is not supported.
	ErrUnsupportedVersion = errors.New("deltastream: unsupported codec version")
)

// Shared thread-safe ZSTD encoder and decoder instances.
var (
	zstdEncoder, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest))
	zstdDecoder, _ = zstd.NewReader(nil)
)

// NodeDeltaRecord is a compact 24-byte cache-line friendly binary record representing
// a single compute node's real-time telemetry delta.
type NodeDeltaRecord struct {
	NodeHash      uint64 // 8 bytes: xxhash64 of the node identifier
	CPUCentipct   uint16 // 2 bytes: CPU utilization in basis points (0 - 10000 = 0.00% - 100.00%)
	MemCentipct   uint16 // 2 bytes: Memory utilization in basis points (0 - 10000)
	DiskCentipct  uint16 // 2 bytes: Disk root utilization in basis points (0 - 10000)
	StatusFlags   uint16 // 2 bytes: Bitflags for node state
	LatencyMicros uint32 // 4 bytes: Ping / probe round-trip latency in microseconds
	PodCount      uint16 // 2 bytes: Running pod count on this node
	Reserved      uint16 // 2 bytes: Padding / reserved for future flags
}

// HashNodeID computes a 64-bit fast non-cryptographic hash for node identification.
func HashNodeID(nodeID string) uint64 {
	return xxhash.Sum64String(nodeID)
}

// PercentToCentipct converts a float percentage (e.g. 75.25%) to centi-percent (7525).
func PercentToCentipct(p float64) uint16 {
	if p <= 0 {
		return 0
	}
	if p >= 655.35 {
		return 65535
	}
	return uint16(math.Round(p * 100))
}

// CentipctToPercent converts centi-percent (e.g. 7525) back to float percentage (75.25).
func CentipctToPercent(c uint16) float64 {
	return float64(c) / 100.0
}

// EncodeFrame serializes a batch of NodeDeltaRecord into a raw binary frame.
// Frame Header layout (24 bytes):
// [0:4]   Magic (0x4B385344)
// [4]     Version (1)
// [5]     Flags (0)
// [6:8]   Reserved (0)
// [8:16]  TimestampMs (int64)
// [16:20] RecordCount (uint32)
// [20:24] Reserved2 (uint32)
func EncodeFrame(timestampMs int64, records []NodeDeltaRecord) ([]byte, error) {
	totalLen := HeaderSize + len(records)*RecordSize
	buf := make([]byte, totalLen)

	// Write 24-byte header
	binary.BigEndian.PutUint32(buf[0:4], MagicHeader)
	buf[4] = CodecVersion
	buf[5] = 0 // Flags
	binary.BigEndian.PutUint16(buf[6:8], 0)
	binary.BigEndian.PutUint64(buf[8:16], uint64(timestampMs))
	binary.BigEndian.PutUint32(buf[16:20], uint32(len(records)))
	binary.BigEndian.PutUint32(buf[20:24], 0)

	// Write 24-byte records
	offset := HeaderSize
	for i := range records {
		r := &records[i]
		binary.BigEndian.PutUint64(buf[offset:offset+8], r.NodeHash)
		binary.BigEndian.PutUint16(buf[offset+8:offset+10], r.CPUCentipct)
		binary.BigEndian.PutUint16(buf[offset+10:offset+12], r.MemCentipct)
		binary.BigEndian.PutUint16(buf[offset+12:offset+14], r.DiskCentipct)
		binary.BigEndian.PutUint16(buf[offset+14:offset+16], r.StatusFlags)
		binary.BigEndian.PutUint32(buf[offset+16:offset+20], r.LatencyMicros)
		binary.BigEndian.PutUint16(buf[offset+20:offset+22], r.PodCount)
		binary.BigEndian.PutUint16(buf[offset+22:offset+24], r.Reserved)
		offset += RecordSize
	}

	return buf, nil
}

// DecodeFrame deserializes a raw binary frame into timestampMs and records slice.
func DecodeFrame(raw []byte) (int64, []NodeDeltaRecord, error) {
	if len(raw) < HeaderSize {
		return 0, nil, ErrCorruptFrame
	}

	magic := binary.BigEndian.Uint32(raw[0:4])
	if magic != MagicHeader {
		return 0, nil, ErrInvalidMagic
	}

	version := raw[4]
	if version != CodecVersion {
		return 0, nil, fmt.Errorf("%w: version=%d", ErrUnsupportedVersion, version)
	}

	timestampMs := int64(binary.BigEndian.Uint64(raw[8:16]))
	recordCount := int(binary.BigEndian.Uint32(raw[16:20]))

	expectedSize := HeaderSize + recordCount*RecordSize
	if len(raw) < expectedSize {
		return 0, nil, io.ErrUnexpectedEOF
	}

	records := make([]NodeDeltaRecord, recordCount)
	offset := HeaderSize
	for i := 0; i < recordCount; i++ {
		records[i] = NodeDeltaRecord{
			NodeHash:      binary.BigEndian.Uint64(raw[offset : offset+8]),
			CPUCentipct:   binary.BigEndian.Uint16(raw[offset+8 : offset+10]),
			MemCentipct:   binary.BigEndian.Uint16(raw[offset+10 : offset+12]),
			DiskCentipct:  binary.BigEndian.Uint16(raw[offset+12 : offset+14]),
			StatusFlags:   binary.BigEndian.Uint16(raw[offset+14 : offset+16]),
			LatencyMicros: binary.BigEndian.Uint32(raw[offset+16 : offset+20]),
			PodCount:      binary.BigEndian.Uint16(raw[offset+20 : offset+22]),
			Reserved:      binary.BigEndian.Uint16(raw[offset+22 : offset+24]),
		}
		offset += RecordSize
	}

	return timestampMs, records, nil
}

// EncodeFrameZstd serializes the records and compresses the frame using ZSTD.
func EncodeFrameZstd(timestampMs int64, records []NodeDeltaRecord) ([]byte, error) {
	raw, err := EncodeFrame(timestampMs, records)
	if err != nil {
		return nil, err
	}
	compressed := zstdEncoder.EncodeAll(raw, make([]byte, 0, len(raw)/2))
	return compressed, nil
}

// DecodeFrameZstd decompresses a ZSTD frame and parses the records.
func DecodeFrameZstd(compressed []byte) (int64, []NodeDeltaRecord, error) {
	decompressed, err := zstdDecoder.DecodeAll(compressed, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("zstd decompression error: %w", err)
	}
	return DecodeFrame(decompressed)
}
