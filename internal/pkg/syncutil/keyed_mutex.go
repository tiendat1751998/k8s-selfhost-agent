package syncutil

import (
	"errors"
	"sync"
)

// ErrOperationInProgress indicates a concurrent operation is already running for the given key.
var ErrOperationInProgress = errors.New("operation in progress")

const numShards = 64

type shard struct {
	mu    sync.Mutex
	locks map[string]struct{}
}

// KeyedMutex provides per-key mutual exclusion without blocking.
// It is partitioned into shards to prevent global lock contention.
type KeyedMutex struct {
	shards [numShards]shard
}

// NewKeyedMutex initializes a new KeyedMutex with allocated shards.
func NewKeyedMutex() *KeyedMutex {
	km := &KeyedMutex{}
	for i := range km.shards {
		km.shards[i].locks = make(map[string]struct{})
	}
	return km
}

func fnv32(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

func (km *KeyedMutex) getShard(key string) *shard {
	idx := fnv32(key) % numShards
	return &km.shards[idx]
}

// TryLock attempts to acquire the lock for key. Returns true if acquired, false otherwise.
func (km *KeyedMutex) TryLock(key string) bool {
	s := km.getShard(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locks == nil {
		s.locks = make(map[string]struct{})
	}
	if _, ok := s.locks[key]; ok {
		return false
	}
	s.locks[key] = struct{}{}
	return true
}

// Unlock releases the lock held for key.
func (km *KeyedMutex) Unlock(key string) {
	s := km.getShard(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locks != nil {
		delete(s.locks, key)
	}
}
