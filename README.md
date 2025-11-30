# Forgejo Pages Server

A Traefik middleware plugin that provides static site hosting for Forgejo and Gitea repositories, similar to GitHub Pages and GitLab Pages.

## Features

- **Static Site Hosting**: Serve static files from `public/` folders in Forgejo/Gitea repositories
- **Automatic HTTPS**: Seamless integration with Traefik's Let's Encrypt ACME resolver
- **ACME Challenge Passthrough**: Automatic handling of Let's Encrypt HTTP challenges for SSL certificate generation
- **HTTP to HTTPS Redirect**: Automatic redirection from HTTP to HTTPS (with ACME challenge exceptions)
- **Custom Domains**: Support for custom domains with manual DNS configuration and automatic SSL certificate provisioning
- **Directory Index Support**: Automatic `index.html` detection for directory URLs (e.g., `/pricing/` → `/pricing/index.html`)
- **Profile Sites**: Personal pages served from `.profile` repository
- **Custom Error Pages**: Configurable error pages from a designated repository
- **Caching**: Built-in memory cache with optional Redis support for improved performance
- **Redis Router Integration**: Automatic Traefik router registration for custom domains via Redis provider
- **High Performance**: Target response time <5ms with caching

## URL Structure

The plugin supports three URL patterns:

1. **Repository Sites**: `https://$username.$domain/$repository/`
   - Serves files from the `public/` folder of `$username/$repository`
   - Example: `https://john.pages.example.com/blog/` serves from `john/blog/public/`

2. **Profile Sites**: `https://$username.$domain/`
   - Serves files from the `public/` folder of `$username/.profile`
   - Example: `https://john.pages.example.com/` serves from `john/.profile/public/`

3. **Custom Domain Sites**: `https://www.example.com/`
   - Serves files from the `public/` folder of a repository with matching `custom_domain` in `.pages` file
   - Example: `https://www.example.com/` serves from the repository that has `custom_domain: www.example.com`

## Requirements

### Repository Setup

For a repository to be served by the plugin, it must have:

1. A `public/` folder containing static files (HTML, CSS, JS, images, etc.)
2. A `.pages` file in the repository root

### .pages File Format

The `.pages` file is a YAML configuration file:

```yaml
enabled: true
custom_domain: example.com  # Optional: custom domain for this site
```

### Directory Index Support

The plugin automatically detects directory URLs and serves `index.html`:

**Example Directory Structure:**
```
public/
├── index.html          # Served at: /
├── about.html         # Served at: /about.html
├── pricing/
│   └── index.html     # Served at: /pricing/ (automatic)
└── docs/
    ├── index.html     # Served at: /docs/ (automatic)
    └── guide.html     # Served at: /docs/guide.html
```

**How it works:**
- Accessing `/pricing/` automatically tries `/pricing/index.html`
- Enables clean URLs without file extensions
- Only applies to paths without file extensions (directories)
- Falls back to 404 if neither the directory nor `index.html` exists

## Installation

### 1. Add Plugin to Traefik Static Configuration

Add the plugin to your Traefik static configuration (`traefik.yml` or command line):

```yaml
experimental:
  plugins:
    pages-server:
      moduleName: code.squarecows.com/SquareCows/pages-server
      version: v0.0.3
```

### 2. Configure Let's Encrypt (ACME)

Configure Traefik's ACME certificate resolver in your static configuration:

```yaml
certificatesResolvers:
  letsencrypt:
    acme:
      email: admin@example.com
      storage: acme.json
      httpChallenge:
        entryPoint: web
      # OR use DNS challenge for wildcard certificates
      dnsChallenge:
        provider: cloudflare
        resolvers:
          - "1.1.1.1:53"
          - "8.8.8.8:53"
```

### 3. Configure Plugin Middleware

Add the plugin as a middleware in your dynamic configuration:

```yaml
http:
  middlewares:
    pages-server:
      plugin:
        pages-server:
          # Required parameters
          pagesDomain: pages.example.com
          forgejoHost: https://git.example.com
          # Optional parameters
          forgejoToken: your-forgejo-api-token  # Optional for public repos
          errorPagesRepo: system/error-pages  # Optional
          redisHost: localhost  # Optional
          redisPort: 6379       # Optional
          redisPassword: ""     # Optional
          cacheTTL: 300         # Optional (seconds)
```

### 4. Configure Routers

Create separate HTTP and HTTPS routers that use the plugin middleware:

```yaml
http:
  routers:
    # HTTPS router for pages domain
    pages-https:
      rule: "HostRegexp(`{subdomain:[a-z0-9-]+}.pages.example.com`)"
      priority: 10
      entryPoints:
        - websecure
      middlewares:
        - pages-server
      service: noop@internal
      tls:
        certResolver: letsencrypt
        domains:
          - main: "pages.example.com"
            sans:
              - "*.pages.example.com"

    # HTTPS router for custom domains (catch-all)
    # Individual domains get their own routers dynamically created in Redis
    pages-custom-domains-https:
      rule: "HostRegexp(`{domain:.+}`)"
      priority: 1
      entryPoints:
        - websecure
      middlewares:
        - pages-server
      service: noop@internal
      # No TLS certResolver - individual routers in Redis handle SSL certificates

    # HTTP router for all domains
    # Middleware handles ACME challenges and HTTPS redirect
    pages-http:
      rule: "HostRegexp(`{domain:.+}`)"
      entryPoints:
        - web
      middlewares:
        - pages-server
      service: noop@internal
```

**Important**: The routers must be split between HTTP (`web`) and HTTPS (`websecure`) entrypoints. The pages-server middleware automatically handles ACME challenges on the HTTP router before redirecting to HTTPS.

## Configuration Reference

### Required Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `pagesDomain` | string | Base domain for serving pages (e.g., `pages.example.com`) |
| `forgejoHost` | string | Forgejo/Gitea instance URL (e.g., `https://git.example.com`) |

### Optional Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forgejoToken` | string | "" | API token for Forgejo (required for private repos and custom domain lookups) |
| `errorPagesRepo` | string | "" | Repository for custom error pages (format: `username/repository`) |
| `enableCustomDomains` | bool | true | Enable custom domain support |
| `customDomainCacheTTL` | int | 600 | Cache TTL for custom domain lookups in seconds |
| `redisHost` | string | "" | Redis server host for caching |
| `redisPort` | int | 6379 | Redis server port |
| `redisPassword` | string | "" | Redis password |
| `cacheTTL` | int | 300 | Cache time-to-live in seconds |
| `traefikRedisRouterEnabled` | bool | true | Enable automatic Traefik router registration for custom domains |
| `traefikRedisCertResolver` | string | "letsencrypt-http" | Certificate resolver to use for dynamically registered custom domains |
| `traefikRedisRouterTTL` | int | 600 | TTL for Traefik router configurations in seconds |
| `traefikRedisRootKey` | string | "traefik" | Redis root key for Traefik configuration |

## Custom Domains

Custom domains allow you to serve your site on your own domain name (e.g., `www.example.com`) instead of the default pages domain (e.g., `username.pages.example.com/repository`).

### How Custom Domains Work

The plugin uses a registration-based approach for custom domains:

1. **Registration**: When you visit your pages URL (`https://username.pages.example.com/repository`), the plugin:
   - Reads your repository's `.pages` file
   - If a `custom_domain` is specified, registers it in the cache
   - Maps the custom domain to your repository

2. **Custom Domain Requests**: When a request arrives at your custom domain:
   - The plugin looks up the domain in the cache
   - If found, serves content from the registered repository
   - If not found, returns a helpful 404 message

3. **Cache Refresh**: The mapping is cached for `customDomainCacheTTL` seconds (default: 600)
   - Visit your pages URL again to refresh the registration
   - Keeps active custom domains fast without searching all repositories

### Setting Up a Custom Domain

1. **Add custom domain to your `.pages` file**:
   ```yaml
   enabled: true
   custom_domain: www.example.com
   ```

2. **Configure DNS with your DNS provider**:
   - Create an A record pointing `www.example.com` to your Traefik server's IP address
   - Or create a CNAME record pointing to your Traefik server's hostname

3. **Configure Traefik static config with Redis provider** (see [Traefik Redis Provider Integration](#traefik-redis-provider-integration)):
   ```yaml
   providers:
     redis:
       endpoints:
         - "localhost:6379"
       rootKey: "traefik"
   ```

4. **Configure Traefik routers to handle custom domains**:
   ```yaml
   http:
     routers:
       # HTTPS router for custom domains (catch-all)
       # Individual domains get their own routers dynamically created in Redis
       pages-custom-domains-https:
         rule: "HostRegexp(`{domain:.+}`)"
         priority: 1  # Lower priority than pages domain router
         entryPoints:
           - websecure
         middlewares:
           - pages-server
         service: noop@internal
         # No TLS certResolver - individual routers in Redis handle SSL certificates

       # HTTP router (handles ACME challenges and redirects)
       pages-http:
         rule: "HostRegexp(`{domain:.+}`)"
         entryPoints:
           - web
         middlewares:
           - pages-server
         service: noop@internal
   ```

5. **Activate your custom domain**:
   - Visit `https://username.pages.example.com/repository` to register the custom domain
   - The plugin reads your `.pages` file and writes a Traefik router configuration to Redis
   - Traefik's Redis provider automatically loads the router and requests an SSL certificate
   - Your custom domain is now active with automatic HTTPS

6. **Traefik automatically handles SSL certificates**:
   - Plugin creates individual routers in Redis for each custom domain
   - Traefik requests SSL certificates via Let's Encrypt automatically
   - Serves your site with HTTPS
   - Handles certificate renewal

### Custom Domain Caching

The plugin caches custom domain → repository mappings:
- Cache TTL: 600 seconds (configurable via `customDomainCacheTTL`)
- Only active custom domains are cached (those that have been visited)
- Visit your pages URL to refresh the registration
- Cache expires automatically after TTL

### Performance Considerations

- **All requests are fast**: Custom domain lookups use cache only (no repository searching)
- **Predictable performance**: <5ms response time for all requests
- **Scalable**: Works efficiently with any number of repositories
- **Cache refresh**: Simply visit your pages URL to keep the custom domain active

### Disabling Custom Domains

If you don't need custom domain support, you can disable it:

```yaml
http:
  middlewares:
    pages-server:
      plugin:
        pages-server:
          pagesDomain: pages.example.com
          forgejoHost: https://git.example.com
          enableCustomDomains: false  # Disable custom domains
```

## Custom Error Pages

To provide custom error pages:

1. Create a repository (e.g., `system/error-pages`)
2. Add a `.pages` file to enable it
3. Create error page files in the `public/` folder:
   - `public/404.html` - Not Found
   - `public/500.html` - Internal Server Error
   - `public/502.html` - Bad Gateway
   - `public/503.html` - Service Unavailable

4. Configure the `errorPagesRepo` parameter:
   ```yaml
   errorPagesRepo: system/error-pages
   ```

## Performance

The plugin includes several performance optimizations:

- **In-memory caching**: Static files are cached in memory (default TTL: 300 seconds)
- **Optional Redis caching**: For distributed deployments
- **Efficient file serving**: Direct content serving without disk I/O
- **Target response time**: <5ms with caching enabled

## ACME Challenge Handling

The plugin automatically handles Let's Encrypt ACME HTTP challenges for SSL certificate generation:

1. **Automatic Detection**: Detects requests to `/.well-known/acme-challenge/*` paths
2. **Passthrough**: Passes ACME challenges to Traefik's handler before HTTPS redirect
3. **Zero Configuration**: No special router rules or configuration needed
4. **Custom Domain Support**: Enables SSL certificate generation for custom domains

### How It Works

When Let's Encrypt validates a domain for SSL certificate generation:

1. Let's Encrypt sends an HTTP request to `http://your-domain/.well-known/acme-challenge/token`
2. The pages-server middleware detects the ACME challenge path
3. The middleware passes the request to Traefik's `next` handler (bypassing HTTPS redirect)
4. Traefik responds with the ACME challenge response
5. Let's Encrypt validates the response and generates the SSL certificate

This happens automatically without any special configuration or manual intervention.

## Security

- **Public repositories only**: By default, only public repositories are accessible
- **Private repository support**: Use `forgejoToken` for authenticated access
- **HTTPS enforcement**: Automatic HTTP to HTTPS redirection (with ACME challenge exceptions)
- **Input validation**: Request parsing with validation
- **No code execution**: Serves static files only

## Development

### Building and Testing

```bash
# Run tests
go test -v ./...

# Run tests with coverage
go test -v -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Project Structure

```
.
├── .traefik.yml           # Plugin manifest
├── go.mod                 # Go module definition
├── pages.go              # Main plugin code
├── pages_test.go         # Plugin tests
├── forgejo_client.go     # Forgejo API client
├── forgejo_client_test.go
├── cache.go              # Caching implementation
├── cache_test.go
├── README.md
└── CHANGELOG.md
```

## Compatibility

- **Traefik**: v2.0+ (with plugin support)
- **Forgejo**: All versions
- **Gitea**: All versions with compatible API
- **Go**: 1.21+
- **Yaegi**: Compatible (uses standard library where possible)

## Redis Caching

The plugin includes a full Redis client implementation using only Go standard library, making it compatible with Traefik's Yaegi interpreter.

### Features

- **RESP Protocol**: Complete implementation of Redis Serialization Protocol
- **Connection Pooling**: Efficient connection reuse for better performance
- **Authentication**: Support for password-protected Redis servers
- **Automatic Fallback**: Falls back to in-memory cache if Redis is unavailable
- **TTL Support**: Automatic key expiration using SETEX command

### Configuration

```yaml
http:
  middlewares:
    pages-server:
      plugin:
        pages-server:
          pagesDomain: pages.example.com
          forgejoHost: https://git.example.com
          redisHost: localhost
          redisPort: 6379
          redisPassword: ""  # Optional
          cacheTTL: 300
          customDomainCacheTTL: 600
```

### Testing Redis

See [REDIS_TESTING.md](REDIS_TESTING.md) for comprehensive testing instructions and manual integration tests.

## Traefik Redis Provider Integration

When using Redis caching, the plugin can automatically register custom domains with Traefik's Redis provider. This enables Traefik to dynamically discover custom domains and automatically request SSL certificates without manual router configuration.

### How It Works

When a custom domain is registered (by visiting the pages URL):

1. **Domain Mapping**: The plugin caches the custom domain → repository mapping (existing behavior)
2. **Router Registration**: The plugin writes Traefik router configuration to Redis
3. **Traefik Discovery**: Traefik's Redis provider reads the router configuration
4. **SSL Certificate**: Traefik automatically requests an SSL certificate for the custom domain
5. **Automatic Routing**: Requests to the custom domain are routed through the pages-server middleware

### Configuration

#### 1. Configure Traefik Static Configuration

Enable Traefik's Redis provider in your static configuration:

```yaml
providers:
  redis:
    endpoints:
      - "valkey:6379"  # Or "localhost:6379"
    rootKey: "traefik"
    # username: ""     # Optional
    # password: ""     # Optional

certificatesResolvers:
  letsencrypt-http:
    acme:
      email: admin@example.com
      storage: acme.json
      httpChallenge:
        entryPoint: web
```

#### 2. Configure Plugin Middleware

```yaml
http:
  middlewares:
    pages-server:
      plugin:
        pages-server:
          pagesDomain: pages.example.com
          forgejoHost: https://git.example.com
          redisHost: valkey  # Enable Redis caching
          redisPort: 6379
          # Traefik Redis provider settings (all optional)
          traefikRedisRouterEnabled: true  # Default: true
          traefikRedisCertResolver: letsencrypt-http  # Default: letsencrypt-http
          traefikRedisRouterTTL: 600  # Default: 600 seconds
          traefikRedisRootKey: traefik  # Default: traefik
```

#### 3. Configure a Noop Service

The dynamically registered routers need a service to route to. Create a noop service:

**Note**: No service configuration is required. The routers use Traefik's built-in `noop@internal` service since the middleware intercepts all requests.

### Router Registration Format

For each custom domain, the plugin writes the following keys to Redis:

```
traefik/http/routers/custom-{domain}/rule = "Host(`example.com`)"
traefik/http/routers/custom-{domain}/entryPoints/0 = "websecure"
traefik/http/routers/custom-{domain}/middlewares/0 = "pages-server@file"
traefik/http/routers/custom-{domain}/service = "noop@internal"
traefik/http/routers/custom-{domain}/tls/certResolver = "letsencrypt-http"
traefik/http/routers/custom-{domain}/priority = "10"
```

Where `{domain}` is the custom domain with dots replaced by dashes (e.g., `example-com`).

### Benefits

- **Zero Configuration**: No manual router creation for each custom domain
- **Automatic SSL**: Traefik automatically requests SSL certificates for custom domains
- **Dynamic Discovery**: Traefik discovers new custom domains automatically
- **TTL Management**: Router configurations expire after TTL (refreshed on each visit)
- **Scalable**: Works with unlimited custom domains without configuration changes

### Disabling Router Registration

If you prefer to manually configure routers or don't use Traefik's Redis provider:

```yaml
http:
  middlewares:
    pages-server:
      plugin:
        pages-server:
          pagesDomain: pages.example.com
          forgejoHost: https://git.example.com
          redisHost: localhost
          traefikRedisRouterEnabled: false  # Disable automatic router registration
```

### Fallback Behavior

- **No Redis**: If `redisHost` is not configured, router registration is automatically skipped
- **In-Memory Cache**: If using in-memory cache, router registration is automatically skipped
- **Disabled**: If `traefikRedisRouterEnabled` is false, router registration is skipped
- **Error Handling**: If router registration fails, an error is logged but the request continues normally

## Limitations

- The plugin runs in Traefik's Yaegi interpreter, which has some limitations compared to compiled Go
- SSL certificate management is handled by Traefik's certificatesResolvers configuration, not by the plugin

## Examples

### Example Repository Structure

```
my-website/
├── .pages                 # Enable pages for this repo
├── public/
│   ├── index.html
│   ├── about.html
│   ├── css/
│   │   └── style.css
│   ├── js/
│   │   └── script.js
│   └── images/
│       └── logo.png
└── README.md
```

### Example .pages File

```yaml
enabled: true
custom_domain: www.mysite.com
```

### Example Error Pages Repository

```
error-pages/
├── .pages
└── public/
    ├── 404.html
    ├── 500.html
    └── 503.html
```

## Troubleshooting

### Site not loading

1. Verify the repository has both `public/` folder and `.pages` file
2. Check that the repository is public or `forgejoToken` is configured
3. Review Traefik logs for error messages
4. Ensure DNS is configured correctly

### Custom domain not working

1. Verify DNS records are correctly configured with your DNS provider
2. Check that the custom domain is specified in `.pages` file
3. **Visit your pages URL** (`https://username.pages.example.com/repository`) to activate the custom domain
4. Allow time for DNS propagation (up to 48 hours)
5. Verify Traefik certificate resolver is configured
6. Ensure the DNS record points to your Traefik server's IP address
7. Check that `enableCustomDomains` is set to `true` (default)
8. Verify the router priority is set correctly (custom domain router should have lower priority than pages domain router)
9. If you see "Custom domain not registered", visit the pages URL first to activate it

### SSL certificate not generating for custom domain

1. **Check router configuration**: Ensure HTTP and HTTPS routers are split correctly
   - HTTP router should use `web` entrypoint only
   - HTTPS router should use `websecure` entrypoint only
   - Do NOT configure both entrypoints on the same router
2. **Verify ACME challenge passthrough**: The middleware automatically handles ACME challenges
   - Check Traefik logs for ACME challenge requests
   - Verify requests to `/.well-known/acme-challenge/*` are reaching Traefik
3. **Check Traefik static configuration**:
   - Remove automatic HTTP to HTTPS redirect from `entryPoints.web` configuration
   - The middleware handles HTTPS redirect (with ACME exceptions)
4. **Verify DNS configuration**:
   - Ensure DNS A record points to your Traefik server
   - Wait for DNS propagation (can take up to 48 hours)
5. **Check certificate resolver**:
   - Verify `certificatesResolvers.letsencrypt` is configured in static config
   - Check Let's Encrypt rate limits (max 50 certificates per domain per week)
   - Review Traefik logs for certificate generation errors

### Performance issues

1. Enable caching with appropriate TTL
2. Consider using Redis for distributed caching
3. Monitor Traefik resource usage
4. Check Forgejo API response times

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Write tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

This project is licensed under the GNU General Public License v3.0 (GPLv3) - see the [LICENSE](LICENSE) file for details.

Copyright (C) 2025 SquareCows

This program is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License for more details.

## Support

For issues, questions, or contributions, please visit:
https://code.squarecows.com/SquareCows/pages-server
