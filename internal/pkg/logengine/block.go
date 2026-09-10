package logengine

import (
	"encoding/binary"
	"fmt"

	"github.com/klauspost/compress/zstd"
)

var (
	zstdEncoder, _ = zstd.NewWriter(
		nil,
		zstd.WithEncoderLevel(zstd.SpeedFastest),
		zstd.WithEncoderConcurrency(1),
		zstd.WithWindowSize(BlockBufferSize),
	)
	zstdDecoder, _ = zstd.NewReader(
		nil,
		zstd.WithDecoderConcurrency(1),
		zstd.WithDecoderMaxWindow(BlockBufferSize),
	)
)

// CompressBlock compresses uncompressed block bytes into dst using ZSTD.
func CompressBlock(uncompressed, dst []byte) []byte {
	return zstdEncoder.EncodeAll(uncompressed, dst[:0])
}

// DecompressBlock decompresses compressed block bytes into dst using ZSTD.
func DecompressBlock(compressed, dst []byte) ([]byte, error) {
	return zstdDecoder.DecodeAll(compressed, dst[:0])
}

// BlockBuilder accumulates log rows and packs them into columnar format with Bloom filter.
type BlockBuilder struct {
	timestamps []int64
	serviceIDs []uint16
	levelIDs   []uint16
	msgLengths []uint32
	messages   []byte
	bloom      BlockBloomFilter
	minTime    int64
	maxTime    int64
}

// NewBlockBuilder initializes a block builder pre-allocated for ~2048 rows.
func NewBlockBuilder() *BlockBuilder {
	return &BlockBuilder{
		timestamps: make([]int64, 0, 2048),
		serviceIDs: make([]uint16, 0, 2048),
		levelIDs:   make([]uint16, 0, 2048),
		msgLengths: make([]uint32, 0, 2048),
		messages:   make([]byte, 0, 128*1024),
	}
}

// Append adds a row to the staging block builder.
func (b *BlockBuilder) Append(ts int64, serviceID, levelID uint16, msg []byte) {
	if len(b.timestamps) == 0 {
		b.minTime = ts
		b.maxTime = ts
	} else {
		if ts < b.minTime {
			b.minTime = ts
		}
		if ts > b.maxTime {
			b.maxTime = ts
		}
	}

	b.timestamps = append(b.timestamps, ts)
	b.serviceIDs = append(b.serviceIDs, serviceID)
	b.levelIDs = append(b.levelIDs, levelID)
	b.msgLengths = append(b.msgLengths, uint32(len(msg)))
	b.messages = append(b.messages, msg...)

	Tokenize(msg, func(tok []byte) {
		b.bloom.Add(tok)
	})
}

// RowCount returns the number of rows currently in the builder.
func (b *BlockBuilder) RowCount() int {
	return len(b.timestamps)
}

// EstimatedSize returns approximate uncompressed bytes of the block.
func (b *BlockBuilder) EstimatedSize() int {
	return len(b.timestamps)*8 + len(b.serviceIDs)*2 + len(b.levelIDs)*2 + len(b.msgLengths)*4 + len(b.messages) + 36
}

// Reset clears the builder for reuse without reallocating slices.
func (b *BlockBuilder) Reset() {
	b.timestamps = b.timestamps[:0]
	b.serviceIDs = b.serviceIDs[:0]
	b.levelIDs = b.levelIDs[:0]
	b.msgLengths = b.msgLengths[:0]
	b.messages = b.messages[:0]
	b.bloom.Reset()
	b.minTime = 0
	b.maxTime = 0
}

// Pack encodes the columnar components safely into dst, growing dst if capacity is exceeded.
func (b *BlockBuilder) Pack(dst []byte) (uncompressed []byte, minTime, maxTime int64, bloom *BlockBloomFilter) {
	count := len(b.timestamps)
	if count == 0 {
		return nil, 0, 0, &b.bloom
	}

	needed := 36 + (count * 18) + len(b.messages)
	if cap(dst) < needed {
		dst = make([]byte, needed)
	} else {
		dst = dst[:cap(dst)]
	}

	offset := 36
	ddBytes := EncodeDoubleDelta(b.timestamps, dst[offset:])
	offset += ddBytes

	for i := 0; i < count; i++ {
		binary.LittleEndian.PutUint16(dst[offset:offset+2], b.serviceIDs[i])
		binary.LittleEndian.PutUint16(dst[offset+2:offset+4], b.levelIDs[i])
		offset += 4
	}
	labelsLen := count * 4

	for i := 0; i < count; i++ {
		binary.LittleEndian.PutUint32(dst[offset:offset+4], b.msgLengths[i])
		offset += 4
	}
	offsetsLen := count * 4

	if offset+len(b.messages) > cap(dst) {
		grown := make([]byte, offset+len(b.messages))
		copy(grown, dst[:offset])
		dst = grown
	}
	copy(dst[offset:], b.messages)
	offset += len(b.messages)

	binary.LittleEndian.PutUint32(dst[0:4], uint32(count))
	binary.LittleEndian.PutUint64(dst[4:12], uint64(b.minTime))
	binary.LittleEndian.PutUint64(dst[12:20], uint64(b.maxTime))
	binary.LittleEndian.PutUint32(dst[20:24], uint32(ddBytes))
	binary.LittleEndian.PutUint32(dst[24:28], uint32(labelsLen))
	binary.LittleEndian.PutUint32(dst[28:32], uint32(offsetsLen))
	binary.LittleEndian.PutUint32(dst[32:36], uint32(len(b.messages)))

	return dst[:offset], b.minTime, b.maxTime, &b.bloom
}

// BlockView provides zero-copy columnar access to a decompressed block.
type BlockView struct {
	RowCount   int
	MinTime    int64
	MaxTime    int64
	Timestamps []int64
	ServiceIDs []uint16
	LevelIDs   []uint16
	MsgLengths []uint32
	rawMsgs    []byte
	msgIndices []int
}

// NewBlockView creates a reusable BlockView.
func NewBlockView() *BlockView {
	return &BlockView{
		Timestamps: make([]int64, 0, 2048),
		ServiceIDs: make([]uint16, 0, 2048),
		LevelIDs:   make([]uint16, 0, 2048),
		MsgLengths: make([]uint32, 0, 2048),
		msgIndices: make([]int, 0, 2048),
	}
}

// Unpack decodes an uncompressed block buffer into the view.
func (v *BlockView) Unpack(data []byte) error {
	if len(data) < 36 {
		return fmt.Errorf("block data too short (%d bytes)", len(data))
	}

	count := int(binary.LittleEndian.Uint32(data[0:4]))
	v.RowCount = count
	v.MinTime = int64(binary.LittleEndian.Uint64(data[4:12]))
	v.MaxTime = int64(binary.LittleEndian.Uint64(data[12:20]))
	ddLen := int(binary.LittleEndian.Uint32(data[20:24]))
	labelsLen := int(binary.LittleEndian.Uint32(data[24:28]))
	offsetsLen := int(binary.LittleEndian.Uint32(data[28:32]))
	msgLen := int(binary.LittleEndian.Uint32(data[32:36]))

	totalExpected := 36 + ddLen + labelsLen + offsetsLen + msgLen
	if len(data) < totalExpected {
		return fmt.Errorf("corrupted block: expected %d bytes, got %d", totalExpected, len(data))
	}

	offset := 36
	if cap(v.Timestamps) < count {
		v.Timestamps = make([]int64, count)
	} else {
		v.Timestamps = v.Timestamps[:count]
	}
	DecodeDoubleDelta(data[offset:offset+ddLen], count, v.Timestamps)
	offset += ddLen

	if cap(v.ServiceIDs) < count {
		v.ServiceIDs = make([]uint16, count)
		v.LevelIDs = make([]uint16, count)
	} else {
		v.ServiceIDs = v.ServiceIDs[:count]
		v.LevelIDs = v.LevelIDs[:count]
	}
	for i := 0; i < count; i++ {
		v.ServiceIDs[i] = binary.LittleEndian.Uint16(data[offset : offset+2])
		v.LevelIDs[i] = binary.LittleEndian.Uint16(data[offset+2 : offset+4])
		offset += 4
	}

	if cap(v.MsgLengths) < count {
		v.MsgLengths = make([]uint32, count)
		v.msgIndices = make([]int, count)
	} else {
		v.MsgLengths = v.MsgLengths[:count]
		v.msgIndices = v.msgIndices[:count]
	}
	for i := 0; i < count; i++ {
		v.MsgLengths[i] = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
	}

	v.rawMsgs = data[offset : offset+msgLen]
	cur := 0
	for i := 0; i < count; i++ {
		v.msgIndices[i] = cur
		cur += int(v.MsgLengths[i])
	}
	return nil
}

// MessageAt returns the raw message bytes of row i without allocations.
func (v *BlockView) MessageAt(i int) []byte {
	if i < 0 || i >= v.RowCount {
		return nil
	}
	start := v.msgIndices[i]
	end := start + int(v.MsgLengths[i])
	return v.rawMsgs[start:end]
}
