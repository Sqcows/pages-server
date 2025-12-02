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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ForgejoClient handles communication with the Forgejo/Gitea API.
type ForgejoClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewForgejoClient creates a new Forgejo API client.
func NewForgejoClient(baseURL, token string) *ForgejoClient {
	return &ForgejoClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// RepositoryInfo contains information about a repository.
type RepositoryInfo struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
}

// FileContent represents a file in a repository.
type FileContent struct {
	Type     string `json:"type"`
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
	Size     int    `json:"size"`
	Name     string `json:"name"`
	Path     string `json:"path"`
}

// PagesConfig represents the configuration from .pages file.
type PagesConfig struct {
	CustomDomain   string `yaml:"custom_domain"`
	Enabled        bool   `yaml:"enabled"`
	Password       string `yaml:"password"`        // SHA256 hash of the password
	DirectoryIndex bool   `yaml:"directory_index"` // Enable directory listing for directories without index.html
}

// doRequest performs an HTTP request to the Forgejo API.
func (fc *ForgejoClient) doRequest(ctx context.Context, method, path string) (*http.Response, error) {
	url := fc.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header if token is provided
	if fc.token != "" {
		req.Header.Set("Authorization", "token "+fc.token)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := fc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// GetRepository retrieves repository information.
func (fc *ForgejoClient) GetRepository(ctx context.Context, owner, repo string) (*RepositoryInfo, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s", owner, repo)

	resp, err := fc.doRequest(ctx, "GET", path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("repository not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var repoInfo RepositoryInfo
	if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &repoInfo, nil
}

// HasPagesFile checks if a repository has a .pages file.
func (fc *ForgejoClient) HasPagesFile(ctx context.Context, owner, repo string) (bool, error) {
	// First check if repository exists and is public or accessible
	repoInfo, err := fc.GetRepository(ctx, owner, repo)
	if err != nil {
		return false, err
	}

	// If repository is private and we don't have a token, deny access
	if repoInfo.Private && fc.token == "" {
		return false, fmt.Errorf("repository is private")
	}

	// Check for .pages file
	path := fmt.Sprintf("/api/v1/repos/%s/%s/contents/.pages", owner, repo)

	resp, err := fc.doRequest(ctx, "GET", path)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	return true, nil
}

// GetFileContent retrieves the content of a file from a repository.
// Returns the file content, content type, and any error.
func (fc *ForgejoClient) GetFileContent(ctx context.Context, owner, repo, filePath string) ([]byte, string, error) {
	// Get repository info to determine default branch
	repoInfo, err := fc.GetRepository(ctx, owner, repo)
	if err != nil {
		return nil, "", err
	}

	// Construct API path for file contents
	path := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", owner, repo, filePath)

	resp, err := fc.doRequest(ctx, "GET", path)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", fmt.Errorf("file not found: %s", filePath)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var fileContent FileContent
	if err := json.NewDecoder(resp.Body).Decode(&fileContent); err != nil {
		return nil, "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Check if it's a file (not a directory)
	if fileContent.Type != "file" {
		return nil, "", fmt.Errorf("path is not a file: %s", filePath)
	}

	// Decode base64 content
	content, err := decodeBase64Content(fileContent.Content)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode file content: %w", err)
	}

	// Determine content type from file extension
	contentType := detectContentType(filePath, content)

	_ = repoInfo // Use repoInfo to avoid unused variable error

	return content, contentType, nil
}

// GetPagesConfig retrieves and parses the .pages configuration file.
func (fc *ForgejoClient) GetPagesConfig(ctx context.Context, owner, repo string) (*PagesConfig, error) {
	content, _, err := fc.GetFileContent(ctx, owner, repo, ".pages")
	if err != nil {
		return nil, err
	}

	// Parse YAML content (simple parsing for now)
	config := &PagesConfig{
		Enabled: true, // Default to enabled if .pages file exists
	}

	// Simple YAML parsing for custom_domain, enabled, and password fields
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "custom_domain:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				config.CustomDomain = strings.TrimSpace(parts[1])
				// Remove quotes if present
				config.CustomDomain = strings.Trim(config.CustomDomain, "\"'")
			}
		}
		if strings.HasPrefix(line, "enabled:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				enabled := strings.TrimSpace(parts[1])
				config.Enabled = enabled == "true" || enabled == "yes"
			}
		}
		if strings.HasPrefix(line, "password:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				config.Password = strings.TrimSpace(parts[1])
				// Remove quotes if present
				config.Password = strings.Trim(config.Password, "\"'")
			}
		}
		if strings.HasPrefix(line, "directory_index:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				directoryIndex := strings.TrimSpace(parts[1])
				config.DirectoryIndex = directoryIndex == "true" || directoryIndex == "yes"
			}
		}
	}

	return config, nil
}

// DirectoryEntry represents a file or directory in a repository.
type DirectoryEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Type  string `json:"type"` // "file" or "dir"
	Size  int64  `json:"size"`
	IsDir bool
}

// ListDirectory lists the contents of a directory in a repository.
func (fc *ForgejoClient) ListDirectory(ctx context.Context, owner, repo, dirPath string) ([]DirectoryEntry, error) {
	// Get repository info to determine default branch
	repoInfo, err := fc.GetRepository(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	// Construct API path for directory contents
	path := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", owner, repo, dirPath)

	resp, err := fc.doRequest(ctx, "GET", path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("directory not found: %s", dirPath)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	// Decode the JSON response - API returns array of FileContent
	var contents []FileContent
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to DirectoryEntry format
	entries := make([]DirectoryEntry, 0, len(contents))
	for _, item := range contents {
		entry := DirectoryEntry{
			Name:  item.Name,
			Path:  item.Path,
			Type:  item.Type,
			Size:  int64(item.Size),
			IsDir: item.Type == "dir",
		}
		entries = append(entries, entry)
	}

	_ = repoInfo // Use repoInfo to avoid unused variable error

	return entries, nil
}

// decodeBase64Content decodes base64-encoded content from Forgejo API.
func decodeBase64Content(encoded string) ([]byte, error) {
	// Remove whitespace and newlines that might be in the Forgejo API response
	encoded = strings.ReplaceAll(encoded, "\n", "")
	encoded = strings.ReplaceAll(encoded, "\r", "")
	encoded = strings.ReplaceAll(encoded, " ", "")

	// Use Go's standard library base64 decoder
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	return decoded, nil
}

