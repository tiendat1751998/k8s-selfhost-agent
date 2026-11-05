package agent

import (
	"sync"
	"time"
)

// HostBackoffManager provides thread-safe unreachable host backoff with fast recovery.
// When dialing an offline LAN host fails, subsequent calls to that host fail fast in ~0ms
// for the duration of the cooldown period without blocking on TCP SYN timeouts.
type HostBackoffManager struct {
	mu       sync.RWMutex
	failures map[string]time.Time
	cooldown time.Duration
}

// NewHostBackoffManager creates a new HostBackoffManager with the specified cooldown.
func NewHostBackoffManager(cooldown time.Duration) *HostBackoffManager {
	if cooldown <= 0 {
		cooldown = 15 * time.Second
	}
	return &HostBackoffManager{
		failures: make(map[string]time.Time),
		cooldown: cooldown,
	}
}

// IsUnreachable checks if an endpoint is currently marked unreachable within the cooldown window.
func (m *HostBackoffManager) IsUnreachable(endpoint string) bool {
	if m == nil {
		return false
	}
	key := NormalizeHostEndpoint(endpoint)
	if key == "" {
		return false
	}

	m.mu.RLock()
	lastFail, exists := m.failures[key]
	m.mu.RUnlock()

	if !exists {
		return false
	}

	if time.Since(lastFail) < m.cooldown {
		return true
	}

	// Cooldown expired, purge entry
	m.mu.Lock()
	delete(m.failures, key)
	m.mu.Unlock()
	return false
}

// MarkFailed marks an endpoint as unreachable at the current timestamp.
func (m *HostBackoffManager) MarkFailed(endpoint string) {
	if m == nil {
		return
	}
	key := NormalizeHostEndpoint(endpoint)
	if key == "" {
		return
	}

	m.mu.Lock()
	m.failures[key] = time.Now()
	m.mu.Unlock()
}

// MarkSuccess clears the unreachable status immediately when a node responds successfully.
func (m *HostBackoffManager) MarkSuccess(endpoint string) {
	if m == nil {
		return
	}
	key := NormalizeHostEndpoint(endpoint)
	if key == "" {
		return
	}

	m.mu.Lock()
	delete(m.failures, key)
	m.mu.Unlock()
}

// Clear purges all recorded host failure states.
func (m *HostBackoffManager) Clear() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.failures = make(map[string]time.Time)
	m.mu.Unlock()
}
