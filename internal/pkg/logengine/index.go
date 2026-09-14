package logengine

import (
	"encoding/binary"
	"io"
	"math"
	"sort"
)

// BlockMarkSize is the fixed size of a block mark on disk (20 bytes).
const BlockMarkSize = 20

// BlockMark stores offsets and sizes for locating and reading a compressed block.
type BlockMark struct {
	CompressedOffset uint64 // 8 bytes
	CompressedLength uint32 // 4 bytes
	UncompressedSize uint32 // 4 bytes
	RowCount         uint32 // 4 bytes
}

// Encode writes the 20-byte binary representation of the mark to dst.
func (m *BlockMark) Encode(dst []byte) {
	binary.LittleEndian.PutUint64(dst[0:8], m.CompressedOffset)
	binary.LittleEndian.PutUint32(dst[8:12], m.CompressedLength)
	binary.LittleEndian.PutUint32(dst[12:16], m.UncompressedSize)
	binary.LittleEndian.PutUint32(dst[16:20], m.RowCount)
}

// Decode reads a 20-byte binary representation into the mark.
func (m *BlockMark) Decode(src []byte) {
	m.CompressedOffset = binary.LittleEndian.Uint64(src[0:8])
	m.CompressedLength = binary.LittleEndian.Uint32(src[8:12])
	m.UncompressedSize = binary.LittleEndian.Uint32(src[12:16])
	m.RowCount = binary.LittleEndian.Uint32(src[16:20])
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
