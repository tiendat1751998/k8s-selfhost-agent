package logengine

import (
	"encoding/binary"
	"fmt"
)

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

		if offset <= len(uncompMeta) {
			UnpackK8sMeta(uncompMeta[offset:], count, &v.NamespaceIDs, &v.PodIDs, &v.ContainerIDs, &v.StreamIDs)
		} else {
			UnpackK8sMeta(nil, count, &v.NamespaceIDs, &v.PodIDs, &v.ContainerIDs, &v.StreamIDs)
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
		v.ServiceIDs = make([]uint16, count)
		v.LevelIDs = make([]uint16, count)
		v.MsgLengths = make([]uint32, count)
		v.msgIndices = make([]int, count)
	} else {
		v.Timestamps = v.Timestamps[:count]
		v.ServiceIDs = v.ServiceIDs[:count]
		v.LevelIDs = v.LevelIDs[:count]
		v.MsgLengths = v.MsgLengths[:count]
		v.msgIndices = v.msgIndices[:count]
	}
	UnpackK8sMeta(nil, count, &v.NamespaceIDs, &v.PodIDs, &v.ContainerIDs, &v.StreamIDs)

	DecodeDoubleDelta(data[offset:offset+ddLen], count, v.Timestamps)
	offset += ddLen

	for i := 0; i < count; i++ {
		v.ServiceIDs[i] = binary.LittleEndian.Uint16(data[offset : offset+2])
		v.LevelIDs[i] = binary.LittleEndian.Uint16(data[offset+2 : offset+4])
		offset += 4
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
