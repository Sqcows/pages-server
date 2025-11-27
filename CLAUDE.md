# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Agent Usage

**IMPORTANT**: Always use the `senior-golang-dev` agent for all development tasks in this project. This agent follows Go best practices, writes comprehensive tests, uses standard library over third-party packages, implements DevSecOps principles, and maintains complete documentation.

## Project Overview

**pages-server** is a Traefik plugin that provides static site hosting functionality for Forgejo (and Gitea) repositories, similar to GitHub Pages and GitLab Pages.

### Key Features

- Serves static files from `public/` folders in Forgejo/Gitea repositories
- Automatic HTTPS via Let's Encrypt with HTTP→HTTPS redirect
- Custom domain support with automatic SSL certificates
- Cloudflare DNS integration for custom domains
- URL format: `$git_username.$configured_domain/$repository`
- Profile sites at `$git_username.$configured_domain/` (from `.profile` directory)
- Only serves repositories with both `public/` folder and `.pages` file
- Custom error pages (e.g., 404) from configurable repository
- Optional Redis caching for static pages

### Important Constraints

- **Must run in Traefik's Yaegi interpreter** (embedded Go interpreter)
- **Prefer standard library** over third-party packages
- **Allowed dependency**: `github.com/go-acme/lego` for Let's Encrypt
- Only access public repositories on Forgejo
- Target performance: <5ms response time

## Configuration

The plugin is configured via Traefik's YAML configuration with the following parameters:

**Required:**
- Pages domain name
- Forgejo host URL
- Forgejo API key (if needed)
- Let's Encrypt endpoint
- Let's Encrypt email
- Cloudflare API key
- Cloudflare zone ID

**Custom Domain Configuration:**
Custom domains are specified in the `.pages` file within each repository.

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
- Dynamically creates Let's Encrypt certificates for custom domains
- Updates Cloudflare DNS records for custom domains

## Development

### Version
Current version: v0.0.1

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
