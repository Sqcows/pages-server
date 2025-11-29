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
	"fmt"
	"net/http"
	"strings"
	"sync"
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
		TraefikRedisRouterTTL:     600,
		TraefikRedisRootKey:       "traefik",
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
	mu                sync.RWMutex
	errorPages        map[int][]byte
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
		cache = NewRedisCache(config.RedisHost, config.RedisPort, config.RedisPassword, config.CacheTTL)
	} else {
		cache = NewMemoryCache(config.CacheTTL)
	}

	// Initialize custom domain cache with separate TTL
	var customDomainCache Cache
	if config.RedisHost != "" {
		customDomainCache = NewRedisCache(config.RedisHost, config.RedisPort, config.RedisPassword, config.CustomDomainCacheTTL)
	} else {
		customDomainCache = NewMemoryCache(config.CustomDomainCacheTTL)
	}

	ps := &PagesServer{
		next:              next,
		name:              name,
		config:            config,
		forgejoClient:     forgejoClient,
		cache:             cache,
		customDomainCache: customDomainCache,
		errorPages:        make(map[int][]byte),
	}

	// Load error pages if configured
	if config.ErrorPagesRepo != "" {
		if err := ps.loadErrorPages(ctx); err != nil {
			// Log error but don't fail - use default error pages
			fmt.Printf("Warning: failed to load error pages: %v\n", err)
		}
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

	var username, repository, filePath string
	var err error

	// Check if this is a pagesDomain request
	if strings.HasSuffix(host, ps.config.PagesDomain) {
		// Parse as standard pagesDomain request
		username, repository, filePath, err = ps.parseRequest(req)
		if err != nil {
			ps.serveError(rw, http.StatusBadRequest, "Invalid request format")
			return
		}
	} else if ps.config.EnableCustomDomains {
		// This might be a custom domain request
		username, repository, err = ps.resolveCustomDomain(req.Context(), host)
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

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s:%s", username, repository, filePath)
	if cached, found := ps.cache.Get(cacheKey); found {
		ps.serveContent(rw, filePath, cached)
		return
	}

	// Verify repository has .pages file
	hasPages, err := ps.forgejoClient.HasPagesFile(req.Context(), username, repository)
	if err != nil || !hasPages {
		ps.serveError(rw, http.StatusNotFound, "Repository not found or not configured for pages")
		return
	}

	// Get the file content from Forgejo
	content, contentType, err := ps.forgejoClient.GetFileContent(req.Context(), username, repository, filePath)
	if err != nil {
		ps.serveError(rw, http.StatusNotFound, "File not found")
		return
	}

	// Cache the content
	ps.cache.Set(cacheKey, content)

	// If this is a pagesDomain request, check for custom domain registration
	if strings.HasSuffix(host, ps.config.PagesDomain) {
		ps.registerCustomDomain(req.Context(), username, repository)
	}

	// Serve the content
	rw.Header().Set("Content-Type", contentType)
	rw.Header().Set("Cache-Control", "public, max-age=300")
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
		repository = ".profile"
		filePath = "public/index.html"
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
		repository = pathParts[0]
		filePath = "public/index.html"
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

// resolveCustomDomain resolves a custom domain to a username and repository.
// It checks the cache only - custom domains must be registered by visiting the pages URL first.
func (ps *PagesServer) resolveCustomDomain(ctx context.Context, domain string) (username, repository string, err error) {
	// Check cache for registered custom domain
	cacheKey := "custom_domain:" + domain
	if cached, found := ps.customDomainCache.Get(cacheKey); found {
		// Parse cached value "username:repository"
		parts := strings.Split(string(cached), ":")
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
	}

	// Custom domain not registered
	return "", "", fmt.Errorf("custom domain not registered - visit the pages URL to activate")
}

// registerCustomDomain registers a custom domain by reading the .pages file and caching the mapping.
// This is called when serving content from a pagesDomain request.
func (ps *PagesServer) registerCustomDomain(ctx context.Context, username, repository string) {
	// Get the .pages configuration
	pagesConfig, err := ps.forgejoClient.GetPagesConfig(ctx, username, repository)
	if err != nil {
		// No .pages file or error reading it, nothing to register
		return
	}

	// If custom domain is configured, register it
	if pagesConfig.CustomDomain != "" {
		cacheKey := "custom_domain:" + pagesConfig.CustomDomain
		cacheValue := username + ":" + repository
		ps.customDomainCache.Set(cacheKey, []byte(cacheValue))

		// Register Traefik router for automatic SSL certificate generation
		if err := ps.registerTraefikRouter(ctx, pagesConfig.CustomDomain); err != nil {
			// Log error but don't fail the request
			// The custom domain will still work, just won't get automatic SSL until next visit
			fmt.Printf("Warning: failed to register Traefik router for %s: %v\n", pagesConfig.CustomDomain, err)
		}
	}
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
		fmt.Sprintf("%s/http/routers/%s/entryPoints/0", rootKey, routerName):    "websecure",
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
func (ps *PagesServer) serveContent(rw http.ResponseWriter, filePath string, content []byte) {
	contentType := detectContentType(filePath, content)
	rw.Header().Set("Content-Type", contentType)
	rw.Header().Set("Cache-Control", "public, max-age=300")
	rw.WriteHeader(http.StatusOK)
	rw.Write(content)
}

// serveError serves an error page.
func (ps *PagesServer) serveError(rw http.ResponseWriter, statusCode int, message string) {
	ps.mu.RLock()
	errorPage, hasCustomError := ps.errorPages[statusCode]
	ps.mu.RUnlock()

	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
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
