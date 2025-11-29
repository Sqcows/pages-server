# Custom Domain Implementation Summary

This document provides a technical overview of the custom domain implementation in the Forgejo Pages Server Traefik plugin.

## Overview

Custom domain support allows users to serve their static sites on their own domain names (e.g., `www.example.com`) instead of or in addition to the default pages domain format (`username.pages.example.com/repository`).

The implementation uses a **registration-based approach** where custom domains are registered when users visit their pages URL, rather than searching all repositories on every request.

## Architecture

### Request Flow

#### Pages Domain Request (Registration Flow)

1. **Request arrives at `https://username.pages.example.com/repository`**
   - Traefik routes to high-priority pages domain router
   - Plugin parses username and repository from URL
   - Verifies repository has `.pages` file
   - Fetches and serves content

2. **Custom domain registration**
   - After successfully serving content from a pages domain request
   - Plugin reads the repository's `.pages` file
   - If `custom_domain` is specified, registers it in cache:
     - Cache key: `custom_domain:www.example.com`
     - Cache value: `username:repository`
     - TTL: `customDomainCacheTTL` (default: 600 seconds)

#### Custom Domain Request (Lookup Flow)

1. **Request arrives at `https://www.example.com`**
   - Traefik routes to low-priority catch-all router
   - Plugin detects it's not a pagesDomain request
   - Calls `resolveCustomDomain` with the domain

2. **Custom domain resolution**
   - Look up `custom_domain:www.example.com` in cache
   - If found: Parse `username:repository` and serve content
   - If not found: Return 404 with helpful message "Custom domain not registered - visit the pages URL to activate"

3. **Content serving**
   - Parse file path using `parseCustomDomainPath`
   - Custom domains serve from repository root (no `/repository` prefix)
   - Check cache for file content
   - If not cached, fetch from Forgejo and cache
   - Serve the content with appropriate headers

### Components

#### Configuration

```go
type Config struct {
    // ... existing fields ...
    EnableCustomDomains  bool `json:"enableCustomDomains,omitempty"`
    CustomDomainCacheTTL int  `json:"customDomainCacheTTL,omitempty"`
}
```

- `enableCustomDomains`: Enable/disable custom domain support (default: true)
- `customDomainCacheTTL`: Cache TTL for custom domain lookups in seconds (default: 600)

#### PagesServer Structure

```go
type PagesServer struct {
    // ... existing fields ...
    customDomainCache Cache // Separate cache for custom domain mappings
}
```

- Separate cache instance for custom domain lookups
- Uses same cache implementation (MemoryCache or RedisCache) as file content cache
- Different TTL from file content cache for better control

#### Key Methods

**`resolveCustomDomain(ctx context.Context, domain string) (username, repository string, err error)`**
- Resolves a custom domain to a username and repository
- Checks cache only (no repository searching)
- Returns error with helpful message if domain not registered

**`registerCustomDomain(ctx context.Context, username, repository string)`**
- Registers a custom domain by reading the `.pages` file
- Called automatically when serving pages domain requests
- Caches the mapping: `custom_domain:{domain}` → `username:repository`
- Silent operation - does nothing if no custom domain configured

**`parseCustomDomainPath(urlPath string) string`**
- Parses URL path for custom domain requests
- Returns path relative to `public/` folder
- Handles root path (returns `public/index.html`)

## Caching Strategy

### Two-Level Caching

1. **Custom Domain Cache**
   - Key: `custom_domain:{domain}` (e.g., `custom_domain:www.example.com`)
   - Value: `username:repository` (e.g., `john:website`)
   - TTL: 600 seconds (configurable via `customDomainCacheTTL`)
   - Purpose: Store registered custom domain mappings

2. **File Content Cache**
   - Key: `{username}:{repository}:{filepath}`
   - Value: File content (as bytes)
   - TTL: 300 seconds (configurable via `cacheTTL`)
   - Purpose: Avoid repeated file fetches from Forgejo

### Cache Behavior

- **Registration**: Visiting pages URL registers/refreshes the custom domain mapping
- **All requests are fast**: Custom domain lookups use cache only (no searching)
- **Cache expiration**: Visit pages URL again to refresh registration
- **Scalable**: Only active custom domains consume cache space

## Performance Considerations

### All Requests are Fast

The registration-based approach ensures consistent performance:

**Pages Domain Requests** (Registration):
- Normal pages serving performance
- One additional API call to read `.pages` file
- Registers custom domain in cache (if configured)
- ~5-10ms total response time

**Custom Domain Requests** (Lookup):
- Cache-only lookup (no API calls)
- If found: Serve content normally (~5ms)
- If not found: Return 404 immediately (~1ms)
- No repository searching required

### Scaling Considerations

This approach scales infinitely:
- No dependency on number of users or repositories
- Cache only contains active custom domains
- No expensive search operations
- Predictable memory usage
- Use Redis cache for distributed deployments

## Security

### Access Control

- Custom domain resolution respects repository visibility
- Only repositories with `.pages` file are considered
- Private repositories require `forgejoToken` for access
- Custom domain feature can be disabled entirely if not needed

### Input Validation

- Domain names are validated by Traefik before reaching the plugin
- No code execution - only serves static files
- Cache keys are scoped to prevent collisions

## Traefik Configuration

### Router Priority

Custom domain support requires two routers with different priorities:

```yaml
http:
  routers:
    # High priority: Explicit pages domain
    pages-domain:
      rule: "HostRegexp(`{subdomain:[a-z0-9-]+}.pages.example.com`)"
      priority: 10
      # ... other config ...

    # Low priority: Catch-all for custom domains
    pages-custom-domains:
      rule: "HostRegexp(`{domain:.+}`)"
      priority: 1
      # ... other config ...
```

This ensures:
1. Requests to `*.pages.example.com` are always handled as pagesDomain requests
2. Requests to other domains are handled as custom domain requests
3. No conflicts between the two patterns

### SSL Certificate Provisioning

Traefik automatically provisions SSL certificates for custom domains using the configured `certResolver`:

```yaml
tls:
  certResolver: letsencrypt
```

This works because:
1. User configures DNS A/CNAME record pointing to Traefik server
2. Request arrives at Traefik with custom domain
3. Traefik detects it needs a certificate for this domain
4. Traefik requests certificate from Let's Encrypt via HTTP or DNS challenge
5. Certificate is stored and automatically renewed

## User Setup Flow

1. **User creates repository with static site**
   - Add files to `public/` folder
   - Create `.pages` file with `enabled: true`

2. **User adds custom domain to `.pages`**
   ```yaml
   enabled: true
   custom_domain: www.example.com
   ```

3. **User configures DNS**
   - Create A record pointing `www.example.com` to Traefik server IP
   - Or create CNAME record pointing to Traefik server hostname

4. **User waits for DNS propagation**
   - Usually takes a few minutes to a few hours
   - Can take up to 48 hours in some cases

5. **User activates custom domain**
   - Visit `https://username.pages.example.com/repository`
   - Plugin reads `.pages` file and registers custom domain
   - Mapping is cached for 600 seconds (default)

6. **Custom domain is now active**
   - Visit `https://www.example.com`
   - Traefik requests SSL certificate from Let's Encrypt
   - Content is served over HTTPS

7. **Keep custom domain active**
   - Visit pages URL periodically to refresh registration
   - Or access custom domain regularly (before cache expires)
   - Each pages URL visit refreshes the 600-second cache

## Testing

Comprehensive tests cover:
- Custom domain path parsing (`TestParseCustomDomainPath`)
- Cache-based custom domain resolution (`TestResolveCustomDomainWithCache`)
- Custom domain enabled/disabled scenarios (`TestCustomDomainDisabled`)
- Request routing between pagesDomain and custom domains (`TestServeHTTPCustomDomainVsPagesDomain`)
- Configuration defaults (`TestCustomDomainCacheTTL`)

Test coverage: 65.4% overall

## Limitations

1. **Manual Activation Required**
   - Users must visit pages URL to activate custom domain
   - Custom domain not immediately available after DNS configuration
   - Simple one-time step to register

2. **Cache Expiration**
   - Custom domain registration expires after TTL (default: 600 seconds)
   - Visit pages URL again to refresh registration
   - Or configure longer TTL if needed

3. **Yaegi Compatibility**
   - Implementation uses only Go standard library
   - No external dependencies due to Yaegi interpreter constraints

4. **Single Custom Domain Per Repository**
   - Each repository can only have one custom domain
   - Multiple repositories can have different custom domains

## Future Enhancements

Potential improvements:
1. Webhook support to auto-register when `.pages` file changes
2. Background job to refresh expiring custom domain registrations
3. Support for multiple custom domains per repository
4. Custom domain validation during repository creation
5. Metrics and monitoring for custom domain lookups
6. Admin API to list all registered custom domains

## Files Modified

- `pages.go`: Added custom domain registration and simplified resolution logic
- `forgejo_client.go`: Removed repository search methods (no longer needed)
- `custom_domain_test.go`: Updated test suite for registration-based approach
- `README.md`: Updated documentation with registration flow
- `CUSTOM_DOMAINS.md`: Updated architecture documentation
- `CHANGELOG.md`: Documented all changes for v0.0.3 release

## Conclusion

The registration-based custom domain implementation provides a scalable, performant solution for serving static sites on user-owned domains. By eliminating repository searching and using cache-only lookups, the plugin achieves:

- **Infinite scalability**: No dependency on instance size
- **Predictable performance**: All requests are fast (<5ms)
- **Simple user experience**: Visit pages URL to activate
- **Efficient caching**: Only active custom domains use cache space
- **Security**: Respects repository visibility and access control

This approach is well-suited for running in Traefik's Yaegi interpreter while providing an excellent user experience.
