# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
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

[Unreleased]: https://code.squarecows.com/SquareCows/pages-server/compare/v0.0.3...HEAD
[0.0.3]: https://code.squarecows.com/SquareCows/pages-server/compare/v0.0.2...v0.0.3
[0.0.2]: https://code.squarecows.com/SquareCows/pages-server/compare/v0.0.1...v0.0.2
[0.0.1]: https://code.squarecows.com/SquareCows/pages-server/releases/tag/v0.0.1
