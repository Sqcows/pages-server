# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Agent Usage

**IMPORTANT**: Always use the `senior-golang-dev` agent for all development tasks in this project. This agent follows Go best practices, writes comprehensive tests, uses standard library over third-party packages, implements DevSecOps principles, and maintains complete documentation.

## Project Overview

**pages-server** is a Traefik plugin that provides static site hosting functionality for Forgejo (and Gitea) repositories, similar to GitHub Pages and GitLab Pages.

### Key Features

- Serves static files from `public/` folders in Forgejo/Gitea repositories
- Automatic HTTPS via Traefik's certificatesResolvers with HTTP→HTTPS redirect
- Custom domain support with registration-based activation and manual DNS configuration (SSL certificates managed by Traefik)
- URL format: `$git_username.$configured_domain/$repository`
- Profile sites at `$git_username.$configured_domain/` (from `.profile` directory)
- Only serves repositories with both `public/` folder and `.pages` file
- Custom error pages (e.g., 404) from configurable repository
- Optional Redis caching for static pages

### Important Constraints

- **Must run in Traefik's Yaegi interpreter** (embedded Go interpreter)
- **Prefer standard library** over third-party packages
- **No external dependencies** - uses only Go standard library
- Only access public repositories on Forgejo (or private with API token)
- Target performance: <5ms response time
- **SSL certificates managed by Traefik** - plugin does not handle certificates

## Configuration

The plugin is configured via Traefik's YAML configuration with the following parameters:

**Required:**
- Pages domain name
- Forgejo host URL

**Optional:**
- Forgejo API token (for private repositories)
- Error pages repository
- Redis caching configuration

**Custom Domain Configuration:**
Custom domains are specified in the `.pages` file within each repository. Custom domains must be activated by visiting the pages URL (e.g., `https://username.pages.domain.com/repository`) to register the domain mapping in cache. Users must manually create DNS records (A or CNAME) pointing to the Traefik server IP address.

**SSL Certificate Configuration:**
SSL certificates are managed by Traefik's `certificatesResolvers` configuration, not by the plugin.

## URL Structure

- Standard repository: `https://$username.$domain/$repository/`
- User profile: `https://$username.$domain/` (serves from `.profile` directory)
- Custom domain: Configured in repository's `.pages` file

## Repository Requirements

For a repository to be served, it must have:
1. A `public/` folder containing static files
2. A `.pages` file in the repository root

## Architecture Notes

- Integrates with Traefik's `web` and `websecure` entrypoints
- See: https://doc.traefik.io/traefik/reference/install-configuration/entrypoints/
- Plugin creation guide: https://plugins.traefik.io/create
- Uses Forgejo API to access repository contents
- SSL certificates managed by Traefik's certificatesResolvers (not by plugin)
- Users manually configure DNS records for custom domains with their DNS provider

## License

This project is licensed under the GNU General Public License v3.0 (GPLv3). All source files must include the GPLv3 license header at the top of the file.

## Development

### Version
Current version: v0.0.5

### Repository
https://code.squarecows.com/SquareCows/pages-server

### Branching Strategy
Feature branches

### Testing Requirements
- Unit tests for core functions
- Integration tests
- Target coverage: >90%

### Documentation Requirements
- README.md with installation, configuration, and usage
- CHANGELOG.md with semantic versioning
- Code comments for complex logic
- API documentation
- Example usage
