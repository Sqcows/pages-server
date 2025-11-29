# Redis Integration Testing Guide

This document provides instructions for testing the Redis implementation in the pages-server Traefik plugin.

## Overview

The plugin now includes a complete Redis client implementation using only Go standard library packages, making it compatible with Traefik's Yaegi interpreter. The implementation includes:

- RESP (Redis Serialization Protocol) parser and encoder
- Connection pooling
- Password authentication support
- Automatic fallback to in-memory cache if Redis is unavailable
- Support for GET, SET, SETEX, DEL, and FLUSHDB commands

## Prerequisites

You need either:
- Redis server (version 6.0 or later)
- Valkey server (Redis fork)
- Docker/Podman to run a Redis/Valkey container

## Setting Up Redis for Testing

### Option 1: Using Docker

```bash
docker run -d --name redis-test -p 6379:6379 redis:latest
```

### Option 2: Using Podman

```bash
podman run -d --name redis-test -p 6379:6379 redis:latest
```

### Option 3: Using Valkey (Redis fork)

```bash
docker run -d --name valkey-test -p 6379:6379 valkey/valkey:latest
```

### Option 4: Using Homebrew (macOS)

```bash
brew install redis
brew services start redis
```

### Option 5: Using apt (Ubuntu/Debian)

```bash
sudo apt-get update
sudo apt-get install redis-server
sudo systemctl start redis-server
```

## Running the Tests

### 1. Run All Tests

```bash
go test -v
```

### 2. Run Only Redis Tests

```bash
go test -v -run Redis
```

### 3. Run Specific Redis Tests

```bash
# Test basic GET/SET operations
go test -v -run TestRedisCacheSetGet

# Test TTL expiration
go test -v -run TestRedisCacheTTL

# Test connection pool
go test -v -run TestRedisCacheConnectionPool

# Test binary data handling
go test -v -run TestRedisCacheBinaryData

# Test large values
go test -v -run TestRedisCacheLargeValue

# Test fallback behavior
go test -v -run TestRedisCacheFallbackOnConnectionFailure
```

## Manual Integration Testing

These tests verify that the plugin can interoperate with Redis CLI commands.

### Test 1: Plugin SET, Redis CLI GET

1. Start the Go test program or use the plugin
2. Set a value using the plugin:

```go
cache := NewRedisCache("localhost", 6379, "", 300)
cache.Set("test-key", []byte("test-value"))
```

3. Retrieve using Redis CLI:

```bash
redis-cli GET test-key
# Should output: "test-value"
```

### Test 2: Redis CLI SET, Plugin GET

1. Set a value using Redis CLI:

```bash
redis-cli SET mykey "myvalue"
```

2. Retrieve using the plugin:

```go
cache := NewRedisCache("localhost", 6379, "", 300)
value, found := cache.Get("mykey")
// found should be true
// string(value) should be "myvalue"
```

### Test 3: Custom Domain Caching

This tests the actual use case in the plugin.

1. Set up a test cache:

```bash
# Simulate custom domain mapping stored by plugin
redis-cli SET "customdomain:example.com" "username/repository"
redis-cli EXPIRE "customdomain:example.com" 600
```

2. Verify the plugin can read it:

```go
cache := NewRedisCache("localhost", 6379, "", 600)
value, found := cache.Get("customdomain:example.com")
// found should be true
// string(value) should be "username/repository"
```

3. Check TTL:

```bash
redis-cli TTL "customdomain:example.com"
# Should show remaining seconds (less than 600)
```

### Test 4: Binary Data

1. Store binary data via plugin:

```go
cache := NewRedisCache("localhost", 6379, "", 300)
binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
cache.Set("binary-key", binaryData)
```

2. Retrieve and verify via Redis CLI:

```bash
redis-cli --raw GET binary-key | xxd
# Should show the binary data correctly
```

### Test 5: Large Values

1. Store a large file (e.g., 1MB) via plugin:

```go
cache := NewRedisCache("localhost", 6379, "", 300)
largeData := make([]byte, 1024*1024)
// Fill with test data
cache.Set("large-key", largeData)
```

2. Verify size in Redis:

```bash
redis-cli STRLEN large-key
# Should output: 1048576
```

### Test 6: Concurrent Access

Run multiple instances accessing the same Redis keys:

```bash
# Terminal 1
go test -v -run TestRedisCacheConcurrency

# Terminal 2 (simultaneously)
go test -v -run TestRedisCacheConcurrency
```

Both should complete successfully without errors.

### Test 7: Connection Pool

Verify connection pooling is working:

```bash
# Monitor Redis connections
redis-cli CLIENT LIST

# In another terminal, run tests
go test -v -run TestRedisCacheConnectionPool

# Check CLIENT LIST again - should show reused connections
```

## Testing with Password Authentication

### Set up Redis with password:

```bash
# Using Docker
docker run -d --name redis-auth -p 6379:6379 redis:latest redis-server --requirepass mypassword

# Or edit redis.conf
requirepass mypassword
```

### Test authentication:

1. Modify the test to enable authentication:

```go
func TestRedisCacheAuthentication(t *testing.T) {
    // Remove the t.Skip() line
    password := "mypassword"
    cache := NewRedisCache("localhost", 6379, password, 300)
    defer cache.Close()

    // Test operations...
}
```

2. Run the authentication test:

```bash
go test -v -run TestRedisCacheAuthentication
```

## Performance Testing

### Benchmark Redis Operations

Create a benchmark file `cache_bench_test.go`:

```go
func BenchmarkRedisCacheGet(b *testing.B) {
    cache := NewRedisCache("localhost", 6379, "", 300)
    defer cache.Close()

    cache.Set("bench-key", []byte("bench-value"))

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        cache.Get("bench-key")
    }
}

func BenchmarkRedisCacheSet(b *testing.B) {
    cache := NewRedisCache("localhost", 6379, "", 300)
    defer cache.Close()

    value := []byte("bench-value")

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        cache.Set(fmt.Sprintf("bench-key-%d", i), value)
    }
}
```

Run benchmarks:

```bash
go test -bench=RedisCacheGet -benchmem
go test -bench=RedisCacheSet -benchmem
```

### Target Performance

The plugin aims for <5ms response time. Redis operations should be:
- GET: <1ms
- SET: <1ms
- Connection from pool: <0.1ms

## Troubleshooting

### Problem: Tests fail with "connection refused"

**Solution**: Ensure Redis is running:

```bash
redis-cli ping
# Should output: PONG
```

### Problem: Tests pass but Redis CLI doesn't show values

**Solution**: Check if tests are falling back to in-memory cache. Look for error messages in test output.

### Problem: Authentication fails

**Solution**: Verify password is correct:

```bash
redis-cli -a mypassword ping
# Should output: PONG
```

### Problem: Slow performance

**Solution**:
1. Check Redis is running locally (not over network)
2. Verify connection pooling is working
3. Check Redis memory usage: `redis-cli INFO memory`

### Problem: Connection pool exhaustion

**Solution**: Increase pool size in `cache.go`:

```go
poolSize: 20,  // Increase from 10
```

## Monitoring Redis in Production

### Check Connection Count

```bash
redis-cli CLIENT LIST | wc -l
```

### Monitor Commands

```bash
redis-cli MONITOR
```

### Check Memory Usage

```bash
redis-cli INFO memory
```

### Check Hit Rate

```bash
redis-cli INFO stats | grep keyspace
```

## Integration with Traefik

To use Redis caching in the Traefik plugin, configure:

```yaml
http:
  middlewares:
    pages-server:
      plugin:
        pages-server:
          pagesDomain: "pages.example.com"
          forgejoHost: "https://git.example.com"
          redisHost: "localhost"
          redisPort: 6379
          redisPassword: ""  # Optional
          cacheTTL: 300
          customDomainCacheTTL: 600
```

## Verifying Production Deployment

1. Check logs for Redis connection messages
2. Verify custom domains are cached:

```bash
redis-cli KEYS "customdomain:*"
```

3. Verify cache TTLs:

```bash
redis-cli TTL "customdomain:example.com"
```

4. Monitor cache hit rate by checking fallback usage

## Clean Up

### Stop and Remove Redis Container

```bash
docker stop redis-test
docker rm redis-test
```

### Clear All Test Data

```bash
redis-cli FLUSHDB
```

### Stop Redis Service

```bash
# Homebrew
brew services stop redis

# Systemd
sudo systemctl stop redis-server
```
