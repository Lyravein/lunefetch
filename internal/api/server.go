// Package api provides a local HTTP server that receives download requests
// from the browser extension via the native messaging host.
package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"
)

const DefaultAddr = "127.0.0.1:7474"

// DownloadRequest is the JSON payload sent by the native host.
type DownloadRequest struct {
	URL string `json:"url"`
}

// Handler is called when the extension sends a new URL to download.
type Handler func(url string)

// Server is a lightweight HTTP server that listens for incoming download
// requests from the native messaging host.
type Server struct {
	srv     *http.Server
	handler Handler
}

// New creates a new Server. Call Start to begin accepting connections.
func New(addr string, h Handler) *Server {
	s := &Server{handler: h}

	mux := http.NewServeMux()
	mux.HandleFunc("/download", s.handleDownload)
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
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

	var req DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, "missing url", http.StatusBadRequest)
		return
	}

	s.handler(req.URL)
	w.WriteHeader(http.StatusAccepted)
}
