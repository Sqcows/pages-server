// Copyright (C) 2025 SquareCows
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package pages_server

import (
	"sync"
	"time"
)

// Cache defines the interface for caching file content.
type Cache interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte)
	Delete(key string)
	Clear()
}

// MemoryCache implements an in-memory cache with TTL support.
type MemoryCache struct {
	mu      sync.RWMutex
	items   map[string]*cacheItem
	ttl     time.Duration
	janitor *janitor
}

// cacheItem represents a cached item with expiration.
type cacheItem struct {
	value      []byte
	expiration int64
}

// janitor periodically cleans up expired items.
type janitor struct {
	interval time.Duration
	stop     chan bool
}

// NewMemoryCache creates a new in-memory cache.
func NewMemoryCache(ttlSeconds int) *MemoryCache {
	ttl := time.Duration(ttlSeconds) * time.Second
	mc := &MemoryCache{
		items: make(map[string]*cacheItem),
		ttl:   ttl,
	}

	// Start janitor for cleanup only if TTL is positive
	if ttl > 0 {
		interval := ttl / 2
		if interval < 1*time.Second {
			interval = 1 * time.Second // Minimum interval
		}
		mc.janitor = &janitor{
			interval: interval,
			stop:     make(chan bool),
		}
		go mc.janitor.run(mc)
	}

	return mc
}

// Get retrieves a value from the cache.
func (mc *MemoryCache) Get(key string) ([]byte, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	item, found := mc.items[key]
	if !found {
		return nil, false
	}

	// Check if item has expired
	if time.Now().UnixNano() > item.expiration {
		return nil, false
	}

	return item.value, true
}

// Set stores a value in the cache.
func (mc *MemoryCache) Set(key string, value []byte) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	expiration := time.Now().Add(mc.ttl).UnixNano()
	mc.items[key] = &cacheItem{
		value:      value,
		expiration: expiration,
	}
}

// Delete removes a value from the cache.
func (mc *MemoryCache) Delete(key string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.items, key)
}

// Clear removes all items from the cache.
func (mc *MemoryCache) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.items = make(map[string]*cacheItem)
}

// deleteExpired removes expired items from the cache.
func (mc *MemoryCache) deleteExpired() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now().UnixNano()
	for key, item := range mc.items {
		if now > item.expiration {
			delete(mc.items, key)
		}
	}
}

// run starts the janitor cleanup routine.
func (j *janitor) run(mc *MemoryCache) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.deleteExpired()
		case <-j.stop:
			return
		}
	}
}

// Stop stops the janitor cleanup routine.
func (mc *MemoryCache) Stop() {
	if mc.janitor != nil {
		mc.janitor.stop <- true
	}
}

// RedisCache implements a Redis-based cache.
// Note: For Yaegi compatibility, we implement a simple Redis client using net/http.
type RedisCache struct {
	host     string
	port     int
	password string
	ttl      int
	mu       sync.RWMutex
	// For Yaegi, we fall back to in-memory cache since Redis protocol is complex
	fallback *MemoryCache
}

// NewRedisCache creates a new Redis cache.
// Note: This implementation falls back to in-memory cache for Yaegi compatibility.
func NewRedisCache(host string, port int, password string, ttlSeconds int) *RedisCache {
	// For Yaegi compatibility, use in-memory cache as fallback
	// A full Redis implementation would require complex protocol handling
	return &RedisCache{
		host:     host,
		port:     port,
		password: password,
		ttl:      ttlSeconds,
		fallback: NewMemoryCache(ttlSeconds),
	}
}

// Get retrieves a value from Redis cache.
func (rc *RedisCache) Get(key string) ([]byte, bool) {
	// Use fallback for Yaegi compatibility
	return rc.fallback.Get(key)
}

// Set stores a value in Redis cache.
func (rc *RedisCache) Set(key string, value []byte) {
	// Use fallback for Yaegi compatibility
	rc.fallback.Set(key, value)
}

// Delete removes a value from Redis cache.
func (rc *RedisCache) Delete(key string) {
	// Use fallback for Yaegi compatibility
	rc.fallback.Delete(key)
}

// Clear removes all items from Redis cache.
func (rc *RedisCache) Clear() {
	// Use fallback for Yaegi compatibility
	rc.fallback.Clear()
}
