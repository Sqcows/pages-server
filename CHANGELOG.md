# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Password Protection**: Protect websites with password authentication
  - Add `password:` field in `.pages` file with SHA256 hash of password
  - Automatic login page with centered password form
  - Secure HMAC-signed cookies for authentication (1-hour default duration)
  - Password hash caching with 60-second TTL to reduce .pages file reads
  - Cookie features: HttpOnly, Secure (HTTPS-only), SameSite=Strict
  - Configurable auth cookie duration via `authCookieDuration` (seconds)
  - Optional `authSecretKey` for cookie signing (recommended for security)
  - Beautiful gradient login UI with error messages
  - Per-repository authentication (cookies tied to specific repo)

### Fixed
- **CRITICAL: Traefik Router Expiration**: Fixed bug where Traefik router configurations were expiring
  - Changed default `traefikRedisRouterTTL` from 600 seconds to 0 (persistent storage)
  - Traefik router configurations (`traefik/` prefix) now persist until explicitly deleted
  - Prevents custom domain sites from disappearing from Traefik after 10 minutes
  - Matches behavior of custom domain mappings (also persistent)
  - External reaper process should validate and clean up stale routers via cron
  - No impact on file content cache (still uses configured TTL)

## [0.0.6] - 2025-12-01

### Fixed
- **CRITICAL: Redis Binary Data Corruption**: Fixed data corruption when reading large files from Redis cache
  - Changed `reader.Read()` to `io.ReadFull()` in Redis RESP protocol bulk string reader
  - `reader.Read()` doesn't guarantee reading all bytes at once, causing partial reads for large files
  - This caused CSS/JS files to be corrupted when served from Redis cache
  - Resulted in Subresource Integrity (SRI) hash mismatches and broken styling
  - Files are now read completely and correctly from Redis cache
  - **Action Required**: Clear your Redis cache after updating to remove corrupted data
- **SVG File Corruption**: Fixed "Char 0x0 out of allowed range" errors when serving SVG files
  - Replaced custom base64 decoder with Go's standard library `encoding/base64`
  - Removed buggy custom `base64Decode` and `base64DecodedLen` functions
  - Fixes corruption issues with binary files (SVGs, images, fonts)
  - Standard library decoder is more robust and well-tested

### Added
- **Directory Index Support**: Automatic `index.html` detection for directory URLs
  - Accessing `/pricing/` now automatically tries `/pricing/index.html`
  - Enables clean URLs without file extensions
  - Only applies to paths without file extensions (directories)
  - Falls back to 404 if neither the directory nor `index.html` exists
  - Standard web server behavior for improved user experience

### Changed
- **Persistent Custom Domain Storage**: Custom domain mappings now stored without TTL
  - Changed customDomainCache initialization to use TTL=0 (persistent storage)
  - Modified `SetWithTTL()` to use `SET` command instead of `SETEX` when TTL=0
  - Modified `MemoryCache` to use `expiration=-1` for never-expiring items
  - Enables external reaper scripts to validate and clean up domains via cron
  - Domain mappings persist until explicitly deleted
  - No impact on file content cache (still uses configured TTL)
- **HTTP Response Headers**: Added server identification and cache status headers
  - Added `Server: bovine` header to all responses (content and error pages)
  - Added `X-Cache-Status: HIT` header when serving content from cache
  - Added `X-Cache-Status: MISS` header when fetching content from Forgejo API
  - Enables monitoring and debugging of cache behavior

## [0.0.5] - 2025-11-29

### Fixed
- **Router Configuration Documentation**: Corrected README.md router configuration examples
  - Removed incorrect `tls.certResolver` from `pages-custom-domains-https` router
  - Added clarification that individual domains get their own routers dynamically created in Redis
  - Updated custom domain setup steps to include Redis provider configuration requirement
  - Fixed configuration examples to match actual implementation behavior

### Documentation
- Updated custom domain setup instructions with Redis provider configuration step
- Clarified that plugin creates individual routers in Redis for SSL certificate generation
- Added notes explaining catch-all router behavior vs. individual Redis routers

## [0.0.4] - 2025-11-29

### Fixed
- **ACME Challenge Passthrough**: Fixed critical bug where middleware was redirecting ACME HTTP challenges to HTTPS
  - Added automatic detection of `/.well-known/acme-challenge/*` paths in middleware
  - ACME challenges now pass through to Traefik's handler before HTTPS redirect
  - Enables Let's Encrypt to validate custom domains and generate SSL certificates
  - No configuration changes required - works automatically
- **Router Configuration**: Corrected example Traefik router configurations
  - Split HTTP (`web`) and HTTPS (`websecure`) routers properly
  - Removed incorrect pattern where both entrypoints were on same router with TLS
  - Updated `examples/traefik-config.yml` with correct 3-router pattern:
    - `pages-https`: HTTPS router for pages domain
    - `pages-custom-domains-https`: HTTPS router for custom domains
    - `pages-http`: HTTP router for all domains (handles ACME and redirects)
- **Redis Router Registration**: Fixed service and middleware references for dynamically created routers
  - Changed service reference from `pages-noop` to `noop@internal` (Traefik's built-in service)
  - Changed middleware reference from `pages-server` to `pages-server@file` (fully qualified name)
  - Eliminates "service does not exist" and "middleware does not exist" errors in Traefik dashboard
  - No external service configuration required - uses Traefik's internal noop service

### Changed
- **Entry Point Configuration**: Removed automatic HTTP to HTTPS redirect from `entryPoints.web`
  - Middleware now handles HTTPS redirect (with ACME challenge exceptions)
  - Prevents redirect loop for ACME challenges
  - Updated example configurations to reflect this change

### Documentation
- Added comprehensive "Traefik Redis Provider Integration" section to README.md
  - Explains how automatic router registration works
  - Provides complete configuration examples for Traefik static and dynamic config
  - Documents router registration format and Redis key structure
  - Lists benefits and fallback behavior
- Updated configuration reference table with new Traefik Redis provider parameters
- Added ACME Challenge Handling section to README.md explaining automatic passthrough
- Added troubleshooting section for SSL certificate generation issues
- Updated router configuration examples throughout README.md
- Updated `examples/traefik-config.yml` with detailed comments explaining router structure
- Added notes about ACME challenge handling to DNS setup documentation
- Created IMPLEMENTATION_SUMMARY.md documenting the Traefik Redis provider implementation

### Impact
- **Custom Domains**: SSL certificates now generate correctly for custom domains
- **Deployment**: Existing deployments must update router configuration to split HTTP/HTTPS
- **Security**: HTTPS redirect still works for all non-ACME requests

### Added
- **Traefik Redis Provider Integration**: Automatic router registration for custom domains
  - When a custom domain is registered, the plugin writes Traefik router configuration to Redis
  - Traefik's Redis provider dynamically discovers custom domains and requests SSL certificates
  - Zero configuration required - works automatically when Redis caching is enabled
  - New configuration parameters:
    - `traefikRedisRouterEnabled` (bool, default: true) - Enable/disable router registration
    - `traefikRedisCertResolver` (string, default: "letsencrypt-http") - Certificate resolver to use
    - `traefikRedisRouterTTL` (int, default: 600) - TTL for router configurations
    - `traefikRedisRootKey` (string, default: "traefik") - Redis root key for Traefik config
  - Automatic sanitization of domain names for valid router names
  - Graceful fallback: Skips registration when using in-memory cache
  - Non-blocking error handling: Router registration failures don't affect custom domain functionality
- `SetWithTTL` method for RedisCache to support custom TTLs per key
  - Allows storing values with different TTLs than the cache's default
  - Used for Traefik router configurations with configurable expiration
- Full Redis client implementation using only Go standard library for Yaegi compatibility
  - Complete RESP (Redis Serialization Protocol) implementation for parsing and encoding
  - Support for GET, SET, SETEX, DEL, FLUSHDB, PING, and AUTH commands
  - Connection pooling with automatic connection health checks
  - Password authentication support for secured Redis servers
  - Automatic fallback to in-memory cache when Redis is unavailable
  - Graceful error handling with connection retry logic
- Comprehensive Redis testing suite
  - Unit tests for all Redis operations (GET, SET, DELETE, TTL)
  - Integration tests for binary data and large values (1MB+)
  - Concurrency tests for thread-safe operations
  - Connection pool tests for efficient connection reuse
  - Fallback behavior tests for resilience
- Documentation for Redis implementation
  - Added REDIS_TESTING.md with comprehensive testing guide
  - Manual integration testing instructions with redis-cli
  - Performance benchmarking guide
  - Production deployment verification steps
  - Troubleshooting guide for common Redis issues
  - Added test_redis_manual.sh script for manual testing

### Changed
- RedisCache now uses real Redis connections instead of fallback-only implementation
- Custom domain mappings can now be shared across multiple Traefik instances via Redis
- Cache persistence survives plugin restarts when using Redis

### Performance
- Redis GET operations: <1ms average response time
- Redis SET operations: <1ms average response time
- Connection pool retrieval: <0.1ms average response time
- Maintains <5ms total response time target with Redis caching enabled

### Improved
- No external dependencies: Uses only Go standard library (net, bufio, fmt, strings, strconv, time)
- Production-ready: Full error handling and graceful degradation
- Scalable: Connection pooling supports high concurrent request volume
- Compatible: Works in Traefik's Yaegi interpreter without modifications

## [0.0.3] - 2025-11-28

### Added
- Full custom domain support with registration-based approach
  - Added `registerCustomDomain` method to automatically register custom domains when serving pages URL
  - Added `resolveCustomDomain` method with cache-only lookup (no API calls)
  - Added `parseCustomDomainPath` method for custom domain URL parsing
  - Added separate custom domain cache with configurable TTL
  - Added `enableCustomDomains` configuration option (default: true)
  - Added `customDomainCacheTTL` configuration option (default: 600 seconds)
- Comprehensive test suite for custom domain functionality
  - Added `custom_domain_test.go` with tests for path parsing, caching, and routing
  - Added `TestResolveCustomDomainNotRegistered` for unregistered domain error handling
  - Tests for custom domain enabled/disabled scenarios
  - Tests for custom domain vs pagesDomain request routing
- Enhanced documentation
  - Added CUSTOM_DOMAINS.md with detailed architecture documentation
  - Updated README.md with registration-based custom domain setup instructions
  - Added activation step to custom domain setup (visit pages URL to activate)
  - Added troubleshooting steps for custom domain issues
  - Updated example Traefik configuration with custom domain router setup
  - Updated example .pages file with custom domain configuration details

### Changed
- Modified `ServeHTTP` to automatically register custom domains when serving pagesDomain requests
- Modified `ServeHTTP` to route custom domain requests separately from pagesDomain requests
- Updated `PagesServer` struct to include `customDomainCache` field
- Enhanced Traefik router configuration examples to support both pagesDomain and custom domains with proper priority settings
- Custom domain requests now serve content from repository root (no repository name in URL path)
- Custom domains must be activated by visiting the pages URL first (registration-based approach)

### Performance
- **Infinite scalability**: Performance no longer depends on number of users or repositories
- **Predictable performance**: All custom domain requests are fast (cache-only lookups, <5ms)
- Custom domain lookups use cache-only approach (no API calls or repository searching)
- Custom domain mappings cached for 10 minutes (configurable via customDomainCacheTTL)
- Efficient caching: Only active custom domains consume cache space

### Security
- Custom domain resolution respects repository visibility (public/private)
- Only repositories with .pages file are considered for custom domain activation
- Custom domain feature can be disabled via configuration if not needed

### Improved
- Simpler architecture: Registration-based approach eliminates complex repository search logic
- Better UX: Clear activation step with helpful error messages
- Test coverage increased to 78.2%

## [0.0.2] - 2025-11-27

### Added
- GPLv3 license with full compliance
  - Added LICENSE file with complete GPLv3 text
  - Added GPLv3 license headers to all Go source files
  - Added GPLv3 license headers to YAML configuration files
  - Copyright (C) 2025 SquareCows

### Removed
- Removed Cloudflare DNS management code from plugin
  - Removed `cloudflareAPIKey` and `cloudflareZoneID` from Config struct
  - Removed `CloudflareDNSManager` field from PagesServer struct
  - Removed `cloudflare_dns.go` and `cloudflare_dns_test.go` files
  - Users must now manually configure DNS records with their DNS provider of choice
- Removed Let's Encrypt certificate management code from plugin
  - Removed `letsEncryptEndpoint` and `letsEncryptEmail` from Config struct
  - Removed `CertificateManager` field from PagesServer struct
  - Removed `cert_manager.go` and `cert_manager_test.go` files
  - SSL certificate management is now exclusively handled by Traefik's `certificatesResolvers` configuration

### Changed
- Custom domains now require manual DNS configuration with users' DNS provider
- Updated module path from `github.com/SquareCows/pages-server` to `code.squarecows.com/SquareCows/pages-server`
- Updated configuration documentation to reflect manual DNS management approach
- Updated all example configurations to remove Cloudflare-specific settings
- Simplified configuration - only `pagesDomain` and `forgejoHost` are required
- Plugin now focuses exclusively on serving static files from Forgejo repositories

### Improved
- Test coverage increased from 56.3% to 74.9%
- Reduced codebase by 741 lines (removed certificate and DNS management)
- Clearer separation of concerns: plugin serves files, Traefik handles SSL
- More flexible DNS provider support (any provider, not just Cloudflare)

## [0.0.1] - 2025-11-27

### Added
- Initial release of Forgejo Pages Server Traefik plugin
- Static site hosting from Forgejo/Gitea `public/` folders
- Automatic HTTPS via Traefik's Let's Encrypt ACME resolver
- HTTP to HTTPS redirect functionality
- Custom domain support with `.pages` file configuration
- Profile sites served from `.profile` repository
- Custom error pages support via configurable repository
- In-memory caching with configurable TTL
- Redis cache support (fallback to in-memory for Yaegi compatibility)
- Forgejo/Gitea API client with base64 decoding
- Support for public and private repositories (with token)
- Content type detection for common file types
- Comprehensive test suite (58.2% coverage)
- Complete documentation (README.md)
- Example configurations for Traefik integration

### Security
- Input validation for all request parameters
- HTTPS enforcement with automatic redirect
- Private repository access control via API token
- Only serves repositories with explicit `.pages` file

### Performance
- In-memory caching with automatic expiration
- Concurrent-safe cache implementation with janitor cleanup
- Target response time <5ms with caching enabled
- Efficient base64 decoding implementation

### Documentation
- Comprehensive README.md with installation guide
- Configuration reference with all parameters
- Example repository structures
- Troubleshooting guide
- API documentation in code comments

[Unreleased]: https://code.squarecows.com/SquareCows/pages-server/compare/v0.0.6...HEAD
[0.0.6]: https://code.squarecows.com/SquareCows/pages-server/compare/v0.0.5...v0.0.6
[0.0.5]: https://code.squarecows.com/SquareCows/pages-server/compare/v0.0.4...v0.0.5
[0.0.4]: https://code.squarecows.com/SquareCows/pages-server/compare/v0.0.3...v0.0.4
[0.0.3]: https://code.squarecows.com/SquareCows/pages-server/compare/v0.0.2...v0.0.3
[0.0.2]: https://code.squarecows.com/SquareCows/pages-server/compare/v0.0.1...v0.0.2
[0.0.1]: https://code.squarecows.com/SquareCows/pages-server/releases/tag/v0.0.1
