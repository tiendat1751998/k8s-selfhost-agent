package logengine

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/klauspost/compress/zstd"
)

const ZstdWindowSize = 128 * 1024

var (
	zstdEncoder, _ = zstd.NewWriter(
		nil,
		zstd.WithEncoderLevel(zstd.SpeedFastest),
		zstd.WithEncoderConcurrency(1),
		zstd.WithWindowSize(ZstdWindowSize),
	)
	zstdDecoder, _ = zstd.NewReader(
		nil,
		zstd.WithDecoderConcurrency(1),
		zstd.WithDecoderMaxWindow(ZstdWindowSize),
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

const BlockMagic uint32 = 0x4C4F4742 // "LOGB"

// BlockBuilder accumulates log rows and packs them into columnar format with Bloom filter.
type BlockBuilder struct {
	timestamps   []int64
	serviceIDs   []uint16
	levelIDs     []uint16
	msgLengths   []uint32
	messages     []byte
	bloom        BlockBloomFilter
	minTime      int64
	maxTime      int64
	levelMask    uint16
	serviceBloom uint64
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

	b.levelMask |= LevelToBit(LevelIDToString(levelID))
	b.serviceBloom |= ServiceBloomBit(serviceID)

	Tokenize(msg, func(tok []byte) {
		b.bloom.Add(tok)
		if bytes.IndexByte(tok, '_') != -1 {
			start := 0
			for i := 0; i <= len(tok); i++ {
				if i == len(tok) || tok[i] == '_' {
					if i-start >= 2 {
						b.bloom.Add(tok[start:i])
					}
					start = i + 1
				}
			}
		}
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
	b.levelMask = 0
	b.serviceBloom = 0
}

func (b *BlockBuilder) LevelMask() uint16 {
	return b.levelMask
}

func (b *BlockBuilder) ServiceBloom() uint64 {
	return b.serviceBloom
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

// PackAndCompress serializes columnar data with late materialization (meta and messages compressed separately).
func (b *BlockBuilder) PackAndCompress(scratchBuf, compBuf []byte) (compressed []byte, uncompressedSize int, minTime, maxTime int64, bloom *BlockBloomFilter, levelMask uint16, serviceBloom uint64) {
	count := len(b.timestamps)
	if count == 0 {
		return nil, 0, 0, 0, &b.bloom, 0, 0
	}

	neededMeta := 24 + (count * 18)
	if cap(scratchBuf) < neededMeta {
		scratchBuf = make([]byte, neededMeta)
	} else {
		scratchBuf = scratchBuf[:cap(scratchBuf)]
	}

	offset := 24
	ddBytes := EncodeDoubleDelta(b.timestamps, scratchBuf[offset:])
	offset += ddBytes

	for i := 0; i < count; i++ {
		binary.LittleEndian.PutUint16(scratchBuf[offset:offset+2], b.serviceIDs[i])
		binary.LittleEndian.PutUint16(scratchBuf[offset+2:offset+4], b.levelIDs[i])
		offset += 4
	}

	for i := 0; i < count; i++ {
		binary.LittleEndian.PutUint32(scratchBuf[offset:offset+4], b.msgLengths[i])
		offset += 4
	}

	binary.LittleEndian.PutUint32(scratchBuf[0:4], uint32(count))
	binary.LittleEndian.PutUint64(scratchBuf[4:12], uint64(b.minTime))
	binary.LittleEndian.PutUint64(scratchBuf[12:20], uint64(b.maxTime))
	binary.LittleEndian.PutUint32(scratchBuf[20:24], uint32(ddBytes))

	metaUncomp := scratchBuf[:offset]
	compMeta := zstdEncoder.EncodeAll(metaUncomp, nil)

	var compMsg []byte
	if len(b.messages) > 0 {
		compMsg = zstdEncoder.EncodeAll(b.messages, nil)
	}

	totalCompLen := 24 + len(compMeta) + len(compMsg)
	if cap(compBuf) < totalCompLen {
		compBuf = make([]byte, totalCompLen)
	} else {
		compBuf = compBuf[:totalCompLen]
	}

	binary.LittleEndian.PutUint32(compBuf[0:4], BlockMagic)
	binary.LittleEndian.PutUint32(compBuf[4:8], uint32(len(compMeta)))
	binary.LittleEndian.PutUint32(compBuf[8:12], uint32(len(compMsg)))
	binary.LittleEndian.PutUint32(compBuf[12:16], uint32(len(metaUncomp)))
	binary.LittleEndian.PutUint32(compBuf[16:20], uint32(len(b.messages)))
	binary.LittleEndian.PutUint32(compBuf[20:24], uint32(count))

	copy(compBuf[24:], compMeta)
	copy(compBuf[24+len(compMeta):], compMsg)

	totalUncompressed := len(metaUncomp) + len(b.messages) + 24
	return compBuf[:totalCompLen], totalUncompressed, b.minTime, b.maxTime, &b.bloom, b.levelMask, b.serviceBloom
}

// BlockView provides zero-copy columnar access to a decompressed block.
type BlockView struct {
	RowCount     int
	MinTime      int64
	MaxTime      int64
	Timestamps   []int64
	ServiceIDs   []uint16
	LevelIDs     []uint16
	MsgLengths   []uint32
	rawMsgs      []byte
	msgIndices   []int
	compMsgSlice []byte
	msgsReady    bool
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

// UnpackMeta decompresses and unpacks ONLY metadata columns for Late Materialization.
func (v *BlockView) UnpackMeta(data []byte, scratch []byte) error {
	v.msgsReady = false
	v.compMsgSlice = nil

	if len(data) >= 24 && binary.LittleEndian.Uint32(data[0:4]) == BlockMagic {
		metaCompLen := int(binary.LittleEndian.Uint32(data[4:8]))
		msgCompLen := int(binary.LittleEndian.Uint32(data[8:12]))

		if len(data) < 24+metaCompLen+msgCompLen {
			return fmt.Errorf("block truncated: expected %d bytes, got %d", 24+metaCompLen+msgCompLen, len(data))
		}

		compMeta := data[24 : 24+metaCompLen]
		v.compMsgSlice = data[24+metaCompLen : 24+metaCompLen+msgCompLen]

		uncompMeta, err := zstdDecoder.DecodeAll(compMeta, scratch[:0])
		if err != nil {
			return fmt.Errorf("decompressing meta frame: %w", err)
		}

		if len(uncompMeta) < 24 {
			return fmt.Errorf("meta frame too short (%d bytes)", len(uncompMeta))
		}

		count := int(binary.LittleEndian.Uint32(uncompMeta[0:4]))
		v.RowCount = count
		v.MinTime = int64(binary.LittleEndian.Uint64(uncompMeta[4:12]))
		v.MaxTime = int64(binary.LittleEndian.Uint64(uncompMeta[12:20]))
		ddLen := int(binary.LittleEndian.Uint32(uncompMeta[20:24]))

		offset := 24
		if cap(v.Timestamps) < count {
			v.Timestamps = make([]int64, count)
		} else {
			v.Timestamps = v.Timestamps[:count]
		}
		DecodeDoubleDelta(uncompMeta[offset:offset+ddLen], count, v.Timestamps)
		offset += ddLen

		if cap(v.ServiceIDs) < count {
			v.ServiceIDs = make([]uint16, count)
			v.LevelIDs = make([]uint16, count)
		} else {
			v.ServiceIDs = v.ServiceIDs[:count]
			v.LevelIDs = v.LevelIDs[:count]
		}
		for i := 0; i < count; i++ {
			v.ServiceIDs[i] = binary.LittleEndian.Uint16(uncompMeta[offset : offset+2])
			v.LevelIDs[i] = binary.LittleEndian.Uint16(uncompMeta[offset+2 : offset+4])
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
			v.MsgLengths[i] = binary.LittleEndian.Uint32(uncompMeta[offset : offset+4])
			offset += 4
		}

		return nil
	}

	return v.Unpack(data)
}

// EnsureMessagesDecompressed decompresses the messages frame if not already decompressed.
func (v *BlockView) EnsureMessagesDecompressed(scratch []byte) error {
	if v.msgsReady {
		return nil
	}
	if len(v.compMsgSlice) == 0 {
		v.rawMsgs = nil
		v.msgsReady = true
		return nil
	}

	uncomp, err := zstdDecoder.DecodeAll(v.compMsgSlice, scratch[:0])
	if err != nil {
		return fmt.Errorf("decompressing messages frame: %w", err)
	}

	if cap(v.rawMsgs) < len(uncomp) {
		v.rawMsgs = make([]byte, len(uncomp))
	} else {
		v.rawMsgs = v.rawMsgs[:len(uncomp)]
	}
	copy(v.rawMsgs, uncomp)

	cur := 0
	for i := 0; i < v.RowCount; i++ {
		v.msgIndices[i] = cur
		cur += int(v.MsgLengths[i])
	}
	v.msgsReady = true
	return nil
}

// Unpack decodes an uncompressed block buffer into the view.
func (v *BlockView) Unpack(data []byte) error {
	if len(data) >= 24 && binary.LittleEndian.Uint32(data[0:4]) == BlockMagic {
		scratch := GetBlockBuffer()
		defer PutBlockBuffer(scratch)
		if err := v.UnpackMeta(data, scratch); err != nil {
			return err
		}
		return v.EnsureMessagesDecompressed(scratch)
	}

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
	v.msgsReady = true
	return nil
}

// MessageAt returns the raw message bytes of row i without allocations.
func (v *BlockView) MessageAt(i int) []byte {
	if i < 0 || i >= v.RowCount || !v.msgsReady {
		return nil
	}
	if i >= len(v.msgIndices) || i >= len(v.MsgLengths) {
		return nil
	}
	start := v.msgIndices[i]
	end := start + int(v.MsgLengths[i])
	if end > len(v.rawMsgs) {
		return nil
	}
	return v.rawMsgs[start:end]
}
