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
	"fmt"
	"testing"
	"time"
)

// TestNewMemoryCache tests the NewMemoryCache function.
func TestNewMemoryCache(t *testing.T) {
	cache := NewMemoryCache(300)

	if cache == nil {
		t.Fatal("NewMemoryCache returned nil")
	}

	if cache.ttl != 300*time.Second {
		t.Errorf("Expected TTL of 300s, got %v", cache.ttl)
	}

	if cache.items == nil {
		t.Error("Cache items map is nil")
	}

	// Clean up
	cache.Stop()
}

// TestMemoryCacheSetGet tests the Set and Get methods.
func TestMemoryCacheSetGet(t *testing.T) {
	cache := NewMemoryCache(300)
	defer cache.Stop()

	key := "test-key"
	value := []byte("test-value")

	// Set a value
	cache.Set(key, value)

	// Get the value
	got, found := cache.Get(key)

	if !found {
		t.Fatal("Expected to find value in cache")
	}

	if string(got) != string(value) {
		t.Errorf("Expected value %q, got %q", string(value), string(got))
	}
}

// TestMemoryCacheGetNotFound tests the Get method with a non-existent key.
func TestMemoryCacheGetNotFound(t *testing.T) {
	cache := NewMemoryCache(300)
	defer cache.Stop()

	_, found := cache.Get("nonexistent")

	if found {
		t.Error("Expected not to find non-existent key")
	}
}

// TestMemoryCacheExpiration tests that cached items expire after TTL.
func TestMemoryCacheExpiration(t *testing.T) {
	cache := NewMemoryCache(1) // 1 second TTL
	defer cache.Stop()

	key := "test-key"
	value := []byte("test-value")

	cache.Set(key, value)

	// Value should be present immediately
	_, found := cache.Get(key)
	if !found {
		t.Error("Expected to find value immediately after setting")
	}

	// Wait for expiration
	time.Sleep(2 * time.Second)

	// Value should be expired
	_, found = cache.Get(key)
	if found {
		t.Error("Expected value to be expired")
	}
}

// TestMemoryCacheDelete tests the Delete method.
func TestMemoryCacheDelete(t *testing.T) {
	cache := NewMemoryCache(300)
	defer cache.Stop()

	key := "test-key"
	value := []byte("test-value")

	cache.Set(key, value)

	// Verify it's there
	_, found := cache.Get(key)
	if !found {
		t.Error("Expected to find value before deletion")
	}

	// Delete it
	cache.Delete(key)

	// Verify it's gone
	_, found = cache.Get(key)
	if found {
		t.Error("Expected value to be deleted")
	}
}

// TestMemoryCacheClear tests the Clear method.
func TestMemoryCacheClear(t *testing.T) {
	cache := NewMemoryCache(300)
	defer cache.Stop()

	// Set multiple values
	cache.Set("key1", []byte("value1"))
	cache.Set("key2", []byte("value2"))
	cache.Set("key3", []byte("value3"))

	// Verify they're there
	if _, found := cache.Get("key1"); !found {
		t.Error("Expected to find key1")
	}
	if _, found := cache.Get("key2"); !found {
		t.Error("Expected to find key2")
	}

	// Clear the cache
	cache.Clear()

	// Verify they're gone
	if _, found := cache.Get("key1"); found {
		t.Error("Expected key1 to be cleared")
	}
	if _, found := cache.Get("key2"); found {
		t.Error("Expected key2 to be cleared")
	}
	if _, found := cache.Get("key3"); found {
		t.Error("Expected key3 to be cleared")
	}
}

// TestMemoryCacheJanitor tests that the janitor cleans up expired items.
func TestMemoryCacheJanitor(t *testing.T) {
	cache := NewMemoryCache(1) // 1 second TTL
	defer cache.Stop()

	// Set a value
	cache.Set("test-key", []byte("test-value"))

	// Wait for janitor to run (it runs every ttl/2, so 500ms)
	time.Sleep(2 * time.Second)

	// Check that the item was cleaned up
	cache.mu.RLock()
	itemCount := len(cache.items)
	cache.mu.RUnlock()

	if itemCount != 0 {
		t.Errorf("Expected janitor to clean up expired items, got %d items", itemCount)
	}
}

// TestMemoryCacheConcurrency tests concurrent access to the cache.
func TestMemoryCacheConcurrency(t *testing.T) {
	cache := NewMemoryCache(300)
	defer cache.Stop()

	done := make(chan bool)

	// Concurrent writes
	for i := 0; i < 100; i++ {
		go func(n int) {
			key := string(rune('A' + (n % 26)))
			value := []byte{byte(n)}
			cache.Set(key, value)
			done <- true
		}(i)
	}

	// Wait for all writes to complete
	for i := 0; i < 100; i++ {
		<-done
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		go func(n int) {
			key := string(rune('A' + (n % 26)))
			cache.Get(key)
			done <- true
		}(i)
	}

	// Wait for all reads to complete
	for i := 0; i < 100; i++ {
		<-done
	}
}

// TestNewRedisCache tests the NewRedisCache function.
func TestNewRedisCache(t *testing.T) {
	cache := NewRedisCache("localhost", 6379, "", 300, 10, 20, 5)
	defer cache.Close()

	if cache == nil {
		t.Fatal("NewRedisCache returned nil")
	}

	if cache.host != "localhost" {
		t.Errorf("Expected host %q, got %q", "localhost", cache.host)
	}

	if cache.port != 6379 {
		t.Errorf("Expected port %d, got %d", 6379, cache.port)
	}

	if cache.fallback == nil {
		t.Error("Expected fallback cache to be initialized")
	}

	if cache.poolSize != 10 {
		t.Errorf("Expected poolSize %d, got %d", 10, cache.poolSize)
	}

	if cache.maxConnections != 20 {
		t.Errorf("Expected maxConnections %d, got %d", 20, cache.maxConnections)
	}

	if cache.connWaitTimeout != 5*time.Second {
		t.Errorf("Expected connWaitTimeout %v, got %v", 5*time.Second, cache.connWaitTimeout)
	}
}

// TestRedisCacheSetGet tests basic SET and GET operations with Redis.
// This test requires a Redis/Valkey server running on localhost:6379.
func TestRedisCacheSetGet(t *testing.T) {
	cache := NewRedisCache("localhost", 6379, "", 300, 10, 20, 5)
	defer cache.Close()

	key := "test-key-setget"
	value := []byte("test-value-123")

	// Set a value
	cache.Set(key, value)

	// Get the value
	got, found := cache.Get(key)

	if !found {
		t.Fatal("Expected to find value in cache")
	}

	if string(got) != string(value) {
		t.Errorf("Expected value %q, got %q", string(value), string(got))
	}

	// Clean up
	cache.Delete(key)
}

// TestRedisCacheDelete tests the DELETE operation.
func TestRedisCacheDelete(t *testing.T) {
	cache := NewRedisCache("localhost", 6379, "", 300, 10, 20, 5)
	defer cache.Close()

	key := "test-key-delete"
	value := []byte("test-value-delete")

	// Set a value
	cache.Set(key, value)

	// Verify it exists
	_, found := cache.Get(key)
	if !found {
		t.Error("Expected to find value before deletion")
	}

	// Delete it
	cache.Delete(key)

	// Verify it's gone
	_, found = cache.Get(key)
	if found {
		t.Error("Expected value to be deleted")
	}
}

// TestRedisCacheTTL tests that values expire after TTL.
// This test requires a Redis/Valkey server running on localhost:6379.
func TestRedisCacheTTL(t *testing.T) {
	cache := NewRedisCache("localhost", 6379, "", 2, 10, 20, 5) // 2 second TTL
	defer cache.Close()

	key := "test-key-ttl"
	value := []byte("test-value-ttl")

	// Set a value
	cache.Set(key, value)

	// Value should be present immediately
	_, found := cache.Get(key)
	if !found {
		t.Error("Expected to find value immediately after setting")
	}

	// Wait for expiration (2 seconds + buffer)
	time.Sleep(3 * time.Second)

	// Value should be expired in Redis
	_, found = cache.Get(key)
	if found {
		t.Error("Expected value to be expired after TTL")
	}
}

// TestRedisCacheGetNotFound tests GET with a non-existent key.
func TestRedisCacheGetNotFound(t *testing.T) {
	cache := NewRedisCache("localhost", 6379, "", 300, 10, 20, 5)
	defer cache.Close()

	_, found := cache.Get("nonexistent-key-12345")
	if found {
		t.Error("Expected not to find non-existent key")
	}
}

// TestRedisCacheFallbackOnConnectionFailure tests fallback to in-memory cache
// when Redis is unavailable.
func TestRedisCacheFallbackOnConnectionFailure(t *testing.T) {
	// Connect to a non-existent Redis server
	cache := NewRedisCache("localhost", 9999, "", 300, 10, 20, 5)
	defer cache.Close()

	key := "test-key-fallback"
	value := []byte("test-value-fallback")

	// Set should fall back to in-memory cache
	cache.Set(key, value)

	// Get should retrieve from in-memory cache
	got, found := cache.Get(key)
	if !found {
		t.Fatal("Expected to find value in fallback cache")
	}

	if string(got) != string(value) {
		t.Errorf("Expected value %q, got %q", string(value), string(got))
	}
}

// TestRedisCacheAuthentication tests Redis authentication.
// This test requires a Redis/Valkey server with requirepass configured.
// Skip if no password-protected Redis is available.
func TestRedisCacheAuthentication(t *testing.T) {
	// This test is skipped by default unless you have a Redis server
	// with password authentication configured
	t.Skip("Skipping authentication test - requires Redis with password")

	password := "test-password"
	cache := NewRedisCache("localhost", 6379, password, 300, 10, 20, 5)
	defer cache.Close()

	key := "test-key-auth"
	value := []byte("test-value-auth")

	cache.Set(key, value)
	got, found := cache.Get(key)

	if !found {
		t.Fatal("Expected to find value after authentication")
	}

	if string(got) != string(value) {
		t.Errorf("Expected value %q, got %q", string(value), string(got))
	}

	cache.Delete(key)
}

// TestRedisCacheClear tests the FLUSHDB operation.
func TestRedisCacheClear(t *testing.T) {
	cache := NewRedisCache("localhost", 6379, "", 300, 10, 20, 5)
	defer cache.Close()

	// Set multiple values
	cache.Set("clear-key-1", []byte("value1"))
	cache.Set("clear-key-2", []byte("value2"))
	cache.Set("clear-key-3", []byte("value3"))

	// Verify they're there
	if _, found := cache.Get("clear-key-1"); !found {
		t.Error("Expected to find clear-key-1")
	}

	// Clear the cache
	cache.Clear()

	// Verify they're gone
	if _, found := cache.Get("clear-key-1"); found {
		t.Error("Expected clear-key-1 to be cleared")
	}
	if _, found := cache.Get("clear-key-2"); found {
		t.Error("Expected clear-key-2 to be cleared")
	}
	if _, found := cache.Get("clear-key-3"); found {
		t.Error("Expected clear-key-3 to be cleared")
	}
}

// TestRedisCacheConcurrency tests concurrent access to Redis cache.
func TestRedisCacheConcurrency(t *testing.T) {
	cache := NewRedisCache("localhost", 6379, "", 300, 10, 20, 5)
	defer cache.Close()

	done := make(chan bool)

	// Concurrent writes
	for i := 0; i < 100; i++ {
		go func(n int) {
			key := fmt.Sprintf("concurrent-key-%d", n%10)
			value := []byte(fmt.Sprintf("value-%d", n))
			cache.Set(key, value)
			done <- true
		}(i)
	}

	// Wait for all writes to complete
	for i := 0; i < 100; i++ {
		<-done
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		go func(n int) {
			key := fmt.Sprintf("concurrent-key-%d", n%10)
			cache.Get(key)
			done <- true
		}(i)
	}

	// Wait for all reads to complete
	for i := 0; i < 100; i++ {
		<-done
	}

	// Clean up
	for i := 0; i < 10; i++ {
		cache.Delete(fmt.Sprintf("concurrent-key-%d", i))
	}
}

// TestRedisCacheBinaryData tests storing and retrieving binary data.
func TestRedisCacheBinaryData(t *testing.T) {
	cache := NewRedisCache("localhost", 6379, "", 300, 10, 20, 5)
	defer cache.Close()

	key := "test-binary-key"
	// Create some binary data with null bytes and special characters
	value := []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0xfd, 0x7f, 0x80, 0x81}

	cache.Set(key, value)
	got, found := cache.Get(key)

	if !found {
		t.Fatal("Expected to find binary value")
	}

	if len(got) != len(value) {
		t.Fatalf("Expected length %d, got %d", len(value), len(got))
	}

	for i := range value {
		if got[i] != value[i] {
			t.Errorf("Byte at position %d: expected 0x%02x, got 0x%02x", i, value[i], got[i])
		}
	}

	cache.Delete(key)
}

// TestRedisCacheLargeValue tests storing and retrieving large values.
func TestRedisCacheLargeValue(t *testing.T) {
	cache := NewRedisCache("localhost", 6379, "", 300, 10, 20, 5)
	defer cache.Close()

	key := "test-large-key"
	// Create a large value (1MB)
	value := make([]byte, 1024*1024)
	for i := range value {
		value[i] = byte(i % 256)
	}

	cache.Set(key, value)
	got, found := cache.Get(key)

	if !found {
		t.Fatal("Expected to find large value")
	}

	if len(got) != len(value) {
		t.Fatalf("Expected length %d, got %d", len(value), len(got))
	}

	// Verify a few random bytes
	for _, i := range []int{0, 1024, 1024 * 512, len(value) - 1} {
		if got[i] != value[i] {
			t.Errorf("Byte at position %d: expected 0x%02x, got 0x%02x", i, value[i], got[i])
		}
	}

	cache.Delete(key)
}

// TestRedisCacheConnectionPool tests that the connection pool works correctly.
func TestRedisCacheConnectionPool(t *testing.T) {
	cache := NewRedisCache("localhost", 6379, "", 300, 10, 20, 5)
	defer cache.Close()

	// Perform multiple operations that should reuse connections
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("pool-test-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))

		cache.Set(key, value)
		got, found := cache.Get(key)

		if !found {
			t.Errorf("Failed to find key %s", key)
		}

		if string(got) != string(value) {
			t.Errorf("Value mismatch for key %s", key)
		}

		cache.Delete(key)
	}
}

// TestRedisCacheSetWithTTL tests the SetWithTTL method.
func TestRedisCacheSetWithTTL(t *testing.T) {
	cache := NewRedisCache("localhost", 6379, "", 300, 10, 20, 5)
	defer cache.Close()

	key := "test-key-setttl"
	value := []byte("test-value-setttl")
	customTTL := 2 // 2 second TTL

	// Set with custom TTL
	err := cache.SetWithTTL(key, value, customTTL)
	if err != nil {
		t.Fatalf("SetWithTTL failed: %v", err)
	}

	// Value should be present immediately
	got, found := cache.Get(key)
	if !found {
		t.Fatal("Expected to find value immediately after setting")
	}

	if string(got) != string(value) {
		t.Errorf("Expected value %q, got %q", string(value), string(got))
	}

	// Wait for expiration (2 seconds + buffer)
	time.Sleep(3 * time.Second)

	// Value should be expired
	_, found = cache.Get(key)
	if found {
		t.Error("Expected value to be expired after custom TTL")
	}
}

// TestRedisCacheSetWithTTLDifferentFromDefault tests that SetWithTTL uses custom TTL not default.
func TestRedisCacheSetWithTTLDifferentFromDefault(t *testing.T) {
	cache := NewRedisCache("localhost", 6379, "", 10, 10, 20, 5) // Default 10 second TTL
	defer cache.Close()

	key := "test-key-custom-ttl"
	value := []byte("test-value-custom-ttl")
	customTTL := 2 // 2 second TTL (shorter than default)

	// Set with custom TTL
	err := cache.SetWithTTL(key, value, customTTL)
	if err != nil {
		t.Fatalf("SetWithTTL failed: %v", err)
	}

	// Value should be present immediately
	_, found := cache.Get(key)
	if !found {
		t.Fatal("Expected to find value immediately after setting")
	}

	// Wait for custom TTL to expire (should expire before default TTL)
	time.Sleep(3 * time.Second)

	// Value should be expired according to custom TTL
	_, found = cache.Get(key)
	if found {
		t.Error("Expected value to be expired after custom TTL, not default TTL")
	}
}

// TestRedisCacheSetWithTTLFallback tests that SetWithTTL falls back to memory cache on failure.
func TestRedisCacheSetWithTTLFallback(t *testing.T) {
	// Connect to a non-existent Redis server
	cache := NewRedisCache("localhost", 9999, "", 300, 10, 20, 5)
	defer cache.Close()

	key := "test-key-ttl-fallback"
	value := []byte("test-value-ttl-fallback")

	// SetWithTTL should fall back to in-memory cache and return error
	err := cache.SetWithTTL(key, value, 300)
	if err == nil {
		t.Error("Expected error when Redis is unavailable")
	}

	// Value should still be retrievable from fallback cache
	got, found := cache.Get(key)
	if !found {
		t.Fatal("Expected to find value in fallback cache")
	}

	if string(got) != string(value) {
		t.Errorf("Expected value %q, got %q", string(value), string(got))
	}
}

// TestRedisCacheConnectionLimiting tests that the connection pool and max connections limit work correctly.
func TestRedisCacheConnectionLimiting(t *testing.T) {
	// Create a cache with small pool and max connections for testing
	poolSize := 2
	maxConnections := 5
	connWaitTimeout := 2 // 2 second timeout
	cache := NewRedisCache("localhost", 6379, "", 300, poolSize, maxConnections, connWaitTimeout)
	defer cache.Close()

	// Test 1: Verify initial pool size
	if len(cache.connPool) < poolSize {
		t.Logf("Warning: Initial pool has %d connections, expected %d", len(cache.connPool), poolSize)
	}

	// Test 2: Verify semaphore has correct capacity
	if cap(cache.connSemaphore) != maxConnections {
		t.Errorf("Expected semaphore capacity %d, got %d", maxConnections, cap(cache.connSemaphore))
	}

	// Test 3: Perform operations up to max connections
	// This should succeed without blocking
	done := make(chan bool, maxConnections)
	results := make(chan error, maxConnections)

	// Start goroutines that hold connections
	for i := 0; i < maxConnections; i++ {
		go func(n int) {
			key := fmt.Sprintf("limit-test-%d", n)
			value := []byte(fmt.Sprintf("value-%d", n))

			// Set will acquire a connection
			cache.Set(key, value)

			// Signal completion
			results <- nil
			<-done // Wait for signal to release
		}(i)
	}

	// Wait for all goroutines to acquire connections
	for i := 0; i < maxConnections; i++ {
		select {
		case err := <-results:
			if err != nil {
				t.Errorf("Operation %d failed: %v", i, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("Timeout waiting for operation %d to complete", i)
		}
	}

	// Signal all goroutines to complete
	for i := 0; i < maxConnections; i++ {
		done <- true
	}

	// Test 4: Clean up - verify all data was written
	for i := 0; i < maxConnections; i++ {
		key := fmt.Sprintf("limit-test-%d", i)
		cache.Delete(key)
	}
}

// TestRedisCacheConnectionWaitTimeout tests that connection acquisition times out correctly.
func TestRedisCacheConnectionWaitTimeout(t *testing.T) {
	// Create a cache with tiny pool and max connections for testing timeout
	poolSize := 1
	maxConnections := 2
	connWaitTimeout := 1 // 1 second timeout
	cache := NewRedisCache("localhost", 6379, "", 300, poolSize, maxConnections, connWaitTimeout)
	defer cache.Close()

	// Hold all available connections
	conn1, err := cache.getConnection()
	if err != nil {
		t.Fatalf("Failed to get first connection: %v", err)
	}
	defer cache.releaseConnection(conn1)

	conn2, err := cache.getConnection()
	if err != nil {
		t.Fatalf("Failed to get second connection: %v", err)
	}
	defer cache.releaseConnection(conn2)

	// Now all maxConnections are in use
	// Try to get another connection - should timeout and fall back to memory cache
	start := time.Now()
	_, err = cache.getConnection()
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected error when all connections are exhausted")
	}

	// Verify it waited approximately the timeout period
	if elapsed < time.Duration(connWaitTimeout)*time.Second {
		t.Errorf("Expected to wait at least %d seconds, waited %v", connWaitTimeout, elapsed)
	}

	if elapsed > time.Duration(connWaitTimeout+2)*time.Second {
		t.Errorf("Waited too long: %v (expected ~%d seconds)", elapsed, connWaitTimeout)
	}
}

// TestRedisCacheConnectionPoolReuse tests that connections are properly reused from the pool.
func TestRedisCacheConnectionPoolReuse(t *testing.T) {
	poolSize := 3
	maxConnections := 10
	cache := NewRedisCache("localhost", 6379, "", 300, poolSize, maxConnections, 5)
	defer cache.Close()

	// Perform multiple operations - should reuse pooled connections
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("reuse-test-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))

		cache.Set(key, value)
		got, found := cache.Get(key)

		if !found {
			t.Errorf("Failed to find key %s", key)
		}

		if string(got) != string(value) {
			t.Errorf("Value mismatch for key %s", key)
		}

		cache.Delete(key)
	}

	// Verify we didn't exhaust the semaphore
	// The pool should have connections available
	poolLen := len(cache.connPool)
	if poolLen == 0 {
		t.Error("Expected connections to be returned to pool after operations")
	}
}

// TestRedisCacheFallbackOnConnectionExhaustion tests fallback to in-memory cache when connections are exhausted.
func TestRedisCacheFallbackOnConnectionExhaustion(t *testing.T) {
	// Create a cache with minimal limits
	poolSize := 1
	maxConnections := 1
	connWaitTimeout := 1 // 1 second timeout
	cache := NewRedisCache("localhost", 6379, "", 300, poolSize, maxConnections, connWaitTimeout)
	defer cache.Close()

	// Hold the only available connection
	conn, err := cache.getConnection()
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}
	defer cache.releaseConnection(conn)

	// Try to perform operations while connection is held
	// Should fall back to in-memory cache
	key := "exhaustion-test"
	value := []byte("test-value")

	cache.Set(key, value)

	// Value should be in fallback cache
	got, found := cache.fallback.Get(key)
	if !found {
		t.Error("Expected value to be in fallback cache when Redis connections exhausted")
	}

	if string(got) != string(value) {
		t.Errorf("Expected value %q, got %q", string(value), string(got))
	}
}
