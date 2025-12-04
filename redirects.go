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
	"net/http"
	"strings"
)

// RedirectRule represents a single redirect rule from .redirects file.
type RedirectRule struct {
	From string // Source URL path
	To   string // Destination URL path
}

// parseRedirectsFile parses the .redirects file content into redirect rules.
// Format: URL:REDIRECT_URL (one per line)
// Lines starting with # are comments and are ignored.
// Empty lines are ignored.
// Returns up to maxRedirects rules.
func parseRedirectsFile(content []byte, maxRedirects int) ([]RedirectRule, error) {
	if maxRedirects <= 0 {
		return nil, fmt.Errorf("maxRedirects must be positive, got %d", maxRedirects)
	}

	lines := strings.Split(string(content), "\n")
	rules := make([]RedirectRule, 0, maxRedirects)

	for i, line := range lines {
		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check if we've reached the limit
		if len(rules) >= maxRedirects {
			return rules, nil
		}

		// Parse redirect rule: FROM:TO
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid redirect format on line %d: %s (expected FROM:TO)", i+1, line)
		}

		from := strings.TrimSpace(parts[0])
		to := strings.TrimSpace(parts[1])

		// Validate that from and to are not empty
		if from == "" {
			return nil, fmt.Errorf("empty source URL on line %d", i+1)
		}
		if to == "" {
			return nil, fmt.Errorf("empty destination URL on line %d", i+1)
		}

		rules = append(rules, RedirectRule{
			From: from,
			To:   to,
		})
	}

	return rules, nil
}

// generateTraefikRedirectRegexMiddleware generates Traefik redirectregex middleware configuration
// from redirect rules.
// Returns a map of Redis keys to values for storing in Redis.
// Note: Traefik's redirectregex middleware only supports ONE redirect per middleware instance.
// For multiple redirects, we create multiple middleware instances.
func generateTraefikRedirectRegexMiddleware(customDomain string, rules []RedirectRule, rootKey string) map[string]string {
	if len(rules) == 0 {
		return nil
	}

	configs := make(map[string]string)
	domainSanitized := strings.ReplaceAll(customDomain, ".", "-")

	// Generate redirectregex middleware configuration
	// See: https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/redirectregex/
	for i, rule := range rules {
		// Each redirect rule gets its own middleware instance
		// Format: traefik/http/middlewares/{name}/redirectregex/{property}
		middlewareName := fmt.Sprintf("redirects-%s-%d", domainSanitized, i)

		regexKey := fmt.Sprintf("%s/http/middlewares/%s/redirectregex/regex", rootKey, middlewareName)
		replacementKey := fmt.Sprintf("%s/http/middlewares/%s/redirectregex/replacement", rootKey, middlewareName)
		permanentKey := fmt.Sprintf("%s/http/middlewares/%s/redirectregex/permanent", rootKey, middlewareName)

		// Build regex pattern for matching the "from" URL
		// Escape special regex characters and match full path
		regexPattern := fmt.Sprintf("^/%s$", escapeRegex(rule.From))

		// Build replacement URL (with leading slash if not present)
		replacement := rule.To
		if !strings.HasPrefix(replacement, "/") && !strings.HasPrefix(replacement, "http://") && !strings.HasPrefix(replacement, "https://") {
			replacement = "/" + replacement
		}

		configs[regexKey] = regexPattern
		configs[replacementKey] = replacement
		configs[permanentKey] = "true" // 301 permanent redirect
	}

	return configs
}

// escapeRegex escapes special regex characters for use in redirectregex patterns.
func escapeRegex(s string) string {
	// Escape backslash FIRST to avoid double-escaping
	result := strings.ReplaceAll(s, "\\", "\\\\")

	// Then escape other regex special characters
	special := []string{".", "+", "*", "?", "^", "$", "(", ")", "[", "]", "{", "}", "|"}
	for _, char := range special {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}
	return result
}

// storeRedirectMiddleware stores redirect middleware configuration in Redis.
func (ps *PagesServer) storeRedirectMiddleware(customDomain string, rules []RedirectRule) error {
	// Only write if we have a working Redis cache
	redisCache, ok := ps.customDomainCache.(*RedisCache)
	if !ok {
		// Using in-memory cache, skip middleware storage
		return fmt.Errorf("Redis cache required for redirect middleware storage")
	}

	// Generate Traefik middleware configuration
	configs := generateTraefikRedirectRegexMiddleware(customDomain, rules, ps.config.TraefikRedisRootKey)
	if configs == nil || len(configs) == 0 {
		return fmt.Errorf("no redirect rules to store")
	}

	// Write each config key to Redis with persistent storage (TTL=0)
	for key, value := range configs {
		if err := redisCache.SetWithTTL(key, []byte(value), 0); err != nil {
			return fmt.Errorf("failed to write redirect middleware config key %s: %w", key, err)
		}
	}

	// Store metadata about redirect count for this domain
	metadataKey := fmt.Sprintf("redirects_meta:%s", customDomain)
	metadataValue := fmt.Sprintf("count=%d", len(rules))
	if err := redisCache.SetWithTTL(metadataKey, []byte(metadataValue), 0); err != nil {
		return fmt.Errorf("failed to write redirect metadata: %w", err)
	}

	// Update the router to include all redirect middleware instances
	if err := ps.updateRouterMiddlewares(customDomain, len(rules)); err != nil {
		return fmt.Errorf("failed to update router middlewares: %w", err)
	}

	return nil
}

// updateRouterMiddlewares updates the Traefik router configuration to include all redirect middlewares.
// The redirect middlewares must be first in the chain so redirects are processed before pages-server.
func (ps *PagesServer) updateRouterMiddlewares(customDomain string, redirectCount int) error {
	// Only write if we have a working Redis cache
	redisCache, ok := ps.customDomainCache.(*RedisCache)
	if !ok {
		return fmt.Errorf("Redis cache required for router middleware update")
	}

	// Create sanitized router name (same as in registerTraefikRouter)
	routerName := "custom-" + strings.ReplaceAll(customDomain, ".", "-")
	rootKey := ps.config.TraefikRedisRootKey
	domainSanitized := strings.ReplaceAll(customDomain, ".", "-")

	middlewareConfigs := make(map[string]string)
	middlewareIndex := 0

	// Add all redirect middlewares first (they process in order)
	for i := 0; i < redirectCount; i++ {
		middlewareName := fmt.Sprintf("redirects-%s-%d", domainSanitized, i)
		// Format: middlewareName@redis (since middleware is stored in Redis)
		middlewareRef := middlewareName + "@redis"
		key := fmt.Sprintf("%s/http/routers/%s/middlewares/%d", rootKey, routerName, middlewareIndex)
		middlewareConfigs[key] = middlewareRef
		middlewareIndex++
	}

	// Add pages-server middleware last
	pagesMiddlewareRef := "pages-server@file"
	key := fmt.Sprintf("%s/http/routers/%s/middlewares/%d", rootKey, routerName, middlewareIndex)
	middlewareConfigs[key] = pagesMiddlewareRef

	// Write each middleware config key with same TTL as router
	for key, value := range middlewareConfigs {
		if err := redisCache.SetWithTTL(key, []byte(value), ps.config.TraefikRedisRouterTTL); err != nil {
			return fmt.Errorf("failed to write router middleware config key %s: %w", key, err)
		}
	}

	return nil
}

// handleLoadRedirects handles the /LOAD_REDIRECTS endpoint for custom domains.
// This endpoint reads the .redirects file from the repository and creates Traefik middleware.
func (ps *PagesServer) handleLoadRedirects(rw http.ResponseWriter, req *http.Request) {
	// Determine if this is a custom domain request
	host := req.Host
	if colonIdx := strings.Index(host, ":"); colonIdx != -1 {
		host = host[:colonIdx]
	}

	// Check if this is a pagesDomain request (not allowed for redirects)
	if strings.HasSuffix(host, ps.config.PagesDomain) {
		ps.serveError(rw, http.StatusBadRequest, "Redirects are only supported on custom domains. Please visit your custom domain to load redirects.")
		return
	}

	// Resolve custom domain to repository
	username, repository, err := ps.resolveCustomDomain(req.Context(), host)
	if err != nil {
		ps.serveError(rw, http.StatusNotFound, "Custom domain not configured. Please visit your pages URL to register your custom domain first.")
		return
	}

	// Verify repository has .pages file with custom domain
	pagesConfig, err := ps.forgejoClient.GetPagesConfig(req.Context(), username, repository)
	if err != nil {
		ps.serveError(rw, http.StatusNotFound, "Repository not configured for pages. Please ensure your repository has a .pages file.")
		return
	}

	if pagesConfig.CustomDomain == "" {
		ps.serveError(rw, http.StatusBadRequest, "Custom domain not configured in .pages file. Please add 'custom_domain: yourdomain.com' to your .pages file.")
		return
	}

	// Get the .redirects file from repository
	redirectsContent, _, err := ps.forgejoClient.GetFileContent(req.Context(), username, repository, ".redirects")
	if err != nil {
		// Provide helpful error message with link to documentation
		errorHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Redirects File Not Found</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }
        .container {
            background: white;
            border-radius: 12px;
            box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
            padding: 40px;
            max-width: 600px;
            width: 100%%;
        }
        h1 {
            color: #333;
            font-size: 24px;
            margin-bottom: 20px;
        }
        p {
            color: #666;
            font-size: 16px;
            line-height: 1.6;
            margin-bottom: 15px;
        }
        .code {
            background: #f5f5f5;
            border-radius: 6px;
            padding: 15px;
            font-family: monospace;
            font-size: 14px;
            margin: 20px 0;
            overflow-x: auto;
        }
        .error-details {
            background: #fff3cd;
            border: 1px solid #ffc107;
            border-radius: 6px;
            padding: 15px;
            margin: 20px 0;
            color: #856404;
        }
        a {
            color: #667eea;
            text-decoration: none;
        }
        a:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Redirects File Not Found</h1>
        <p>The <code>.redirects</code> file was not found in your repository <strong>%s/%s</strong>.</p>

        <div class="error-details">
            <strong>Error:</strong> %s
        </div>

        <p>To use redirects, create a <code>.redirects</code> file in the root of your repository with the following format:</p>

        <div class="code">
# Redirects file format
# FROM:TO (one per line)

old-page:new-page
index.html:home/
blog/old-post:blog/new-post
        </div>

        <p>For more information, please see the <a href="https://code.squarecows.com/SquareCows/pages-server/wiki" target="_blank">Bovine Pages Server documentation</a>.</p>
    </div>
</body>
</html>`, username, repository, err.Error())

		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		rw.Header().Set("Server", "bovine")
		rw.WriteHeader(http.StatusNotFound)
		rw.Write([]byte(errorHTML))
		return
	}

	// Parse redirects file
	rules, err := parseRedirectsFile(redirectsContent, ps.config.MaxRedirects)
	if err != nil {
		ps.serveError(rw, http.StatusBadRequest, fmt.Sprintf("Invalid .redirects file: %s", err.Error()))
		return
	}

	// Store redirect middleware in Redis
	if err := ps.storeRedirectMiddleware(pagesConfig.CustomDomain, rules); err != nil {
		ps.serveError(rw, http.StatusInternalServerError, fmt.Sprintf("Failed to store redirect middleware: %s", err.Error()))
		return
	}

	// Success response
	successHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Redirects Loaded Successfully</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }
        .container {
            background: white;
            border-radius: 12px;
            box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
            padding: 40px;
            max-width: 600px;
            width: 100%%;
        }
        h1 {
            color: #28a745;
            font-size: 24px;
            margin-bottom: 20px;
        }
        p {
            color: #666;
            font-size: 16px;
            line-height: 1.6;
            margin-bottom: 15px;
        }
        .success-icon {
            font-size: 48px;
            text-align: center;
            margin-bottom: 20px;
        }
        .details {
            background: #f5f5f5;
            border-radius: 6px;
            padding: 15px;
            margin: 20px 0;
        }
        .details strong {
            color: #333;
        }
        ul {
            margin: 15px 0;
            padding-left: 20px;
        }
        li {
            color: #666;
            margin: 5px 0;
        }
        .note {
            background: #d1ecf1;
            border: 1px solid #bee5eb;
            border-radius: 6px;
            padding: 15px;
            margin: 20px 0;
            color: #0c5460;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="success-icon">✓</div>
        <h1>Redirects Loaded Successfully</h1>

        <div class="details">
            <p><strong>Domain:</strong> %s</p>
            <p><strong>Repository:</strong> %s/%s</p>
            <p><strong>Redirects Loaded:</strong> %d</p>
        </div>

        <p>Your redirect rules have been successfully loaded and configured in Traefik. The following redirects are now active:</p>

        <ul>
%s
        </ul>

        <div class="note">
            <strong>Note:</strong> Redirects may take a few seconds to become active as Traefik updates its configuration from Redis.
        </div>
    </div>
</body>
</html>`, pagesConfig.CustomDomain, username, repository, len(rules), formatRedirectList(rules))

	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.Header().Set("Server", "bovine")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte(successHTML))
}

// formatRedirectList formats redirect rules as HTML list items.
func formatRedirectList(rules []RedirectRule) string {
	var sb strings.Builder
	for _, rule := range rules {
		sb.WriteString(fmt.Sprintf("            <li><code>/%s</code> → <code>%s</code></li>\n", rule.From, rule.To))
	}
	return sb.String()
}
