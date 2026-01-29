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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestNewForgejoClient tests the NewForgejoClient function.
func TestNewForgejoClient(t *testing.T) {
	client := NewForgejoClient("https://git.example.com", "test-token")

	if client == nil {
		t.Fatal("NewForgejoClient returned nil")
	}

	if client.baseURL != "https://git.example.com" {
		t.Errorf("Expected baseURL %q, got %q", "https://git.example.com", client.baseURL)
	}

	if client.token != "test-token" {
		t.Errorf("Expected token %q, got %q", "test-token", client.token)
	}
}

// TestNewForgejoClientTrimsSlash tests that NewForgejoClient trims trailing slashes.
func TestNewForgejoClientTrimsSlash(t *testing.T) {
	client := NewForgejoClient("https://git.example.com/", "test-token")

	if client.baseURL != "https://git.example.com" {
		t.Errorf("Expected baseURL without trailing slash, got %q", client.baseURL)
	}
}

// TestGetRepository tests the GetRepository method.
func TestGetRepository(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/repos/testuser/testrepo" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		if r.Header.Get("Authorization") != "token test-token" {
			t.Errorf("Expected Authorization header with token")
		}

		repo := RepositoryInfo{
			Name:          "testrepo",
			FullName:      "testuser/testrepo",
			Private:       false,
			DefaultBranch: "main",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repo)
	}))
	defer server.Close()

	client := NewForgejoClient(server.URL, "test-token")
	repo, err := client.GetRepository(context.Background(), "testuser", "testrepo")

	if err != nil {
		t.Fatalf("GetRepository failed: %v", err)
	}

	if repo.Name != "testrepo" {
		t.Errorf("Expected name %q, got %q", "testrepo", repo.Name)
	}

	if repo.FullName != "testuser/testrepo" {
		t.Errorf("Expected full_name %q, got %q", "testuser/testrepo", repo.FullName)
	}

	if repo.Private != false {
		t.Errorf("Expected private to be false, got %v", repo.Private)
	}

	if repo.DefaultBranch != "main" {
		t.Errorf("Expected default_branch %q, got %q", "main", repo.DefaultBranch)
	}
}

// TestGetRepositoryNotFound tests the GetRepository method with a non-existent repository.
func TestGetRepositoryNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewForgejoClient(server.URL, "test-token")
	_, err := client.GetRepository(context.Background(), "testuser", "nonexistent")

	if err == nil {
		t.Fatal("Expected error for non-existent repository")
	}

	if err.Error() != "repository not found" {
		t.Errorf("Expected 'repository not found' error, got %q", err.Error())
	}
}

// TestHasPagesFile tests the HasPagesFile method.
func TestHasPagesFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/repos/testuser/testrepo" {
			repo := RepositoryInfo{
				Name:          "testrepo",
				FullName:      "testuser/testrepo",
				Private:       false,
				DefaultBranch: "main",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(repo)
			return
		}

		if r.URL.Path == "/api/v1/repos/testuser/testrepo/contents/.pages" {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewForgejoClient(server.URL, "test-token")
	hasPages, err := client.HasPagesFile(context.Background(), "testuser", "testrepo")

	if err != nil {
		t.Fatalf("HasPagesFile failed: %v", err)
	}

	if !hasPages {
		t.Error("Expected hasPages to be true")
	}
}

// TestHasPagesFileNotFound tests the HasPagesFile method when .pages doesn't exist.
func TestHasPagesFileNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/repos/testuser/testrepo" {
			repo := RepositoryInfo{
				Name:          "testrepo",
				FullName:      "testuser/testrepo",
				Private:       false,
				DefaultBranch: "main",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(repo)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewForgejoClient(server.URL, "test-token")
	hasPages, err := client.HasPagesFile(context.Background(), "testuser", "testrepo")

	if err != nil {
		t.Fatalf("HasPagesFile failed: %v", err)
	}

	if hasPages {
		t.Error("Expected hasPages to be false")
	}
}

// TestHasPagesFilePrivateRepo tests the HasPagesFile method with a private repository.
func TestHasPagesFilePrivateRepo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repo := RepositoryInfo{
			Name:          "testrepo",
			FullName:      "testuser/testrepo",
			Private:       true,
			DefaultBranch: "main",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repo)
	}))
	defer server.Close()

	// Client without token
	client := NewForgejoClient(server.URL, "")
	_, err := client.HasPagesFile(context.Background(), "testuser", "testrepo")

	if err == nil {
		t.Fatal("Expected error for private repository without token")
	}

	if err.Error() != "repository is private" {
		t.Errorf("Expected 'repository is private' error, got %q", err.Error())
	}
}

// TestGetFileContent tests the GetFileContent method.
func TestGetFileContent(t *testing.T) {
	testContent := "Hello, World!"
	encodedContent := base64Encode([]byte(testContent))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/repos/testuser/testrepo" {
			repo := RepositoryInfo{
				Name:          "testrepo",
				FullName:      "testuser/testrepo",
				Private:       false,
				DefaultBranch: "main",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(repo)
			return
		}

		if r.URL.Path == "/api/v1/repos/testuser/testrepo/contents/public/index.html" {
			fileContent := FileContent{
				Type:     "file",
				Encoding: "base64",
				Content:  encodedContent,
				Size:     len(testContent),
				Name:     "index.html",
				Path:     "public/index.html",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(fileContent)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewForgejoClient(server.URL, "test-token")
	content, contentType, err := client.GetFileContent(context.Background(), "testuser", "testrepo", "public/index.html")

	if err != nil {
		t.Fatalf("GetFileContent failed: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Expected content %q, got %q", testContent, string(content))
	}

	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Expected content type %q, got %q", "text/html; charset=utf-8", contentType)
	}
}

// TestGetFileContentNotFound tests the GetFileContent method with a non-existent file.
func TestGetFileContentNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/repos/testuser/testrepo" {
			repo := RepositoryInfo{
				Name:          "testrepo",
				FullName:      "testuser/testrepo",
				Private:       false,
				DefaultBranch: "main",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(repo)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewForgejoClient(server.URL, "test-token")
	_, _, err := client.GetFileContent(context.Background(), "testuser", "testrepo", "nonexistent.html")

	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}
}

// TestGetPagesConfig tests the GetPagesConfig method.
func TestGetPagesConfig(t *testing.T) {
	pagesContent := `enabled: true
custom_domain: example.com
`
	encodedContent := base64Encode([]byte(pagesContent))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/repos/testuser/testrepo" {
			repo := RepositoryInfo{
				Name:          "testrepo",
				FullName:      "testuser/testrepo",
				Private:       false,
				DefaultBranch: "main",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(repo)
			return
		}

		if r.URL.Path == "/api/v1/repos/testuser/testrepo/contents/.pages" {
			fileContent := FileContent{
				Type:     "file",
				Encoding: "base64",
				Content:  encodedContent,
				Size:     len(pagesContent),
				Name:     ".pages",
				Path:     ".pages",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(fileContent)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewForgejoClient(server.URL, "test-token")
	config, err := client.GetPagesConfig(context.Background(), "testuser", "testrepo")

	if err != nil {
		t.Fatalf("GetPagesConfig failed: %v", err)
	}

	if !config.Enabled {
		t.Error("Expected Enabled to be true")
	}

	if config.CustomDomain != "example.com" {
		t.Errorf("Expected CustomDomain %q, got %q", "example.com", config.CustomDomain)
	}
}

// TestBase64Decode tests the base64Decode function.
func TestBase64Decode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "simple text",
			input: "SGVsbG8sIFdvcmxkIQ==",
			want:  "Hello, World!",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "padding",
			input: "YQ==",
			want:  "a",
		},
		{
			name:  "no padding",
			input: "YWJj",
			want:  "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := decodeBase64Content(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if string(decoded) != tt.want {
				t.Errorf("Expected %q, got %q", tt.want, string(decoded))
			}
		})
	}
}

// TestParseInlineArray tests the parseInlineArray function for enable_branches parsing.
func TestParseInlineArray(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty brackets",
			input:    "[]",
			expected: []string{},
		},
		{
			name:     "single item",
			input:    "[stage]",
			expected: []string{"stage"},
		},
		{
			name:     "multiple items",
			input:    "[stage, qa]",
			expected: []string{"stage", "qa"},
		},
		{
			name:     "quoted items",
			input:    `["stage", "qa"]`,
			expected: []string{"stage", "qa"},
		},
		{
			name:     "single quoted items",
			input:    "['stage', 'qa']",
			expected: []string{"stage", "qa"},
		},
		{
			name:     "mixed quotes",
			input:    `["stage", 'qa', dev]`,
			expected: []string{"stage", "qa", "dev"},
		},
		{
			name:     "with spaces",
			input:    "[  stage  ,  qa  ]",
			expected: []string{"stage", "qa"},
		},
		{
			name:     "branch with slash",
			input:    "[feature/new-ui, release/v1]",
			expected: []string{"feature/new-ui", "release/v1"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseInlineArray(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d items, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Item %d: expected %q, got %q", i, expected, result[i])
				}
			}
		})
	}
}

// TestGetPagesConfigWithEnableBranches tests parsing enable_branches from .pages file.
func TestGetPagesConfigWithEnableBranches(t *testing.T) {
	tests := []struct {
		name            string
		pagesContent    string
		expectedDomain  string
		expectedEnabled bool
		expectedBranches []string
	}{
		{
			name: "inline array format",
			pagesContent: `enabled: true
custom_domain: example.com
enable_branches: ["stage", "qa"]
`,
			expectedDomain:   "example.com",
			expectedEnabled:  true,
			expectedBranches: []string{"stage", "qa"},
		},
		{
			name: "multiline array format",
			pagesContent: `enabled: true
custom_domain: example.com
enable_branches:
  - stage
  - qa
  - dev
`,
			expectedDomain:   "example.com",
			expectedEnabled:  true,
			expectedBranches: []string{"stage", "qa", "dev"},
		},
		{
			name: "no enable_branches",
			pagesContent: `enabled: true
custom_domain: example.com
`,
			expectedDomain:   "example.com",
			expectedEnabled:  true,
			expectedBranches: []string{},
		},
		{
			name: "empty enable_branches",
			pagesContent: `enabled: true
enable_branches: []
`,
			expectedDomain:   "",
			expectedEnabled:  true,
			expectedBranches: []string{},
		},
		{
			name: "single branch inline",
			pagesContent: `enabled: true
custom_domain: bovine.example.com
enable_branches: [staging]
`,
			expectedDomain:   "bovine.example.com",
			expectedEnabled:  true,
			expectedBranches: []string{"staging"},
		},
		{
			name: "branch names with special characters",
			pagesContent: `enabled: true
custom_domain: example.com
enable_branches:
  - feature/new-ui
  - release_v1.0
  - hotfix-123
`,
			expectedDomain:   "example.com",
			expectedEnabled:  true,
			expectedBranches: []string{"feature/new-ui", "release_v1.0", "hotfix-123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encodedContent := base64Encode([]byte(tt.pagesContent))

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/repos/testuser/testrepo" {
					repo := RepositoryInfo{
						Name:          "testrepo",
						FullName:      "testuser/testrepo",
						Private:       false,
						DefaultBranch: "main",
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(repo)
					return
				}

				if r.URL.Path == "/api/v1/repos/testuser/testrepo/contents/.pages" {
					fileContent := FileContent{
						Type:     "file",
						Encoding: "base64",
						Content:  encodedContent,
						Size:     len(tt.pagesContent),
						Name:     ".pages",
						Path:     ".pages",
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(fileContent)
					return
				}

				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			client := NewForgejoClient(server.URL, "test-token")
			config, err := client.GetPagesConfig(context.Background(), "testuser", "testrepo")

			if err != nil {
				t.Fatalf("GetPagesConfig failed: %v", err)
			}

			if config.CustomDomain != tt.expectedDomain {
				t.Errorf("Expected CustomDomain %q, got %q", tt.expectedDomain, config.CustomDomain)
			}

			if config.Enabled != tt.expectedEnabled {
				t.Errorf("Expected Enabled %v, got %v", tt.expectedEnabled, config.Enabled)
			}

			if len(config.EnableBranches) != len(tt.expectedBranches) {
				t.Errorf("Expected %d branches, got %d: %v", len(tt.expectedBranches), len(config.EnableBranches), config.EnableBranches)
				return
			}

			for i, expected := range tt.expectedBranches {
				if config.EnableBranches[i] != expected {
					t.Errorf("Branch %d: expected %q, got %q", i, expected, config.EnableBranches[i])
				}
			}
		})
	}
}

// TestGetBranch tests the GetBranch method.
func TestGetBranch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/repos/testuser/testrepo/branches/main" {
			branch := BranchInfo{
				Name: "main",
			}
			branch.Commit.ID = "abc123"
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(branch)
			return
		}

		if r.URL.Path == "/api/v1/repos/testuser/testrepo/branches/feature%2Fnew-ui" ||
			r.URL.Path == "/api/v1/repos/testuser/testrepo/branches/feature/new-ui" {
			branch := BranchInfo{
				Name: "feature/new-ui",
			}
			branch.Commit.ID = "def456"
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(branch)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewForgejoClient(server.URL, "test-token")

	// Test existing branch
	branch, err := client.GetBranch(context.Background(), "testuser", "testrepo", "main")
	if err != nil {
		t.Fatalf("GetBranch failed: %v", err)
	}
	if branch.Name != "main" {
		t.Errorf("Expected branch name %q, got %q", "main", branch.Name)
	}

	// Test non-existent branch
	_, err = client.GetBranch(context.Background(), "testuser", "testrepo", "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent branch, got nil")
	}
}

// TestGetFileContentFromBranch tests the GetFileContentFromBranch method.
func TestGetFileContentFromBranch(t *testing.T) {
	testContent := "Hello from staging branch!"
	encodedContent := base64Encode([]byte(testContent))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if ref parameter is present
		ref := r.URL.Query().Get("ref")

		if r.URL.Path == "/api/v1/repos/testuser/testrepo/contents/public/index.html" {
			fileContent := FileContent{
				Type:     "file",
				Encoding: "base64",
				Size:     len(testContent),
				Name:     "index.html",
				Path:     "public/index.html",
			}

			if ref == "staging" {
				fileContent.Content = encodedContent
			} else if ref == "" {
				fileContent.Content = base64Encode([]byte("Hello from default branch!"))
			} else {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(fileContent)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewForgejoClient(server.URL, "test-token")

	// Test with specific branch
	content, _, err := client.GetFileContentFromBranch(context.Background(), "testuser", "testrepo", "public/index.html", "staging")
	if err != nil {
		t.Fatalf("GetFileContentFromBranch failed: %v", err)
	}
	if string(content) != testContent {
		t.Errorf("Expected content %q, got %q", testContent, string(content))
	}

	// Test with empty branch (default branch)
	content, _, err = client.GetFileContentFromBranch(context.Background(), "testuser", "testrepo", "public/index.html", "")
	if err != nil {
		t.Fatalf("GetFileContentFromBranch with empty branch failed: %v", err)
	}
	if string(content) != "Hello from default branch!" {
		t.Errorf("Expected default branch content, got %q", string(content))
	}
}

// Helper function to encode base64 for tests.
func base64Encode(data []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var result []byte

	for i := 0; i < len(data); i += 3 {
		b0, b1, b2 := data[i], byte(0), byte(0)
		if i+1 < len(data) {
			b1 = data[i+1]
		}
		if i+2 < len(data) {
			b2 = data[i+2]
		}

		result = append(result, alphabet[(b0>>2)&0x3F])
		result = append(result, alphabet[((b0<<4)|(b1>>4))&0x3F])

		if i+1 < len(data) {
			result = append(result, alphabet[((b1<<2)|(b2>>6))&0x3F])
		} else {
			result = append(result, '=')
		}

		if i+2 < len(data) {
			result = append(result, alphabet[b2&0x3F])
		} else {
			result = append(result, '=')
		}
	}

	return string(result)
}
