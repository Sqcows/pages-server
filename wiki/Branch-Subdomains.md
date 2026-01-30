# Branch Subdomains

Branch subdomains allow you to serve content from specific Git branches as subdomains of your custom domain. This is useful for staging environments, QA testing, preview deployments, or any scenario where you need to preview changes before merging to the main branch.

## Overview

When you configure `enable_branches` in your repository's `.pages` file, each listed branch gets its own subdomain:

| Domain | Branch | Use Case |
|--------|--------|----------|
| `example.com` | `main` (default) | Production site |
| `stage.example.com` | `stage` | Staging environment |
| `qa.example.com` | `qa` | QA testing |
| `dev.example.com` | `dev` | Development preview |

## How It Works

1. **Main Domain**: Your `custom_domain` serves content from the default branch (usually `main` or `master`)
2. **Branch Subdomains**: Each branch in `enable_branches` gets a subdomain prefix
3. **Automatic Registration**: When you visit your pages URL, all branch subdomains are registered
4. **Automatic SSL**: Traefik automatically requests SSL certificates for each subdomain

## Setting Up Branch Subdomains

### Step 1: Configure Your `.pages` File

Add the `enable_branches` field to your repository's `.pages` file:

```yaml
enabled: true
custom_domain: example.com
enable_branches:
  - stage
  - qa
  - dev
```

Or using inline array format:

```yaml
enabled: true
custom_domain: example.com
enable_branches: ["stage", "qa", "dev"]
```

### Step 2: Configure DNS

You need DNS records for each branch subdomain. Choose one of these approaches:

#### Option A: Wildcard DNS (Recommended)

Create a wildcard record that covers all subdomains:

```
*.example.com.     A     YOUR_TRAEFIK_IP
example.com.       A     YOUR_TRAEFIK_IP
```

This approach is simpler and automatically handles any new branches you add.

#### Option B: Individual DNS Records

Create separate records for each subdomain:

```
example.com.         A     YOUR_TRAEFIK_IP
stage.example.com.   A     YOUR_TRAEFIK_IP
qa.example.com.      A     YOUR_TRAEFIK_IP
dev.example.com.     A     YOUR_TRAEFIK_IP
```

### Step 3: Create the Branches

Ensure each branch listed in `enable_branches` exists in your repository:

```bash
# Create and push the stage branch
git checkout -b stage
git push origin stage

# Create and push the qa branch
git checkout -b qa
git push origin qa

# Create and push the dev branch
git checkout -b dev
git push origin dev
```

### Step 4: Activate Branch Subdomains

Visit your main pages URL to register all domains:

```
https://username.pages.example.com/repository
```

The plugin will:
1. Register the main custom domain (`example.com`)
2. Verify each branch exists in the repository
3. Register each branch subdomain (`stage.example.com`, `qa.example.com`, `dev.example.com`)
4. Create Traefik routers for automatic SSL certificate generation

You'll see log messages confirming each registration:

```
INFO: Registered branch subdomain stage.example.com -> username/repository (branch: stage)
INFO: Registered branch subdomain qa.example.com -> username/repository (branch: qa)
INFO: Registered branch subdomain dev.example.com -> username/repository (branch: dev)
```

## Branch Name Sanitization

Git branch names are automatically converted to valid DNS subdomain labels:

| Git Branch | Subdomain | Rule Applied |
|------------|-----------|--------------|
| `stage` | `stage` | No change |
| `feature/new-ui` | `feature-new-ui` | `/` → `-` |
| `release_v1.0` | `release-v1-0` | `_` and `.` → `-` |
| `Feature/NEW_UI` | `feature-new-ui` | Lowercase + sanitize |
| `hotfix-123` | `hotfix-123` | No change |
| `feature//double` | `feature-double` | Collapse multiple `-` |

### Sanitization Rules

1. Slashes (`/`) are replaced with hyphens (`-`)
2. Underscores (`_`) are replaced with hyphens (`-`)
3. Dots (`.`) are replaced with hyphens (`-`)
4. Everything is converted to lowercase
5. Multiple consecutive hyphens are collapsed to a single hyphen
6. Leading and trailing hyphens are removed
7. Maximum 63 characters (DNS label limit)

## Branch Password Protection

You can protect branch subdomains with a separate password from your main site. This is useful when:

- Production site is public, but staging/dev should be private
- Different teams need different access levels
- You want to preview changes without public exposure

### Setting Up Branch Password Protection

#### Step 1: Generate a Password Hash

```bash
# Generate SHA256 hash of your branch password
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
  - dev
branchesPassword: 5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8
```

#### Step 3: Commit and Push

```bash
git add .pages
git commit -m "Add branch password protection"
git push
```

### How Branch Password Protection Works

| Request | Password Check | Cookie |
|---------|----------------|--------|
| `example.com` (main branch) | `password` field | `pages_auth_{username}_{repository}` |
| `stage.example.com` (branch) | `branchesPassword` field | `pages_branch_auth_{username}_{repository}` |
| `qa.example.com` (branch) | `branchesPassword` field | `pages_branch_auth_{username}_{repository}` |

**Key Points:**
- `branchesPassword` ONLY affects branch subdomains
- Main domain uses the separate `password` field
- Branch auth cookies are unique per repository
- One authentication covers all branches for that repository

### Example Configurations

#### Public Production, Private Staging

```yaml
enabled: true
custom_domain: example.com
enable_branches:
  - stage
  - dev
branchesPassword: <staging-password-hash>
# No password field - production is public
```

Result:
- `example.com` - Public (no password)
- `stage.example.com` - Password protected
- `dev.example.com` - Password protected

#### Private Production, Different Staging Password

```yaml
enabled: true
custom_domain: example.com
enable_branches:
  - stage
password: <production-password-hash>
branchesPassword: <staging-password-hash>
```

Result:
- `example.com` - Protected by production password
- `stage.example.com` - Protected by staging password
- Different passwords for different environments

#### All Public

```yaml
enabled: true
custom_domain: example.com
enable_branches:
  - stage
  - qa
# No password or branchesPassword - everything is public
```

## Cache Keys

Branch content is cached separately from default branch content. The cache key format includes the branch:

```
username:repository:branch:filepath
```

This ensures that:
- Content from different branches is cached independently
- Updating one branch doesn't affect cached content from other branches
- Each branch has its own cache lifetime

## Edge Cases and Warnings

| Scenario | Behavior |
|----------|----------|
| Branch doesn't exist | Warning logged, subdomain not registered |
| Branch subdomain conflicts with existing domain | Error logged, subdomain not registered |
| Empty `enable_branches` array | No additional subdomains created |
| `enable_branches` without `custom_domain` | Warning logged, branches ignored |
| Invalid branch name for subdomain | Warning logged, branch skipped |

### Common Warning Messages

```
WARNING: Branch name 'feature//test' cannot be sanitized for subdomain, skipping
WARNING: Branch 'nonexistent' not found in repository user/repo, skipping subdomain registration
WARNING: enable_branches is configured for user/repo but custom_domain is not set. Branch subdomains require a custom domain.
ERROR: Branch subdomain stage.example.com is already registered to different-user:different-repo, cannot register to user:repo
```

## Security Considerations

1. **Use Different Passwords**: Don't reuse production passwords for staging environments
2. **Strong Passwords**: Use long, random passwords for branch access
3. **HTTPS Required**: Branch passwords require HTTPS (cookies are Secure-only)
4. **Configure authSecretKey**: Enable HMAC cookie signing for added security
5. **Per-Repository Isolation**: Branch auth cookies are scoped to each repository

## Removing Branch Subdomains

To remove branch subdomains:

1. Remove branches from `enable_branches` in your `.pages` file
2. Commit and push the change
3. Visit your pages URL to update the registration
4. The removed subdomains will no longer be served

To remove branch password protection:

```yaml
enabled: true
custom_domain: example.com
enable_branches:
  - stage
# branchesPassword: removed - branches are now public
```

## Troubleshooting

### Branch subdomain not working

1. Verify the branch exists in your repository
2. Check that the branch is listed in `enable_branches`
3. Ensure you have a `custom_domain` configured
4. Visit your pages URL to trigger registration
5. Check DNS records are configured correctly
6. Wait for DNS propagation (up to 48 hours)

### Branch password not working

1. Verify `branchesPassword` is set in your `.pages` file
2. Ensure you're using the SHA256 hash, not plaintext
3. Clear browser cookies and try again
4. Check that `authSecretKey` is configured in Traefik middleware
5. Verify your site is served over HTTPS

### SSL certificate not generating for branch subdomain

1. Ensure DNS is configured for the subdomain
2. Check that Traefik's Redis provider is working
3. Visit the pages URL to trigger router registration
4. Check Traefik logs for certificate generation errors
5. Verify Let's Encrypt rate limits haven't been exceeded
