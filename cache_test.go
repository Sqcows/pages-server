package pages_server

import (
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
	cache := NewRedisCache("localhost", 6379, "", 300)

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
}

// TestRedisCacheFallback tests that RedisCache falls back to MemoryCache.
func TestRedisCacheFallback(t *testing.T) {
	cache := NewRedisCache("localhost", 6379, "", 300)

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

	// Test delete
	cache.Delete(key)
	_, found = cache.Get(key)
	if found {
		t.Error("Expected value to be deleted")
	}
}
