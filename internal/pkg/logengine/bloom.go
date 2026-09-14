package logengine

import (
	"github.com/cespare/xxhash/v2"
)

// BloomFilterSize is the fixed size in bytes for a block token bloom filter (1024 bytes = 8192 bits).
const (
	BloomFilterSize = 1024
	BloomFilterBits = 8192
)

// BlockBloomFilter is a fixed 1024-byte Bloom filter using 4 hash functions via xxhash.Sum64.
type BlockBloomFilter [BloomFilterSize]byte

// NewBlockBloomFilter initializes an empty BlockBloomFilter.
func NewBlockBloomFilter() *BlockBloomFilter {
	return &BlockBloomFilter{}
}

// hashToken lowercases ASCII in a stack buffer to ensure case-insensitive matching with zero heap allocation.
func hashToken(token []byte) uint64 {
	var stackBuf [64]byte
	var buf []byte
	if len(token) <= len(stackBuf) {
		buf = stackBuf[:len(token)]
	} else {
		buf = make([]byte, len(token))
	}
	for i, c := range token {
		if c >= 'A' && c <= 'Z' {
			buf[i] = c + 32
		} else {
			buf[i] = c
		}
	}
	return xxhash.Sum64(buf)
}

// Add inserts a token into the Bloom filter using the Kirsch-Mitzenmacher technique with 4 hash values.
func (b *BlockBloomFilter) Add(token []byte) {
	if len(token) == 0 {
		return
	}
	h := hashToken(token)
	h1 := uint32(h)
	h2 := uint32(h>>32) | 1

	for i := uint32(0); i < 4; i++ {
		bit := (h1 + i*h2) & (BloomFilterBits - 1)
		b[bit>>3] |= 1 << (bit & 7)
	}
}

// Contains checks whether a token might be in the Bloom filter.
func (b *BlockBloomFilter) Contains(token []byte) bool {
	if len(token) == 0 {
		return false
	}
	h := hashToken(token)
	h1 := uint32(h)
	h2 := uint32(h>>32) | 1

	for i := uint32(0); i < 4; i++ {
		bit := (h1 + i*h2) & (BloomFilterBits - 1)
		if b[bit>>3]&(1<<(bit&7)) == 0 {
			return false
		}
	}
	return true
}

// Reset clears all bits in the Bloom filter.
func (b *BlockBloomFilter) Reset() {
	*b = BlockBloomFilter{}
}

// Bytes returns a slice pointing to the underlying 1024 bytes.
func (b *BlockBloomFilter) Bytes() []byte {
	return b[:]
}

// CopyFrom copies raw bytes into the filter.
func (b *BlockBloomFilter) CopyFrom(src []byte) {
	copy(b[:], src)
}

// isAlphaNum reports whether b is an alphanumeric character or underscore.
func isAlphaNum(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

// Tokenize parses alphanumeric tokens from msg without allocating strings or heap memory.
func Tokenize(msg []byte, onToken func([]byte)) {
	n := len(msg)
	i := 0
	for i < n {
		for i < n && !isAlphaNum(msg[i]) {
			i++
		}
		if i >= n {
			break
		}
		start := i
		for i < n && isAlphaNum(msg[i]) {
			i++
		}
		onToken(msg[start:i])
	}
}
