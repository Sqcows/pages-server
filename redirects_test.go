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
	"fmt"
	"strings"
	"testing"
)

// TestParseRedirectsFile tests parsing of .redirects file content.
func TestParseRedirectsFile(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		maxRedirects  int
		expectedRules []RedirectRule
		expectError   bool
		errorContains string
	}{
		{
			name: "valid redirects",
			content: `moo:bar
index.html:myapp/`,
			maxRedirects: 25,
			expectedRules: []RedirectRule{
				{From: "moo", To: "bar"},
				{From: "index.html", To: "myapp/"},
			},
			expectError: false,
		},
		{
			name: "redirects with comments",
			content: `# This is a comment
old-page:new-page
# Another comment
blog/old:blog/new`,
			maxRedirects: 25,
			expectedRules: []RedirectRule{
				{From: "old-page", To: "new-page"},
				{From: "blog/old", To: "blog/new"},
			},
			expectError: false,
		},
		{
			name: "redirects with empty lines",
			content: `page1:page2

page3:page4

`,
			maxRedirects: 25,
			expectedRules: []RedirectRule{
				{From: "page1", To: "page2"},
				{From: "page3", To: "page4"},
			},
			expectError: false,
		},
		{
			name: "redirects with whitespace",
			content: `  page1  :  page2
	page3	:	page4	`,
			maxRedirects: 25,
			expectedRules: []RedirectRule{
				{From: "page1", To: "page2"},
				{From: "page3", To: "page4"},
			},
			expectError: false,
		},
		{
			name: "max redirects limit",
			content: `page1:page2
page3:page4
page5:page6`,
			maxRedirects: 2,
			expectedRules: []RedirectRule{
				{From: "page1", To: "page2"},
				{From: "page3", To: "page4"},
			},
			expectError: false,
		},
		{
			name:          "invalid format - missing colon",
			content:       `page1 page2`,
			maxRedirects:  25,
			expectError:   true,
			errorContains: "invalid redirect format",
		},
		{
			name:          "invalid format - empty source",
			content:       `:page2`,
			maxRedirects:  25,
			expectError:   true,
			errorContains: "empty source URL",
		},
		{
			name:          "invalid format - empty destination",
			content:       `page1:`,
			maxRedirects:  25,
			expectError:   true,
			errorContains: "empty destination URL",
		},
		{
			name:          "invalid maxRedirects",
			content:       `page1:page2`,
			maxRedirects:  0,
			expectError:   true,
			errorContains: "maxRedirects must be positive",
		},
		{
			name:          "empty content",
			content:       ``,
			maxRedirects:  25,
			expectedRules: []RedirectRule{},
			expectError:   false,
		},
		{
			name: "only comments and empty lines",
			content: `# Comment 1
# Comment 2

# Comment 3`,
			maxRedirects:  25,
			expectedRules: []RedirectRule{},
			expectError:   false,
		},
		{
			name: "colons in destination URL",
			content: `old:https://example.com/new
page:http://example.com:8080/path`,
			maxRedirects: 25,
			expectedRules: []RedirectRule{
				{From: "old", To: "https://example.com/new"},
				{From: "page", To: "http://example.com:8080/path"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := parseRedirectsFile([]byte(tt.content), tt.maxRedirects)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorContains)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(rules) != len(tt.expectedRules) {
				t.Fatalf("Expected %d rules, got %d", len(tt.expectedRules), len(rules))
			}

			for i, expected := range tt.expectedRules {
				if rules[i].From != expected.From {
					t.Errorf("Rule %d: expected From='%s', got '%s'", i, expected.From, rules[i].From)
				}
				if rules[i].To != expected.To {
					t.Errorf("Rule %d: expected To='%s', got '%s'", i, expected.To, rules[i].To)
				}
			}
		})
	}
}

// TestEscapeRegex tests the escapeRegex function.
func TestEscapeRegex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special characters",
			input:    "simple",
			expected: "simple",
		},
		{
			name:     "dot character",
			input:    "file.html",
			expected: `file\.html`,
		},
		{
			name:     "multiple special characters",
			input:    "file.html?query=1&page=2",
			expected: `file\.html\?query=1&page=2`,
		},
		{
			name:     "regex metacharacters",
			input:    "test+page*[0-9]",
			expected: `test\+page\*\[0-9\]`,
		},
		{
			name:     "all special characters",
			input:    ".+*?^$()[]{}|\\",
			expected: `\.\+\*\?\^\$\(\)\[\]\{\}\|\\`,
		},
		{
			name:     "path with slashes",
			input:    "blog/2024/post",
			expected: "blog/2024/post",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeRegex(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestGenerateTraefikRedirectRegexMiddleware tests middleware configuration generation.
func TestGenerateTraefikRedirectRegexMiddleware(t *testing.T) {
	tests := []struct {
		name         string
		customDomain string
		rules        []RedirectRule
		rootKey      string
		expectNil    bool
		checkKeys    []string
		checkValues  map[string]string
	}{
		{
			name:         "single redirect",
			customDomain: "example.com",
			rules: []RedirectRule{
				{From: "old", To: "new"},
			},
			rootKey:   "traefik",
			expectNil: false,
			checkKeys: []string{
				"traefik/http/middlewares/redirects-example-com-0/redirectRegex/regex",
				"traefik/http/middlewares/redirects-example-com-0/redirectRegex/replacement",
				"traefik/http/middlewares/redirects-example-com-0/redirectRegex/permanent",
			},
			checkValues: map[string]string{
				"traefik/http/middlewares/redirects-example-com-0/redirectRegex/regex":       "^/old$",
				"traefik/http/middlewares/redirects-example-com-0/redirectRegex/replacement": "/new",
				"traefik/http/middlewares/redirects-example-com-0/redirectRegex/permanent":   "true",
			},
		},
		{
			name:         "multiple redirects",
			customDomain: "test.example.com",
			rules: []RedirectRule{
				{From: "page1", To: "page2"},
				{From: "blog/old", To: "blog/new"},
			},
			rootKey:   "traefik",
			expectNil: false,
			checkKeys: []string{
				"traefik/http/middlewares/redirects-test-example-com-0/redirectRegex/regex",
				"traefik/http/middlewares/redirects-test-example-com-1/redirectRegex/regex",
			},
			checkValues: map[string]string{
				"traefik/http/middlewares/redirects-test-example-com-0/redirectRegex/regex":       "^/page1$",
				"traefik/http/middlewares/redirects-test-example-com-0/redirectRegex/replacement": "/page2",
				"traefik/http/middlewares/redirects-test-example-com-1/redirectRegex/regex":       "^/blog/old$",
				"traefik/http/middlewares/redirects-test-example-com-1/redirectRegex/replacement": "/blog/new",
			},
		},
		{
			name:         "absolute URL redirect",
			customDomain: "example.com",
			rules: []RedirectRule{
				{From: "old", To: "https://newdomain.com/new"},
			},
			rootKey:   "traefik",
			expectNil: false,
			checkValues: map[string]string{
				"traefik/http/middlewares/redirects-example-com-0/redirectRegex/replacement": "https://newdomain.com/new",
			},
		},
		{
			name:         "redirect with trailing slash",
			customDomain: "example.com",
			rules: []RedirectRule{
				{From: "old", To: "new/"},
			},
			rootKey:   "traefik",
			expectNil: false,
			checkValues: map[string]string{
				"traefik/http/middlewares/redirects-example-com-0/redirectRegex/replacement": "/new/",
			},
		},
		{
			name:         "no redirects",
			customDomain: "example.com",
			rules:        []RedirectRule{},
			rootKey:      "traefik",
			expectNil:    true,
		},
		{
			name:         "special characters in path",
			customDomain: "example.com",
			rules: []RedirectRule{
				{From: "file.html", To: "newfile.html"},
			},
			rootKey:   "traefik",
			expectNil: false,
			checkValues: map[string]string{
				"traefik/http/middlewares/redirects-example-com-0/redirectRegex/regex": `^/file\.html$`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configs := generateTraefikRedirectRegexMiddleware(tt.customDomain, tt.rules, tt.rootKey)

			if tt.expectNil {
				if configs != nil && len(configs) > 0 {
					t.Errorf("Expected nil or empty configs, got %d keys", len(configs))
				}
				return
			}

			if configs == nil {
				t.Fatal("Expected configs, got nil")
			}

			// Check that all expected keys exist
			for _, key := range tt.checkKeys {
				if _, exists := configs[key]; !exists {
					t.Errorf("Expected key '%s' not found in configs", key)
				}
			}

			// Check specific values
			for key, expectedValue := range tt.checkValues {
				if actualValue, exists := configs[key]; !exists {
					t.Errorf("Expected key '%s' not found in configs", key)
				} else if actualValue != expectedValue {
					t.Errorf("Key '%s': expected value '%s', got '%s'", key, expectedValue, actualValue)
				}
			}

			// Verify all rules have regex, replacement, and permanent keys
			// Each rule gets its own middleware instance
			for i := range tt.rules {
				domainSanitized := strings.ReplaceAll(tt.customDomain, ".", "-")
				middlewareName := fmt.Sprintf("redirects-%s-%d", domainSanitized, i)
				regexKey := tt.rootKey + "/http/middlewares/" + middlewareName + "/redirectRegex/regex"
				replacementKey := tt.rootKey + "/http/middlewares/" + middlewareName + "/redirectRegex/replacement"
				permanentKey := tt.rootKey + "/http/middlewares/" + middlewareName + "/redirectRegex/permanent"

				if _, exists := configs[regexKey]; !exists {
					t.Errorf("Missing regex key for rule %d: %s", i, regexKey)
				}
				if _, exists := configs[replacementKey]; !exists {
					t.Errorf("Missing replacement key for rule %d: %s", i, replacementKey)
				}
				if value, exists := configs[permanentKey]; !exists {
					t.Errorf("Missing permanent key for rule %d: %s", i, permanentKey)
				} else if value != "true" {
					t.Errorf("Permanent key for rule %d should be 'true', got '%s'", i, value)
				}
			}
		})
	}
}

// TestFormatRedirectList tests HTML list formatting.
func TestFormatRedirectList(t *testing.T) {
	tests := []struct {
		name     string
		rules    []RedirectRule
		expected string
	}{
		{
			name: "single redirect",
			rules: []RedirectRule{
				{From: "old", To: "new"},
			},
			expected: "            <li><code>/old</code> → <code>new</code></li>\n",
		},
		{
			name: "multiple redirects",
			rules: []RedirectRule{
				{From: "page1", To: "page2"},
				{From: "blog/old", To: "blog/new"},
			},
			expected: "            <li><code>/page1</code> → <code>page2</code></li>\n            <li><code>/blog/old</code> → <code>blog/new</code></li>\n",
		},
		{
			name:     "empty rules",
			rules:    []RedirectRule{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRedirectList(tt.rules)
			if result != tt.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, result)
			}
		})
	}
}
