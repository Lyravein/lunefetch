// Package api provides a local HTTP server that receives download requests
// from the browser extension via the native messaging host.
package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultAddr = "127.0.0.1:7474"

// DownloadRequest is the JSON payload sent by the native host.
type DownloadRequest struct {
	URL      string `json:"url"`
	Filename string `json:"filename,omitempty"`
	SaveDir  string `json:"save_dir,omitempty"`
}

// Handler is called when the extension sends a new URL to download.
type Handler func(request DownloadRequest) bool

// Server is a lightweight HTTP server that listens for incoming download
// requests from the native messaging host.
type Server struct {
	srv     *http.Server
	handler Handler
	token   string
}

// New creates a new Server. Call Start to begin accepting connections.
func New(addr, token string, h Handler) *Server {
	s := &Server{handler: h, token: token}

	mux := http.NewServeMux()
	mux.HandleFunc("/download", s.handleDownload)
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})

	s.srv = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	return s
}

// Start begins listening in a background goroutine. It returns once the
// listener is bound (or immediately if binding fails).
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return err
	}
	go s.srv.Serve(ln)
	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.srv.Shutdown(ctx)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if mediaType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0])); mediaType != "application/json" {
		http.Error(w, "content-type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	var req DownloadRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := ensureJSONEOF(decoder); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	u, err := url.ParseRequestURI(req.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		http.Error(w, "invalid HTTP URL", http.StatusBadRequest)
		return
	}
	if req.Filename != "" && (filepath.Base(req.Filename) != req.Filename || strings.ContainsAny(req.Filename, "\x00/\\")) {
		http.Error(w, "invalid filename hint", http.StatusBadRequest)
		return
	}
	if req.SaveDir != "" && (!filepath.IsAbs(req.SaveDir) || strings.ContainsRune(req.SaveDir, '\x00')) {
		http.Error(w, "invalid destination hint", http.StatusBadRequest)
		return
	}

	if !s.handler(req) {
		http.Error(w, "download queue is busy", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) authorized(r *http.Request) bool {
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return len(got) == len(s.token) && subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) == 1
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return fmt.Errorf("trailing JSON")
}

// TokenPath is shared with the native host as a user-private bearer token.
func TokenPath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "lunefetch", "api-token")
}

// LoadOrCreateToken returns the local API token, creating it with mode 0600.
func LoadOrCreateToken() (string, error) {
	path := TokenPath()
	if data, err := os.ReadFile(path); err == nil {
		token := strings.TrimSpace(string(data))
		if len(token) < 32 {
			return "", fmt.Errorf("API token is invalid")
		}
		if err := os.Chmod(path, 0600); err != nil {
			return "", err
		}
		return token, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := fmt.Sprintf("%x", raw)
	if err := os.WriteFile(path, []byte(token+"\n"), 0600); err != nil {
		return "", err
	}
	return token, nil
}
