package logengine

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"sort"
	"strings"
)

// BlockMarkSize is the fixed size of a block mark on disk (32 bytes).
const BlockMarkSize = 32

// BlockMark stores offsets and sizes for locating and reading a compressed block.
type BlockMark struct {
	CompressedOffset uint64 // 8 bytes (offset 0..8)
	CompressedLength uint32 // 4 bytes (offset 8..12)
	UncompressedSize uint32 // 4 bytes (offset 12..16)
	RowCount         uint32 // 4 bytes (offset 16..20)
	LevelMask        uint16 // 2 bytes (offset 20..22)
	Reserved         uint16 // 2 bytes (offset 22..24)
	ServiceBloom     uint64 // 8 bytes (offset 24..32)
}

// Encode writes the 32-byte binary representation of the mark to dst.
func (m *BlockMark) Encode(dst []byte) {
	binary.LittleEndian.PutUint64(dst[0:8], m.CompressedOffset)
	binary.LittleEndian.PutUint32(dst[8:12], m.CompressedLength)
	binary.LittleEndian.PutUint32(dst[12:16], m.UncompressedSize)
	binary.LittleEndian.PutUint32(dst[16:20], m.RowCount)
	binary.LittleEndian.PutUint16(dst[20:22], m.LevelMask)
	binary.LittleEndian.PutUint16(dst[22:24], m.Reserved)
	binary.LittleEndian.PutUint64(dst[24:32], m.ServiceBloom)
}

// Decode reads a 32-byte binary representation into the mark.
func (m *BlockMark) Decode(src []byte) {
	m.CompressedOffset = binary.LittleEndian.Uint64(src[0:8])
	m.CompressedLength = binary.LittleEndian.Uint32(src[8:12])
	m.UncompressedSize = binary.LittleEndian.Uint32(src[12:16])
	m.RowCount = binary.LittleEndian.Uint32(src[16:20])
	m.LevelMask = binary.LittleEndian.Uint16(src[20:22])
	m.Reserved = binary.LittleEndian.Uint16(src[22:24])
	m.ServiceBloom = binary.LittleEndian.Uint64(src[24:32])
}

const (
	LevelBitTrace uint16 = 1 << 0
	LevelBitDebug uint16 = 1 << 1
	LevelBitInfo  uint16 = 1 << 2
	LevelBitWarn  uint16 = 1 << 3
	LevelBitError uint16 = 1 << 4
	LevelBitFatal uint16 = 1 << 5
	LevelBitOther uint16 = 1 << 6
)

const (
	LevelUnknown uint16 = 0
	LevelTrace   uint16 = 1
	LevelDebug   uint16 = 2
	LevelInfo    uint16 = 3
	LevelWarn    uint16 = 4
	LevelError   uint16 = 5
	LevelFatal   uint16 = 6
)

func ParseLevelID(lvl string) uint16 {
	switch strings.ToLower(strings.TrimSpace(lvl)) {
	case "trace":
		return LevelTrace
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error", "err":
		return LevelError
	case "fatal", "critical", "panic":
		return LevelFatal
	default:
		return LevelUnknown
	}
}

func LevelIDToString(id uint16) string {
	switch id {
	case LevelTrace:
		return "trace"
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	case LevelFatal:
		return "fatal"
	default:
		return "info"
	}
}

func LevelToBit(lvl string) uint16 {
	switch strings.ToLower(strings.TrimSpace(lvl)) {
	case "trace":
		return LevelBitTrace
	case "debug":
		return LevelBitDebug
	case "info":
		return LevelBitInfo
	case "warn", "warning":
		return LevelBitWarn
	case "error", "err":
		return LevelBitError
	case "fatal", "critical", "panic":
		return LevelBitFatal
	default:
		return LevelBitOther
	}
}

func LevelFilterMask(filter string) uint16 {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return 0xFFFF
	}
	switch filter {
	case "error", "err", "fatal", "critical":
		return LevelBitError | LevelBitFatal
	case "warn", "warning":
		return LevelBitWarn | LevelBitError | LevelBitFatal
	case "info":
		return LevelBitInfo | LevelBitWarn | LevelBitError | LevelBitFatal
	case "debug", "trace":
		return 0xFFFF
	default:
		return LevelBitOther | LevelBitInfo | LevelBitWarn | LevelBitError | LevelBitFatal
	}
}

func ServiceBloomBit(serviceID uint16) uint64 {
	// 64-bit FNV-1a hash mask of serviceID
	var h uint64 = 14695981039346656037
	h ^= uint64(byte(serviceID))
	h *= 1099511628211
	h ^= uint64(byte(serviceID >> 8))
	h *= 1099511628211
	b1 := uint64(1) << (h % 64)
	b2 := uint64(1) << ((h >> 6) % 64)
	return b1 | b2
}

// PrimaryIndexEntry stores the time boundaries of a block for sparse indexing.
type PrimaryIndexEntry struct {
	MinTime    int64
	MaxTime    int64
	BlockIndex int
}

// PrimaryIndex maintains an in-memory sparse index for fast binary search time pruning.
type PrimaryIndex struct {
	entries []PrimaryIndexEntry
}

// NewPrimaryIndex creates an empty PrimaryIndex.
func NewPrimaryIndex() *PrimaryIndex {
	return &PrimaryIndex{
		entries: make([]PrimaryIndexEntry, 0, 128),
	}
}

// Add appends a new block's time range to the primary index.
func (idx *PrimaryIndex) Add(minTime, maxTime int64) {
	idx.entries = append(idx.entries, PrimaryIndexEntry{
		MinTime:    minTime,
		MaxTime:    maxTime,
		BlockIndex: len(idx.entries),
	})
}

// Len returns the count of indexed blocks.
func (idx *PrimaryIndex) Len() int {
	return len(idx.entries)
}

// Search performs binary search to find candidate block indices intersecting [since, until].
func (idx *PrimaryIndex) Search(since, until int64) []int {
	n := len(idx.entries)
	if n == 0 {
		return nil
	}
	if since == 0 {
		since = math.MinInt64
	}
	if until == 0 {
		until = math.MaxInt64
	}

	// Find the first block that could contain timestamps >= since (i.e. MaxTime >= since)
	start := sort.Search(n, func(i int) bool {
		return idx.entries[i].MaxTime >= since
	})
	if start >= n {
		return nil
	}

	// Collect matching blocks where MinTime <= until
	var matched []int
	for i := start; i < n; i++ {
		if idx.entries[i].MinTime > until {
			// Because blocks are predominantly time-ordered, once MinTime exceeds until,
			// subsequent blocks are likely out of range as well.
			break
		}
		matched = append(matched, idx.entries[i].BlockIndex)
	}

	return matched
}

// WritePrimaryIndex serializes the primary index entries (16 bytes each) to w.
func (idx *PrimaryIndex) WritePrimaryIndex(w io.Writer) error {
	buf := make([]byte, 16)
	for _, e := range idx.entries {
		binary.LittleEndian.PutUint64(buf[0:8], uint64(e.MinTime))
		binary.LittleEndian.PutUint64(buf[8:16], uint64(e.MaxTime))
		if _, err := w.Write(buf); err != nil {
			return err
		}
	}
	return nil
}

// ReadPrimaryIndex loads primary index entries from r.
func (idx *PrimaryIndex) ReadPrimaryIndex(r io.Reader) error {
	buf := make([]byte, 16)
	idx.entries = idx.entries[:0]
	blockIdx := 0
	for {
		_, err := io.ReadFull(r, buf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return err
		}
		minT := int64(binary.LittleEndian.Uint64(buf[0:8]))
		maxT := int64(binary.LittleEndian.Uint64(buf[8:16]))
		idx.entries = append(idx.entries, PrimaryIndexEntry{
			MinTime:    minT,
			MaxTime:    maxT,
			BlockIndex: blockIdx,
		})
		blockIdx++
	}
	return nil
}

func matchFilter(actual, pattern string) bool {
	if strings.EqualFold(actual, pattern) {
		return true
	}
	if len(pattern) <= 3 {
		return false
	}
	actLower := strings.ToLower(actual)
	patLower := strings.ToLower(pattern)
	if strings.HasPrefix(actLower, patLower) {
		if len(actLower) > len(patLower) && isBoundary(actLower[len(patLower)]) {
			return true
		}
	}
	if strings.HasSuffix(actLower, patLower) {
		if len(actLower) > len(patLower) && isBoundary(actLower[len(actLower)-len(patLower)-1]) {
			return true
		}
	}
	return false
}

func isBoundary(b byte) bool {
	return b == '-' || b == '_' || b == '.' || b == '/' || b == ':' || (b < 'a' || b > 'z') && (b < '0' || b > '9')
}

func matchLevel(actual, filter string) bool {
	filter = strings.TrimSpace(strings.ToLower(filter))
	if filter == "" {
		return true
	}
	actual = strings.TrimSpace(strings.ToLower(actual))
	switch filter {
	case "error", "err", "fatal", "critical":
		return actual == "error" || actual == "fatal" || actual == "critical"
	case "warn", "warning":
		return actual == "error" || actual == "fatal" || actual == "critical" || actual == "warn"
	case "info":
		return actual == "error" || actual == "warn" || actual == "info"
	case "debug", "trace":
		return true
	default:
		return actual == filter || strings.Contains(actual, filter)
	}
}

func bytesContainsFold(b []byte, subLower string) bool {
	n := len(subLower)
	if n == 0 {
		return true
	}
	if len(b) < n {
		return false
	}
	f0L, f0U := subLower[0], subLower[0]
	if f0L >= 'a' && f0L <= 'z' {
		f0U -= 32
	} else if f0L >= 'A' && f0L <= 'Z' {
		f0L += 32
	}
	limit := len(b) - n
	for offset := 0; offset <= limit; {
		i1, i2 := bytes.IndexByte(b[offset:], f0L), -1
		if f0L != f0U {
			i2 = bytes.IndexByte(b[offset:], f0U)
		}
		cand := i1
		if i1 == -1 || (i2 != -1 && i2 < i1) {
			cand = i2
		}
		if cand == -1 || offset+cand > limit {
			return false
		}
		pos := offset + cand
		match := true
		for j := 1; j < n; j++ {
			c := b[pos+j]
			if c >= 'A' && c <= 'Z' {
				c += 32
			}
			sc := subLower[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if c != sc {
				match = false
				break
			}
		}
		if match {
			return true
		}
		offset = pos + 1
	}
	return false
}

