# Forgejo Pages Server

A Traefik middleware plugin that provides static site hosting for Forgejo and Gitea repositories, similar to GitHub Pages and GitLab Pages.

## Features

- **Static Site Hosting**: Serve static files from `public/` folders in Forgejo/Gitea repositories
- **Automatic HTTPS**: Seamless integration with Traefik's Let's Encrypt ACME resolver
- **HTTP to HTTPS Redirect**: Automatic redirection from HTTP to HTTPS
- **Custom Domains**: Support for custom domains with manual DNS configuration
- **Profile Sites**: Personal pages served from `.profile` repository
- **Custom Error Pages**: Configurable error pages from a designated repository
- **Caching**: Built-in memory cache with optional Redis support
- **High Performance**: Target response time <5ms with caching

## URL Structure

The plugin supports two URL patterns:

1. **Repository Sites**: `https://$username.$domain/$repository/`
   - Serves files from the `public/` folder of `$username/$repository`
   - Example: `https://john.pages.example.com/blog/` serves from `john/blog/public/`

2. **Profile Sites**: `https://$username.$domain/`
   - Serves files from the `public/` folder of `$username/.profile`
   - Example: `https://john.pages.example.com/` serves from `john/.profile/public/`

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

## Installation

### 1. Add Plugin to Traefik Static Configuration

Add the plugin to your Traefik static configuration (`traefik.yml` or command line):

```yaml
experimental:
  plugins:
    pages-server:
      moduleName: code.squarecows.com/SquareCows/pages-server
      version: v0.0.1
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

### 4. Configure Router

Create a router that uses the plugin middleware:

```yaml
http:
  routers:
    pages:
      rule: "HostRegexp(`{subdomain:[a-z0-9-]+}.pages.example.com`)"
      entryPoints:
        - websecure
      middlewares:
        - pages-server
      service: noop@internal  # Plugin handles the response
      tls:
        certResolver: letsencrypt
        domains:
          - main: "pages.example.com"
            sans:
              - "*.pages.example.com"
```

## Configuration Reference

### Required Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `pagesDomain` | string | Base domain for serving pages (e.g., `pages.example.com`) |
| `forgejoHost` | string | Forgejo/Gitea instance URL (e.g., `https://git.example.com`) |

### Optional Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forgejoToken` | string | "" | API token for Forgejo (required for private repos) |
| `errorPagesRepo` | string | "" | Repository for custom error pages (format: `username/repository`) |
| `redisHost` | string | "" | Redis server host for caching |
| `redisPort` | int | 6379 | Redis server port |
| `redisPassword` | string | "" | Redis password |
| `cacheTTL` | int | 300 | Cache time-to-live in seconds |

## Custom Domains

To use a custom domain for your site:

1. Add the custom domain to your repository's `.pages` file:
   ```yaml
   enabled: true
   custom_domain: www.example.com
   ```

2. Manually create DNS records with your DNS provider:
   - Create an A record pointing `www.example.com` to your Traefik server's IP address
   - Or create a CNAME record pointing to your pages domain (e.g., `username.pages.example.com`)

3. Traefik's certificatesResolvers will automatically:
   - Request SSL certificates via Let's Encrypt
   - Serve your site on the custom domain with HTTPS

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

## Security

- **Public repositories only**: By default, only public repositories are accessible
- **Private repository support**: Use `forgejoToken` for authenticated access
- **HTTPS enforcement**: Automatic HTTP to HTTPS redirection
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

## Limitations

- The plugin runs in Traefik's Yaegi interpreter, which has some limitations compared to compiled Go
- SSL certificate management is handled by Traefik's certificatesResolvers configuration, not by the plugin
- Redis caching currently falls back to in-memory cache due to Yaegi compatibility

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
3. Allow time for DNS propagation (up to 48 hours)
4. Verify Traefik certificate resolver is configured
5. Ensure the DNS record points to your Traefik server's IP address

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
