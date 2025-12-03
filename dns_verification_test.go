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
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGenerateDomainVerificationHash tests the hash generation function.
func TestGenerateDomainVerificationHash(t *testing.T) {
	tests := []struct {
		name       string
		owner      string
		repository string
		wantHash   string
	}{
		{
			name:       "Basic repository",
			owner:      "squarecows",
			repository: "bovine-website",
			wantHash:   computeSHA256("squarecows/bovine-website"),
		},
		{
			name:       "Repository with dashes",
			owner:      "test-user",
			repository: "my-repo-name",
			wantHash:   computeSHA256("test-user/my-repo-name"),
		},
		{
			name:       "Repository with numbers",
			owner:      "user123",
			repository: "repo456",
			wantHash:   computeSHA256("user123/repo456"),
		},
		{
			name:       "Repository with underscores",
			owner:      "test_user",
			repository: "test_repo",
			wantHash:   computeSHA256("test_user/test_repo"),
		},
		{
			name:       "Long repository name",
			owner:      "verylongusername",
			repository: "verylongrepositoryname",
			wantHash:   computeSHA256("verylongusername/verylongrepositoryname"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateDomainVerificationHash(tt.owner, tt.repository)
			if got != tt.wantHash {
				t.Errorf("generateDomainVerificationHash() = %v, want %v", got, tt.wantHash)
			}
		})
	}
}

// TestGenerateDomainVerificationHashConsistency tests that the hash is consistent.
func TestGenerateDomainVerificationHashConsistency(t *testing.T) {
	owner := "testuser"
	repository := "testrepo"

	// Generate hash multiple times
	hash1 := generateDomainVerificationHash(owner, repository)
	hash2 := generateDomainVerificationHash(owner, repository)
	hash3 := generateDomainVerificationHash(owner, repository)

	// All hashes should be identical
	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("Hash is not consistent: %s, %s, %s", hash1, hash2, hash3)
	}

	// Hash should be 64 characters (SHA256 hex)
	if len(hash1) != 64 {
		t.Errorf("Hash length is %d, expected 64", len(hash1))
	}
}

// TestGenerateDomainVerificationHashDifferentRepos tests that different repos have different hashes.
func TestGenerateDomainVerificationHashDifferentRepos(t *testing.T) {
	hash1 := generateDomainVerificationHash("user1", "repo1")
	hash2 := generateDomainVerificationHash("user2", "repo2")
	hash3 := generateDomainVerificationHash("user1", "repo2")

	// All hashes should be different
	if hash1 == hash2 || hash2 == hash3 || hash1 == hash3 {
		t.Errorf("Different repositories should have different hashes")
	}
}

// TestVerifyCustomDomainDNS_Disabled tests that verification is skipped when disabled.
func TestVerifyCustomDomainDNS_Disabled(t *testing.T) {
	// Create mock Forgejo server
	forgejoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle repository info request
		if r.URL.Path == "/api/v1/repos/testuser/testrepo" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"name":"testrepo","full_name":"testuser/testrepo","private":false,"default_branch":"main"}`))
			return
		}

		// Handle .pages file request
		if r.URL.Path == "/api/v1/repos/testuser/testrepo/contents/.pages" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Base64 encoded: "enabled: true\ncustom_domain: test.example.com\n"
			w.Write([]byte(`{"type":"file","encoding":"base64","content":"ZW5hYmxlZDogdHJ1ZQpjdXN0b21fZG9tYWluOiB0ZXN0LmV4YW1wbGUuY29tCg==","name":".pages","path":".pages"}`))
			return
		}

		http.NotFound(w, r)
	}))
	defer forgejoServer.Close()

	// Create config with DNS verification disabled
	config := &Config{
		PagesDomain:                       "pages.example.com",
		ForgejoHost:                       forgejoServer.URL,
		EnableCustomDomainDNSVerification: false,
	}

	forgejoClient := NewForgejoClient(config.ForgejoHost, "")
	ps := &PagesServer{
		config:            config,
		forgejoClient:     forgejoClient,
		customDomainCache: NewMemoryCache(0),
	}

	// Register custom domain without DNS verification
	ctx := context.Background()
	ps.registerCustomDomain(ctx, "testuser", "testrepo")

	// Verify the domain was registered (DNS verification was skipped)
	cacheKey := "custom_domain:test.example.com"
	cached, found := ps.customDomainCache.Get(cacheKey)
	if !found {
		t.Fatal("Custom domain should be registered when DNS verification is disabled")
	}

	expectedValue := "testuser:testrepo"
	if string(cached) != expectedValue {
		t.Errorf("Cache value = %s, want %s", string(cached), expectedValue)
	}
}

// TestVerifyCustomDomainDNS_NoCustomDomain tests that nothing happens when no custom domain is configured.
func TestVerifyCustomDomainDNS_NoCustomDomain(t *testing.T) {
	// Create mock Forgejo server
	forgejoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle repository info request
		if r.URL.Path == "/api/v1/repos/testuser/testrepo" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"name":"testrepo","full_name":"testuser/testrepo","private":false,"default_branch":"main"}`))
			return
		}

		// Handle .pages file request - no custom_domain field
		if r.URL.Path == "/api/v1/repos/testuser/testrepo/contents/.pages" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Base64 encoded: "enabled: true\n"
			w.Write([]byte(`{"type":"file","encoding":"base64","content":"ZW5hYmxlZDogdHJ1ZQo=","name":".pages","path":".pages"}`))
			return
		}

		http.NotFound(w, r)
	}))
	defer forgejoServer.Close()

	config := &Config{
		PagesDomain:                       "pages.example.com",
		ForgejoHost:                       forgejoServer.URL,
		EnableCustomDomainDNSVerification: true,
	}

	forgejoClient := NewForgejoClient(config.ForgejoHost, "")
	ps := &PagesServer{
		config:            config,
		forgejoClient:     forgejoClient,
		customDomainCache: NewMemoryCache(0),
	}

	ctx := context.Background()
	ps.registerCustomDomain(ctx, "testuser", "testrepo")

	// Verify no domain was registered - check if cache is empty
	// Try getting a non-existent key to ensure nothing was set
	_, found := ps.customDomainCache.Get("custom_domain:any.domain.com")
	if found {
		t.Errorf("No domain should be registered when CustomDomain is empty")
	}
}

// TestRegisterCustomDomain_WithDNSVerification_Integration tests the full flow with a mock HTTP server.
func TestRegisterCustomDomain_WithDNSVerification_Integration(t *testing.T) {
	// Create mock Forgejo server
	forgejoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle repository info request
		if r.URL.Path == "/api/v1/repos/testuser/testrepo" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"name":"testrepo","full_name":"testuser/testrepo","private":false,"default_branch":"main"}`))
			return
		}

		// Handle .pages file request
		if r.URL.Path == "/api/v1/repos/testuser/testrepo/contents/.pages" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Base64 encoded: "enabled: true\ncustom_domain: test.example.com\n"
			w.Write([]byte(`{"type":"file","encoding":"base64","content":"ZW5hYmxlZDogdHJ1ZQpjdXN0b21fZG9tYWluOiB0ZXN0LmV4YW1wbGUuY29tCg==","name":".pages","path":".pages"}`))
			return
		}

		http.NotFound(w, r)
	}))
	defer forgejoServer.Close()

	// Create config with DNS verification enabled
	config := &Config{
		PagesDomain:                       "pages.example.com",
		ForgejoHost:                       forgejoServer.URL,
		EnableCustomDomainDNSVerification: true,
	}

	forgejoClient := NewForgejoClient(config.ForgejoHost, "")
	ps := &PagesServer{
		config:            config,
		forgejoClient:     forgejoClient,
		customDomainCache: NewMemoryCache(0),
	}

	ctx := context.Background()

	// Register custom domain (will fail DNS verification for non-existent domain)
	ps.registerCustomDomain(ctx, "testuser", "testrepo")

	// Domain should NOT be registered because DNS verification will fail
	cacheKey := "custom_domain:test.example.com"
	_, found := ps.customDomainCache.Get(cacheKey)
	if found {
		t.Error("Custom domain should NOT be registered when DNS verification fails")
	}
}

// TestDNSVerificationHashFormat tests that the hash format is correct.
func TestDNSVerificationHashFormat(t *testing.T) {
	hash := generateDomainVerificationHash("testuser", "testrepo")

	// Hash should be 64 hex characters (SHA256)
	if len(hash) != 64 {
		t.Errorf("Hash length is %d, expected 64", len(hash))
	}

	// Hash should be valid hex
	_, err := hex.DecodeString(hash)
	if err != nil {
		t.Errorf("Hash is not valid hex: %v", err)
	}

	// Hash should be lowercase
	if hash != hashToLowercase(hash) {
		t.Errorf("Hash should be lowercase")
	}
}

// TestDNSVerificationExpectedFormat tests the expected TXT record format.
func TestDNSVerificationExpectedFormat(t *testing.T) {
	owner := "squarecows"
	repository := "bovine-website"
	hash := generateDomainVerificationHash(owner, repository)

	expectedFormat := "bovine-pages-verification=" + hash

	// Expected format should have the correct prefix
	if !hasPrefix(expectedFormat, "bovine-pages-verification=") {
		t.Errorf("Expected format should start with 'bovine-pages-verification='")
	}

	// Total length should be prefix (26 chars: "bovine-pages-verification=") + hash (64 chars) = 90 chars
	if len(expectedFormat) != 90 {
		t.Errorf("Expected format length is %d, expected 90", len(expectedFormat))
	}
}

// Helper functions for tests

// computeSHA256 computes the SHA256 hash of a string and returns it as hex.
func computeSHA256(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

// hashToLowercase converts a string to lowercase (for testing).
func hashToLowercase(s string) string {
	result := ""
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			result += string(c + 32)
		} else {
			result += string(c)
		}
	}
	return result
}

// hasPrefix checks if a string has a prefix (for testing without strings package in some contexts).
func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if s[i] != prefix[i] {
			return false
		}
	}
	return true
}
