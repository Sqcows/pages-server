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
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// This file contains regression tests for the vulnerabilities reported in
// Vulnerability_Report_Sqcows_pages-server.pdf:
//   BUG A: Authentication Bypass (missing AuthSecretKey)      - CWE-287
//   BUG B: Stored XSS via .redirects file                      - CWE-79
//   BUG C: HTML injection via username/repository/custom_domain - CWE-79

// --- BUG A: Authentication Bypass -------------------------------------------

// TestBugA_AuthBypassFailsClosedWithoutSecretKey reproduces the report's PoC and
// asserts the fix: with no AuthSecretKey, an arbitrary non-empty cookie must NOT
// authenticate. Previously verification fell back to "cookieValue != """, letting
// anyone bypass password protection.
func TestBugA_AuthBypassFailsClosedWithoutSecretKey(t *testing.T) {
	ps := &PagesServer{config: &Config{AuthSecretKey: "", AuthCookieDuration: 3600}}

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "pages_auth_alice_private-repo", Value: "x"})
	if ps.isAuthenticated(req, "alice", "private-repo") {
		t.Error("SECURITY: isAuthenticated returned true with arbitrary cookie and no AuthSecretKey (auth bypass)")
	}

	breq := httptest.NewRequest("GET", "/", nil)
	breq.AddCookie(&http.Cookie{Name: "pages_branch_auth_alice_private-repo", Value: "x"})
	if ps.isBranchAuthenticated(breq, "alice", "private-repo") {
		t.Error("SECURITY: isBranchAuthenticated returned true with arbitrary cookie and no AuthSecretKey (auth bypass)")
	}
}

// TestBugA_NewGeneratesSecretKeyWhenMissing verifies New() makes the plugin secure
// by default by auto-generating a strong random signing key when none is configured.
func TestBugA_NewGeneratesSecretKeyWhenMissing(t *testing.T) {
	cfg := &Config{
		PagesDomain: "pages.example.com",
		ForgejoHost: "http://localhost:3000",
		// AuthSecretKey deliberately left empty
	}

	h, err := New(context.Background(), nil, cfg, "test")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if h == nil {
		t.Fatal("New returned nil handler")
	}
	if cfg.AuthSecretKey == "" {
		t.Fatal("SECURITY: New did not generate an AuthSecretKey when none was configured")
	}
	if len(cfg.AuthSecretKey) != 64 {
		t.Errorf("expected 64-char hex key, got %d chars", len(cfg.AuthSecretKey))
	}
	if _, err := hex.DecodeString(cfg.AuthSecretKey); err != nil {
		t.Errorf("generated key is not valid hex: %v", err)
	}
}

// TestBugA_NewPreservesConfiguredSecretKey verifies an explicitly configured key is
// never overwritten by the auto-generation logic.
func TestBugA_NewPreservesConfiguredSecretKey(t *testing.T) {
	cfg := &Config{
		PagesDomain:   "pages.example.com",
		ForgejoHost:   "http://localhost:3000",
		AuthSecretKey: "my-explicit-key",
	}
	if _, err := New(context.Background(), nil, cfg, "test"); err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if cfg.AuthSecretKey != "my-explicit-key" {
		t.Errorf("New overwrote an explicitly configured AuthSecretKey: got %q", cfg.AuthSecretKey)
	}
}

// TestBugA_ForgedAndTamperedCookiesRejected verifies HMAC verification with a real
// key: only legitimately signed cookies for the exact repository are accepted.
func TestBugA_ForgedAndTamperedCookiesRejected(t *testing.T) {
	ps := &PagesServer{config: &Config{AuthSecretKey: "real-secret", AuthCookieDuration: 3600}}
	username, repository := "alice", "private-repo"

	valid := ps.createAuthCookie(username, repository).Value
	if !ps.verifyAuthCookie(valid, username, repository) {
		t.Fatal("legitimately created cookie failed verification")
	}

	// Forged: correct format, wrong signature.
	forged := strings.SplitN(valid, "|", 2)[0] + "|deadbeefdeadbeef"
	if ps.verifyAuthCookie(forged, username, repository) {
		t.Error("SECURITY: forged cookie with bad signature accepted")
	}

	// Cross-repository reuse: a valid cookie must not authenticate a different repo.
	if ps.verifyAuthCookie(valid, username, "other-repo") {
		t.Error("SECURITY: cookie accepted for a different repository")
	}

	// A cookie signed with a different key must be rejected.
	other := &PagesServer{config: &Config{AuthSecretKey: "different-secret", AuthCookieDuration: 3600}}
	otherCookie := other.createAuthCookie(username, repository).Value
	if ps.verifyAuthCookie(otherCookie, username, repository) {
		t.Error("SECURITY: cookie signed with a different key accepted")
	}
}

// TestBugA_GenerateRandomSecretKey verifies the generated key format and uniqueness.
func TestBugA_GenerateRandomSecretKey(t *testing.T) {
	k1, err := generateRandomSecretKey()
	if err != nil {
		t.Fatalf("generateRandomSecretKey error: %v", err)
	}
	if len(k1) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(k1))
	}
	if _, err := hex.DecodeString(k1); err != nil {
		t.Errorf("key is not valid hex: %v", err)
	}
	k2, _ := generateRandomSecretKey()
	if k1 == k2 {
		t.Error("generateRandomSecretKey returned identical keys across calls")
	}
}

// --- BUG B: Stored XSS via .redirects file ----------------------------------

// TestBugB_RedirectListEscapesXSS reproduces the report's PoC and asserts the fix:
// attacker-controlled .redirects values must be HTML-escaped in the rendered list.
func TestBugB_RedirectListEscapesXSS(t *testing.T) {
	rules := []RedirectRule{
		{From: `<script>alert(document.cookie)</script>`, To: `"><img src=x onerror=alert(1)>`},
	}
	out := formatRedirectList(rules)

	if strings.Contains(out, "<script>") || strings.Contains(out, "<img") {
		t.Errorf("SECURITY: unescaped HTML in formatRedirectList output:\n%s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Errorf("expected escaped payload (&lt;script&gt;) in output:\n%s", out)
	}
}

// --- BUG C: HTML injection in generated pages -------------------------------

// TestBugC_LoginPageEscapesRepoInfo asserts username/repository are escaped on the
// password login page.
func TestBugC_LoginPageEscapesRepoInfo(t *testing.T) {
	ps := &PagesServer{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	ps.serveLoginPage(rec, req, "<b>user</b>", `<img src=x onerror=alert(1)>`, `"><script>alert(1)</script>`)
	body := rec.Body.String()

	if strings.Contains(body, "<b>user</b>") ||
		strings.Contains(body, "<img src=x") ||
		strings.Contains(body, "<script>alert(1)</script>") {
		t.Errorf("SECURITY: unescaped HTML injected into login page:\n%s", body)
	}
}

// TestBugC_BranchLoginPageEscapesError asserts error text is escaped on the branch
// login page.
func TestBugC_BranchLoginPageEscapesError(t *testing.T) {
	ps := &PagesServer{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	ps.serveBranchLoginPage(rec, req, `<script>alert(1)</script>`)
	body := rec.Body.String()

	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Errorf("SECURITY: unescaped error message on branch login page:\n%s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Errorf("expected escaped error message on branch login page:\n%s", body)
	}
}

// TestBugC_ServeErrorEscapesMessage asserts the default error page escapes its
// message, which may embed error strings derived from user-controlled input.
func TestBugC_ServeErrorEscapesMessage(t *testing.T) {
	ps := &PagesServer{errorPages: map[int][]byte{}}
	rec := httptest.NewRecorder()

	ps.serveError(rec, http.StatusNotFound, `<script>alert(1)</script>`)
	body := rec.Body.String()

	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Errorf("SECURITY: unescaped message in serveError output:\n%s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Errorf("expected escaped message in serveError output:\n%s", body)
	}
}
