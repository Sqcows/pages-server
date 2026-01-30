# Getting Started

This guide will help you install and configure Bovine Pages Server with Traefik.

## Prerequisites

- **Traefik v2.0+** with plugin support enabled
- **Forgejo or Gitea** instance with public or token-accessible repositories
- **Redis** (optional but recommended for caching and custom domains)
- **Domain name** configured to point to your Traefik server

## Architecture Overview

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Browser   │────▶│   Traefik   │────▶│   Forgejo   │
│             │◀────│  + Plugin   │◀────│   /Gitea    │
└─────────────┘     └─────────────┘     └─────────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │    Redis    │
                    │   (Cache)   │
                    └─────────────┘
```

## Quick Start

### Step 1: Add Plugin to Traefik

Add the plugin to your Traefik static configuration (`traefik.yml`):

```yaml
experimental:
  plugins:
    pages-server:
      moduleName: github.com/sqcows/pages-server
      version: v0.3.2
```

### Step 2: Configure Middleware

Create the plugin middleware in your dynamic configuration:

```yaml
http:
  middlewares:
    pages-server:
      plugin:
        pages-server:
          pagesDomain: pages.example.com
          forgejoHost: https://git.example.com
```

### Step 3: Configure Routers

Add routers to use the middleware:

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

    # HTTP router (ACME challenges + redirect)
    pages-http:
      rule: "HostRegexp(`{domain:.+}`)"
      entryPoints:
        - web
      middlewares:
        - pages-server
      service: noop@internal
```

### Step 4: Create a Repository

Create a repository in Forgejo/Gitea with:

1. A `.pages` file in the root:
   ```yaml
   enabled: true
   ```

2. A `public/` folder with your static files:
   ```
   my-website/
   ├── .pages
   └── public/
       ├── index.html
       ├── style.css
       └── script.js
   ```

### Step 5: Access Your Site

Visit your site at:
```
https://username.pages.example.com/my-website/
```

## Complete Docker Compose Example

```yaml
version: '3.8'

services:
  traefik:
    image: traefik:v3.0
    command:
      - "--api.insecure=true"
      - "--providers.docker=true"
      - "--providers.file.directory=/etc/traefik/dynamic"
      - "--providers.redis.endpoints=redis:6379"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.letsencrypt.acme.httpchallenge=true"
      - "--certificatesresolvers.letsencrypt.acme.httpchallenge.entrypoint=web"
      - "--certificatesresolvers.letsencrypt.acme.email=admin@example.com"
      - "--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json"
      - "--experimental.plugins.pages-server.modulename=github.com/sqcows/pages-server"
      - "--experimental.plugins.pages-server.version=v0.3.2"
    ports:
      - "80:80"
      - "443:443"
      - "8080:8080"
    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock:ro"
      - "./traefik/dynamic:/etc/traefik/dynamic"
      - "./letsencrypt:/letsencrypt"
    depends_on:
      - redis

  redis:
    image: redis:7-alpine
    volumes:
      - redis-data:/data

volumes:
  redis-data:
```

Create `traefik/dynamic/pages.yml`:

```yaml
http:
  middlewares:
    pages-server:
      plugin:
        pages-server:
          pagesDomain: pages.example.com
          forgejoHost: https://git.example.com
          redisHost: redis
          redisPort: 6379

  routers:
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

    pages-custom-https:
      rule: "HostRegexp(`{domain:.+}`)"
      priority: 1
      entryPoints:
        - websecure
      middlewares:
        - pages-server
      service: noop@internal

    pages-http:
      rule: "HostRegexp(`{domain:.+}`)"
      entryPoints:
        - web
      middlewares:
        - pages-server
      service: noop@internal
```

## Setting Up a Landing Page

You can configure a custom landing page that's displayed when visitors access the base pages domain (e.g., `https://pages.example.com/`).

### Step 1: Create an Error Pages Repository

Create a repository (e.g., `system/error-pages`) with:

```
error-pages/
├── .pages
└── public/
    ├── index.html      # Landing page for base domain
    ├── 404.html        # Custom 404 error page
    └── 500.html        # Custom 500 error page
```

### Step 2: Add to Configuration

```yaml
http:
  middlewares:
    pages-server:
      plugin:
        pages-server:
          pagesDomain: pages.example.com
          forgejoHost: https://git.example.com
          errorPagesRepo: "system/error-pages"
```

Now `https://pages.example.com/` will display your landing page.

## Next Steps

- [[Configuration]] - Full configuration reference
- [[Custom Domains]] - Set up custom domains
- [[Branch Subdomains]] - Use branch-specific subdomains
- [[Password Protection]] - Protect sites with passwords
- [[Troubleshooting]] - Common issues and solutions

## Example Repository Structure

```
my-website/
├── .pages                 # Enable pages
├── .redirects             # Optional: URL redirects
└── public/
    ├── index.html         # Homepage
    ├── about.html         # About page
    ├── css/
    │   └── style.css      # Stylesheet
    ├── js/
    │   └── app.js         # JavaScript
    └── images/
        └── logo.png       # Images
```

### Minimal `.pages` File

```yaml
enabled: true
```

### Full `.pages` File

```yaml
enabled: true
custom_domain: www.example.com
enable_branches:
  - stage
  - qa
password: <sha256-hash>           # Main branch password
branchesPassword: <sha256-hash>   # Branch password
directory_index: true             # Enable directory listings
```

## Verify Installation

1. Check Traefik logs for plugin loading:
   ```bash
   docker logs traefik 2>&1 | grep pages-server
   ```

2. Check the Traefik dashboard (port 8080) for the middleware

3. Access a test repository:
   ```bash
   curl -I https://username.pages.example.com/test-repo/
   ```

4. Check response headers:
   ```
   Server: bovine
   X-Cache-Status: MISS  # or HIT if cached
   ```
