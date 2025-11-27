## Project Overview

**Project Name**: pages-server

**Description**: This project will be a plugin for treafik that acts as a HTTP(s) server for static files stored in a public folder in a git repository hosted on forgejo. It will be configured to create DNS entries in cloudflare given a specific domain configuration.

Websites will be served via HTTPS configured by lets encrypt. HTTP requests should be forced to HTTPS. URLS will be in the format of $git_username.$configured_domain/$repository The sites will only be serverd if that repository has a public/ folder AND a .pages file in the route. If a request is made to $git_username.$configured_domain/ this should point to the .profile directory.

Users can also add a custom domain name for the site which will be configured in the .profile file in the repository. Upon the plugin reading that file after it should set up a custom SSL certificate via lets encrypt for that domain in traefik also.

The plugin should also have a configurable repository used for serving custom error pages such as 404.

All settings should be done in the traefik config for the pluing.

**Purpose**: This plugin brings static web server functionality to forgejo and gitea similar to https://docs.github.com/en/pages and https://docs.gitlab.com/user/project/pages/.

## Requirements

### Core Functionality
- [x] Be a plugin for traefik
- [x] Connect to forgejo repositories to serve static web pages
- [x] Serve all files via SSL supported by lets encrypt
- [x] Support custom domain names and SSL for those by reading a .pages file

### API/Interface Requirements
- **Type**: Plugin for Traefik
- **Endpoints/Commands**: Integrate with traefiks web and websecure Entrypoints https://doc.traefik.io/traefik/reference/install-configuration/entrypoints/

### Configuration
- [x] Configuration file format: YAML
- [x] Required configuration parameters: pages domain name, forgejo host url, forgejo API key if needed, endpoint for lets encrypt, email for lets encrypt, cloudflare API key, cloudflare zoneID
- [x] Optional configuration parameters: 

### Security Requirements
- [x] Authentication method: none
- [x] Authorization requirements: none
- [x] Credential management: server credentials will be handled in the traefik config, individual custom domains will be in the .pages file loaded on a per repository basis
- [x] Data encryption: HTTP redirect HTTPS in traefik config
- [x] Other security considerations: Only allow access to public repositories on forgejo

### Data Requirements
- [x] Data storage: option for caching static pages in redis, but not mandatory
- [x] Data format: n/a
- [x] Data validation: check public/ folder exists in repository and presence of .pages file

### Testing Requirements
- [x] Unit tests for core functions
- [x] Integration tests
- [x] Desired test coverage: [e.g., >90%]

### Documentation Requirements
- [x] README.md with installation, configuration, and usage
- [x] CHANGELOG.md with semantic versioning
- [x] Code comments explaining complex logic
- [x] API documentation (if applicable)
- [x] Example usage

### Dependencies
- **Prefer standard library**: use standard lib as a preference
- **Allowed third-party packages**: https://github.com/go-acme/lego

### Performance Requirements
- [x] Response time requirements: <~5ms

### Deployment
- [x] Target environment: golang file ready to use with traefik proxy see these notes: https://plugins.traefik.io/create
- [x] Build requirements: code must run in https://github.com/traefik/yaegi as this is the embedded golang interpreator in traefik
- [x] Runtime requirements: latest golang version

### Version Control
- [x] Initial version number: v0.0.1
- [x] Git repository: https://code.squarecows.com/SquareCows/pages-server
- [x] Branching strategy: feature

## Questions or Clarifications Needed

[List any areas where you need the developer to ask clarifying questions, or note "Developer should ask questions as needed"]

