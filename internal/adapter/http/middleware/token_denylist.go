package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// TokenDenylist maintains revoked JWT access tokens until their natural expiration.
type TokenDenylist struct {
	mu              sync.RWMutex
	revoked         map[string]time.Time
	cleanupInterval time.Duration
	stopCh          chan struct{}
	stopOnce        sync.Once
}

// NewTokenDenylist constructs a new thread-safe TokenDenylist with deterministic inline sweeping.
func NewTokenDenylist(cleanupInterval time.Duration) *TokenDenylist {
	if cleanupInterval <= 0 {
		cleanupInterval = 5 * time.Minute
	}
	return &TokenDenylist{
		revoked:         make(map[string]time.Time),
		cleanupInterval: cleanupInterval,
		stopCh:          make(chan struct{}),
	}
}

// Close stops any resources associated with TokenDenylist.
func (d *TokenDenylist) Close() {
	d.stopOnce.Do(func() {
		close(d.stopCh)
	})
}

func (d *TokenDenylist) cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	for hash, exp := range d.revoked {
		if now.After(exp) {
			delete(d.revoked, hash)
		}
	}
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// Revoke records a token's SHA-256 hash in the denylist until its expiration exp.
func (d *TokenDenylist) Revoke(token string, exp time.Time) {
	if token == "" {
		return
	}
	tokenHash := hashToken(token)

	d.mu.Lock()
	defer d.mu.Unlock()

	// Bound memory: if over 50k entries, immediately sweep expired entries
	if len(d.revoked) > 50000 {
		now := time.Now()
		for k, v := range d.revoked {
			if now.After(v) {
				delete(d.revoked, k)
			}
		}
	}
	d.revoked[tokenHash] = exp
}

// IsRevoked checks if the token has been revoked and has not yet expired.
func (d *TokenDenylist) IsRevoked(token string) bool {
	if token == "" {
		return false
	}
	tokenHash := hashToken(token)

	d.mu.RLock()
	exp, exists := d.revoked[tokenHash]
	d.mu.RUnlock()

	if !exists {
		return false
	}
	if time.Now().After(exp) {
		d.mu.Lock()
		delete(d.revoked, tokenHash)
		d.mu.Unlock()
		return false
	}
	return true
}

var defaultTokenDenylist = NewTokenDenylist(5 * time.Minute)

// GetDefaultTokenDenylist returns the package-level default token denylist.
func GetDefaultTokenDenylist() *TokenDenylist {
	return defaultTokenDenylist
}

// CloseDefaultTokenDenylist closes the default token denylist.
func CloseDefaultTokenDenylist() {
	if defaultTokenDenylist != nil {
		defaultTokenDenylist.Close()
	}
}

// RevokeToken revokes a token on the default token denylist.
func RevokeToken(token string, exp time.Time) {
	defaultTokenDenylist.Revoke(token, exp)
}

// IsTokenRevoked checks if a token is revoked on the default token denylist.
func IsTokenRevoked(token string) bool {
	return defaultTokenDenylist.IsRevoked(token)
}
