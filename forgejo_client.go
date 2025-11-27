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
	CustomDomain string `yaml:"custom_domain"`
	Enabled      bool   `yaml:"enabled"`
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

	// Simple YAML parsing for custom_domain field
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
	}

	return config, nil
}

// decodeBase64Content decodes base64-encoded content from Forgejo API.
func decodeBase64Content(encoded string) ([]byte, error) {
	// Remove whitespace and newlines
	encoded = strings.ReplaceAll(encoded, "\n", "")
	encoded = strings.ReplaceAll(encoded, "\r", "")
	encoded = strings.ReplaceAll(encoded, " ", "")

	// Decode base64
	decoded := make([]byte, base64DecodedLen(len(encoded)))
	n, err := base64Decode(decoded, []byte(encoded))
	if err != nil {
		return nil, err
	}

	return decoded[:n], nil
}

// base64Decode decodes base64 data using standard encoding.
// This is a simplified implementation suitable for Yaegi.
func base64Decode(dst, src []byte) (int, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

	// Build reverse lookup table
	var decode [256]byte
	for i := range decode {
		decode[i] = 0xFF
	}
	for i := 0; i < len(alphabet); i++ {
		decode[alphabet[i]] = byte(i)
	}
	decode['='] = 0

	srcLen := len(src)
	dstLen := 0

	for i := 0; i < srcLen; i += 4 {
		// Get 4 characters
		b0, b1, b2, b3 := byte(0xFF), byte(0xFF), byte(0xFF), byte(0xFF)
		if i < srcLen {
			b0 = decode[src[i]]
		}
		if i+1 < srcLen {
			b1 = decode[src[i+1]]
		}
		if i+2 < srcLen {
			b2 = decode[src[i+2]]
		}
		if i+3 < srcLen {
			b3 = decode[src[i+3]]
		}

		if b0 == 0xFF || b1 == 0xFF {
			return 0, fmt.Errorf("invalid base64 data")
		}

		// Decode to 3 bytes
		dst[dstLen] = (b0 << 2) | (b1 >> 4)
		dstLen++

		if b2 != 0xFF && src[i+2] != '=' {
			dst[dstLen] = (b1 << 4) | (b2 >> 2)
			dstLen++
		}

		if b3 != 0xFF && src[i+3] != '=' {
			dst[dstLen] = (b2 << 6) | b3
			dstLen++
		}
	}

	return dstLen, nil
}

// base64DecodedLen returns the maximum length of decoded data.
func base64DecodedLen(n int) int {
	return (n * 3) / 4
}
