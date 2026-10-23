package logengine

import (
	"encoding/binary"
	"time"

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
	namespaceIDs []uint16
	podIDs       []uint16
	containerIDs []uint16
	streamIDs    []uint8
	bloom        BlockBloomFilter
	minTime      int64
	maxTime      int64
	levelMask    uint16
	serviceBloom uint64
	reservedMask uint16
}

// NewBlockBuilder initializes a block builder pre-allocated for ~2048 rows.
func NewBlockBuilder() *BlockBuilder {
	return &BlockBuilder{
		timestamps:   make([]int64, 0, 2048),
		serviceIDs:   make([]uint16, 0, 2048),
		levelIDs:     make([]uint16, 0, 2048),
		msgLengths:   make([]uint32, 0, 2048),
		messages:     make([]byte, 0, 128*1024),
		namespaceIDs: make([]uint16, 0, 2048),
		podIDs:       make([]uint16, 0, 2048),
		containerIDs: make([]uint16, 0, 2048),
		streamIDs:    make([]uint8, 0, 2048),
	}
}

// Append adds a row to the staging block builder.
func (b *BlockBuilder) Append(ts int64, serviceID, levelID uint16, msg []byte) {
	b.AppendWithMeta(ts, serviceID, levelID, 0, 0, 0, 0, string(msg))
}

// AppendString appends a row with a string message avoiding heap allocation.
func (b *BlockBuilder) AppendString(ts int64, serviceID, levelID uint16, msg string) {
	b.AppendWithMeta(ts, serviceID, levelID, 0, 0, 0, 0, msg)
}

// AppendWithMeta appends a row with full multi-dimensional K8s metadata.
func (b *BlockBuilder) AppendWithMeta(ts int64, serviceID, levelID, nsID, podID, contID uint16, streamID uint8, msg string) {
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

	b.namespaceIDs = append(b.namespaceIDs, nsID)
	b.podIDs = append(b.podIDs, podID)
	b.containerIDs = append(b.containerIDs, contID)
	b.streamIDs = append(b.streamIDs, streamID)

	b.levelMask |= LevelToBit(LevelIDToString(levelID))
	b.serviceBloom |= ServiceBloomBit(serviceID)

	if streamID == StreamStdout {
		b.reservedMask |= StreamBitStdout
	} else if streamID == StreamStderr {
		b.reservedMask |= StreamBitStderr
	} else if streamID != 0 {
		b.reservedMask |= StreamBitOther
	}

	TokenizeString(msg, func(tok []byte) {
		b.bloom.Add(tok)
		if len(tok) >= 4 {
			b.bloom.AddNGrams4(tok)
		}
	})
}

// RowCount returns the number of rows currently in the builder.
func (b *BlockBuilder) RowCount() int {
	return len(b.timestamps)
}

// EstimatedSize returns approximate uncompressed bytes of the block.
func (b *BlockBuilder) EstimatedSize() int {
	return len(b.timestamps)*15 + len(b.serviceIDs)*2 + len(b.levelIDs)*2 + len(b.msgLengths)*4 + len(b.messages) + 36
}

// Reset clears the builder for reuse without reallocating slices.
func (b *BlockBuilder) Reset() {
	b.timestamps = b.timestamps[:0]
	b.serviceIDs = b.serviceIDs[:0]
	b.levelIDs = b.levelIDs[:0]
	b.msgLengths = b.msgLengths[:0]
	b.messages = b.messages[:0]
	b.namespaceIDs = b.namespaceIDs[:0]
	b.podIDs = b.podIDs[:0]
	b.containerIDs = b.containerIDs[:0]
	b.streamIDs = b.streamIDs[:0]
	b.bloom.Reset()
	b.minTime = 0
	b.maxTime = 0
	b.levelMask = 0
	b.serviceBloom = 0
	b.reservedMask = 0
}

func (b *BlockBuilder) LevelMask() uint16    { return b.levelMask }
func (b *BlockBuilder) ServiceBloom() uint64 { return b.serviceBloom }
func (b *BlockBuilder) ReservedMask() uint16 { return b.reservedMask }
func (b *BlockBuilder) AddReservedBits(mask uint16) { b.reservedMask |= mask }

// Pack encodes the columnar components safely into dst, growing dst if capacity is exceeded.
func (b *BlockBuilder) Pack(dst []byte) (uncompressed []byte, minTime, maxTime int64, bloom *BlockBloomFilter) {
	count := len(b.timestamps)
	if count == 0 {
		return nil, 0, 0, &b.bloom
	}

	needed := 36 + (count * 25) + len(b.messages)
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
		binary.LittleEndian.PutUint32(dst[offset+4:offset+8], b.msgLengths[i])
		offset += 8
	}
	offset += PackK8sMeta(dst[offset:], count, b.namespaceIDs, b.podIDs, b.containerIDs, b.streamIDs)

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
	binary.LittleEndian.PutUint32(dst[24:28], uint32(count*4))
	binary.LittleEndian.PutUint32(dst[28:32], uint32(count*4))
	binary.LittleEndian.PutUint32(dst[32:36], uint32(len(b.messages)))
	return dst[:offset], b.minTime, b.maxTime, &b.bloom
}

// PackAndCompress serializes columnar data with late materialization (meta and messages compressed separately).
func (b *BlockBuilder) PackAndCompress(scratchBuf, compBuf []byte) (compressed []byte, uncompressedSize int, minTime, maxTime int64, bloom *BlockBloomFilter, levelMask uint16, serviceBloom uint64) {
	count := len(b.timestamps)
	if count == 0 {
		return nil, 0, 0, 0, &b.bloom, 0, 0
	}

	neededMeta := 24 + (count * 25)
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

	k8sBytes := PackK8sMeta(scratchBuf[offset:], count, b.namespaceIDs, b.podIDs, b.containerIDs, b.streamIDs)
	offset += k8sBytes

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
	NamespaceIDs []uint16
	PodIDs       []uint16
	ContainerIDs []uint16
	StreamIDs    []uint8
	rawMsgs      []byte
	msgIndices   []int
	compMsgSlice []byte
	msgsReady    bool
}

// NewBlockView creates a reusable BlockView.
func NewBlockView() *BlockView {
	return &BlockView{
		Timestamps:   make([]int64, 0, 2048),
		ServiceIDs:   make([]uint16, 0, 2048),
		LevelIDs:     make([]uint16, 0, 2048),
		MsgLengths:   make([]uint32, 0, 2048),
		NamespaceIDs: make([]uint16, 0, 2048),
		PodIDs:       make([]uint16, 0, 2048),
		ContainerIDs: make([]uint16, 0, 2048),
		StreamIDs:    make([]uint8, 0, 2048),
		msgIndices:   make([]int, 0, 2048),
	}
}

// SnapshotEntries extracts all rows currently staged in b as domain Entry values.
func (b *BlockBuilder) SnapshotEntries(dict *LabelDictionary, metaDict *MetaDictionary) []Entry {
	count := len(b.timestamps)
	if count == 0 {
		return nil
	}
	entries := make([]Entry, count)
	curMsgOffset := 0
	for i := 0; i < count; i++ {
		ts := time.Unix(0, b.timestamps[i]).UTC()
		var svc string
		if dict != nil {
			svc, _ = dict.Lookup(b.serviceIDs[i])
		}
		lvl := LevelIDToString(b.levelIDs[i])

		var ns, pod, cont string
		if metaDict != nil && i < len(b.namespaceIDs) {
			ns, pod, cont = metaDict.Lookup(b.namespaceIDs[i], b.podIDs[i], b.containerIDs[i])
		}
		var stream string
		if i < len(b.streamIDs) {
			stream = StreamIDToString(b.streamIDs[i])
		}

		msgLen := int(b.msgLengths[i])
		msgEnd := curMsgOffset + msgLen
		var msg string
		if msgEnd <= len(b.messages) {
			msg = string(b.messages[curMsgOffset:msgEnd])
		}
		curMsgOffset = msgEnd

		entries[i] = Entry{
			Timestamp:     ts,
			Service:       svc,
			Level:         lvl,
			Message:       msg,
			Namespace:     ns,
			PodName:       pod,
			ContainerName: cont,
			Stream:        stream,
		}
	}
	return entries
}

