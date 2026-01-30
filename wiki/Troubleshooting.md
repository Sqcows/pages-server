# Troubleshooting

This page covers common issues and their solutions when using Bovine Pages Server.

## Site Not Loading

### Symptoms
- 404 Not Found
- "Repository not found or not configured for pages"

### Solutions

1. **Verify repository has `.pages` file**
   ```bash
   # Check if .pages file exists
   curl https://git.example.com/api/v1/repos/username/repository/contents/.pages
   ```

2. **Verify repository has `public/` folder**
   ```bash
   # Check if public folder exists
   curl https://git.example.com/api/v1/repos/username/repository/contents/public
   ```

3. **Check repository visibility**
   - If repository is private, ensure `forgejoToken` is configured
   - Verify the token has read access to the repository

4. **Check Traefik logs**
   ```bash
   docker logs traefik 2>&1 | grep -i error
   ```

5. **Verify middleware is loaded**
   - Check Traefik dashboard at port 8080
   - Look for `pages-server` middleware

## Custom Domain Not Working

### Symptoms
- "Custom domain not configured"
- "Custom domain not registered"

### Solutions

1. **Activate the custom domain first**

   Visit your pages URL to register the domain:
   ```
   https://username.pages.example.com/repository
   ```

2. **Verify `.pages` file has `custom_domain`**
   ```yaml
   enabled: true
   custom_domain: www.example.com
   ```

3. **Check DNS configuration**
   ```bash
   # Verify A record
   dig A www.example.com

   # Verify it points to Traefik IP
   dig +short www.example.com
   ```

4. **Wait for DNS propagation**
   - Can take up to 48 hours
   - Usually complete within 5-15 minutes

5. **Check Traefik Redis provider**
   ```bash
   # Verify router was created in Redis
   redis-cli KEYS "traefik/http/routers/custom-*"
   ```

6. **Verify `enableCustomDomains` is true**
   - Default is `true`, but check if explicitly set to `false`

## SSL Certificate Not Generating

### Symptoms
- Browser shows "Not Secure" or certificate error
- Let's Encrypt validation failing

### Solutions

1. **Verify HTTP router is separate from HTTPS**

   HTTP router should only use `web` entrypoint:
   ```yaml
   pages-http:
     entryPoints:
       - web  # NOT websecure
   ```

2. **Check ACME challenge passthrough**

   The plugin automatically handles `/.well-known/acme-challenge/*` paths.

   ```bash
   # Test ACME path
   curl -v http://www.example.com/.well-known/acme-challenge/test
   ```

3. **Check Let's Encrypt rate limits**
   - Max 50 certificates per domain per week
   - Max 5 duplicate certificates per week
   - Check https://letsencrypt.org/docs/rate-limits/

4. **Review Traefik ACME logs**
   ```bash
   docker logs traefik 2>&1 | grep -i acme
   ```

5. **Verify certificate resolver configuration**
   ```yaml
   certificatesResolvers:
     letsencrypt-http:
       acme:
         email: admin@example.com
         storage: acme.json
         httpChallenge:
           entryPoint: web
   ```

## Branch Subdomain Not Working

### Symptoms
- Branch subdomain returns 404
- "Custom domain not configured" on branch subdomain

### Solutions

1. **Verify branch exists in repository**
   ```bash
   git branch -r | grep stage
   ```

2. **Check branch is listed in `enable_branches`**
   ```yaml
   enable_branches:
     - stage
     - qa
   ```

3. **Verify `custom_domain` is configured**

   Branch subdomains require a custom domain:
   ```yaml
   custom_domain: example.com
   enable_branches:
     - stage
   ```

4. **Check DNS for branch subdomain**
   ```bash
   dig A stage.example.com
   ```

5. **Activate branch subdomains**

   Visit the pages URL to register:
   ```
   https://username.pages.example.com/repository
   ```

6. **Check plugin logs for warnings**
   ```bash
   docker logs traefik 2>&1 | grep -i "branch"
   ```

## Password Protection Not Working

### Symptoms
- Login page doesn't appear
- "Incorrect password" with correct password
- Cookie not persisting

### Solutions

1. **Verify password hash is correct**
   ```bash
   # Generate hash (note: use -n to avoid newline)
   echo -n "mypassword" | shasum -a 256
   ```

2. **Check `.pages` file has correct field**
   - Main branch: use `password`
   - Branch subdomains: use `branchesPassword`

3. **Verify site is served over HTTPS**
   - Cookies are `Secure` and require HTTPS
   - Check for mixed content issues

4. **Clear browser cookies and try again**

5. **Configure `authSecretKey` in Traefik**
   ```yaml
   authSecretKey: "your-secret-key"
   ```

6. **Check password cache TTL**
   - Password hashes are cached for 60 seconds
   - Wait 60 seconds after updating `.pages` file

## Redis Connection Issues

### Symptoms
- Slow response times
- Cache not working
- "Failed to connect to Redis" in logs

### Solutions

1. **Verify Redis is running**
   ```bash
   redis-cli ping
   # Should return: PONG
   ```

2. **Check Redis connectivity from Traefik**
   ```bash
   docker exec traefik nc -zv redis 6379
   ```

3. **Check Redis password**
   ```yaml
   redisPassword: "your-redis-password"
   ```

4. **Check connection pool settings**
   ```yaml
   redisPoolSize: 10
   redisMaxConnections: 20
   redisConnWaitTimeout: 5
   ```

5. **Monitor Redis connections**
   ```bash
   redis-cli INFO clients
   ```

## Performance Issues

### Symptoms
- Slow page loads
- High latency

### Solutions

1. **Enable Redis caching**
   ```yaml
   redisHost: "localhost"
   redisPort: 6379
   cacheTTL: 300
   ```

2. **Check cache hit rate**

   Look for `X-Cache-Status` header:
   - `HIT` - Served from cache
   - `MISS` - Fetched from Forgejo

3. **Increase cache TTL**
   ```yaml
   cacheTTL: 600  # 10 minutes
   ```

4. **Check Forgejo API response times**
   ```bash
   time curl https://git.example.com/api/v1/repos/user/repo/contents/public/index.html
   ```

5. **Monitor connection pool**

   If seeing "waiting for connection" messages, increase pool size:
   ```yaml
   redisPoolSize: 20
   redisMaxConnections: 40
   ```

## Common Error Messages

### "Repository not found or not configured for pages"

- Repository doesn't exist
- Repository is private and no token configured
- No `.pages` file in repository

### "Custom domain not registered - visit the pages URL to activate"

- Custom domain mapping not in cache
- Visit `https://username.pages.example.com/repository` first

### "Invalid request format"

- Malformed URL
- Missing username subdomain
- Check URL structure

### "Failed to get Redis connection"

- Redis not reachable
- Connection pool exhausted
- Check Redis configuration

### "DNS TXT record not found or incorrect"

- DNS verification enabled but TXT record missing
- TXT record has wrong hash
- DNS not propagated yet

## Getting Help

If you can't resolve an issue:

1. Check Traefik logs for detailed error messages
2. Enable debug logging if available
3. Open an issue at https://github.com/sqcows/pages-server/issues

When reporting issues, include:

- Traefik version
- Plugin version
- Relevant configuration (sanitized)
- Error messages from logs
- Steps to reproduce
