package logengine

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/cespare/xxhash/v2"
)

const (
	StreamUnknown uint8 = 0
	StreamStdout  uint8 = 1
	StreamStderr  uint8 = 2

	StreamBitStdout uint16 = 1 << 0
	StreamBitStderr uint16 = 1 << 1
	StreamBitOther  uint16 = 1 << 2
)

// StreamToID converts stream string to 1-byte ID.
func StreamToID(s string) uint8 {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "stdout":
		return StreamStdout
	case "stderr":
		return StreamStderr
	default:
		return StreamUnknown
	}
}

// StreamIDToString converts 1-byte ID to stream string.
func StreamIDToString(id uint8) string {
	switch id {
	case StreamStdout:
		return "stdout"
	case StreamStderr:
		return "stderr"
	default:
		return ""
	}
}

// StreamToBit returns the bitmask for stream filtering.
func StreamToBit(stream string) uint16 {
	switch strings.ToLower(strings.TrimSpace(stream)) {
	case "stdout":
		return StreamBitStdout
	case "stderr":
		return StreamBitStderr
	default:
		return StreamBitOther
	}
}

// MetaDictionaryData represents serialized metadata dictionaries for persistence.
type MetaDictionaryData struct {
	Namespaces []string `json:"namespaces"`
	Pods       []string `json:"pods"`
	Containers []string `json:"containers"`
}

// MetaDictionary manages separate low-cardinality dictionaries for K8s metadata.
// Persisted to meta_dict.json so LabelDictionary (dict.json) exports ONLY services!
type MetaDictionary struct {
	mu         sync.RWMutex
	namespaces *LabelDictionary
	pods       *LabelDictionary
	containers *LabelDictionary
}

// NewMetaDictionary creates an empty MetaDictionary with ID 0 reserved for unset/empty.
func NewMetaDictionary() *MetaDictionary {
	ns := NewLabelDictionary()
	ns.GetOrInsert("")
	pods := NewLabelDictionary()
	pods.GetOrInsert("")
	conts := NewLabelDictionary()
	conts.GetOrInsert("")

	return &MetaDictionary{
		namespaces: ns,
		pods:       pods,
		containers: conts,
	}
}

// GetOrInsert allocates or retrieves dictionary IDs for namespace, pod, and container.
func (m *MetaDictionary) GetOrInsert(ns, pod, container string) (uint16, uint16, uint16) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var nsID, podID, contID uint16
	if ns != "" {
		nsID = m.namespaces.GetOrInsert(ns)
	}
	if pod != "" {
		podID = m.pods.GetOrInsert(pod)
	}
	if container != "" {
		contID = m.containers.GetOrInsert(container)
	}
	return nsID, podID, contID
}

// Lookup resolves dictionary IDs back into namespace, pod, and container strings.
func (m *MetaDictionary) Lookup(nsID, podID, contID uint16) (string, string, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var ns, pod, cont string
	if nsID > 0 {
		ns, _ = m.namespaces.Lookup(nsID)
	}
	if podID > 0 {
		pod, _ = m.pods.Lookup(podID)
	}
	if contID > 0 {
		cont, _ = m.containers.Lookup(contID)
	}
	return ns, pod, cont
}

// LookupNamespace resolves a namespace ID.
func (m *MetaDictionary) LookupNamespace(id uint16) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.namespaces.Lookup(id)
}

// LookupPod resolves a pod ID.
func (m *MetaDictionary) LookupPod(id uint16) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pods.Lookup(id)
}

// LookupContainer resolves a container ID.
func (m *MetaDictionary) LookupContainer(id uint16) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.containers.Lookup(id)
}

// LookupNamespaceID looks up ID for namespace.
func (m *MetaDictionary) LookupNamespaceID(name string) (uint16, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.namespaces.LookupID(name)
}

// LookupPodID looks up ID for pod.
func (m *MetaDictionary) LookupPodID(name string) (uint16, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pods.LookupID(name)
}

// LookupContainerID looks up ID for container.
func (m *MetaDictionary) LookupContainerID(name string) (uint16, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.containers.LookupID(name)
}

// Size returns total entries across metadata dictionaries.
func (m *MetaDictionary) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.namespaces.Size() + m.pods.Size() + m.containers.Size()
}

// Export serializes metadata dictionaries.
func (m *MetaDictionary) Export() MetaDictionaryData {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return MetaDictionaryData{
		Namespaces: m.namespaces.Export(),
		Pods:       m.pods.Export(),
		Containers: m.containers.Export(),
	}
}

// Import restores metadata dictionaries from serialized data.
func (m *MetaDictionary) Import(data MetaDictionaryData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	importWithEmptyZero(m.namespaces, data.Namespaces)
	importWithEmptyZero(m.pods, data.Pods)
	importWithEmptyZero(m.containers, data.Containers)
}

func importWithEmptyZero(dict *LabelDictionary, entries []string) {
	if len(entries) == 0 {
		dict.Import([]string{""})
		return
	}
	if entries[0] == "" {
		dict.Import(entries)
		return
	}
	merged := make([]string, 1+len(entries))
	merged[0] = ""
	copy(merged[1:], entries)
	dict.Import(merged)
}

// Save persists metadata dictionary to filePath.
func (m *MetaDictionary) Save(filePath string) error {
	data := m.Export()
	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal meta dict: %w", err)
	}
	return os.WriteFile(filePath, bytes, 0644)
}

// Load loads metadata dictionary from filePath.
func (m *MetaDictionary) Load(filePath string) error {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var data MetaDictionaryData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return fmt.Errorf("unmarshal meta dict: %w", err)
	}
	m.Import(data)
	return nil
}

// PackK8sMeta encodes columnar metadata (nsID uint16, podID uint16, contID uint16, streamID uint8).
// Appends 7 bytes per row to dst.
func PackK8sMeta(dst []byte, count int, nsIDs, podIDs, contIDs []uint16, streamIDs []uint8) int {
	needed := count * 7
	if len(dst) < needed {
		return 0
	}
	offset := 0
	for i := 0; i < count; i++ {
		var ns, pod, cont uint16
		var stream uint8
		if i < len(nsIDs) {
			ns = nsIDs[i]
		}
		if i < len(podIDs) {
			pod = podIDs[i]
		}
		if i < len(contIDs) {
			cont = contIDs[i]
		}
		if i < len(streamIDs) {
			stream = streamIDs[i]
		}
		binary.LittleEndian.PutUint16(dst[offset:offset+2], ns)
		binary.LittleEndian.PutUint16(dst[offset+2:offset+4], pod)
		binary.LittleEndian.PutUint16(dst[offset+4:offset+6], cont)
		dst[offset+6] = stream
		offset += 7
	}
	return offset
}

// UnpackK8sMeta decodes columnar metadata into preallocated slices.
// If src does not contain metadata (legacy block), slices are zero-filled.
func UnpackK8sMeta(src []byte, count int, nsIDs, podIDs, contIDs *[]uint16, streamIDs *[]uint8) {
	if cap(*nsIDs) < count {
		*nsIDs = make([]uint16, count)
		*podIDs = make([]uint16, count)
		*contIDs = make([]uint16, count)
		*streamIDs = make([]uint8, count)
	} else {
		*nsIDs = (*nsIDs)[:count]
		*podIDs = (*podIDs)[:count]
		*contIDs = (*contIDs)[:count]
		*streamIDs = (*streamIDs)[:count]
	}

	needed := count * 7
	if len(src) < needed {
		// Legacy block: zero-fill metadata
		for i := 0; i < count; i++ {
			(*nsIDs)[i] = 0
			(*podIDs)[i] = 0
			(*contIDs)[i] = 0
			(*streamIDs)[i] = 0
		}
		return
	}

	offset := 0
	for i := 0; i < count; i++ {
		(*nsIDs)[i] = binary.LittleEndian.Uint16(src[offset : offset+2])
		(*podIDs)[i] = binary.LittleEndian.Uint16(src[offset+2 : offset+4])
		(*contIDs)[i] = binary.LittleEndian.Uint16(src[offset+4 : offset+6])
		(*streamIDs)[i] = src[offset+6]
		offset += 7
	}
}

// MetaBloomBit returns a bitmask (bits 3..15) for a given string name.
func MetaBloomBit(name string) uint16 {
	if name == "" {
		return 0
	}
	h := xxhash.Sum64String(strings.ToLower(strings.TrimSpace(name)))
	bit := 3 + (h % 13)
	return uint16(1 << bit)
}
