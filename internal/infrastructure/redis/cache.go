// Package redis provides a generic cache manager using Redis as backing store.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// CacheManager provides a generic read-through cache backed by Redis.
type CacheManager struct {
	client *Client
}

// NewCacheManager creates a new cache manager wrapping the given Redis client.
func NewCacheManager(client *Client) *CacheManager {
	return &CacheManager{client: client}
}

// GetOrSet retrieves a cached value by key. If the key is missing or expired,
// the fallback function is called, and its result is stored with the given TTL.
//
// Usage:
//
//	var result MyStruct
//	err := cache.GetOrSet(ctx, "mykey", 5*time.Minute, &result, func() (interface{}, error) {
//	    return fetchFromDB(ctx)
//	})
func (cm *CacheManager) GetOrSet(ctx context.Context, key string, ttl time.Duration, dest interface{}, fallback func() (interface{}, error)) error {
	log := logger.WithContext(ctx)

	// Try cache first
	cached, err := cm.client.Get(ctx, key)
	if err == nil && cached != "" {
		if jsonErr := json.Unmarshal([]byte(cached), dest); jsonErr == nil {
			log.Debug("cache hit", zap.String("key", key))
			return nil
		}
		// JSON parse failed — treat as miss and refresh
		log.Warn("cache value corrupt, refreshing", zap.String("key", key))
	}

	// Cache miss — call fallback
	log.Debug("cache miss", zap.String("key", key))
	value, err := fallback()
	if err != nil {
		return fmt.Errorf("cache fallback for key %q: %w", key, err)
	}

	// Store result in dest
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshaling cache value for key %q: %w", key, err)
	}
	if jsonErr := json.Unmarshal(data, dest); jsonErr != nil {
		return fmt.Errorf("unmarshaling cache value for key %q: %w", key, jsonErr)
	}

	// Write to cache (fire-and-forget, don't fail the request on cache write error)
	if setErr := cm.client.Set(ctx, key, string(data), ttl); setErr != nil {
		log.Warn("failed to write cache", zap.String("key", key), zap.Error(setErr))
	}

	return nil
}

// Invalidate removes a key from the cache.
func (cm *CacheManager) Invalidate(ctx context.Context, keys ...string) error {
	return cm.client.Delete(ctx, keys...)
}

// InvalidatePattern removes all keys matching a prefix pattern.
// Note: This uses SCAN internally and should not be used in hot paths.
func (cm *CacheManager) InvalidatePattern(ctx context.Context, pattern string) error {
	log := logger.WithContext(ctx)

	iter := cm.client.Conn().Scan(ctx, 0, pattern, 100).Iterator()
	var deleted int
	for iter.Next(ctx) {
		if err := cm.client.Delete(ctx, iter.Val()); err != nil {
			log.Warn("failed to delete cache key", zap.String("key", iter.Val()), zap.Error(err))
		} else {
			deleted++
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("scanning cache keys with pattern %q: %w", pattern, err)
	}

	log.Debug("invalidated cache pattern", zap.String("pattern", pattern), zap.Int("deleted", deleted))
	return nil
}
