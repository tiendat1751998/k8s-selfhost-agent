package logengine

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoubleDeltaCodec_EmptyAndSingle(t *testing.T) {
	out := make([]byte, 1024)

	// Empty
	n := EncodeDoubleDelta(nil, out)
	assert.Equal(t, 0, n)
	decoded := make([]int64, 0)
	DecodeDoubleDelta(out[:n], 0, decoded)

	// Single timestamp
	ts := []int64{1700000000000}
	n = EncodeDoubleDelta(ts, out)
	assert.Greater(t, n, 0)
	decoded = make([]int64, 1)
	DecodeDoubleDelta(out[:n], 1, decoded)
	assert.Equal(t, ts, decoded)
}

func TestDoubleDeltaCodec_Roundtrip(t *testing.T) {
	testCases := []struct {
		name       string
		timestamps []int64
	}{
		{
			name:       "two timestamps",
			timestamps: []int64{1000000, 1001000},
		},
		{
			name: "constant interval",
			timestamps: func() []int64 {
				ts := make([]int64, 2048)
				base := int64(1700000000000)
				for i := range ts {
					ts[i] = base + int64(i*1000)
				}
				return ts
			}(),
		},
		{
			name: "jittered interval",
			timestamps: func() []int64 {
				r := rand.New(rand.NewSource(42))
				ts := make([]int64, 2048)
				cur := int64(1700000000000)
				for i := range ts {
					cur += int64(900 + r.Intn(200))
					ts[i] = cur
				}
				return ts
			}(),
		},
	}

	out := make([]byte, 65536)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			n := EncodeDoubleDelta(tc.timestamps, out)
			require.Greater(t, n, 0)

			decoded := make([]int64, len(tc.timestamps))
			DecodeDoubleDelta(out[:n], len(tc.timestamps), decoded)
			require.Equal(t, tc.timestamps, decoded)
		})
	}
}

func TestDoubleDeltaCodec_CompressionRatio(t *testing.T) {
	count := 2048
	ts := make([]int64, count)
	base := time.Now().UnixNano()
	for i := range ts {
		ts[i] = base + int64(i)*int64(10*time.Millisecond)
	}

	out := make([]byte, count*8)
	n := EncodeDoubleDelta(ts, out)

	// Uncompressed 2048 * 8 = 16384 bytes
	// With double delta constant interval, DD is 0 (1 byte per item)
	t.Logf("2048 timestamps uncompressed: %d bytes, encoded: %d bytes (%.2fx)", count*8, n, float64(count*8)/float64(n))
	assert.Less(t, n, count*2, "constant interval double delta should compress close to 1 byte/item")
}

func TestLabelDictionary(t *testing.T) {
	dict := NewLabelDictionary()

	id1 := dict.GetOrInsert("frontend")
	id2 := dict.GetOrInsert("backend")
	id3 := dict.GetOrInsert("frontend")

	assert.Equal(t, id1, id3, "same string must yield same ID")
	assert.NotEqual(t, id1, id2, "different strings must yield different IDs")

	s1, ok1 := dict.Lookup(id1)
	assert.True(t, ok1)
	assert.Equal(t, "frontend", s1)

	s2, ok2 := dict.Lookup(id2)
	assert.True(t, ok2)
	assert.Equal(t, "backend", s2)

	_, okNone := dict.Lookup(9999)
	assert.False(t, okNone)
}

func TestLabelDictionary_Concurrent(t *testing.T) {
	dict := NewLabelDictionary()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("service-%d", j%10)
				id := dict.GetOrInsert(key)
				val, ok := dict.Lookup(id)
				if !ok || val != key {
					t.Errorf("lookup mismatch: expected %s, got %s", key, val)
				}
			}
		}(i)
	}

	wg.Wait()
}
