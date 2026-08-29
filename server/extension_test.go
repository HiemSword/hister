package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/asciimoo/hister/server/testutil"
)

func TestExtensionCORSPreflightAllowsSafari(t *testing.T) {
	_, handler := newTokenTestServer(t, false)
	origin := "safari-web-extension://aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	rec := testutil.ServeHTTP(t, handler, http.MethodOptions, "/api/add", nil, map[string]string{
		"Origin":                         origin,
		"Access-Control-Request-Method":  "POST",
		"Access-Control-Request-Headers": "content-type,x-access-token",
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS /api/add status = %d, want %d; body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, origin)
	}
	allowHeaders := strings.ToLower(rec.Header().Get("Access-Control-Allow-Headers"))
	if !strings.Contains(allowHeaders, "x-access-token") {
		t.Fatalf("Access-Control-Allow-Headers = %q, want to include x-access-token", rec.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestExtensionCORSPreflightRejectsUntrustedOrigin(t *testing.T) {
	_, handler := newTokenTestServer(t, false)
	rec := testutil.ServeHTTP(t, handler, http.MethodOptions, "/api/add", nil, map[string]string{
		"Origin":                         "https://evil.example",
		"Access-Control-Request-Method":  "POST",
		"Access-Control-Request-Headers": "content-type",
	})
	if rec.Header().Get("Access-Control-Allow-Origin") == "https://evil.example" {
		t.Fatal("untrusted origin received Access-Control-Allow-Origin")
	}
	if rec.Code == http.StatusNoContent {
		t.Fatal("OPTIONS from an untrusted origin was treated as an extension preflight")
	}
}

func TestExtensionCSRFAllowsSafariHistoryAndProfile(t *testing.T) {
	_, handler := newTokenTestServer(t, false)
	origin := "safari-web-extension://aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	headers := map[string]string{
		"Origin":         origin,
		"Content-Type":   "application/json",
		"X-Access-Token": "secret",
	}

	history := testutil.ServeHTTP(
		t,
		handler,
		http.MethodPost,
		"/api/history",
		strings.NewReader(`{"url":"https://example.com","title":"Example","query":"q"}`),
		headers,
	)
	if history.Code == http.StatusForbidden {
		t.Fatalf("POST /api/history from Safari extension was CSRF-blocked: %s", history.Body.String())
	}

	profile := testutil.ServeHTTP(t, handler, http.MethodGet, "/api/profile", nil, headers)
	if profile.Code == http.StatusForbidden {
		t.Fatalf("GET /api/profile from Safari extension was CSRF-blocked: %s", profile.Body.String())
	}
	if got := profile.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Fatalf("GET /api/profile ACAO = %q, want %q", got, origin)
	}
}
