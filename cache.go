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
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
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

// RedisCache implements a Redis-based cache using the RESP protocol.
// This implementation uses only Go standard library for Yaegi compatibility.
type RedisCache struct {
	host     string
	port     int
	password string
	ttl      int
	mu       sync.RWMutex
	fallback *MemoryCache // Used only if Redis connection fails
	connPool chan net.Conn
	poolSize int
	timeout  time.Duration
}

// NewRedisCache creates a new Redis cache with connection pooling.
func NewRedisCache(host string, port int, password string, ttlSeconds int) *RedisCache {
	rc := &RedisCache{
		host:     host,
		port:     port,
		password: password,
		ttl:      ttlSeconds,
		fallback: NewMemoryCache(ttlSeconds),
		poolSize: 10,
		timeout:  5 * time.Second,
	}

	// Initialize connection pool
	rc.connPool = make(chan net.Conn, rc.poolSize)

	// Pre-populate pool with connections
	// If initial connections fail, we'll fall back to in-memory cache
	for i := 0; i < rc.poolSize; i++ {
		conn, err := rc.newConnection()
		if err == nil {
			rc.connPool <- conn
		}
	}

	return rc
}

// newConnection creates a new Redis connection and authenticates if password is set.
func (rc *RedisCache) newConnection() (net.Conn, error) {
	// Connect to Redis server
	// Use net.JoinHostPort for proper IPv6 support
	addr := net.JoinHostPort(rc.host, strconv.Itoa(rc.port))
	conn, err := net.DialTimeout("tcp", addr, rc.timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Authenticate if password is provided
	if rc.password != "" {
		err = rc.sendCommand(conn, "AUTH", rc.password)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to authenticate: %w", err)
		}

		// Read authentication response
		_, err = rc.readResponse(conn)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
	}

	return conn, nil
}

// getConnection retrieves a connection from the pool or creates a new one.
func (rc *RedisCache) getConnection() (net.Conn, error) {
	select {
	case conn := <-rc.connPool:
		// Test connection with PING
		err := rc.sendCommand(conn, "PING")
		if err != nil {
			conn.Close()
			// Connection is dead, create a new one
			return rc.newConnection()
		}

		// Read PING response
		_, err = rc.readResponse(conn)
		if err != nil {
			conn.Close()
			return rc.newConnection()
		}

		return conn, nil
	default:
		// Pool is empty, create new connection
		return rc.newConnection()
	}
}

// releaseConnection returns a connection to the pool.
func (rc *RedisCache) releaseConnection(conn net.Conn) {
	if conn == nil {
		return
	}

	select {
	case rc.connPool <- conn:
		// Connection returned to pool
	default:
		// Pool is full, close the connection
		conn.Close()
	}
}

// sendCommand sends a Redis command using RESP protocol.
// RESP format: *<number of arguments>\r\n$<length of argument 1>\r\n<argument 1>\r\n...
func (rc *RedisCache) sendCommand(conn net.Conn, args ...string) error {
	// Set write deadline
	conn.SetWriteDeadline(time.Now().Add(rc.timeout))

	// Build RESP array
	var cmd strings.Builder
	cmd.WriteString(fmt.Sprintf("*%d\r\n", len(args)))
	for _, arg := range args {
		cmd.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg))
	}

	// Send command
	_, err := conn.Write([]byte(cmd.String()))
	return err
}

// readResponse reads a Redis response using RESP protocol.
// Returns the response as interface{} which can be:
// - string (for simple strings and bulk strings)
// - int64 (for integers)
// - error (for errors)
// - nil (for null bulk strings)
// - []interface{} (for arrays)
func (rc *RedisCache) readResponse(conn net.Conn) (interface{}, error) {
	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(rc.timeout))

	reader := bufio.NewReader(conn)

	// Read first byte to determine response type
	typeByte, err := reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read response type: %w", err)
	}

	switch typeByte {
	case '+': // Simple string
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read simple string: %w", err)
		}
		return strings.TrimSuffix(line, "\r\n"), nil

	case '-': // Error
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read error: %w", err)
		}
		return nil, fmt.Errorf("redis error: %s", strings.TrimSuffix(line, "\r\n"))

	case ':': // Integer
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read integer: %w", err)
		}
		val, err := strconv.ParseInt(strings.TrimSuffix(line, "\r\n"), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse integer: %w", err)
		}
		return val, nil

	case '$': // Bulk string
		// Read length
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read bulk string length: %w", err)
		}
		length, err := strconv.Atoi(strings.TrimSuffix(line, "\r\n"))
		if err != nil {
			return nil, fmt.Errorf("failed to parse bulk string length: %w", err)
		}

		// -1 indicates null
		if length == -1 {
			return nil, nil
		}

		// Read the actual data
		data := make([]byte, length+2) // +2 for \r\n
		_, err = reader.Read(data)
		if err != nil {
			return nil, fmt.Errorf("failed to read bulk string data: %w", err)
		}

		return data[:length], nil // Return without \r\n

	case '*': // Array
		// Read number of elements
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read array length: %w", err)
		}
		count, err := strconv.Atoi(strings.TrimSuffix(line, "\r\n"))
		if err != nil {
			return nil, fmt.Errorf("failed to parse array length: %w", err)
		}

		// Read each element recursively
		result := make([]interface{}, count)
		for i := 0; i < count; i++ {
			elem, err := rc.readResponse(conn)
			if err != nil {
				return nil, fmt.Errorf("failed to read array element %d: %w", i, err)
			}
			result[i] = elem
		}

		return result, nil

	default:
		return nil, fmt.Errorf("unknown response type: %c", typeByte)
	}
}

// Get retrieves a value from Redis cache.
func (rc *RedisCache) Get(key string) ([]byte, bool) {
	conn, err := rc.getConnection()
	if err != nil {
		// Fall back to in-memory cache if Redis is unavailable
		return rc.fallback.Get(key)
	}
	defer rc.releaseConnection(conn)

	// Send GET command
	err = rc.sendCommand(conn, "GET", key)
	if err != nil {
		return rc.fallback.Get(key)
	}

	// Read response
	resp, err := rc.readResponse(conn)
	if err != nil {
		return rc.fallback.Get(key)
	}

	// Handle nil response (key not found)
	if resp == nil {
		return nil, false
	}

	// Convert response to []byte
	if data, ok := resp.([]byte); ok {
		return data, true
	}

	return nil, false
}

// Set stores a value in Redis cache with TTL.
func (rc *RedisCache) Set(key string, value []byte) {
	conn, err := rc.getConnection()
	if err != nil {
		// Fall back to in-memory cache if Redis is unavailable
		rc.fallback.Set(key, value)
		return
	}
	defer rc.releaseConnection(conn)

	// Send SETEX command (SET with expiration)
	// SETEX key seconds value
	err = rc.sendCommand(conn, "SETEX", key, strconv.Itoa(rc.ttl), string(value))
	if err != nil {
		rc.fallback.Set(key, value)
		return
	}

	// Read response (should be +OK)
	_, err = rc.readResponse(conn)
	if err != nil {
		rc.fallback.Set(key, value)
		return
	}

	// Also update fallback cache for consistency
	rc.fallback.Set(key, value)
}

// Delete removes a value from Redis cache.
func (rc *RedisCache) Delete(key string) {
	conn, err := rc.getConnection()
	if err != nil {
		// Fall back to in-memory cache if Redis is unavailable
		rc.fallback.Delete(key)
		return
	}
	defer rc.releaseConnection(conn)

	// Send DEL command
	err = rc.sendCommand(conn, "DEL", key)
	if err != nil {
		rc.fallback.Delete(key)
		return
	}

	// Read response (returns number of keys deleted)
	_, err = rc.readResponse(conn)
	if err != nil {
		rc.fallback.Delete(key)
		return
	}

	// Also delete from fallback cache
	rc.fallback.Delete(key)
}

// Clear removes all items from Redis cache.
// Note: This uses FLUSHDB which clears the entire database.
// In production, you may want to use key prefixes and delete by pattern instead.
func (rc *RedisCache) Clear() {
	conn, err := rc.getConnection()
	if err != nil {
		// Fall back to in-memory cache if Redis is unavailable
		rc.fallback.Clear()
		return
	}
	defer rc.releaseConnection(conn)

	// Send FLUSHDB command
	err = rc.sendCommand(conn, "FLUSHDB")
	if err != nil {
		rc.fallback.Clear()
		return
	}

	// Read response
	_, err = rc.readResponse(conn)
	if err != nil {
		rc.fallback.Clear()
		return
	}

	// Also clear fallback cache
	rc.fallback.Clear()
}

// Close closes all connections in the pool.
func (rc *RedisCache) Close() {
	close(rc.connPool)
	for conn := range rc.connPool {
		if conn != nil {
			conn.Close()
		}
	}
	if rc.fallback != nil {
		rc.fallback.Stop()
	}
}
