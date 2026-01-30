// Copyright (C) 2025 SquareCows
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Package pages_server provides a Traefik middleware plugin for serving static sites
// from Forgejo/Gitea repositories with automatic HTTPS and custom domain support.
package pages_server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Config holds the plugin configuration.
type Config struct {
	// PagesDomain is the base domain for serving pages (e.g., pages.example.com)
	PagesDomain string `json:"pagesDomain,omitempty"`

	// ForgejoHost is the base URL of the Forgejo/Gitea instance (e.g., https://git.example.com)
	ForgejoHost string `json:"forgejoHost,omitempty"`

	// ForgejoToken is the API token for accessing Forgejo (optional for public repos)
	ForgejoToken string `json:"forgejoToken,omitempty"`

	// ErrorPagesRepo is the repository containing custom error pages (format: username/repository)
	ErrorPagesRepo string `json:"errorPagesRepo,omitempty"`

	// EnableCustomDomains enables custom domain support (default: true)
	EnableCustomDomains bool `json:"enableCustomDomains,omitempty"`

	// RedisHost is the Redis server host for caching (optional)
	RedisHost string `json:"redisHost,omitempty"`

	// RedisPort is the Redis server port (optional, default: 6379)
	RedisPort int `json:"redisPort,omitempty"`

	// RedisPassword is the Redis password (optional)
	RedisPassword string `json:"redisPassword,omitempty"`

	// CacheTTL is the cache time-to-live in seconds (default: 300)
	CacheTTL int `json:"cacheTTL,omitempty"`

	// CustomDomainCacheTTL is the cache TTL for custom domain lookups in seconds (default: 600)
	CustomDomainCacheTTL int `json:"customDomainCacheTTL,omitempty"`

	// TraefikRedisRouterEnabled enables automatic Traefik router registration for custom domains (default: true)
	TraefikRedisRouterEnabled bool `json:"traefikRedisRouterEnabled,omitempty"`

	// TraefikRedisCertResolver specifies which Traefik certificate resolver to use (default: "letsencrypt-http")
	TraefikRedisCertResolver string `json:"traefikRedisCertResolver,omitempty"`

	// TraefikRedisRouterTTL is the TTL for Traefik router configurations in seconds (default: same as CustomDomainCacheTTL)
	TraefikRedisRouterTTL int `json:"traefikRedisRouterTTL,omitempty"`

	// TraefikRedisRootKey is the Redis root key for Traefik configuration (default: "traefik")
	TraefikRedisRootKey string `json:"traefikRedisRootKey,omitempty"`

	// AuthCookieDuration is the duration in seconds for authentication cookies (default: 3600 = 1 hour)
	AuthCookieDuration int `json:"authCookieDuration,omitempty"`

	// AuthSecretKey is the secret key for signing authentication cookies (required for cookie security)
	AuthSecretKey string `json:"authSecretKey,omitempty"`

	// EnableCustomDomainDNSVerification enables DNS TXT record verification for custom domains (default: false)
	// When enabled, custom domains require a DNS TXT record with a hash of the repository name
	// to prevent malicious users from claiming domains they don't own
	EnableCustomDomainDNSVerification bool `json:"enableCustomDomainDNSVerification,omitempty"`

	// MaxRedirects is the maximum number of redirect rules to read from .redirects file (default: 25)
	// Limits resource exhaustion from extremely large redirect files
	MaxRedirects int `json:"maxRedirects,omitempty"`

	// RedisPoolSize is the size of the Redis connection pool (default: 10)
	// The pool maintains this many idle connections for reuse
	RedisPoolSize int `json:"redisPoolSize,omitempty"`

	// RedisMaxConnections is the maximum total number of Redis connections allowed (default: 20)
	// This includes both pooled and in-use connections. Requests will block when this limit is reached.
	RedisMaxConnections int `json:"redisMaxConnections,omitempty"`

	// RedisConnWaitTimeout is the timeout in seconds for waiting for an available connection (default: 5)
	// If a connection is not available within this time, the request falls back to in-memory cache
	RedisConnWaitTimeout int `json:"redisConnWaitTimeout,omitempty"`
}

// CreateConfig creates and initializes the plugin configuration.
func CreateConfig() *Config {
	return &Config{
		EnableCustomDomains:       true,
		RedisPort:                 6379,
		CacheTTL:                  300,
		CustomDomainCacheTTL:      600,
		TraefikRedisRouterEnabled: true,
		TraefikRedisCertResolver:  "letsencrypt-http",
		TraefikRedisRouterTTL:     0,  // Persistent - cleaned by reaper
		TraefikRedisRootKey:       "traefik",
		AuthCookieDuration:        3600, // 1 hour
		MaxRedirects:              25,   // Default max redirects
		RedisPoolSize:             10,   // Default pool size
		RedisMaxConnections:       20,   // Default max total connections
		RedisConnWaitTimeout:      5,    // Default wait timeout in seconds
	}
}

// PagesServer is the main plugin structure.
type PagesServer struct {
	next              http.Handler
	name              string
	config            *Config
	forgejoClient     *ForgejoClient
	cache             Cache
	customDomainCache Cache // Cache for custom domain -> (username/repo) mappings
	passwordCache     Cache // Cache for password hashes with 60-second TTL
	mu                sync.RWMutex
	errorPages        map[int][]byte
	landingPage       []byte // Landing page from error pages repository (public/index.html)
}

// customDomainMapping represents a mapping from a custom domain to a repository.
type customDomainMapping struct {
	Username   string
	Repository string
}

// New creates a new instance of the PagesServer plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	// Validate required configuration
	if config.PagesDomain == "" {
		return nil, fmt.Errorf("pagesDomain is required")
	}
	if config.ForgejoHost == "" {
		return nil, fmt.Errorf("forgejoHost is required")
	}

	// Initialize Forgejo client
	forgejoClient := NewForgejoClient(config.ForgejoHost, config.ForgejoToken)

	// Initialize cache (Redis or in-memory)
	var cache Cache
	if config.RedisHost != "" {
		cache = NewRedisCache(config.RedisHost, config.RedisPort, config.RedisPassword, config.CacheTTL,
			config.RedisPoolSize, config.RedisMaxConnections, config.RedisConnWaitTimeout)
	} else {
		cache = NewMemoryCache(config.CacheTTL)
	}

	// Initialize custom domain cache without TTL (persistent until cleaned by reaper)
	// Custom domain mappings don't expire - they persist until removed by external reaper script
	var customDomainCache Cache
	if config.RedisHost != "" {
		customDomainCache = NewRedisCache(config.RedisHost, config.RedisPort, config.RedisPassword, 0,
			config.RedisPoolSize, config.RedisMaxConnections, config.RedisConnWaitTimeout) // TTL=0 means no expiry
	} else {
		customDomainCache = NewMemoryCache(0) // In-memory also uses no expiry
	}

	// Initialize password cache with 60-second TTL
	// Password hashes are cached briefly to reduce .pages file reads
	var passwordCache Cache
	if config.RedisHost != "" {
		passwordCache = NewRedisCache(config.RedisHost, config.RedisPort, config.RedisPassword, 60,
			config.RedisPoolSize, config.RedisMaxConnections, config.RedisConnWaitTimeout) // 60-second TTL
	} else {
		passwordCache = NewMemoryCache(60) // 60-second TTL
	}

	ps := &PagesServer{
		next:              next,
		name:              name,
		config:            config,
		forgejoClient:     forgejoClient,
		cache:             cache,
		customDomainCache: customDomainCache,
		passwordCache:     passwordCache,
		errorPages:        make(map[int][]byte),
	}

	// Load error pages if configured
	if config.ErrorPagesRepo != "" {
		if err := ps.loadErrorPages(ctx); err != nil {
			// Log error but don't fail - use default error pages
			fmt.Printf("Warning: failed to load error pages: %v\n", err)
		}
	}

	// Register pagesDomain router in Traefik for SSL and HTTP→HTTPS redirect
	if err := ps.registerPagesDomainRouter(ctx); err != nil {
		fmt.Printf("Warning: failed to register pagesDomain router: %v\n", err)
	}

	return ps, nil
}

// ServeHTTP implements the http.Handler interface.
func (ps *PagesServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Pass through ACME HTTP-01 challenge requests to Traefik's certificate resolver.
	// Let's Encrypt ACME challenges MUST be served over HTTP and cannot be redirected to HTTPS.
	// These requests come from Let's Encrypt's validation servers to verify domain ownership.
	if strings.HasPrefix(req.URL.Path, "/.well-known/acme-challenge/") {
		ps.next.ServeHTTP(rw, req)
		return
	}

	// Handle /LOAD_REDIRECTS endpoint for custom domains
	// This endpoint reads .redirects file and creates Traefik middleware
	if req.URL.Path == "/LOAD_REDIRECTS" {
		ps.handleLoadRedirects(rw, req)
		return
	}

	// Enforce HTTPS redirect if not already HTTPS
	if req.Header.Get("X-Forwarded-Proto") == "http" {
		// Construct HTTPS URL using URL.Path to avoid double-prefixing issues
		httpsURL := "https://" + req.Host + req.URL.Path
		if req.URL.RawQuery != "" {
			httpsURL += "?" + req.URL.RawQuery
		}
		http.Redirect(rw, req, httpsURL, http.StatusMovedPermanently)
		return
	}

	// Determine if this is a custom domain request or a pagesDomain request
	host := req.Host
	if colonIdx := strings.Index(host, ":"); colonIdx != -1 {
		host = host[:colonIdx]
	}

	// Check if this is a request to the base pagesDomain (no subdomain)
	// e.g., https://pages.example.com/ (instead of https://username.pages.example.com/)
	if host == ps.config.PagesDomain {
		// Serve landing page if configured, otherwise show error
		ps.mu.RLock()
		landingPage := ps.landingPage
		ps.mu.RUnlock()

		if landingPage != nil {
			rw.Header().Set("Content-Type", "text/html; charset=utf-8")
			rw.Header().Set("Cache-Control", "public, max-age=300")
			rw.Header().Set("Server", "bovine")
			rw.WriteHeader(http.StatusOK)
			rw.Write(landingPage)
			return
		}

		// No landing page configured, serve error
		ps.serveError(rw, http.StatusBadRequest, "Invalid request format")
		return
	}

	var username, repository, filePath, branch string
	var err error

	// Check if this is a pagesDomain request
	if strings.HasSuffix(host, ps.config.PagesDomain) {
		// Parse as standard pagesDomain request
		username, repository, filePath, err = ps.parseRequest(req)
		if err != nil {
			ps.serveError(rw, http.StatusBadRequest, "Invalid request format")
			return
		}
		// Standard pages domain requests always use the default branch
		branch = ""
	} else if ps.config.EnableCustomDomains {
		// This might be a custom domain request (possibly a branch subdomain)
		username, repository, branch, err = ps.resolveCustomDomain(req.Context(), host)
		if err != nil {
			ps.serveError(rw, http.StatusNotFound, "Custom domain not configured")
			return
		}
		// Parse file path from URL (custom domains serve from repository root)
		filePath = ps.parseCustomDomainPath(req.URL.Path)
	} else {
		// Custom domains disabled and not a pagesDomain request
		ps.serveError(rw, http.StatusBadRequest, "Invalid domain")
		return
	}

	// Check password protection:
	// - Branch requests (branch != ""): use pagesConfig.BranchesPassword from .pages file
	// - Main branch requests (branch == ""): use pagesConfig.Password from .pages file
	if branch != "" {
		// This is a branch subdomain request - check per-repository BranchesPassword
		// Need to fetch PagesConfig to get branchesPassword
		pagesConfig, err := ps.forgejoClient.GetPagesConfig(req.Context(), username, repository)
		if err == nil && pagesConfig.BranchesPassword != "" {
			// Branch password protection is enabled for this repository
			if !ps.isBranchAuthenticated(req, username, repository) {
				// User is not authenticated for branch access
				if req.Method == "POST" {
					// Handle password submission
					if err := req.ParseForm(); err != nil {
						ps.serveBranchLoginPage(rw, req, "Invalid form data")
						return
					}

					submittedPassword := req.FormValue("password")
					submittedHash := hashPassword(submittedPassword)

					if submittedHash == pagesConfig.BranchesPassword {
						// Password correct - set branch auth cookie and redirect
						cookie := ps.createBranchAuthCookie(username, repository)
						http.SetCookie(rw, cookie)

						// Redirect to the same URL without POST data
						redirectURL := req.URL.String()
						http.Redirect(rw, req, redirectURL, http.StatusSeeOther)
						return
					} else {
						// Password incorrect
						ps.serveBranchLoginPage(rw, req, "Incorrect password")
						return
					}
				} else {
					// Show branch login page
					ps.serveBranchLoginPage(rw, req, "")
					return
				}
			}
			// User is authenticated for branch access, continue to serve content
		}
	} else {
		// This is a main branch request - check per-repository password from .pages file
		passwordHash, err := ps.getPasswordHash(req.Context(), username, repository)
		if err == nil && passwordHash != "" {
			// Repository is password protected
			if !ps.isAuthenticated(req, username, repository) {
				// User is not authenticated
				if req.Method == "POST" {
					// Handle password submission
					if err := req.ParseForm(); err != nil {
						ps.serveLoginPage(rw, req, username, repository, "Invalid form data")
						return
					}

					submittedPassword := req.FormValue("password")
					submittedHash := hashPassword(submittedPassword)

					if submittedHash == passwordHash {
						// Password correct - set auth cookie and redirect
						cookie := ps.createAuthCookie(username, repository)
						http.SetCookie(rw, cookie)

						// Redirect to the same URL without POST data
						redirectURL := req.URL.String()
						http.Redirect(rw, req, redirectURL, http.StatusSeeOther)
						return
					} else {
						// Password incorrect
						ps.serveLoginPage(rw, req, username, repository, "Incorrect password")
						return
					}
				} else {
					// Show login page
					ps.serveLoginPage(rw, req, username, repository, "")
					return
				}
			}
			// User is authenticated, continue to serve content
		}
	}

	// Check cache first (include branch in cache key for branch-specific content)
	cacheKey := fmt.Sprintf("%s:%s:%s:%s", username, repository, branch, filePath)
	if cached, found := ps.cache.Get(cacheKey); found {
		ps.serveContent(rw, filePath, cached, "HIT")
		return
	}

	// Verify repository has .pages file
	hasPages, err := ps.forgejoClient.HasPagesFile(req.Context(), username, repository)
	if err != nil || !hasPages {
		ps.serveError(rw, http.StatusNotFound, "Repository not found or not configured for pages")
		return
	}

	// Get the file content from Forgejo (use branch-aware method)
	content, contentType, err := ps.forgejoClient.GetFileContentFromBranch(req.Context(), username, repository, filePath, branch)
	if err != nil {
		// If file not found and path doesn't end with a file extension, try index.html
		if !hasFileExtension(filePath) {
			indexPath := filePath + "/index.html"
			indexContent, indexContentType, indexErr := ps.forgejoClient.GetFileContentFromBranch(req.Context(), username, repository, indexPath, branch)
			if indexErr == nil {
				// Found index.html in directory
				content = indexContent
				contentType = indexContentType
				filePath = indexPath // Update cache key to use index.html path
			} else {
				// Neither original file nor index.html exists
				// Check if directory_index is enabled for directory listing
				pagesConfig, configErr := ps.forgejoClient.GetPagesConfig(req.Context(), username, repository)
				if configErr == nil && pagesConfig.DirectoryIndex {
					// Try to list directory contents (branch-aware)
					entries, listErr := ps.forgejoClient.ListDirectoryFromBranch(req.Context(), username, repository, filePath, branch)
					if listErr == nil && len(entries) > 0 {
						// Serve directory listing
						ps.serveDirectoryListing(rw, req, username, repository, filePath, entries)
						return
					}
				}
				// Directory listing disabled or failed, return 404
				ps.serveError(rw, http.StatusNotFound, "File not found")
				return
			}
		} else {
			// File has extension and doesn't exist
			ps.serveError(rw, http.StatusNotFound, "File not found")
			return
		}
	}

	// Cache the content
	ps.cache.Set(cacheKey, content)

	// If this is a pagesDomain request, check for custom domain registration
	if strings.HasSuffix(host, ps.config.PagesDomain) {
		ps.registerCustomDomain(req.Context(), username, repository)
	}

	// Serve the content with cache MISS header
	rw.Header().Set("Content-Type", contentType)
	rw.Header().Set("Cache-Control", "public, max-age=300")
	rw.Header().Set("Server", "bovine")
	rw.Header().Set("X-Cache-Status", "MISS")
	rw.WriteHeader(http.StatusOK)
	rw.Write(content)
}

// parseRequest parses the incoming HTTP request to extract username, repository, and file path.
// URL format: $username.$domain/$repository/path/to/file
// Profile format: $username.$domain/ -> serves from .profile directory
func (ps *PagesServer) parseRequest(req *http.Request) (username, repository, filePath string, err error) {
	host := req.Host
	if colonIdx := strings.Index(host, ":"); colonIdx != -1 {
		host = host[:colonIdx]
	}

	// Extract username from subdomain
	pagesDomain := ps.config.PagesDomain
	if !strings.HasSuffix(host, pagesDomain) {
		return "", "", "", fmt.Errorf("invalid domain: %s", host)
	}

	// Remove the pages domain to get the username
	subdomain := strings.TrimSuffix(host, "."+pagesDomain)
	if subdomain == "" || subdomain == pagesDomain {
		return "", "", "", fmt.Errorf("no username subdomain found")
	}

	username = subdomain

	// Parse the path
	path := strings.TrimPrefix(req.URL.Path, "/")
	path = strings.TrimSuffix(path, "/") // Remove trailing slash for consistent parsing

	// Check if this is a profile site (no repository in path)
	if path == "" {
		// Profile site root: $username.$domain/
		// Use public (no trailing slash) to allow index.html or directory listing logic
		repository = ".profile"
		filePath = "public"
		return username, repository, filePath, nil
	}

	// Split path to check if it's profile or repository
	pathParts := strings.Split(path, "/")

	// If only one part and it looks like a file (has extension), it's a profile file
	// Otherwise it's a repository name
	if len(pathParts) == 1 {
		if strings.Contains(pathParts[0], ".") {
			// Profile site with file: $username.$domain/about.html
			repository = ".profile"
			filePath = "public/" + path
			return username, repository, filePath, nil
		}
		// Repository root: $username.$domain/myrepo
		// Use public (no trailing slash) so it can fall through to index.html or directory listing logic
		repository = pathParts[0]
		filePath = "public"
		return username, repository, filePath, nil
	}

	// Multiple parts means repository with path
	// Repository site: $username.$domain/$repository/path/to/file
	repository = pathParts[0]
	remainingPath := strings.Join(pathParts[1:], "/")
	if remainingPath == "" {
		remainingPath = "index.html"
	}
	filePath = "public/" + remainingPath

	return username, repository, filePath, nil
}

// resolveCustomDomain resolves a custom domain to a username, repository, and optional branch.
// It checks the cache only - custom domains must be registered by visiting the pages URL first.
// The branch is returned for branch subdomains (e.g., stage.example.com -> stage branch).
func (ps *PagesServer) resolveCustomDomain(ctx context.Context, domain string) (username, repository, branch string, err error) {
	// Check cache for registered custom domain
	cacheKey := "custom_domain:" + domain
	if cached, found := ps.customDomainCache.Get(cacheKey); found {
		// Parse cached value "username:repository" or "username:repository:branch"
		parts := strings.Split(string(cached), ":")
		if len(parts) == 2 {
			return parts[0], parts[1], "", nil
		}
		if len(parts) == 3 {
			return parts[0], parts[1], parts[2], nil
		}
	}

	// Custom domain not registered
	return "", "", "", fmt.Errorf("custom domain not registered - visit the pages URL to activate")
}

// registerCustomDomain registers a custom domain by reading the .pages file and caching the mapping.
// This is called when serving content from a pagesDomain request.
// It also registers branch subdomains if enable_branches is configured.
func (ps *PagesServer) registerCustomDomain(ctx context.Context, username, repository string) {
	// Get the .pages configuration
	pagesConfig, err := ps.forgejoClient.GetPagesConfig(ctx, username, repository)
	if err != nil {
		// No .pages file or error reading it, nothing to register
		return
	}

	// If custom domain is configured, register it
	if pagesConfig.CustomDomain != "" {
		// Perform DNS verification if enabled
		if ps.config.EnableCustomDomainDNSVerification {
			verified, err := ps.verifyCustomDomainDNS(pagesConfig.CustomDomain, username, repository)
			if err != nil {
				// DNS verification failed - log the error with details
				expectedHash := generateDomainVerificationHash(username, repository)
				fmt.Printf("ERROR: DNS verification failed for custom domain %s (repository: %s/%s): %v\n",
					pagesConfig.CustomDomain, username, repository, err)
				fmt.Printf("HINT: Add this DNS TXT record to %s:\n  bovine-pages-verification=%s\n",
					pagesConfig.CustomDomain, expectedHash)
				return
			}
			if !verified {
				// DNS verification returned false without error (shouldn't happen, but handle gracefully)
				expectedHash := generateDomainVerificationHash(username, repository)
				fmt.Printf("ERROR: DNS verification failed for custom domain %s (repository: %s/%s)\n",
					pagesConfig.CustomDomain, username, repository)
				fmt.Printf("HINT: Add this DNS TXT record to %s:\n  bovine-pages-verification=%s\n",
					pagesConfig.CustomDomain, expectedHash)
				return
			}
			// DNS verification succeeded - log success
			fmt.Printf("INFO: DNS verification successful for custom domain %s (repository: %s/%s)\n",
				pagesConfig.CustomDomain, username, repository)
		}

		// Check if custom domain is already registered to a different repository
		cacheKey := "custom_domain:" + pagesConfig.CustomDomain
		if existingMapping, found := ps.customDomainCache.Get(cacheKey); found {
			existingRepo := string(existingMapping)
			currentRepo := username + ":" + repository

			// If domain is already registered to a different repository, reject the registration
			if existingRepo != currentRepo {
				fmt.Printf("ERROR: Custom domain %s is already registered to %s, cannot register to %s\n",
					pagesConfig.CustomDomain, existingRepo, currentRepo)
				return
			}
			// Domain is already registered to this repository, continue to update mappings
		}

		// Forward mapping: custom_domain:domain -> username:repository
		cacheValue := username + ":" + repository
		ps.customDomainCache.Set(cacheKey, []byte(cacheValue))

		// Reverse mapping: username:repository -> custom_domain
		// This allows looking up the custom domain from the repository
		reverseCacheKey := username + ":" + repository
		ps.customDomainCache.Set(reverseCacheKey, []byte(pagesConfig.CustomDomain))

		// Register Traefik router for automatic SSL certificate generation
		if err := ps.registerTraefikRouter(ctx, pagesConfig.CustomDomain); err != nil {
			// Log error but don't fail the request
			// The custom domain will still work, just won't get automatic SSL until next visit
			fmt.Printf("Warning: failed to register Traefik router for %s: %v\n", pagesConfig.CustomDomain, err)
		}

		// Register branch subdomains if enable_branches is configured
		ps.registerBranchSubdomains(ctx, username, repository, pagesConfig.CustomDomain, pagesConfig.EnableBranches)
	} else if len(pagesConfig.EnableBranches) > 0 {
		// Warn if enable_branches is configured without custom_domain
		fmt.Printf("WARNING: enable_branches is configured for %s/%s but custom_domain is not set. Branch subdomains require a custom domain.\n",
			username, repository)
	}
}

// registerBranchSubdomains registers branch-specific subdomains for a custom domain.
// For example, with custom_domain "example.com" and enable_branches ["stage", "qa"],
// this registers stage.example.com and qa.example.com to serve from those branches.
func (ps *PagesServer) registerBranchSubdomains(ctx context.Context, username, repository, customDomain string, branches []string) {
	if len(branches) == 0 {
		return
	}

	for _, branch := range branches {
		// Sanitize branch name for subdomain (replace invalid characters)
		sanitizedBranch := sanitizeBranchForSubdomain(branch)
		if sanitizedBranch == "" {
			fmt.Printf("WARNING: Branch name '%s' cannot be sanitized for subdomain, skipping\n", branch)
			continue
		}

		// Construct the branch subdomain
		branchDomain := sanitizedBranch + "." + customDomain

		// Check if the branch exists in the repository
		_, err := ps.forgejoClient.GetBranch(ctx, username, repository, branch)
		if err != nil {
			fmt.Printf("WARNING: Branch '%s' not found in repository %s/%s, skipping subdomain registration: %v\n",
				branch, username, repository, err)
			continue
		}

		// Check if branch subdomain conflicts with an existing domain registration
		branchCacheKey := "custom_domain:" + branchDomain
		if existingMapping, found := ps.customDomainCache.Get(branchCacheKey); found {
			existingRepo := string(existingMapping)
			// Expected format for branch domains: "username:repository:branch"
			currentRepo := username + ":" + repository + ":" + branch

			if existingRepo != currentRepo {
				fmt.Printf("ERROR: Branch subdomain %s is already registered to %s, cannot register to %s\n",
					branchDomain, existingRepo, currentRepo)
				continue
			}
			// Branch subdomain is already registered to this repository/branch, continue to update
		}

		// Forward mapping: custom_domain:branchDomain -> username:repository:branch
		cacheValue := username + ":" + repository + ":" + branch
		ps.customDomainCache.Set(branchCacheKey, []byte(cacheValue))

		// Reverse mapping for branch: username:repository:branch:branchName -> branchDomain
		// This allows looking up the branch subdomain from the repository/branch
		reverseCacheKey := username + ":" + repository + ":branch:" + branch
		ps.customDomainCache.Set(reverseCacheKey, []byte(branchDomain))

		// Register Traefik router for the branch subdomain
		if err := ps.registerTraefikRouter(ctx, branchDomain); err != nil {
			fmt.Printf("Warning: failed to register Traefik router for branch subdomain %s: %v\n", branchDomain, err)
		}

		fmt.Printf("INFO: Registered branch subdomain %s -> %s/%s (branch: %s)\n",
			branchDomain, username, repository, branch)
	}
}

// sanitizeBranchForSubdomain converts a git branch name to a valid subdomain label.
// Rules:
// - Replace "/" with "-" (feature/new-ui -> feature-new-ui)
// - Replace "_" with "-" (my_branch -> my-branch)
// - Convert to lowercase
// - Remove leading/trailing hyphens
// - Collapse multiple consecutive hyphens
// - Max length 63 characters (DNS label limit)
// Returns empty string if the branch name cannot be sanitized to a valid subdomain.
func sanitizeBranchForSubdomain(branch string) string {
	if branch == "" {
		return ""
	}

	// Convert to lowercase
	result := strings.ToLower(branch)

	// Replace common invalid characters
	result = strings.ReplaceAll(result, "/", "-")
	result = strings.ReplaceAll(result, "_", "-")
	result = strings.ReplaceAll(result, ".", "-")

	// Remove any characters that aren't alphanumeric or hyphen
	var cleaned strings.Builder
	for _, r := range result {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			cleaned.WriteRune(r)
		}
	}
	result = cleaned.String()

	// Collapse multiple consecutive hyphens
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}

	// Remove leading and trailing hyphens
	result = strings.Trim(result, "-")

	// Enforce DNS label max length (63 characters)
	if len(result) > 63 {
		result = result[:63]
		// Remove trailing hyphen if truncation created one
		result = strings.TrimSuffix(result, "-")
	}

	// Validate: must start with alphanumeric
	if len(result) == 0 {
		return ""
	}
	if !((result[0] >= 'a' && result[0] <= 'z') || (result[0] >= '0' && result[0] <= '9')) {
		return ""
	}

	return result
}

// generateDomainVerificationHash creates a SHA256 hash for domain verification.
// The hash is computed from the full repository path (owner/repository).
// This hash must be present in a DNS TXT record at the custom domain to prove ownership.
func generateDomainVerificationHash(owner, repository string) string {
	repoPath := owner + "/" + repository
	hash := sha256.Sum256([]byte(repoPath))
	return hex.EncodeToString(hash[:])
}

// verifyCustomDomainDNS verifies that a DNS TXT record exists at the custom domain
// containing the correct verification hash for the repository.
// Returns true if verification succeeds, false otherwise.
// Errors are logged but don't cause the function to return an error - instead they return false.
func (ps *PagesServer) verifyCustomDomainDNS(domain, owner, repository string) (bool, error) {
	// Generate expected verification hash
	expectedHash := generateDomainVerificationHash(owner, repository)
	expectedValue := "bovine-pages-verification=" + expectedHash

	// Lookup TXT records for the domain
	txtRecords, err := net.LookupTXT(domain)
	if err != nil {
		// DNS lookup failed - could be timeout, NXDOMAIN, or other DNS error
		return false, fmt.Errorf("DNS TXT lookup failed for %s: %w", domain, err)
	}

	// Check if any TXT record matches our expected verification value
	for _, record := range txtRecords {
		// Use constant-time comparison to prevent timing attacks
		// Compare the full record value including the prefix
		if subtle.ConstantTimeCompare([]byte(record), []byte(expectedValue)) == 1 {
			return true, nil
		}

		// Also check if the record contains the hash with spaces or variations
		// Some DNS providers may add/remove spaces
		cleanRecord := strings.ReplaceAll(record, " ", "")
		cleanExpected := strings.ReplaceAll(expectedValue, " ", "")
		if subtle.ConstantTimeCompare([]byte(cleanRecord), []byte(cleanExpected)) == 1 {
			return true, nil
		}
	}

	// No matching TXT record found
	return false, fmt.Errorf("DNS TXT record not found or incorrect for %s (expected: %s)", domain, expectedValue)
}

// registerPagesDomainRouter writes Traefik router configuration to Redis for the base pagesDomain.
// This enables automatic SSL certificate generation and HTTP→HTTPS redirect for the pages domain itself.
func (ps *PagesServer) registerPagesDomainRouter(ctx context.Context) error {
	if !ps.config.TraefikRedisRouterEnabled {
		return nil
	}

	// Only write if we have a working Redis cache
	redisCache, ok := ps.customDomainCache.(*RedisCache)
	if !ok {
		// Using in-memory cache, skip Traefik router registration
		return nil
	}

	// Create router name for the base pages domain
	routerName := "pages-domain-base"
	rootKey := ps.config.TraefikRedisRootKey

	// Write router configuration keys to Redis
	// This registers the base pagesDomain (e.g., pages.example.com) for SSL
	configs := map[string]string{
		fmt.Sprintf("%s/http/routers/%s/rule", rootKey, routerName):             fmt.Sprintf("Host(`%s`)", ps.config.PagesDomain),
		fmt.Sprintf("%s/http/routers/%s/entryPoints/0", rootKey, routerName):    "web",
		fmt.Sprintf("%s/http/routers/%s/entryPoints/1", rootKey, routerName):    "websecure",
		fmt.Sprintf("%s/http/routers/%s/middlewares/0", rootKey, routerName):    "pages-server@file",
		fmt.Sprintf("%s/http/routers/%s/service", rootKey, routerName):          "noop@internal",
		fmt.Sprintf("%s/http/routers/%s/tls/certResolver", rootKey, routerName): ps.config.TraefikRedisCertResolver,
		fmt.Sprintf("%s/http/routers/%s/priority", rootKey, routerName):         "10",
	}

	// Write each config key with TTL
	for key, value := range configs {
		if err := redisCache.SetWithTTL(key, []byte(value), ps.config.TraefikRedisRouterTTL); err != nil {
			return fmt.Errorf("failed to write pagesDomain router config key %s: %w", key, err)
		}
	}

	return nil
}

// registerTraefikRouter writes Traefik router configuration to Redis for automatic SSL certificate generation.
// This enables Traefik's Redis provider to dynamically discover custom domains and request certificates.
func (ps *PagesServer) registerTraefikRouter(ctx context.Context, customDomain string) error {
	if !ps.config.TraefikRedisRouterEnabled {
		return nil
	}

	// Only write if we have a working Redis cache
	redisCache, ok := ps.customDomainCache.(*RedisCache)
	if !ok {
		// Using in-memory cache, skip Traefik router registration
		return nil
	}

	// Create sanitized router name (replace dots with dashes for valid router names)
	routerName := "custom-" + strings.ReplaceAll(customDomain, ".", "-")
	rootKey := ps.config.TraefikRedisRootKey

	// Write router configuration keys to Redis
	// Each configuration line becomes a separate Redis key
	// This format is expected by Traefik's Redis provider
	configs := map[string]string{
		fmt.Sprintf("%s/http/routers/%s/rule", rootKey, routerName):             fmt.Sprintf("Host(`%s`)", customDomain),
		fmt.Sprintf("%s/http/routers/%s/entryPoints/0", rootKey, routerName):    "web",
		fmt.Sprintf("%s/http/routers/%s/entryPoints/1", rootKey, routerName):    "websecure",
		fmt.Sprintf("%s/http/routers/%s/middlewares/0", rootKey, routerName):    "pages-server@file",
		fmt.Sprintf("%s/http/routers/%s/service", rootKey, routerName):          "noop@internal",
		fmt.Sprintf("%s/http/routers/%s/tls/certResolver", rootKey, routerName): ps.config.TraefikRedisCertResolver,
		fmt.Sprintf("%s/http/routers/%s/priority", rootKey, routerName):         "10",
	}

	// Write each config key with TTL
	for key, value := range configs {
		if err := redisCache.SetWithTTL(key, []byte(value), ps.config.TraefikRedisRouterTTL); err != nil {
			return fmt.Errorf("failed to write Traefik router config key %s: %w", key, err)
		}
	}

	return nil
}

// parseCustomDomainPath parses the URL path for a custom domain request.
// Custom domains serve directly from the public/ folder without the repository name in the path.
func (ps *PagesServer) parseCustomDomainPath(urlPath string) string {
	// Remove leading and trailing slashes
	path := strings.TrimPrefix(urlPath, "/")
	path = strings.TrimSuffix(path, "/")

	// If empty, serve index.html
	if path == "" {
		return "public/index.html"
	}

	// Add public/ prefix
	return "public/" + path
}

// serveContent serves file content with appropriate headers.
func (ps *PagesServer) serveContent(rw http.ResponseWriter, filePath string, content []byte, cacheStatus string) {
	contentType := detectContentType(filePath, content)
	rw.Header().Set("Content-Type", contentType)
	rw.Header().Set("Cache-Control", "public, max-age=300")
	rw.Header().Set("Server", "bovine")
	rw.Header().Set("X-Cache-Status", cacheStatus)
	rw.WriteHeader(http.StatusOK)
	rw.Write(content)
}

// serveDirectoryListing serves an HTML directory listing.
func (ps *PagesServer) serveDirectoryListing(rw http.ResponseWriter, req *http.Request, username, repository, dirPath string, entries []DirectoryEntry) {
	// Remove "public/" prefix from dirPath for display
	displayPath := strings.TrimPrefix(dirPath, "public/")
	if displayPath == "" {
		displayPath = "/"
	} else {
		displayPath = "/" + displayPath
	}

	// Build HTML
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Index of %s</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            background: #f5f5f5;
            padding: 20px;
        }
        .container {
            max-width: 1000px;
            margin: 0 auto;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        header {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            padding: 30px;
        }
        h1 {
            font-size: 24px;
            font-weight: 600;
            margin-bottom: 8px;
        }
        .breadcrumb {
            font-size: 14px;
            opacity: 0.9;
        }
        table {
            width: 100%%;
            border-collapse: collapse;
        }
        thead {
            background: #f8f9fa;
            border-bottom: 2px solid #dee2e6;
        }
        th {
            text-align: left;
            padding: 15px 20px;
            font-weight: 600;
            font-size: 13px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            color: #495057;
        }
        td {
            padding: 12px 20px;
            border-bottom: 1px solid #f0f0f0;
        }
        tr:hover {
            background: #f8f9fa;
        }
        a {
            color: #667eea;
            text-decoration: none;
            display: flex;
            align-items: center;
            gap: 8px;
        }
        a:hover {
            color: #764ba2;
            text-decoration: underline;
        }
        .icon {
            width: 20px;
            height: 20px;
            flex-shrink: 0;
        }
        .icon-folder { fill: #fbbf24; }
        .icon-file { fill: #60a5fa; }
        .icon-parent { fill: #9ca3af; }
        .size {
            color: #6c757d;
            font-size: 14px;
            text-align: right;
        }
        .type {
            color: #6c757d;
            font-size: 14px;
            text-align: center;
        }
        footer {
            padding: 20px;
            text-align: center;
            color: #6c757d;
            font-size: 13px;
            border-top: 1px solid #f0f0f0;
        }
        @media (max-width: 768px) {
            .size, .type { display: none; }
            th:nth-child(2), th:nth-child(3),
            td:nth-child(2), td:nth-child(3) { display: none; }
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>Index of %s</h1>
            <div class="breadcrumb">%s/%s</div>
        </header>
        <table>
            <thead>
                <tr>
                    <th>Name</th>
                    <th>Type</th>
                    <th>Size</th>
                </tr>
            </thead>
            <tbody>`, displayPath, displayPath, username, repository)

	// Add parent directory link if not at root
	if displayPath != "/" {
		parentPath := req.URL.Path
		if strings.HasSuffix(parentPath, "/") {
			parentPath = strings.TrimSuffix(parentPath, "/")
		}
		lastSlash := strings.LastIndex(parentPath, "/")
		if lastSlash > 0 {
			parentPath = parentPath[:lastSlash] + "/"
		} else {
			parentPath = "/"
		}
		html += fmt.Sprintf(`
                <tr>
                    <td>
                        <a href="%s">
                            <svg class="icon icon-parent" viewBox="0 0 20 20" fill="currentColor">
                                <path fill-rule="evenodd" d="M9.707 16.707a1 1 0 01-1.414 0l-6-6a1 1 0 010-1.414l6-6a1 1 0 011.414 1.414L5.414 9H17a1 1 0 110 2H5.414l4.293 4.293a1 1 0 010 1.414z" clip-rule="evenodd"/>
                            </svg>
                            <span>Parent Directory</span>
                        </a>
                    </td>
                    <td class="type">-</td>
                    <td class="size">-</td>
                </tr>`, parentPath)
	}

	// Add directory entries
	for _, entry := range entries {
		icon := "icon-file"
		typeName := "File"
		sizeStr := formatSize(entry.Size)
		href := entry.Name

		if entry.IsDir {
			icon = "icon-folder"
			typeName = "Directory"
			sizeStr = "-"
			href = entry.Name + "/"
		}

		html += fmt.Sprintf(`
                <tr>
                    <td>
                        <a href="%s">
                            <svg class="icon %s" viewBox="0 0 20 20" fill="currentColor">
                                %s
                            </svg>
                            <span>%s</span>
                        </a>
                    </td>
                    <td class="type">%s</td>
                    <td class="size">%s</td>
                </tr>`,
			href,
			icon,
			getSVGPath(entry.IsDir),
			entry.Name,
			typeName,
			sizeStr)
	}

	html += `
            </tbody>
        </table>
        <footer>
            Powered by Bovine Pages Server
        </footer>
    </div>
</body>
</html>`

	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.Header().Set("Cache-Control", "public, max-age=60")
	rw.Header().Set("Server", "bovine")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte(html))
}

// getSVGPath returns the SVG path for file or folder icon.
func getSVGPath(isDir bool) string {
	if isDir {
		return `<path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z"/>`
	}
	return `<path fill-rule="evenodd" d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z" clip-rule="evenodd"/>`
}

// formatSize formats a file size in bytes to human-readable format.
func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
}

// serveError serves an error page.
func (ps *PagesServer) serveError(rw http.ResponseWriter, statusCode int, message string) {
	ps.mu.RLock()
	errorPage, hasCustomError := ps.errorPages[statusCode]
	ps.mu.RUnlock()

	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.Header().Set("Server", "bovine")
	rw.WriteHeader(statusCode)

	if hasCustomError {
		rw.Write(errorPage)
	} else {
		// Default error page
		fmt.Fprintf(rw, `<!DOCTYPE html>
<html>
<head><title>Error %d</title></head>
<body>
<h1>Error %d</h1>
<p>%s</p>
</body>
</html>`, statusCode, statusCode, message)
	}
}

// loadErrorPages loads custom error pages from the configured repository.
func (ps *PagesServer) loadErrorPages(ctx context.Context) error {
	if ps.config.ErrorPagesRepo == "" {
		return nil
	}

	parts := strings.Split(ps.config.ErrorPagesRepo, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid error pages repository format: %s", ps.config.ErrorPagesRepo)
	}

	username, repository := parts[0], parts[1]

	// Load landing page (public/index.html) for base domain access
	landingPath := "public/index.html"
	if landingContent, _, err := ps.forgejoClient.GetFileContent(ctx, username, repository, landingPath); err == nil {
		ps.mu.Lock()
		ps.landingPage = landingContent
		ps.mu.Unlock()
	}

	// Common error pages to load
	errorCodes := []int{400, 403, 404, 500, 502, 503}

	for _, code := range errorCodes {
		filePath := fmt.Sprintf("public/%d.html", code)
		content, _, err := ps.forgejoClient.GetFileContent(ctx, username, repository, filePath)
		if err == nil {
			ps.mu.Lock()
			ps.errorPages[code] = content
			ps.mu.Unlock()
		}
	}

	return nil
}

// detectContentType determines the MIME type based on file extension and content.
func detectContentType(filePath string, content []byte) string {
	// Common file extensions
	ext := strings.ToLower(filePath)
	switch {
	case strings.HasSuffix(ext, ".html"), strings.HasSuffix(ext, ".htm"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(ext, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(ext, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(ext, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(ext, ".png"):
		return "image/png"
	case strings.HasSuffix(ext, ".jpg"), strings.HasSuffix(ext, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(ext, ".gif"):
		return "image/gif"
	case strings.HasSuffix(ext, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(ext, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(ext, ".woff"):
		return "font/woff"
	case strings.HasSuffix(ext, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(ext, ".ttf"):
		return "font/ttf"
	case strings.HasSuffix(ext, ".pdf"):
		return "application/pdf"
	case strings.HasSuffix(ext, ".xml"):
		return "application/xml; charset=utf-8"
	case strings.HasSuffix(ext, ".txt"):
		return "text/plain; charset=utf-8"
	default:
		// Use http.DetectContentType for unknown types
		return http.DetectContentType(content)
	}
}

// hasFileExtension checks if a file path has a file extension.
// Used to detect directory paths vs file paths for automatic index.html handling.
func hasFileExtension(path string) bool {
	// Get the last path segment
	segments := strings.Split(path, "/")
	if len(segments) == 0 {
		return false
	}
	lastSegment := segments[len(segments)-1]

	// Check if it contains a dot (indicating an extension)
	// But not if it starts with a dot (hidden file)
	if strings.Contains(lastSegment, ".") && !strings.HasPrefix(lastSegment, ".") {
		return true
	}

	return false
}

// getPasswordHash retrieves the password hash for a repository, checking cache first.
func (ps *PagesServer) getPasswordHash(ctx context.Context, username, repository string) (string, error) {
	// Check password cache first
	cacheKey := fmt.Sprintf("password:%s:%s", username, repository)
	if cached, found := ps.passwordCache.Get(cacheKey); found {
		return string(cached), nil
	}

	// Cache miss - fetch .pages config from Forgejo
	pagesConfig, err := ps.forgejoClient.GetPagesConfig(ctx, username, repository)
	if err != nil {
		return "", err
	}

	// Cache the password hash (even if empty) with 60-second TTL
	ps.passwordCache.Set(cacheKey, []byte(pagesConfig.Password))

	return pagesConfig.Password, nil
}

// isAuthenticated checks if the request has a valid authentication cookie.
func (ps *PagesServer) isAuthenticated(req *http.Request, username, repository string) bool {
	cookieName := fmt.Sprintf("pages_auth_%s_%s", username, repository)
	cookie, err := req.Cookie(cookieName)
	if err != nil {
		return false
	}

	// Verify cookie signature
	return ps.verifyAuthCookie(cookie.Value, username, repository)
}

// verifyAuthCookie verifies the authentication cookie signature.
func (ps *PagesServer) verifyAuthCookie(cookieValue, username, repository string) bool {
	if ps.config.AuthSecretKey == "" {
		// If no secret key configured, fall back to simple validation
		return cookieValue != ""
	}

	// Cookie format: <timestamp>|<signature>
	parts := strings.Split(cookieValue, "|")
	if len(parts) != 2 {
		return false
	}

	timestamp := parts[0]
	signature := parts[1]

	// Check if cookie has expired
	if !ps.isTimestampValid(timestamp) {
		return false
	}

	// Verify signature
	expectedSignature := ps.generateSignature(timestamp, username, repository)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// generateSignature creates an HMAC signature for the cookie.
func (ps *PagesServer) generateSignature(timestamp, username, repository string) string {
	message := fmt.Sprintf("%s:%s:%s", timestamp, username, repository)
	h := hmac.New(sha256.New, []byte(ps.config.AuthSecretKey))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// isTimestampValid checks if the timestamp is within the allowed duration.
func (ps *PagesServer) isTimestampValid(timestamp string) bool {
	// Parse timestamp as Unix seconds
	var ts int64
	if _, err := fmt.Sscanf(timestamp, "%d", &ts); err != nil {
		return false
	}

	cookieTime := time.Unix(ts, 0)
	expirationDuration := time.Duration(ps.config.AuthCookieDuration) * time.Second

	return time.Since(cookieTime) < expirationDuration
}

// createAuthCookie creates a signed authentication cookie.
func (ps *PagesServer) createAuthCookie(username, repository string) *http.Cookie {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	var cookieValue string
	if ps.config.AuthSecretKey != "" {
		signature := ps.generateSignature(timestamp, username, repository)
		cookieValue = fmt.Sprintf("%s|%s", timestamp, signature)
	} else {
		// Fallback without signature if no secret key
		cookieValue = timestamp
	}

	cookieName := fmt.Sprintf("pages_auth_%s_%s", username, repository)

	return &http.Cookie{
		Name:     cookieName,
		Value:    cookieValue,
		Path:     "/",
		MaxAge:   ps.config.AuthCookieDuration,
		HttpOnly: true,
		Secure:   true, // HTTPS only
		SameSite: http.SameSiteStrictMode,
	}
}

// hashPassword creates a SHA256 hash of the password.
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// isBranchAuthenticated checks if the request has a valid branch authentication cookie.
// Cookie is unique per repository to ensure branch passwords are repository-specific.
func (ps *PagesServer) isBranchAuthenticated(req *http.Request, username, repository string) bool {
	cookieName := fmt.Sprintf("pages_branch_auth_%s_%s", username, repository)
	cookie, err := req.Cookie(cookieName)
	if err != nil {
		return false
	}

	// Verify cookie signature
	return ps.verifyBranchAuthCookie(cookie.Value, username, repository)
}

// verifyBranchAuthCookie verifies the branch authentication cookie signature.
// Signature includes username and repository to ensure cookies are repository-specific.
func (ps *PagesServer) verifyBranchAuthCookie(cookieValue, username, repository string) bool {
	if ps.config.AuthSecretKey == "" {
		// If no secret key configured, fall back to simple validation
		return cookieValue != ""
	}

	// Cookie format: <timestamp>|<signature>
	parts := strings.Split(cookieValue, "|")
	if len(parts) != 2 {
		return false
	}

	timestamp := parts[0]
	signature := parts[1]

	// Check if cookie has expired
	if !ps.isTimestampValid(timestamp) {
		return false
	}

	// Verify signature
	expectedSignature := ps.generateBranchSignature(timestamp, username, repository)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// generateBranchSignature creates an HMAC signature for the branch auth cookie.
// Signature includes username and repository to ensure cookies are repository-specific.
func (ps *PagesServer) generateBranchSignature(timestamp, username, repository string) string {
	message := fmt.Sprintf("%s:%s:%s:branch", timestamp, username, repository)
	h := hmac.New(sha256.New, []byte(ps.config.AuthSecretKey))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// createBranchAuthCookie creates a signed authentication cookie for branch access.
// Cookie is unique per repository to ensure branch passwords are repository-specific.
func (ps *PagesServer) createBranchAuthCookie(username, repository string) *http.Cookie {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	var cookieValue string
	if ps.config.AuthSecretKey != "" {
		signature := ps.generateBranchSignature(timestamp, username, repository)
		cookieValue = fmt.Sprintf("%s|%s", timestamp, signature)
	} else {
		// Fallback without signature if no secret key
		cookieValue = timestamp
	}

	cookieName := fmt.Sprintf("pages_branch_auth_%s_%s", username, repository)

	return &http.Cookie{
		Name:     cookieName,
		Value:    cookieValue,
		Path:     "/",
		MaxAge:   ps.config.AuthCookieDuration,
		HttpOnly: true,
		Secure:   true, // HTTPS only
		SameSite: http.SameSiteStrictMode,
	}
}

// serveBranchLoginPage serves the password login page for branch access.
func (ps *PagesServer) serveBranchLoginPage(rw http.ResponseWriter, req *http.Request, errorMsg string) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.Header().Set("Server", "bovine")
	rw.WriteHeader(http.StatusOK)

	errorHTML := ""
	if errorMsg != "" {
		errorHTML = fmt.Sprintf(`<p class="error">%s</p>`, errorMsg)
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Branch Access Protected</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }
        .container {
            background: white;
            border-radius: 12px;
            box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
            padding: 40px;
            max-width: 400px;
            width: 100%%;
        }
        h1 {
            color: #333;
            font-size: 24px;
            margin-bottom: 10px;
            text-align: center;
        }
        p {
            color: #666;
            font-size: 14px;
            margin-bottom: 30px;
            text-align: center;
        }
        .error {
            color: #dc3545;
            background: #f8d7da;
            border: 1px solid #f5c6cb;
            padding: 12px;
            border-radius: 6px;
            margin-bottom: 20px;
        }
        form {
            display: flex;
            flex-direction: column;
            gap: 20px;
        }
        input[type="password"] {
            padding: 14px 16px;
            border: 2px solid #e1e4e8;
            border-radius: 6px;
            font-size: 16px;
            transition: border-color 0.2s;
        }
        input[type="password"]:focus {
            outline: none;
            border-color: #667eea;
        }
        button {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            padding: 14px;
            border: none;
            border-radius: 6px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        button:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
        }
        button:active {
            transform: translateY(0);
        }
        .branch-info {
            background: #f6f8fa;
            padding: 12px;
            border-radius: 6px;
            margin-bottom: 20px;
            text-align: center;
            color: #586069;
            font-size: 13px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔒 Branch Access Protected</h1>
        <p>This branch subdomain requires authentication</p>
        <div class="branch-info">
            <strong>Branch Subdomain Access</strong>
        </div>
        %s
        <form method="POST" action="">
            <input type="password" name="password" placeholder="Enter branch password" required autofocus>
            <button type="submit">Login</button>
        </form>
    </div>
</body>
</html>`, errorHTML)

	rw.Write([]byte(html))
}

// serveLoginPage serves the password login page.
func (ps *PagesServer) serveLoginPage(rw http.ResponseWriter, req *http.Request, username, repository string, errorMsg string) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.Header().Set("Server", "bovine")
	rw.WriteHeader(http.StatusOK)

	errorHTML := ""
	if errorMsg != "" {
		errorHTML = fmt.Sprintf(`<p class="error">%s</p>`, errorMsg)
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Password Protected</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }
        .container {
            background: white;
            border-radius: 12px;
            box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
            padding: 40px;
            max-width: 400px;
            width: 100%%;
        }
        h1 {
            color: #333;
            font-size: 24px;
            margin-bottom: 10px;
            text-align: center;
        }
        p {
            color: #666;
            font-size: 14px;
            margin-bottom: 30px;
            text-align: center;
        }
        .error {
            color: #dc3545;
            background: #f8d7da;
            border: 1px solid #f5c6cb;
            padding: 12px;
            border-radius: 6px;
            margin-bottom: 20px;
        }
        form {
            display: flex;
            flex-direction: column;
            gap: 20px;
        }
        input[type="password"] {
            padding: 14px 16px;
            border: 2px solid #e1e4e8;
            border-radius: 6px;
            font-size: 16px;
            transition: border-color 0.2s;
        }
        input[type="password"]:focus {
            outline: none;
            border-color: #667eea;
        }
        button {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            padding: 14px;
            border: none;
            border-radius: 6px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        button:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
        }
        button:active {
            transform: translateY(0);
        }
        .repo-info {
            background: #f6f8fa;
            padding: 12px;
            border-radius: 6px;
            margin-bottom: 20px;
            text-align: center;
            color: #586069;
            font-size: 13px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔒 Password Protected</h1>
        <p>This page requires authentication</p>
        <div class="repo-info">
            <strong>%s/%s</strong>
        </div>
        %s
        <form method="POST" action="">
            <input type="password" name="password" placeholder="Enter password" required autofocus>
            <button type="submit">Login</button>
        </form>
    </div>
</body>
</html>`, username, repository, errorHTML)

	rw.Write([]byte(html))
}

