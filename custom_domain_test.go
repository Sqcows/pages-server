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

package pages_server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestParseCustomDomainPath tests the parseCustomDomainPath function.
func TestParseCustomDomainPath(t *testing.T) {
	ps := &PagesServer{
		config: &Config{
			PagesDomain: "pages.example.com",
		},
	}

	tests := []struct {
		name     string
		urlPath  string
		expected string
	}{
		{
			name:     "root path",
			urlPath:  "/",
			expected: "public/index.html",
		},
		{
			name:     "empty path",
			urlPath:  "",
			expected: "public/index.html",
		},
		{
			name:     "file in root",
			urlPath:  "/about.html",
			expected: "public/about.html",
		},
		{
			name:     "nested path",
			urlPath:  "/assets/style.css",
			expected: "public/assets/style.css",
		},
		{
			name:     "path with trailing slash",
			urlPath:  "/blog/",
			expected: "public/blog",
		},
		{
			name:     "deeply nested path",
			urlPath:  "/docs/api/v1/reference.html",
			expected: "public/docs/api/v1/reference.html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ps.parseCustomDomainPath(tt.urlPath)
			if result != tt.expected {
				t.Errorf("parseCustomDomainPath(%q) = %q, want %q", tt.urlPath, result, tt.expected)
			}
		})
	}
}

// TestResolveCustomDomainWithCache tests the resolveCustomDomain function with cached values.
func TestResolveCustomDomainWithCache(t *testing.T) {
	cache := NewMemoryCache(300)
	ps := &PagesServer{
		config: &Config{
			PagesDomain:         "pages.example.com",
			ForgejoHost:         "https://git.example.com",
			EnableCustomDomains: true,
		},
		customDomainCache: cache,
	}

	// Pre-populate cache with new key format
	domain := "www.example.com"
	cacheKey := "custom_domain:" + domain
	cacheValue := []byte("testuser:testrepository")
	cache.Set(cacheKey, cacheValue)

	// Resolve from cache
	username, repository, err := ps.resolveCustomDomain(context.Background(), domain)
	if err != nil {
		t.Fatalf("resolveCustomDomain failed: %v", err)
	}

	if username != "testuser" {
		t.Errorf("Expected username %q, got %q", "testuser", username)
	}
	if repository != "testrepository" {
		t.Errorf("Expected repository %q, got %q", "testrepository", repository)
	}
}

// TestResolveCustomDomainNotRegistered tests that unregistered custom domains return an error.
func TestResolveCustomDomainNotRegistered(t *testing.T) {
	cache := NewMemoryCache(300)
	ps := &PagesServer{
		config: &Config{
			PagesDomain:         "pages.example.com",
			ForgejoHost:         "https://git.example.com",
			EnableCustomDomains: true,
		},
		customDomainCache: cache,
	}

	// Try to resolve unregistered domain
	domain := "unregistered.example.com"
	username, repository, err := ps.resolveCustomDomain(context.Background(), domain)

	if err == nil {
		t.Fatalf("Expected error for unregistered domain, got nil")
	}

	if username != "" || repository != "" {
		t.Errorf("Expected empty username and repository, got %q and %q", username, repository)
	}

	expectedErrMsg := "custom domain not registered"
	if !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("Expected error message to contain %q, got %q", expectedErrMsg, err.Error())
	}
}

// TestCustomDomainDisabled tests that custom domains are rejected when disabled.
func TestCustomDomainDisabled(t *testing.T) {
	config := &Config{
		PagesDomain:         "pages.example.com",
		ForgejoHost:         "https://git.example.com",
		EnableCustomDomains: false,
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler, err := New(context.Background(), next, config, "test-plugin")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Request with custom domain (not matching pagesDomain)
	req := httptest.NewRequest("GET", "https://www.example.com/", nil)
	req.Host = "www.example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// TestServeHTTPCustomDomainVsPagesDomain tests that the plugin correctly distinguishes
// between custom domain and pagesDomain requests.
func TestServeHTTPCustomDomainVsPagesDomain(t *testing.T) {
	ps := &PagesServer{
		config: &Config{
			PagesDomain:         "pages.example.com",
			ForgejoHost:         "https://git.example.com",
			EnableCustomDomains: true,
		},
		cache:             NewMemoryCache(300),
		customDomainCache: NewMemoryCache(300),
		passwordCache:     NewMemoryCache(60),
		forgejoClient:     NewForgejoClient("https://git.example.com", ""),
		errorPages:        make(map[int][]byte),
	}

	tests := []struct {
		name              string
		host              string
		isPagesDomain     bool
		isCustomDomain    bool
		expectedErrStatus int
	}{
		{
			name:           "pagesDomain request",
			host:           "user1.pages.example.com",
			isPagesDomain:  true,
			isCustomDomain: false,
		},
		{
			name:           "root pagesDomain (no subdomain)",
			host:           "pages.example.com",
			isPagesDomain:  true,
			isCustomDomain: false,
			// Will fail because no username subdomain
			expectedErrStatus: http.StatusBadRequest,
		},
		{
			name:           "custom domain",
			host:           "www.example.com",
			isPagesDomain:  false,
			isCustomDomain: true,
			// Will fail because we haven't set up a mock API response
			expectedErrStatus: http.StatusNotFound,
		},
		{
			name:           "another custom domain",
			host:           "blog.mysite.org",
			isPagesDomain:  false,
			isCustomDomain: true,
			expectedErrStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "https://"+tt.host+"/", nil)
			req.Host = tt.host
			req.Header.Set("X-Forwarded-Proto", "https")
			rec := httptest.NewRecorder()

			ps.ServeHTTP(rec, req)

			// We can't fully test the success case without mocking the Forgejo API,
			// but we can verify the request routing logic
			if tt.expectedErrStatus != 0 && rec.Code != tt.expectedErrStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedErrStatus, rec.Code)
			}
		})
	}
}

// TestCustomDomainCacheTTL tests that custom domain cache respects TTL.
func TestCustomDomainCacheTTL(t *testing.T) {
	config := CreateConfig()

	if config.CustomDomainCacheTTL != 600 {
		t.Errorf("Expected default CustomDomainCacheTTL to be 600, got %d", config.CustomDomainCacheTTL)
	}

	if config.EnableCustomDomains != true {
		t.Errorf("Expected default EnableCustomDomains to be true, got %v", config.EnableCustomDomains)
	}
}

// TestTraefikRouterConfigDefaults tests that Traefik router configuration has correct defaults.
func TestTraefikRouterConfigDefaults(t *testing.T) {
	config := CreateConfig()

	if config.TraefikRedisRouterEnabled != true {
		t.Errorf("Expected default TraefikRedisRouterEnabled to be true, got %v", config.TraefikRedisRouterEnabled)
	}

	if config.TraefikRedisCertResolver != "letsencrypt-http" {
		t.Errorf("Expected default TraefikRedisCertResolver to be 'letsencrypt-http', got %q", config.TraefikRedisCertResolver)
	}

	if config.TraefikRedisRouterTTL != 0 {
		t.Errorf("Expected default TraefikRedisRouterTTL to be 0 (persistent), got %d", config.TraefikRedisRouterTTL)
	}

	if config.TraefikRedisRootKey != "traefik" {
		t.Errorf("Expected default TraefikRedisRootKey to be 'traefik', got %q", config.TraefikRedisRootKey)
	}
}

// TestRegisterTraefikRouterDisabled tests that router registration is skipped when disabled.
func TestRegisterTraefikRouterDisabled(t *testing.T) {
	redisCache := NewRedisCache("localhost", 6379, "", 600, 10, 20, 5)
	defer redisCache.Close()

	ps := &PagesServer{
		config: &Config{
			TraefikRedisRouterEnabled: false,
			TraefikRedisCertResolver:  "letsencrypt-http",
			TraefikRedisRouterTTL:     600,
			TraefikRedisRootKey:       "traefik",
		},
		customDomainCache: redisCache,
	}

	// Should return nil without error when disabled
	err := ps.registerTraefikRouter(context.Background(), "test.example.com")
	if err != nil {
		t.Errorf("Expected no error when router registration is disabled, got: %v", err)
	}
}

// TestRegisterTraefikRouterWithMemoryCache tests that router registration is skipped with in-memory cache.
func TestRegisterTraefikRouterWithMemoryCache(t *testing.T) {
	memoryCache := NewMemoryCache(600)
	defer memoryCache.Stop()

	ps := &PagesServer{
		config: &Config{
			TraefikRedisRouterEnabled: true,
			TraefikRedisCertResolver:  "letsencrypt-http",
			TraefikRedisRouterTTL:     600,
			TraefikRedisRootKey:       "traefik",
		},
		customDomainCache: memoryCache,
	}

	// Should return nil without error when using memory cache
	err := ps.registerTraefikRouter(context.Background(), "test.example.com")
	if err != nil {
		t.Errorf("Expected no error when using memory cache, got: %v", err)
	}
}

// TestRegisterTraefikRouterWithRedis tests that router configuration is written to Redis correctly.
func TestRegisterTraefikRouterWithRedis(t *testing.T) {
	redisCache := NewRedisCache("localhost", 6379, "", 600, 10, 20, 5)
	defer redisCache.Close()

	customDomain := "test.example.com"
	routerName := "custom-test-example-com"

	ps := &PagesServer{
		config: &Config{
			TraefikRedisRouterEnabled: true,
			TraefikRedisCertResolver:  "letsencrypt-http",
			TraefikRedisRouterTTL:     600,
			TraefikRedisRootKey:       "traefik",
		},
		customDomainCache: redisCache,
	}

	// Register the router
	err := ps.registerTraefikRouter(context.Background(), customDomain)
	if err != nil {
		t.Fatalf("registerTraefikRouter failed: %v", err)
	}

	// Verify all router configuration keys were written to Redis
	expectedConfigs := map[string]string{
		"traefik/http/routers/" + routerName + "/rule":             "Host(`test.example.com`)",
		"traefik/http/routers/" + routerName + "/entryPoints/0":    "websecure",
		"traefik/http/routers/" + routerName + "/middlewares/0":    "pages-server@file",
		"traefik/http/routers/" + routerName + "/service":          "noop@internal",
		"traefik/http/routers/" + routerName + "/tls/certResolver": "letsencrypt-http",
		"traefik/http/routers/" + routerName + "/priority":         "10",
	}

	for key, expectedValue := range expectedConfigs {
		value, found := redisCache.Get(key)
		if !found {
			t.Errorf("Expected to find router config key %q in Redis", key)
			continue
		}

		if string(value) != expectedValue {
			t.Errorf("Router config key %q: expected value %q, got %q", key, expectedValue, string(value))
		}
	}

	// Clean up
	for key := range expectedConfigs {
		redisCache.Delete(key)
	}
}

// TestRegisterTraefikRouterSanitizesRouterName tests that router names are properly sanitized.
func TestRegisterTraefikRouterSanitizesRouterName(t *testing.T) {
	redisCache := NewRedisCache("localhost", 6379, "", 600, 10, 20, 5)
	defer redisCache.Close()

	tests := []struct {
		domain      string
		routerName  string
		description string
	}{
		{
			domain:      "example.com",
			routerName:  "custom-example-com",
			description: "simple domain",
		},
		{
			domain:      "sub.example.com",
			routerName:  "custom-sub-example-com",
			description: "subdomain",
		},
		{
			domain:      "deep.sub.example.com",
			routerName:  "custom-deep-sub-example-com",
			description: "deep subdomain",
		},
	}

	ps := &PagesServer{
		config: &Config{
			TraefikRedisRouterEnabled: true,
			TraefikRedisCertResolver:  "letsencrypt-http",
			TraefikRedisRouterTTL:     600,
			TraefikRedisRootKey:       "traefik",
		},
		customDomainCache: redisCache,
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			// Register the router
			err := ps.registerTraefikRouter(context.Background(), tt.domain)
			if err != nil {
				t.Fatalf("registerTraefikRouter failed: %v", err)
			}

			// Verify the router was created with the sanitized name
			ruleKey := fmt.Sprintf("traefik/http/routers/%s/rule", tt.routerName)
			value, found := redisCache.Get(ruleKey)
			if !found {
				t.Errorf("Expected to find router config key %q in Redis", ruleKey)
			}

			expectedRule := fmt.Sprintf("Host(`%s`)", tt.domain)
			if string(value) != expectedRule {
				t.Errorf("Expected rule %q, got %q", expectedRule, string(value))
			}

			// Clean up
			keysToDelete := []string{
				fmt.Sprintf("traefik/http/routers/%s/rule", tt.routerName),
				fmt.Sprintf("traefik/http/routers/%s/entryPoints/0", tt.routerName),
				fmt.Sprintf("traefik/http/routers/%s/middlewares/0", tt.routerName),
				fmt.Sprintf("traefik/http/routers/%s/service", tt.routerName),
				fmt.Sprintf("traefik/http/routers/%s/tls/certResolver", tt.routerName),
				fmt.Sprintf("traefik/http/routers/%s/priority", tt.routerName),
			}
			for _, key := range keysToDelete {
				redisCache.Delete(key)
			}
		})
	}
}

// TestRegisterTraefikRouterCustomRootKey tests that custom Redis root keys are respected.
func TestRegisterTraefikRouterCustomRootKey(t *testing.T) {
	redisCache := NewRedisCache("localhost", 6379, "", 600, 10, 20, 5)
	defer redisCache.Close()

	customDomain := "custom-root.example.com"
	customRootKey := "my-traefik"
	routerName := "custom-custom-root-example-com"

	ps := &PagesServer{
		config: &Config{
			TraefikRedisRouterEnabled: true,
			TraefikRedisCertResolver:  "letsencrypt-http",
			TraefikRedisRouterTTL:     600,
			TraefikRedisRootKey:       customRootKey,
		},
		customDomainCache: redisCache,
	}

	// Register the router
	err := ps.registerTraefikRouter(context.Background(), customDomain)
	if err != nil {
		t.Fatalf("registerTraefikRouter failed: %v", err)
	}

	// Verify the custom root key was used
	ruleKey := fmt.Sprintf("%s/http/routers/%s/rule", customRootKey, routerName)
	value, found := redisCache.Get(ruleKey)
	if !found {
		t.Errorf("Expected to find router config key %q with custom root key", ruleKey)
	}

	expectedRule := fmt.Sprintf("Host(`%s`)", customDomain)
	if string(value) != expectedRule {
		t.Errorf("Expected rule %q, got %q", expectedRule, string(value))
	}

	// Clean up
	keysToDelete := []string{
		fmt.Sprintf("%s/http/routers/%s/rule", customRootKey, routerName),
		fmt.Sprintf("%s/http/routers/%s/entryPoints/0", customRootKey, routerName),
		fmt.Sprintf("%s/http/routers/%s/middlewares/0", customRootKey, routerName),
		fmt.Sprintf("%s/http/routers/%s/service", customRootKey, routerName),
		fmt.Sprintf("%s/http/routers/%s/tls/certResolver", customRootKey, routerName),
		fmt.Sprintf("%s/http/routers/%s/priority", customRootKey, routerName),
	}
	for _, key := range keysToDelete {
		redisCache.Delete(key)
	}
}

// TestRegisterTraefikRouterCustomCertResolver tests that custom cert resolvers are used.
func TestRegisterTraefikRouterCustomCertResolver(t *testing.T) {
	redisCache := NewRedisCache("localhost", 6379, "", 600, 10, 20, 5)
	defer redisCache.Close()

	customDomain := "custom-cert.example.com"
	customCertResolver := "letsencrypt-dns"
	routerName := "custom-custom-cert-example-com"

	ps := &PagesServer{
		config: &Config{
			TraefikRedisRouterEnabled: true,
			TraefikRedisCertResolver:  customCertResolver,
			TraefikRedisRouterTTL:     600,
			TraefikRedisRootKey:       "traefik",
		},
		customDomainCache: redisCache,
	}

	// Register the router
	err := ps.registerTraefikRouter(context.Background(), customDomain)
	if err != nil {
		t.Fatalf("registerTraefikRouter failed: %v", err)
	}

	// Verify the custom cert resolver was used
	certResolverKey := fmt.Sprintf("traefik/http/routers/%s/tls/certResolver", routerName)
	value, found := redisCache.Get(certResolverKey)
	if !found {
		t.Errorf("Expected to find cert resolver config key %q", certResolverKey)
	}

	if string(value) != customCertResolver {
		t.Errorf("Expected cert resolver %q, got %q", customCertResolver, string(value))
	}

	// Clean up
	keysToDelete := []string{
		fmt.Sprintf("traefik/http/routers/%s/rule", routerName),
		fmt.Sprintf("traefik/http/routers/%s/entryPoints/0", routerName),
		fmt.Sprintf("traefik/http/routers/%s/middlewares/0", routerName),
		fmt.Sprintf("traefik/http/routers/%s/service", routerName),
		fmt.Sprintf("traefik/http/routers/%s/tls/certResolver", routerName),
		fmt.Sprintf("traefik/http/routers/%s/priority", routerName),
	}
	for _, key := range keysToDelete {
		redisCache.Delete(key)
	}
}
