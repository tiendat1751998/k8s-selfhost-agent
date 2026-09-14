package logengine

import (
	"encoding/binary"
	"sync"
)

// EncodeDoubleDelta compresses monotonic timestamps using DoubleDelta encoding with ZigZag varint.
// It returns the number of bytes written to out.
func EncodeDoubleDelta(timestamps []int64, out []byte) int {
	n := len(timestamps)
	if n == 0 {
		return 0
	}

	pos := binary.PutVarint(out, timestamps[0])
	if n == 1 {
		return pos
	}

	d1 := timestamps[1] - timestamps[0]
	pos += binary.PutVarint(out[pos:], d1)
	if n == 2 {
		return pos
	}

	prevD := d1
	for i := 2; i < n; i++ {
		d := timestamps[i] - timestamps[i-1]
		dd := d - prevD
		pos += binary.PutVarint(out[pos:], dd)
		prevD = d
	}

	return pos
}

// DecodeDoubleDelta reconstructs exact int64 timestamps from DoubleDelta encoded bytes.
func DecodeDoubleDelta(in []byte, count int, out []int64) {
	if count <= 0 || len(in) == 0 {
		return
	}

	t0, bytesRead := binary.Varint(in)
	if bytesRead <= 0 {
		return
	}
	out[0] = t0
	if count == 1 {
		return
	}

	pos := bytesRead
	d1, bytesRead := binary.Varint(in[pos:])
	if bytesRead <= 0 {
		return
	}
	pos += bytesRead

	out[1] = out[0] + d1
	prevD := d1
	prevT := out[1]

	for i := 2; i < count; i++ {
		if pos >= len(in) {
			break
		}
		dd, n := binary.Varint(in[pos:])
		if n <= 0 {
			break
		}
		pos += n

		d := prevD + dd
		t := prevT + d
		out[i] = t
		prevD = d
		prevT = t
	}
}

// LabelDictionary is a thread-safe intern table mapping string labels to uint16 IDs.
type LabelDictionary struct {
	mu      sync.RWMutex
	strToID map[string]uint16
	idToStr []string
}

// NewLabelDictionary creates an initialized LabelDictionary.
func NewLabelDictionary() *LabelDictionary {
	return &LabelDictionary{
		strToID: make(map[string]uint16),
		idToStr: make([]string, 0, 64),
	}
}

// GetOrInsert returns the existing uint16 ID for label or inserts it and returns a new ID.
func (d *LabelDictionary) GetOrInsert(label string) uint16 {
	d.mu.RLock()
	if id, exists := d.strToID[label]; exists {
		d.mu.RUnlock()
		return id
	}
	d.mu.RUnlock()

	d.mu.Lock()
	defer d.mu.Unlock()

	if id, exists := d.strToID[label]; exists {
		return id
	}

	id := uint16(len(d.idToStr))
	d.strToID[label] = id
	d.idToStr = append(d.idToStr, label)
	return id
}

// Lookup retrieves the string label associated with the uint16 ID.
func (d *LabelDictionary) Lookup(id uint16) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	idx := int(id)
	if idx < 0 || idx >= len(d.idToStr) {
		return "", false
	}
	return d.idToStr[idx], true
}

// Export returns a snapshot copy of the label entries for persistence.
func (d *LabelDictionary) Export() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	res := make([]string, len(d.idToStr))
	copy(res, d.idToStr)
	return res
}

// Import populates the dictionary from a serialized slice of strings.
func (d *LabelDictionary) Import(entries []string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.strToID = make(map[string]uint16, len(entries))
	d.idToStr = make([]string, len(entries))
	copy(d.idToStr, entries)
	for i, s := range entries {
		d.strToID[s] = uint16(i)
	}
}
