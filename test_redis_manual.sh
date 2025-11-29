#!/bin/bash
# Manual Redis Integration Test Script
# This script demonstrates Redis integration with the pages-server plugin

set -e

echo "=========================================="
echo "Redis Integration Test for pages-server"
echo "=========================================="
echo ""

# Check if Redis is running
echo "1. Checking if Redis is running..."
if redis-cli ping >/dev/null 2>&1; then
    echo "   ✓ Redis is running"
else
    echo "   ✗ Redis is not running"
    echo ""
    echo "Please start Redis first:"
    echo "  - Docker: docker run -d -p 6379:6379 redis:latest"
    echo "  - Homebrew: brew services start redis"
    echo "  - System: sudo systemctl start redis-server"
    exit 1
fi
echo ""

# Clean up any existing test data
echo "2. Cleaning up existing test data..."
redis-cli DEL test-plugin-key test-cli-key customdomain:test.example.com >/dev/null 2>&1 || true
echo "   ✓ Cleaned up"
echo ""

# Test 1: Set via Redis CLI, verify plugin can read it
echo "3. Test 1: Redis CLI → Plugin"
echo "   Setting value via redis-cli..."
redis-cli SET test-cli-key "Hello from Redis CLI" >/dev/null
echo "   ✓ Value set: test-cli-key = 'Hello from Redis CLI'"
echo ""
echo "   Now run Go test to verify plugin can read it:"
echo "   go test -v -run TestRedisCacheSetGet"
echo ""

# Test 2: Custom domain example
echo "4. Test 2: Custom Domain Mapping"
echo "   Setting custom domain mapping via redis-cli..."
redis-cli SETEX "customdomain:test.example.com" 600 "testuser/testrepo" >/dev/null
echo "   ✓ Custom domain set: test.example.com → testuser/testrepo (TTL: 600s)"
echo ""
echo "   Verify with redis-cli:"
redis-cli GET "customdomain:test.example.com"
echo ""
echo "   Check TTL:"
TTL=$(redis-cli TTL "customdomain:test.example.com")
echo "   TTL remaining: ${TTL}s"
echo ""

# Test 3: Show current Redis stats
echo "5. Redis Statistics"
echo "   Current keys in database:"
redis-cli DBSIZE
echo ""
echo "   List all test keys:"
redis-cli KEYS "*test*" || echo "   (none)"
echo ""

# Test 4: Connection info
echo "6. Redis Connection Info"
echo "   Connected clients:"
redis-cli CLIENT LIST | wc -l | xargs echo "  "
echo ""

echo "=========================================="
echo "Manual Tests Complete!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "1. Run: go test -v -run Redis"
echo "2. Check the values are accessible from both plugin and redis-cli"
echo "3. Verify TTL expiration works"
echo ""
echo "To clean up test data:"
echo "  redis-cli DEL test-cli-key customdomain:test.example.com"
echo ""
