package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// absDir returns a destination that filepath.IsAbs accepts on the running
// platform. A Unix path like /tmp/downloads has no volume letter, so Windows
// treats it as relative and the handler rejects it.
func absDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if !filepath.IsAbs(dir) {
		t.Fatalf("t.TempDir() = %q, want an absolute path", dir)
	}
	return dir
}

func request(t *testing.T, s *Server, token, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/download", strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rr := httptest.NewRecorder()
	s.handleDownload(rr, req)
	return rr
}

func TestDownloadRequiresAuthentication(t *testing.T) {
	s := New(DefaultAddr, "secret-token", func(DownloadRequest) bool { return true })
	if got := request(t, s, "", "application/json", `{"url":"https://example.com/file"}`).Code; got != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", got, http.StatusUnauthorized)
	}
}

func TestDownloadValidatesRequest(t *testing.T) {
	s := New(DefaultAddr, "secret-token", func(DownloadRequest) bool { return true })
	tests := []struct {
		name        string
		contentType string
		body        string
		want        int
	}{
		{"content type", "text/plain", `{"url":"https://example.com/file"}`, http.StatusUnsupportedMediaType},
		{"scheme", "application/json", `{"url":"file:///etc/passwd"}`, http.StatusBadRequest},
		{"filename traversal", "application/json", `{"url":"https://example.com/file","filename":"../file"}`, http.StatusBadRequest},
		{"relative destination", "application/json", `{"url":"https://example.com/file","save_dir":"downloads"}`, http.StatusBadRequest},
		{"unknown field", "application/json", `{"url":"https://example.com/file","extra":true}`, http.StatusBadRequest},
		{"trailing JSON", "application/json", `{"url":"https://example.com/file"}{}`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := request(t, s, "secret-token", tt.contentType, tt.body).Code; got != tt.want {
				t.Fatalf("status = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDownloadBackpressure(t *testing.T) {
	s := New(DefaultAddr, "secret-token", func(DownloadRequest) bool { return false })
	if got := request(t, s, "secret-token", "application/json", `{"url":"https://example.com/file"}`).Code; got != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", got, http.StatusServiceUnavailable)
	}
}

func TestDownloadAccepted(t *testing.T) {
	var gotRequest DownloadRequest
	s := New(DefaultAddr, "secret-token", func(req DownloadRequest) bool {
		gotRequest = req
		return true
	})

	saveDir := absDir(t)
	body, err := json.Marshal(map[string]string{
		"url":      "https://example.com/file",
		"filename": "file.zip",
		"save_dir": saveDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := request(t, s, "secret-token", "application/json; charset=utf-8", string(body)).Code; got != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", got, http.StatusAccepted)
	}
	if gotRequest.URL != "https://example.com/file" || gotRequest.Filename != "file.zip" || gotRequest.SaveDir != saveDir {
		t.Fatalf("request = %+v", gotRequest)
	}
}
