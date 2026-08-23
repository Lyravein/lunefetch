package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTokenPathIsAbsoluteAndUserPrivate(t *testing.T) {
	got := TokenPath()
	if !filepath.IsAbs(got) {
		t.Fatalf("TokenPath = %q, want an absolute path", got)
	}
	if strings.HasPrefix(got, "."+string(filepath.Separator)) || got == filepath.Join(".config", "lunefetch", "api-token") {
		t.Fatalf("TokenPath = %q, want it outside the working directory", got)
	}
	if filepath.Base(got) != "api-token" {
		t.Fatalf("TokenPath = %q, want it to end in api-token", got)
	}
}

func TestLoadOrCreateTokenCreatesPrivateTokenAndReuses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not meaningful on Windows")
	}
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))

	token, err := LoadOrCreateToken()
	if err != nil {
		t.Fatalf("LoadOrCreateToken: %v", err)
	}
	if len(token) < 32 {
		t.Fatalf("token length = %d, want at least 32", len(token))
	}

	path := TokenPath()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat token: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("token mode = %v, want 0600", perm)
	}

	again, err := LoadOrCreateToken()
	if err != nil {
		t.Fatalf("second LoadOrCreateToken: %v", err)
	}
	if again != token {
		t.Fatal("token was regenerated instead of reused")
	}
}

func TestLoadOrCreateTokenRejectsShortToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))

	path := TokenPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("too-short\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateToken(); err == nil {
		t.Fatal("a short token was accepted")
	}
}

// The API must not be usable as a generic cross-origin endpoint, and it must
// never answer without the bearer token.
func TestServerRejectsUnauthenticatedAndNonJSONRequests(t *testing.T) {
	s := New(DefaultAddr, "secret-token-secret-token-secret", func(DownloadRequest) bool { return true })

	req := httptest.NewRequest(http.MethodPost, "/download", strings.NewReader(`{"url":"https://example.com/f.zip"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.srv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", rec.Code)
	}

	req = httptest.NewRequest(http.MethodOptions, "/download", nil)
	rec = httptest.NewRecorder()
	s.srv.Handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatal("OPTIONS preflight was answered; the API should not be CORS-reachable")
	}
	if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", origin)
	}
}
