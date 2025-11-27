# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://code.squarecows.com/SquareCows/pages-server/compare/v0.0.2...HEAD
[0.0.2]: https://code.squarecows.com/SquareCows/pages-server/compare/v0.0.1...v0.0.2
[0.0.1]: https://code.squarecows.com/SquareCows/pages-server/releases/tag/v0.0.1
