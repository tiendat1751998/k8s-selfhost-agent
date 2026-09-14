package logengine

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBloomFilter_ZeroFalseNegatives(t *testing.T) {
	bf := NewBlockBloomFilter()
	tokens := make([][]byte, 300)
	for i := 0; i < 300; i++ {
		tokens[i] = []byte(fmt.Sprintf("token_key_%05d", i))
		bf.Add(tokens[i])
	}

	for i, tok := range tokens {
		if !bf.Contains(tok) {
			t.Fatalf("false negative detected for token %d: %s", i, string(tok))
		}
	}
}

func TestBloomFilter_FalsePositiveRate(t *testing.T) {
	bf := NewBlockBloomFilter()
	// Typical block token count: 300 distinct tokens
	for i := 0; i < 300; i++ {
		bf.Add([]byte(fmt.Sprintf("inserted_token_%05d", i)))
	}

	// Test 10,000 non-inserted tokens
	testCount := 10000
	falsePositives := 0
	for i := 0; i < testCount; i++ {
		query := []byte(fmt.Sprintf("non_existent_token_%06d", i))
		if bf.Contains(query) {
			falsePositives++
		}
	}

	fpRate := float64(falsePositives) / float64(testCount)
	t.Logf("False positive rate with 300 tokens: %.4f%% (%d / %d)", fpRate*100, falsePositives, testCount)
	assert.Less(t, fpRate, 0.01, "false positive rate must be strictly under 1%")
}

func TestTokenize_Parsing(t *testing.T) {
	msg := []byte("2026-09-11 12:00:00 [ERROR] failed to connect to db host: 192.168.1.10:5432")
	var tokens []string
	Tokenize(msg, func(tok []byte) {
		tokens = append(tokens, string(tok))
	})

	expected := []string{
		"2026", "09", "11", "12", "00", "00",
		"ERROR", "failed", "to", "connect", "to", "db", "host",
		"192", "168", "1", "10", "5432",
	}
	assert.Equal(t, expected, tokens)
}

func TestTokenize_ZeroAllocations(t *testing.T) {
	msg := []byte("2026-09-11 12:00:00 [ERROR] failed to connect to db host: 192.168.1.10:5432")
	allocs := testing.AllocsPerRun(100, func() {
		Tokenize(msg, func(tok []byte) {
			// no-op consumer
			_ = tok
		})
	})
	assert.Equal(t, float64(0), allocs, "Tokenize must have zero heap allocations")
}
