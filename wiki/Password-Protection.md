# Password Protection

Bovine Pages Server provides two types of password protection to secure your static sites:

1. **Repository Password** - Protects the main branch (production site)
2. **Branches Password** - Protects branch subdomains (staging/dev environments)

Both use SHA256 password hashing and secure HMAC-signed cookies for authentication.

## Overview

| Feature | Repository Password | Branches Password |
|---------|---------------------|-------------------|
| **Config Field** | `password` | `branchesPassword` |
| **Protects** | Main branch only | All branch subdomains |
| **Cookie Name** | `pages_auth_{user}_{repo}` | `pages_branch_auth_{user}_{repo}` |
| **Use Case** | Protect production site | Protect staging/dev |

## Repository Password Protection

Protect your main production site with a password.

### Setting Up Repository Password

#### Step 1: Generate a Password Hash

```bash
# Generate SHA256 hash of your password
echo -n "mypassword" | shasum -a 256

# Example output:
# 89e01536ac207279409d4de1e5253e01f4a1769e696db0d6062ca9b8f56767c8
```

#### Step 2: Add to Your `.pages` File

```yaml
enabled: true
password: 89e01536ac207279409d4de1e5253e01f4a1769e696db0d6062ca9b8f56767c8
```

#### Step 3: Configure Traefik (Optional but Recommended)

Add authentication settings to your Traefik middleware configuration:

```yaml
http:
  middlewares:
    pages-server:
      plugin:
        pages-server:
          pagesDomain: pages.example.com
          forgejoHost: https://git.example.com
          authCookieDuration: 3600  # Cookie validity in seconds (default: 1 hour)
          authSecretKey: "your-random-secret-key-here"  # For HMAC cookie signing
```

#### Step 4: Commit and Push

```bash
git add .pages
git commit -m "Add password protection"
git push
```

### How It Works

1. User visits your site (e.g., `https://username.pages.example.com/myrepo/`)
2. Plugin checks for `password` field in the repository's `.pages` file
3. If password is set and user has no valid cookie, login page is displayed
4. User enters password, which is hashed and compared to stored hash
5. If correct, a signed cookie is set and user is redirected to the site
6. Cookie remains valid for configured duration (default: 1 hour)

## Branches Password Protection

Protect branch subdomains (staging, QA, development) with a separate password from your main site.

### Setting Up Branches Password

#### Step 1: Generate a Password Hash

```bash
# Generate SHA256 hash of your staging password
echo -n "staging-password-123" | shasum -a 256

# Example output:
# 5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8
```

#### Step 2: Add to Your `.pages` File

```yaml
enabled: true
custom_domain: example.com
enable_branches:
  - stage
  - qa
branchesPassword: 5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8
```

#### Step 3: Commit and Push

```bash
git add .pages
git commit -m "Add branch password protection"
git push
```

### How It Works

1. User visits a branch subdomain (e.g., `https://stage.example.com/`)
2. Plugin detects this is a branch request and checks `branchesPassword`
3. If set and user has no valid branch cookie, login page is displayed
4. User enters password, which is hashed and compared
5. If correct, a branch auth cookie is set
6. One branch login covers all branches for that repository

## Example Configurations

### Public Production, Private Staging

The most common use case - production is public, but staging/dev need passwords:

```yaml
enabled: true
custom_domain: example.com
enable_branches:
  - stage
  - dev
branchesPassword: 5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8
# No password field - production is public
```

| URL | Access |
|-----|--------|
| `example.com` | Public |
| `stage.example.com` | Password required |
| `dev.example.com` | Password required |

### Private Production, Different Staging Password

Different teams get different passwords:

```yaml
enabled: true
custom_domain: example.com
enable_branches:
  - stage
password: <production-team-hash>
branchesPassword: <dev-team-hash>
```

| URL | Access |
|-----|--------|
| `example.com` | Production password |
| `stage.example.com` | Dev team password |

### Everything Private, Same Password

Use the same password for everything:

```yaml
enabled: true
custom_domain: example.com
enable_branches:
  - stage
password: <shared-hash>
branchesPassword: <shared-hash>
```

### Everything Public

No password protection:

```yaml
enabled: true
custom_domain: example.com
enable_branches:
  - stage
# No password or branchesPassword
```

## Security Features

### Password Hashing

- Passwords are stored as SHA256 hashes, never in plaintext
- The hash is 64 hexadecimal characters
- Even if the `.pages` file is exposed, the actual password remains secret

### HMAC-Signed Cookies

When `authSecretKey` is configured in Traefik:

- Cookies include an HMAC signature
- Prevents cookie tampering and forgery
- Signature includes timestamp, username, and repository

Cookie format: `<timestamp>|<hmac_signature>`

### Secure Cookie Attributes

All authentication cookies have:

| Attribute | Value | Purpose |
|-----------|-------|---------|
| `HttpOnly` | `true` | Prevents JavaScript access (XSS protection) |
| `Secure` | `true` | Only sent over HTTPS |
| `SameSite` | `Strict` | Prevents CSRF attacks |
| `Path` | `/` | Applies to all paths |
| `MaxAge` | Configurable | Cookie expiration |

### Per-Repository Isolation

- Each repository has its own cookie
- Authenticating for one repository doesn't grant access to others
- Branch cookies are also repository-specific

## Generating Strong Passwords

### Using OpenSSL (Linux/Mac)

```bash
# Generate a random password
openssl rand -base64 32
# Output: Kj8+dLmN3xRt7Yq2aP5vB1cF9hW4sE6g0iO3uA8wZ=

# Hash it for .pages file
echo -n "Kj8+dLmN3xRt7Yq2aP5vB1cF9hW4sE6g0iO3uA8wZ=" | shasum -a 256
```

### Using Python

```python
import hashlib
import secrets

# Generate random password
password = secrets.token_urlsafe(32)
print(f"Password: {password}")

# Generate hash for .pages file
hash_hex = hashlib.sha256(password.encode()).hexdigest()
print(f"Hash: {hash_hex}")
```

### Using Node.js

```javascript
const crypto = require('crypto');

// Generate random password
const password = crypto.randomBytes(32).toString('base64');
console.log('Password:', password);

// Generate hash for .pages file
const hash = crypto.createHash('sha256').update(password).digest('hex');
console.log('Hash:', hash);
```

## Configuration Reference

### Traefik Middleware Options

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `authCookieDuration` | int | 3600 | Cookie validity in seconds |
| `authSecretKey` | string | "" | Secret key for HMAC signing |

### .pages File Options

| Field | Type | Description |
|-------|------|-------------|
| `password` | string | SHA256 hash for main branch protection |
| `branchesPassword` | string | SHA256 hash for branch subdomain protection |

## Cookie Lifespan Recommendations

| Use Case | Duration | Seconds |
|----------|----------|---------|
| Public preview sites | 1-4 hours | 3600-14400 |
| Private team sites | 8-24 hours | 28800-86400 |
| Internal company sites | 7 days | 604800 |
| High-security sites | 15-30 minutes | 900-1800 |

## Login Page

The plugin provides a styled login page with:

- Gradient purple background
- Centered card design
- Repository/branch information display
- Error messages for incorrect passwords
- Auto-focus on password field
- Responsive mobile design

## Removing Password Protection

### Remove Repository Password

```yaml
enabled: true
# password: removed
```

### Remove Branches Password

```yaml
enabled: true
custom_domain: example.com
enable_branches:
  - stage
# branchesPassword: removed
```

After removing, commit and push the change. The password requirement is removed immediately (password cache has 60-second TTL).

## Troubleshooting

### Login page doesn't appear

1. Verify the `password` or `branchesPassword` field is in your `.pages` file
2. Check that the hash is valid (64 hex characters)
3. Wait 60 seconds for password cache to update
4. Clear browser cache and try again

### "Incorrect password" with correct password

1. Verify you're using the SHA256 **hash** in `.pages`, not plaintext
2. Re-generate the hash: `echo -n "password" | shasum -a 256`
3. Ensure no trailing newline: use `echo -n` (no newline)
4. Check for copy/paste errors in the hash

### Cookie not persisting

1. Ensure your site is served over HTTPS
2. Check browser cookie settings
3. Verify `authCookieDuration` is set correctly
4. Check for browser extensions blocking cookies

### Cookie rejected / authentication failing

1. Configure `authSecretKey` in Traefik middleware
2. Ensure secret key is consistent across Traefik instances
3. Check server clock synchronization (cookie timestamps)

### Branch password not working

1. Verify you're accessing a branch subdomain, not the main domain
2. Check that `branchesPassword` is set (not `password`)
3. Ensure the branch is listed in `enable_branches`
4. Clear cookies and try again
