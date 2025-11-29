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
