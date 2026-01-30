# Bovine Pages Server Wiki

Welcome to the Bovine Pages Server documentation wiki. This wiki provides comprehensive guides for setting up, configuring, and using the Bovine Pages Server Traefik plugin.

## What is Bovine Pages Server?

Bovine Pages Server is a Traefik middleware plugin that provides static site hosting for Forgejo and Gitea repositories, similar to GitHub Pages and GitLab Pages. It enables you to serve static websites directly from your Git repositories with automatic HTTPS, custom domains, and advanced features like branch subdomains and password protection.

## Key Features

- **Static Site Hosting** - Serve static files from `public/` folders in repositories
- **Automatic HTTPS** - Seamless Let's Encrypt integration via Traefik
- **Landing Page** - Customizable landing page for the base pages domain
- **Custom Domains** - Use your own domain names with automatic SSL
- **Branch Subdomains** - Serve different branches on subdomains (e.g., `stage.example.com`)
- **Password Protection** - Protect sites with SHA256 passwords and secure cookies
- **Branch Password Protection** - Separate passwords for staging/dev environments
- **URL Redirects** - 301 permanent redirects via `.redirects` file
- **Directory Listings** - Optional Apache-style directory listings
- **Redis Caching** - High-performance caching with Redis support
- **Automatic Router Registration** - Plugin auto-registers the base pagesDomain with Traefik on startup

## Quick Navigation

### Getting Started
- [[Getting Started]] - Installation and basic setup
- [[Configuration]] - All configuration options explained

### Features
- [[Custom Domains]] - Setting up custom domains
- [[Branch Subdomains]] - Using branch-specific subdomains
- [[Password Protection]] - Protecting sites with passwords
- [[URL Redirects]] - Setting up URL redirects
- [[Directory Listings]] - Enabling directory listings

### Reference
- [[Configuration Reference]] - Complete parameter reference
- [[.pages File Format]] - Repository configuration file
- [[Troubleshooting]] - Common issues and solutions

### Advanced
- [[Redis Caching]] - Setting up Redis for caching
- [[Traefik Integration]] - Advanced Traefik configuration
- [[Cache Management]] - Managing and clearing cache

## URL Structure

The plugin supports multiple URL patterns:

| Pattern | Example | Source |
|---------|---------|--------|
| **Landing Page** | `https://pages.example.com/` | `errorPagesRepo` → `public/index.html` |
| Repository Sites | `https://john.pages.example.com/blog/` | `john/blog/public/` |
| Profile Sites | `https://john.pages.example.com/` | `john/.profile/public/` |
| Custom Domains | `https://www.example.com/` | Repository with matching `custom_domain` |
| Branch Subdomains | `https://stage.example.com/` | Specific branch of repository |

## Repository Requirements

For a repository to be served, it must have:

1. A `public/` folder containing static files (HTML, CSS, JS, images, etc.)
2. A `.pages` file in the repository root

### Example `.pages` File

```yaml
enabled: true
custom_domain: example.com
enable_branches:
  - stage
  - qa
password: <sha256-hash>           # Protects main branch
branchesPassword: <sha256-hash>   # Protects branch subdomains
directory_index: true
```

## Support

- **Issues**: [GitHub Issues](https://github.com/sqcows/pages-server/issues)
- **Source Code**: [GitHub Repository](https://github.com/sqcows/pages-server)

## License

Bovine Pages Server is licensed under the GNU General Public License v3.0 (GPLv3).

Copyright (C) 2025 SquareCows
