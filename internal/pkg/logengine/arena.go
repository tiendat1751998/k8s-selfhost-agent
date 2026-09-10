package logengine

import (
	"runtime/debug"
	"sync"
)

// BlockBufferSize is the fixed size of a block scratch buffer (256KB).
const BlockBufferSize = 256 * 1024

// BlockBufferPool manages fixed-size 256KB byte buffers using sync.Pool to eliminate heap churn.
var BlockBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, BlockBufferSize)
		return &buf
	},
}

// InitEngineRuntime tunes the Go runtime memory limits and garbage collection
// to keep total process RSS strictly under 100MB (<50MB typical).
func InitEngineRuntime() {
	debug.SetMemoryLimit(85 * 1024 * 1024)
	debug.SetGCPercent(50)
}

// GetBlockBuffer retrieves a 256KB slice from the pool.
func GetBlockBuffer() []byte {
	bp := BlockBufferPool.Get().(*[]byte)
	buf := *bp
	if cap(buf) < BlockBufferSize {
		buf = make([]byte, BlockBufferSize)
	}
	return buf[:BlockBufferSize]
}

// PutBlockBuffer returns a buffer to the pool if it meets the size capacity.
func PutBlockBuffer(buf []byte) {
	if cap(buf) >= BlockBufferSize {
		b := buf[:BlockBufferSize]
		BlockBufferPool.Put(&b)
	}
}
