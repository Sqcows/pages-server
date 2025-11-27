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

	// LetsEncryptEndpoint is the ACME endpoint for Let's Encrypt
	LetsEncryptEndpoint string `json:"letsEncryptEndpoint,omitempty"`

	// LetsEncryptEmail is the email address for Let's Encrypt registration
	LetsEncryptEmail string `json:"letsEncryptEmail,omitempty"`

	// CloudflareAPIKey is the API key for Cloudflare DNS management
	CloudflareAPIKey string `json:"cloudflareAPIKey,omitempty"`

	// CloudflareZoneID is the Zone ID for Cloudflare DNS
	CloudflareZoneID string `json:"cloudflareZoneID,omitempty"`

	// ErrorPagesRepo is the repository containing custom error pages (format: username/repository)
	ErrorPagesRepo string `json:"errorPagesRepo,omitempty"`

	// RedisHost is the Redis server host for caching (optional)
	RedisHost string `json:"redisHost,omitempty"`

	// RedisPort is the Redis server port (optional, default: 6379)
	RedisPort int `json:"redisPort,omitempty"`

	// RedisPassword is the Redis password (optional)
	RedisPassword string `json:"redisPassword,omitempty"`

	// CacheTTL is the cache time-to-live in seconds (default: 300)
	CacheTTL int `json:"cacheTTL,omitempty"`
}

// CreateConfig creates and initializes the plugin configuration.
func CreateConfig() *Config {
	return &Config{
		RedisPort: 6379,
		CacheTTL:  300,
	}
}

// PagesServer is the main plugin structure.
type PagesServer struct {
	next          http.Handler
	name          string
	config        *Config
	forgejoClient *ForgejoClient
	certManager   *CertificateManager
	dnsManager    *CloudflareDNSManager
	cache         Cache
	mu            sync.RWMutex
	errorPages    map[int][]byte
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
	if config.LetsEncryptEndpoint == "" {
		return nil, fmt.Errorf("letsEncryptEndpoint is required")
	}
	if config.LetsEncryptEmail == "" {
		return nil, fmt.Errorf("letsEncryptEmail is required")
	}

	// Initialize Forgejo client
	forgejoClient := NewForgejoClient(config.ForgejoHost, config.ForgejoToken)

	// Initialize certificate manager for Let's Encrypt
	certManager, err := NewCertificateManager(config.LetsEncryptEndpoint, config.LetsEncryptEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate manager: %w", err)
	}

	// Initialize Cloudflare DNS manager
	var dnsManager *CloudflareDNSManager
	if config.CloudflareAPIKey != "" && config.CloudflareZoneID != "" {
		dnsManager = NewCloudfllareDNSManager(config.CloudflareAPIKey, config.CloudflareZoneID)
	}

	// Initialize cache (Redis or in-memory)
	var cache Cache
	if config.RedisHost != "" {
		cache = NewRedisCache(config.RedisHost, config.RedisPort, config.RedisPassword, config.CacheTTL)
	} else {
		cache = NewMemoryCache(config.CacheTTL)
	}

	ps := &PagesServer{
		next:          next,
		name:          name,
		config:        config,
		forgejoClient: forgejoClient,
		certManager:   certManager,
		dnsManager:    dnsManager,
		cache:         cache,
		errorPages:    make(map[int][]byte),
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

	// Parse the request to determine username, repository, and file path
	username, repository, filePath, err := ps.parseRequest(req)
	if err != nil {
		ps.serveError(rw, http.StatusBadRequest, "Invalid request format")
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
