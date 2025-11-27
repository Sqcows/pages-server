package pages_server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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
		PagesDomain:         "pages.example.com",
		ForgejoHost:         "https://git.example.com",
		LetsEncryptEndpoint: "https://acme-staging-v02.api.letsencrypt.org/directory",
		LetsEncryptEmail:    "admin@example.com",
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
				ForgejoHost:         "https://git.example.com",
				LetsEncryptEndpoint: "https://acme-staging-v02.api.letsencrypt.org/directory",
				LetsEncryptEmail:    "admin@example.com",
			},
			errMsg: "pagesDomain is required",
		},
		{
			name: "missing ForgejoHost",
			config: &Config{
				PagesDomain:         "pages.example.com",
				LetsEncryptEndpoint: "https://acme-staging-v02.api.letsencrypt.org/directory",
				LetsEncryptEmail:    "admin@example.com",
			},
			errMsg: "forgejoHost is required",
		},
		{
			name: "missing LetsEncryptEndpoint",
			config: &Config{
				PagesDomain:      "pages.example.com",
				ForgejoHost:      "https://git.example.com",
				LetsEncryptEmail: "admin@example.com",
			},
			errMsg: "letsEncryptEndpoint is required",
		},
		{
			name: "missing LetsEncryptEmail",
			config: &Config{
				PagesDomain:         "pages.example.com",
				ForgejoHost:         "https://git.example.com",
				LetsEncryptEndpoint: "https://acme-staging-v02.api.letsencrypt.org/directory",
			},
			errMsg: "letsEncryptEmail is required",
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
			wantFilePath:   "public/index.html",
			wantErr:        false,
		},
		{
			name:           "repository root",
			host:           "user1.pages.example.com",
			path:           "/myrepo",
			wantUsername:   "user1",
			wantRepository: "myrepo",
			wantFilePath:   "public/index.html",
			wantErr:        false,
		},
		{
			name:           "profile site",
			host:           "user1.pages.example.com",
			path:           "/",
			wantUsername:   "user1",
			wantRepository: ".profile",
			wantFilePath:   "public/index.html",
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
		PagesDomain:         "pages.example.com",
		ForgejoHost:         "https://git.example.com",
		LetsEncryptEndpoint: "https://acme-staging-v02.api.letsencrypt.org/directory",
		LetsEncryptEmail:    "admin@example.com",
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
