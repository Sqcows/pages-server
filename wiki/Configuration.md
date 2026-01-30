# Configuration Reference

This page documents all configuration options for Bovine Pages Server.

## Traefik Middleware Configuration

The plugin is configured in Traefik's dynamic configuration as a middleware.

### Required Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `pagesDomain` | string | Base domain for serving pages (e.g., `pages.example.com`) |
| `forgejoHost` | string | Forgejo/Gitea instance URL (e.g., `https://git.example.com`) |

### Optional Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forgejoToken` | string | "" | API token for Forgejo (required for private repos) |
| `errorPagesRepo` | string | "" | Repository for error pages and landing page (format: `username/repository`) |
| `enableCustomDomains` | bool | true | Enable custom domain support |
| `customDomainCacheTTL` | int | 600 | Deprecated: Custom domain mappings are now persistent |
| `enableCustomDomainDNSVerification` | bool | false | Enable DNS TXT record verification for custom domains |

### Redis Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `redisHost` | string | "" | Redis server host for caching |
| `redisPort` | int | 6379 | Redis server port |
| `redisPassword` | string | "" | Redis password |
| `redisPoolSize` | int | 10 | Size of idle connection pool |
| `redisMaxConnections` | int | 20 | Maximum total connections |
| `redisConnWaitTimeout` | int | 5 | Seconds to wait for connection |
| `cacheTTL` | int | 300 | Cache TTL in seconds for file content |

### Traefik Redis Provider Integration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `traefikRedisRouterEnabled` | bool | true | Enable automatic Traefik router registration |
| `traefikRedisCertResolver` | string | "letsencrypt-http" | Certificate resolver for custom domains |
| `traefikRedisRouterTTL` | int | 0 | TTL for router configs (0 = persistent) |
| `traefikRedisRootKey` | string | "traefik" | Redis root key for Traefik config |

#### Automatic PagesDomain Router Registration

When `traefikRedisRouterEnabled` is `true` and Redis is configured, the plugin automatically registers the base `pagesDomain` (e.g., `pages.example.com`) as a Traefik router on startup. This provides:

- **Automatic SSL** - Traefik requests a certificate for the base pages domain
- **HTTP to HTTPS redirect** - Handled by the middleware
- **Landing page serving** - The `errorPagesRepo` index.html is served at the base domain

No manual Traefik router configuration is needed for the base pages domain.

### Authentication Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `authCookieDuration` | int | 3600 | Authentication cookie validity in seconds |
| `authSecretKey` | string | "" | Secret key for HMAC cookie signing |

### Redirect Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `maxRedirects` | int | 25 | Maximum redirect rules from `.redirects` file |

## Complete Example

```yaml
http:
  middlewares:
    pages-server:
      plugin:
        pages-server:
          # Required
          pagesDomain: pages.example.com
          forgejoHost: https://git.example.com

          # Optional - Forgejo access
          forgejoToken: "your-api-token"

          # Optional - Error pages
          errorPagesRepo: "system/error-pages"

          # Optional - Custom domains
          enableCustomDomains: true
          enableCustomDomainDNSVerification: false

          # Optional - Redis caching
          redisHost: "localhost"
          redisPort: 6379
          redisPassword: ""
          redisPoolSize: 10
          redisMaxConnections: 20
          redisConnWaitTimeout: 5
          cacheTTL: 300

          # Optional - Traefik Redis provider
          traefikRedisRouterEnabled: true
          traefikRedisCertResolver: "letsencrypt-http"
          traefikRedisRouterTTL: 0
          traefikRedisRootKey: "traefik"

          # Optional - Authentication
          authCookieDuration: 3600
          authSecretKey: "your-random-secret-key"

          # Optional - Redirects
          maxRedirects: 25
```

## .pages File Configuration

The `.pages` file is a YAML file in each repository's root that configures how the repository is served.

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `enabled` | boolean | No | Enable/disable pages (default: true) |
| `custom_domain` | string | No | Custom domain for this site |
| `enable_branches` | array | No | Branch subdomains (requires `custom_domain`) |
| `password` | string | No | SHA256 hash for main branch password protection |
| `branchesPassword` | string | No | SHA256 hash for branch subdomain password protection |
| `directory_index` | boolean | No | Enable directory listings (default: false) |

### Basic Example

```yaml
enabled: true
```

### Full Example

```yaml
enabled: true
custom_domain: www.example.com
enable_branches:
  - stage
  - qa
  - dev
password: 89e01536ac207279409d4de1e5253e01f4a1769e696db0d6062ca9b8f56767c8
branchesPassword: 5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8
directory_index: true
```

### Alternative Array Syntax

```yaml
enabled: true
custom_domain: www.example.com
enable_branches: ["stage", "qa", "dev"]
```

## .redirects File Configuration

The `.redirects` file configures URL redirects for custom domains.

### Format

```
# Comment line
FROM:TO
```

### Example

```
# Redirect old URLs to new structure
old-page:new-page
blog/old-post:blog/new-post
legacy:https://newsite.com/
```

### Rules

- One redirect per line
- Format: `FROM:TO`
- Comments start with `#`
- Empty lines are ignored
- Maximum redirects controlled by `maxRedirects` config
- Only works on custom domains

## Environment Variables

The plugin doesn't directly read environment variables, but you can use Traefik's environment variable substitution in configuration files:

```yaml
http:
  middlewares:
    pages-server:
      plugin:
        pages-server:
          forgejoToken: "{{ env "FORGEJO_TOKEN" }}"
          authSecretKey: "{{ env "AUTH_SECRET_KEY" }}"
          redisPassword: "{{ env "REDIS_PASSWORD" }}"
```

## Security Recommendations

### Production Configuration

```yaml
http:
  middlewares:
    pages-server:
      plugin:
        pages-server:
          pagesDomain: pages.example.com
          forgejoHost: https://git.example.com

          # Enable DNS verification for security
          enableCustomDomainDNSVerification: true

          # Configure authentication
          authSecretKey: "use-a-strong-random-key-here"
          authCookieDuration: 3600

          # Use Redis for caching
          redisHost: "redis"
          redisPort: 6379
          redisPassword: "redis-password"

          # Connection pooling
          redisPoolSize: 10
          redisMaxConnections: 20
```

### Generating Auth Secret Key

```bash
# Generate a secure random key
openssl rand -hex 32
```

Use the output as your `authSecretKey` value.

## Error Pages and Landing Page

The `errorPagesRepo` parameter configures a repository that provides custom error pages and a landing page for the base pages domain.

### Repository Structure

```
error-pages/
├── .pages
└── public/
    ├── index.html      # Landing page (served at https://pages.example.com/)
    ├── 404.html        # Not Found error page
    ├── 500.html        # Internal Server Error page
    ├── 502.html        # Bad Gateway error page
    ├── 503.html        # Service Unavailable error page
    └── assets/
        ├── style.css   # Shared styles
        └── logo.png    # Branding
```

### Landing Page

The `public/index.html` file from the error pages repository is served as the **default landing page** when visitors access the base pagesDomain URL (e.g., `https://pages.example.com/` without any subdomain).

**Use cases:**
- Welcome page explaining your Pages service
- Documentation and getting started guides
- Directory of hosted sites
- Branding and promotional content

**Behavior:**
- `https://pages.example.com/` → serves `public/index.html` from `errorPagesRepo`
- `https://john.pages.example.com/` → serves from user's repositories (normal behavior)
- If no `errorPagesRepo` is configured or no `index.html` exists, returns 400 Bad Request

### Configuration Example

```yaml
http:
  middlewares:
    pages-server:
      plugin:
        pages-server:
          pagesDomain: pages.example.com
          forgejoHost: https://git.example.com
          errorPagesRepo: "system/error-pages"  # Repository for error pages and landing page
```
