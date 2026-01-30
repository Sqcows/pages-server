# Custom Domains

Custom domains allow you to serve your site on your own domain name (e.g., `www.example.com`) instead of the default pages domain (e.g., `username.pages.example.com/repository`).

## Overview

| Feature | Pages Domain | Custom Domain |
|---------|--------------|---------------|
| URL Format | `username.pages.example.com/repo` | `www.yourdomain.com` |
| Setup | Automatic | Manual DNS + activation |
| SSL | Wildcard certificate | Individual certificates |
| Branch Subdomains | Not supported | Supported |

## How Custom Domains Work

The plugin uses a registration-based approach:

1. **Configuration**: Add `custom_domain` to your repository's `.pages` file
2. **DNS Setup**: Create DNS records pointing to your Traefik server
3. **Activation**: Visit your pages URL to register the custom domain
4. **SSL Certificate**: Traefik automatically requests an SSL certificate
5. **Serving**: Requests to your custom domain serve content from your repository

## Setting Up a Custom Domain

### Step 1: Add Custom Domain to `.pages` File

Create or update your repository's `.pages` file:

```yaml
enabled: true
custom_domain: www.example.com
```

Commit and push the change:

```bash
git add .pages
git commit -m "Add custom domain"
git push
```

### Step 2: Configure DNS

Add DNS records with your DNS provider pointing to your Traefik server's IP address.

#### Option A: A Record (Recommended)

```
www.example.com.    A    YOUR_TRAEFIK_IP
```

#### Option B: CNAME Record

```
www.example.com.    CNAME    your-traefik-server.example.com.
```

**Note**: DNS propagation can take up to 48 hours, though it's usually complete within 5-15 minutes.

### Step 3: Configure Traefik

Ensure Traefik is configured with the Redis provider for dynamic router discovery:

**Static Configuration (traefik.yml)**:

```yaml
providers:
  redis:
    endpoints:
      - "localhost:6379"
    rootKey: "traefik"

certificatesResolvers:
  letsencrypt-http:
    acme:
      email: admin@example.com
      storage: acme.json
      httpChallenge:
        entryPoint: web
```

**Dynamic Configuration**:

```yaml
http:
  routers:
    # Catch-all for custom domains
    pages-custom-domains-https:
      rule: "HostRegexp(`{domain:.+}`)"
      priority: 1
      entryPoints:
        - websecure
      middlewares:
        - pages-server
      service: noop@internal

    # HTTP router for ACME challenges
    pages-http:
      rule: "HostRegexp(`{domain:.+}`)"
      entryPoints:
        - web
      middlewares:
        - pages-server
      service: noop@internal
```

### Step 4: Activate Your Custom Domain

Visit your pages URL to register the custom domain:

```
https://username.pages.example.com/repository
```

The plugin will:
1. Read your `.pages` file
2. Register the custom domain mapping
3. Write Traefik router configuration to Redis
4. Traefik automatically requests an SSL certificate

### Step 5: Verify It Works

After DNS propagation and SSL certificate generation:

```
https://www.example.com
```

Your site should now be accessible on your custom domain with HTTPS.

## DNS Verification (Security Feature)

To prevent malicious users from claiming domains they don't own, you can enable DNS TXT record verification.

### How It Works

1. User computes a SHA256 hash of their repository path
2. User adds a TXT record to their domain with the hash
3. Plugin verifies the TXT record before registering the domain

### Enabling DNS Verification

Add to your Traefik configuration:

```yaml
http:
  middlewares:
    pages-server:
      plugin:
        pages-server:
          pagesDomain: pages.example.com
          forgejoHost: https://git.example.com
          enableCustomDomainDNSVerification: true
```

### Setting Up DNS Verification

#### Step 1: Compute the Verification Hash

```bash
# For repository: squarecows/my-website
echo -n "squarecows/my-website" | shasum -a 256
# Output: f208d828f20d47c865802bb5c64c4b7832cd0f7fa974bc085f2d3e0a0a4b7c79
```

#### Step 2: Add DNS TXT Record

Add this TXT record to your custom domain:

```
TXT www.example.com bovine-pages-verification=f208d828f20d47c865802bb5c64c4b7832cd0f7fa974bc085f2d3e0a0a4b7c79
```

#### Step 3: Verify the TXT Record

```bash
dig TXT www.example.com
# or
nslookup -type=TXT www.example.com
```

#### Step 4: Activate the Custom Domain

Visit your pages URL to trigger verification and registration.

### Verification Hash Format

- **Input**: Repository path in format `owner/repository`
- **Algorithm**: SHA256
- **Output**: 64-character hexadecimal hash
- **TXT Record Format**: `bovine-pages-verification=<hash>`

## Branch Subdomains

Custom domains support branch subdomains for serving different branches on separate subdomains.

See [[Branch Subdomains]] for complete documentation.

### Quick Example

```yaml
enabled: true
custom_domain: example.com
enable_branches:
  - stage
  - qa
```

Results in:
- `example.com` → main branch
- `stage.example.com` → stage branch
- `qa.example.com` → qa branch

## Custom Domain Storage

### Persistent Storage

Custom domain mappings are stored without expiration:
- Mappings persist until explicitly deleted
- No TTL expiration
- Survives plugin restarts

### Cache Keys

The plugin stores two mappings for each custom domain:

| Key | Value | Purpose |
|-----|-------|---------|
| `custom_domain:www.example.com` | `username:repository` | Forward lookup |
| `username:repository` | `www.example.com` | Reverse lookup |

### Cleanup

Use external reaper scripts to validate and clean up inactive domains:
- Check if repository still exists
- Check if `.pages` file still has the custom domain
- Remove stale mappings

See the `reaper/` directory for cleanup scripts.

## Conflict Prevention

The plugin prevents custom domain conflicts:

- If a domain is already registered to a different repository, registration fails
- Error is logged with details of the conflicting repository
- Same repository can re-register (update) its domain

## Disabling Custom Domains

If you don't need custom domain support:

```yaml
http:
  middlewares:
    pages-server:
      plugin:
        pages-server:
          pagesDomain: pages.example.com
          forgejoHost: https://git.example.com
          enableCustomDomains: false
```

## Troubleshooting

### Custom domain not working

1. Verify DNS records are correctly configured
2. Check that `custom_domain` is in your `.pages` file
3. Visit your pages URL to activate the domain
4. Allow time for DNS propagation
5. Check Traefik logs for errors

### "Custom domain not registered" error

1. Visit your pages URL first: `https://username.pages.example.com/repository`
2. This registers the custom domain mapping
3. Then try your custom domain

### SSL certificate not generating

1. Verify DNS points to your Traefik server
2. Check ACME challenge is passing through
3. Ensure Traefik's Redis provider is configured
4. Check Let's Encrypt rate limits
5. Review Traefik logs for certificate errors

### DNS verification failing

1. Wait for DNS propagation (up to 48 hours)
2. Verify TXT record exists: `dig TXT www.example.com`
3. Check hash is computed correctly
4. Ensure no extra spaces in TXT record
5. Check plugin logs for expected hash

### Domain registered to wrong repository

1. The original repository must remove the `custom_domain` from its `.pages` file
2. Run the reaper script to clean up stale mappings
3. Then register from the correct repository
