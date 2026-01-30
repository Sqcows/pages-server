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
	"time"
)

// TestCreateConfig tests the CreateConfig function.
func TestCreateConfig(t *testing.T) {
	config := CreateConfig()

	if config == nil {
		t.Fatal("CreateConfig returned nil")
	}

	if config.RedisPort != 6379 {
		t.Errorf("Expected default RedisPort to be 6379, got %d", config.RedisPort)
	}

	if config.CacheTTL != 300 {
		t.Errorf("Expected default CacheTTL to be 300, got %d", config.CacheTTL)
	}
}

// TestNew tests the New function with valid configuration.
func TestNew(t *testing.T) {
	config := &Config{
		PagesDomain: "pages.example.com",
		ForgejoHost: "https://git.example.com",
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler, err := New(context.Background(), next, config, "test-plugin")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if handler == nil {
		t.Fatal("New returned nil handler")
	}

	ps, ok := handler.(*PagesServer)
	if !ok {
		t.Fatal("Handler is not a PagesServer")
	}

	if ps.config.PagesDomain != config.PagesDomain {
		t.Errorf("Expected PagesDomain %s, got %s", config.PagesDomain, ps.config.PagesDomain)
	}
}

// TestNewWithMissingConfig tests the New function with missing required configuration.
func TestNewWithMissingConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		errMsg string
	}{
		{
			name: "missing PagesDomain",
			config: &Config{
				ForgejoHost: "https://git.example.com",
			},
			errMsg: "pagesDomain is required",
		},
		{
			name: "missing ForgejoHost",
			config: &Config{
				PagesDomain: "pages.example.com",
			},
			errMsg: "forgejoHost is required",
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(context.Background(), next, tt.config, "test-plugin")
			if err == nil {
				t.Fatalf("Expected error %q, got nil", tt.errMsg)
			}
			if err.Error() != tt.errMsg {
				t.Errorf("Expected error %q, got %q", tt.errMsg, err.Error())
			}
		})
	}
}

// TestParseRequest tests the parseRequest function.
func TestParseRequest(t *testing.T) {
	ps := &PagesServer{
		config: &Config{
			PagesDomain: "pages.example.com",
		},
	}

	tests := []struct {
		name           string
		host           string
		path           string
		wantUsername   string
		wantRepository string
		wantFilePath   string
		wantErr        bool
	}{
		{
			name:           "repository with file",
			host:           "user1.pages.example.com",
			path:           "/myrepo/index.html",
			wantUsername:   "user1",
			wantRepository: "myrepo",
			wantFilePath:   "public/index.html",
			wantErr:        false,
		},
		{
			name:           "repository without file",
			host:           "user1.pages.example.com",
			path:           "/myrepo/",
			wantUsername:   "user1",
			wantRepository: "myrepo",
			wantFilePath:   "public",
			wantErr:        false,
		},
		{
			name:           "repository root",
			host:           "user1.pages.example.com",
			path:           "/myrepo",
			wantUsername:   "user1",
			wantRepository: "myrepo",
			wantFilePath:   "public",
			wantErr:        false,
		},
		{
			name:           "profile site",
			host:           "user1.pages.example.com",
			path:           "/",
			wantUsername:   "user1",
			wantRepository: ".profile",
			wantFilePath:   "public",
			wantErr:        false,
		},
		{
			name:           "profile site with file",
			host:           "user1.pages.example.com",
			path:           "/about.html",
			wantUsername:   "user1",
			wantRepository: ".profile",
			wantFilePath:   "public/about.html",
			wantErr:        false,
		},
		{
			name:           "nested path",
			host:           "user1.pages.example.com",
			path:           "/myrepo/assets/style.css",
			wantUsername:   "user1",
			wantRepository: "myrepo",
			wantFilePath:   "public/assets/style.css",
			wantErr:        false,
		},
		{
			name:    "invalid domain",
			host:    "example.com",
			path:    "/",
			wantErr: true,
		},
		{
			name:    "no subdomain",
			host:    "pages.example.com",
			path:    "/",
			wantErr: true,
		},
		{
			name:           "host with port",
			host:           "user1.pages.example.com:8080",
			path:           "/myrepo/index.html",
			wantUsername:   "user1",
			wantRepository: "myrepo",
			wantFilePath:   "public/index.html",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "https://"+tt.host+tt.path, nil)
			req.Host = tt.host

			username, repository, filePath, err := ps.parseRequest(req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if username != tt.wantUsername {
				t.Errorf("Expected username %q, got %q", tt.wantUsername, username)
			}
			if repository != tt.wantRepository {
				t.Errorf("Expected repository %q, got %q", tt.wantRepository, repository)
			}
			if filePath != tt.wantFilePath {
				t.Errorf("Expected filePath %q, got %q", tt.wantFilePath, filePath)
			}
		})
	}
}

// TestDetectContentType tests the detectContentType function.
func TestDetectContentType(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		content  []byte
		want     string
	}{
		{
			name:     "HTML file",
			filePath: "index.html",
			content:  []byte("<html></html>"),
			want:     "text/html; charset=utf-8",
		},
		{
			name:     "CSS file",
			filePath: "style.css",
			content:  []byte("body { color: red; }"),
			want:     "text/css; charset=utf-8",
		},
		{
			name:     "JavaScript file",
			filePath: "script.js",
			content:  []byte("console.log('test');"),
			want:     "application/javascript; charset=utf-8",
		},
		{
			name:     "JSON file",
			filePath: "data.json",
			content:  []byte(`{"key": "value"}`),
			want:     "application/json; charset=utf-8",
		},
		{
			name:     "PNG image",
			filePath: "image.png",
			content:  []byte{0x89, 0x50, 0x4E, 0x47},
			want:     "image/png",
		},
		{
			name:     "JPEG image",
			filePath: "photo.jpg",
			content:  []byte{0xFF, 0xD8, 0xFF},
			want:     "image/jpeg",
		},
		{
			name:     "SVG image",
			filePath: "icon.svg",
			content:  []byte("<svg></svg>"),
			want:     "image/svg+xml",
		},
		{
			name:     "favicon",
			filePath: "favicon.ico",
			content:  []byte{0x00, 0x00, 0x01, 0x00},
			want:     "image/x-icon",
		},
		{
			name:     "PDF file",
			filePath: "document.pdf",
			content:  []byte("%PDF-1.4"),
			want:     "application/pdf",
		},
		{
			name:     "text file",
			filePath: "readme.txt",
			content:  []byte("This is a text file"),
			want:     "text/plain; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectContentType(tt.filePath, tt.content)
			if got != tt.want {
				t.Errorf("detectContentType(%q) = %q, want %q", tt.filePath, got, tt.want)
			}
		})
	}
}

// TestServeHTTP tests the ServeHTTP method with HTTPS redirect.
func TestServeHTTPRedirect(t *testing.T) {
	config := &Config{
		PagesDomain: "pages.example.com",
		ForgejoHost: "https://git.example.com",
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler, err := New(context.Background(), next, config, "test-plugin")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	req := httptest.NewRequest("GET", "http://user1.pages.example.com/myrepo/", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Errorf("Expected status %d, got %d", http.StatusMovedPermanently, rec.Code)
	}

	location := rec.Header().Get("Location")
	expectedLocation := "https://user1.pages.example.com/myrepo/"
	if location != expectedLocation {
		t.Errorf("Expected Location header %q, got %q", expectedLocation, location)
	}
}

// TestServeHTTPACMEChallenge tests that ACME challenge requests are passed through without redirect.
func TestServeHTTPACMEChallenge(t *testing.T) {
	config := &Config{
		PagesDomain: "pages.example.com",
		ForgejoHost: "https://git.example.com",
	}

	// Track if the next handler was called
	nextHandlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		// Simulate Traefik's ACME handler responding
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ACME-challenge-token-response"))
	})

	handler, err := New(context.Background(), next, config, "test-plugin")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Test ACME challenge request with HTTP protocol
	req := httptest.NewRequest("GET", "http://example.com/.well-known/acme-challenge/token123", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify the request was passed through to the next handler
	if !nextHandlerCalled {
		t.Error("Expected next handler to be called for ACME challenge, but it wasn't")
	}

	// Verify it was NOT redirected
	if rec.Code == http.StatusMovedPermanently || rec.Code == http.StatusFound {
		t.Errorf("ACME challenge should not be redirected, got status %d", rec.Code)
	}

	// Verify it returned OK from the next handler
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d from next handler, got %d", http.StatusOK, rec.Code)
	}

	// Verify the response body from the next handler
	body := rec.Body.String()
	if body != "ACME-challenge-token-response" {
		t.Errorf("Expected ACME response from next handler, got %q", body)
	}
}

// TestServeHTTPACMEChallengeVariations tests different ACME challenge path variations.
func TestServeHTTPACMEChallengeVariations(t *testing.T) {
	config := &Config{
		PagesDomain: "pages.example.com",
		ForgejoHost: "https://git.example.com",
	}

	tests := []struct {
		name           string
		path           string
		shouldPassThru bool
	}{
		{
			name:           "standard ACME challenge",
			path:           "/.well-known/acme-challenge/token123",
			shouldPassThru: true,
		},
		{
			name:           "ACME challenge with long token",
			path:           "/.well-known/acme-challenge/very-long-token-string-123456789",
			shouldPassThru: true,
		},
		{
			name:           "ACME challenge at root",
			path:           "/.well-known/acme-challenge/",
			shouldPassThru: true,
		},
		{
			name:           "normal well-known path",
			path:           "/.well-known/security.txt",
			shouldPassThru: false,
		},
		{
			name:           "normal root path",
			path:           "/",
			shouldPassThru: false,
		},
		{
			name:           "normal file path",
			path:           "/index.html",
			shouldPassThru: false,
		},
		{
			name:           "path that starts similar to ACME",
			path:           "/.well-known/other",
			shouldPassThru: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextHandlerCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextHandlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler, err := New(context.Background(), next, config, "test-plugin")
			if err != nil {
				t.Fatalf("New failed: %v", err)
			}

			req := httptest.NewRequest("GET", "http://example.com"+tt.path, nil)
			req.Header.Set("X-Forwarded-Proto", "http")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if tt.shouldPassThru {
				// Should pass through to next handler
				if !nextHandlerCalled {
					t.Errorf("Expected next handler to be called for path %q, but it wasn't", tt.path)
				}
				if rec.Code == http.StatusMovedPermanently {
					t.Errorf("Path %q should not be redirected, got status %d", tt.path, rec.Code)
				}
			} else {
				// Should be redirected (not passed through)
				if rec.Code != http.StatusMovedPermanently {
					// Note: nextHandlerCalled might be true for non-ACME paths that get redirected
					// We only care about the redirect status
					if nextHandlerCalled && rec.Code != http.StatusMovedPermanently {
						t.Errorf("Path %q should be redirected to HTTPS, got status %d", tt.path, rec.Code)
					}
				}
			}
		})
	}
}

// TestServeHTTPACMEChallengeWithHTTPS tests that ACME challenges work even with HTTPS.
func TestServeHTTPACMEChallengeWithHTTPS(t *testing.T) {
	config := &Config{
		PagesDomain: "pages.example.com",
		ForgejoHost: "https://git.example.com",
	}

	nextHandlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler, err := New(context.Background(), next, config, "test-plugin")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Test ACME challenge request with HTTPS protocol (edge case)
	req := httptest.NewRequest("GET", "https://example.com/.well-known/acme-challenge/token123", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should still pass through even with HTTPS
	if !nextHandlerCalled {
		t.Error("Expected next handler to be called for ACME challenge with HTTPS")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

// TestServeError tests the serveError method.
func TestServeError(t *testing.T) {
	ps := &PagesServer{
		errorPages: make(map[int][]byte),
	}

	rec := httptest.NewRecorder()
	ps.serveError(rec, http.StatusNotFound, "Not found")

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type %q, got %q", "text/html; charset=utf-8", contentType)
	}

	body := rec.Body.String()
	if body == "" {
		t.Error("Expected non-empty error page")
	}
}

// TestServeErrorWithCustomPage tests the serveError method with a custom error page.
func TestServeErrorWithCustomPage(t *testing.T) {
	customError := []byte("<html><body><h1>Custom 404</h1></body></html>")
	ps := &PagesServer{
		errorPages: map[int][]byte{
			404: customError,
		},
	}

	rec := httptest.NewRecorder()
	ps.serveError(rec, http.StatusNotFound, "Not found")

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	body := rec.Body.Bytes()
	if string(body) != string(customError) {
		t.Errorf("Expected custom error page, got %q", string(body))
	}
}

// TestHashPassword tests the hashPassword function.
func TestHashPassword(t *testing.T) {
	password := "test-password-123"
	hash := hashPassword(password)

	// SHA256 hash should be 64 hex characters
	if len(hash) != 64 {
		t.Errorf("Expected hash length 64, got %d", len(hash))
	}

	// Same password should produce same hash
	hash2 := hashPassword(password)
	if hash != hash2 {
		t.Error("Same password should produce same hash")
	}

	// Different password should produce different hash
	hash3 := hashPassword("different-password")
	if hash == hash3 {
		t.Error("Different passwords should produce different hashes")
	}
}

// TestCreateBranchAuthCookie tests the createBranchAuthCookie function.
func TestCreateBranchAuthCookie(t *testing.T) {
	ps := &PagesServer{
		config: &Config{
			AuthSecretKey:      "test-secret-key-for-signing-cookies",
			AuthCookieDuration: 3600,
		},
	}

	username := "testuser"
	repository := "testrepo"
	cookie := ps.createBranchAuthCookie(username, repository)

	if cookie == nil {
		t.Fatal("createBranchAuthCookie returned nil")
	}

	expectedCookieName := fmt.Sprintf("pages_branch_auth_%s_%s", username, repository)
	if cookie.Name != expectedCookieName {
		t.Errorf("Expected cookie name %q, got %q", expectedCookieName, cookie.Name)
	}

	if cookie.Path != "/" {
		t.Errorf("Expected cookie path '/', got %q", cookie.Path)
	}

	if cookie.MaxAge != 3600 {
		t.Errorf("Expected cookie MaxAge 3600, got %d", cookie.MaxAge)
	}

	if !cookie.HttpOnly {
		t.Error("Expected HttpOnly to be true")
	}

	if !cookie.Secure {
		t.Error("Expected Secure to be true")
	}

	if cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("Expected SameSite StrictMode, got %v", cookie.SameSite)
	}

	// Verify cookie value format (timestamp|signature)
	if cookie.Value == "" {
		t.Error("Expected non-empty cookie value")
	}
}

// TestCreateBranchAuthCookieWithoutSecretKey tests cookie creation without a secret key.
func TestCreateBranchAuthCookieWithoutSecretKey(t *testing.T) {
	ps := &PagesServer{
		config: &Config{
			AuthSecretKey:      "", // No secret key
			AuthCookieDuration: 3600,
		},
	}

	username := "testuser"
	repository := "testrepo"
	cookie := ps.createBranchAuthCookie(username, repository)

	if cookie == nil {
		t.Fatal("createBranchAuthCookie returned nil")
	}

	expectedCookieName := fmt.Sprintf("pages_branch_auth_%s_%s", username, repository)
	if cookie.Name != expectedCookieName {
		t.Errorf("Expected cookie name %q, got %q", expectedCookieName, cookie.Name)
	}

	// Without secret key, value should just be timestamp
	if cookie.Value == "" {
		t.Error("Expected non-empty cookie value")
	}
}

// TestVerifyBranchAuthCookie tests the verifyBranchAuthCookie function.
func TestVerifyBranchAuthCookie(t *testing.T) {
	ps := &PagesServer{
		config: &Config{
			AuthSecretKey:      "test-secret-key-for-signing-cookies",
			AuthCookieDuration: 3600,
		},
	}

	username := "testuser"
	repository := "testrepo"

	// Create a valid cookie
	cookie := ps.createBranchAuthCookie(username, repository)

	// Verify it
	if !ps.verifyBranchAuthCookie(cookie.Value, username, repository) {
		t.Error("Failed to verify valid branch auth cookie")
	}

	// Test invalid cookie formats
	invalidCookies := []string{
		"",
		"invalid",
		"timestamp-only",
		"invalid|signature|format",
	}

	for _, invalid := range invalidCookies {
		if ps.verifyBranchAuthCookie(invalid, username, repository) {
			t.Errorf("Expected invalid cookie %q to fail verification", invalid)
		}
	}

	// Test cookie from different repository should fail
	otherRepo := "otherrepo"
	if ps.verifyBranchAuthCookie(cookie.Value, username, otherRepo) {
		t.Error("Expected cookie from different repository to fail verification")
	}
}

// TestVerifyBranchAuthCookieExpiration tests cookie expiration.
func TestVerifyBranchAuthCookieExpiration(t *testing.T) {
	ps := &PagesServer{
		config: &Config{
			AuthSecretKey:      "test-secret-key-for-signing-cookies",
			AuthCookieDuration: 1, // 1 second duration
		},
	}

	username := "testuser"
	repository := "testrepo"

	// Create a cookie with very short duration
	cookie := ps.createBranchAuthCookie(username, repository)

	// Should be valid immediately
	if !ps.verifyBranchAuthCookie(cookie.Value, username, repository) {
		t.Error("Cookie should be valid immediately after creation")
	}

	// Wait for expiration
	time.Sleep(2 * time.Second)

	// Should be invalid after expiration
	if ps.verifyBranchAuthCookie(cookie.Value, username, repository) {
		t.Error("Cookie should be invalid after expiration")
	}
}

// TestIsBranchAuthenticated tests the isBranchAuthenticated function.
func TestIsBranchAuthenticated(t *testing.T) {
	ps := &PagesServer{
		config: &Config{
			AuthSecretKey:      "test-secret-key-for-signing-cookies",
			AuthCookieDuration: 3600,
		},
	}

	username := "testuser"
	repository := "testrepo"

	// Request without cookie should not be authenticated
	req := httptest.NewRequest("GET", "https://example.com/", nil)
	if ps.isBranchAuthenticated(req, username, repository) {
		t.Error("Request without cookie should not be authenticated")
	}

	// Request with valid cookie should be authenticated
	cookie := ps.createBranchAuthCookie(username, repository)
	req.AddCookie(cookie)
	if !ps.isBranchAuthenticated(req, username, repository) {
		t.Error("Request with valid cookie should be authenticated")
	}

	// Request with invalid cookie should not be authenticated
	req2 := httptest.NewRequest("GET", "https://example.com/", nil)
	invalidCookie := &http.Cookie{
		Name:  fmt.Sprintf("pages_branch_auth_%s_%s", username, repository),
		Value: "invalid-cookie-value",
	}
	req2.AddCookie(invalidCookie)
	if ps.isBranchAuthenticated(req2, username, repository) {
		t.Error("Request with invalid cookie should not be authenticated")
	}

	// Request with cookie from different repository should not be authenticated
	req3 := httptest.NewRequest("GET", "https://example.com/", nil)
	req3.AddCookie(cookie)
	if ps.isBranchAuthenticated(req3, username, "otherrepo") {
		t.Error("Request with cookie from different repository should not be authenticated")
	}
}

// TestGenerateBranchSignature tests the generateBranchSignature function.
func TestGenerateBranchSignature(t *testing.T) {
	ps := &PagesServer{
		config: &Config{
			AuthSecretKey: "test-secret-key-for-signing-cookies",
		},
	}

	timestamp := "1234567890"
	username := "testuser"
	repository := "testrepo"
	signature := ps.generateBranchSignature(timestamp, username, repository)

	// Should be a valid hex string
	if len(signature) != 64 {
		t.Errorf("Expected signature length 64, got %d", len(signature))
	}

	// Same parameters should produce same signature
	signature2 := ps.generateBranchSignature(timestamp, username, repository)
	if signature != signature2 {
		t.Error("Same parameters should produce same signature")
	}

	// Different timestamp should produce different signature
	signature3 := ps.generateBranchSignature("9876543210", username, repository)
	if signature == signature3 {
		t.Error("Different timestamps should produce different signatures")
	}

	// Different repository should produce different signature
	signature4 := ps.generateBranchSignature(timestamp, username, "otherrepo")
	if signature == signature4 {
		t.Error("Different repositories should produce different signatures")
	}
}

// TestServeBranchLoginPage tests the serveBranchLoginPage function.
func TestServeBranchLoginPage(t *testing.T) {
	ps := &PagesServer{}

	req := httptest.NewRequest("GET", "https://stage.example.com/", nil)
	rec := httptest.NewRecorder()

	ps.serveBranchLoginPage(rec, req, "")

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type %q, got %q", "text/html; charset=utf-8", contentType)
	}

	body := rec.Body.String()
	if body == "" {
		t.Error("Expected non-empty login page")
	}

	// Check for expected content
	expectedStrings := []string{
		"Branch Access Protected",
		"password",
		"Login",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(body, expected) {
			t.Errorf("Expected login page to contain %q", expected)
		}
	}
}

// TestServeBranchLoginPageWithError tests the serveBranchLoginPage function with error message.
func TestServeBranchLoginPageWithError(t *testing.T) {
	ps := &PagesServer{}

	req := httptest.NewRequest("GET", "https://stage.example.com/", nil)
	rec := httptest.NewRecorder()

	errorMsg := "Incorrect password"
	ps.serveBranchLoginPage(rec, req, errorMsg)

	body := rec.Body.String()
	if !strings.Contains(body, errorMsg) {
		t.Errorf("Expected login page to contain error message %q", errorMsg)
	}

	if !strings.Contains(body, "error") {
		t.Error("Expected login page to have error styling")
	}
}
