// native-host is the Firefox native messaging host for download-manager.
// Firefox spawns this binary and communicates via stdin/stdout using the
// native messaging protocol: each message is prefixed with a 4-byte
// little-endian uint32 length, followed by a JSON payload.
//
// This host receives a download URL from the extension and forwards it to
// the running download-manager TUI via its local HTTP API on port 7474.
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const apiURL = "http://127.0.0.1:7474/download"
const maxMessageSize = 64 << 10

var version = "dev"

type inMessage struct {
	Action   string `json:"action,omitempty"`
	URL      string `json:"url,omitempty"`
	Filename string `json:"filename,omitempty"`
	SaveDir  string `json:"save_dir,omitempty"`
}

type outMessage struct {
	Success bool   `json:"success"`
	Outcome string `json:"outcome"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

const (
	outcomeAccepted       = "accepted"
	outcomeAppUnavailable = "app_unavailable"
	outcomeInvalidURL     = "invalid_url"
	outcomeUnauthorized   = "unauthorized"
	outcomeQueueFull      = "queue_full"
	outcomeInternalError  = "internal_error"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("lunefetch-native-host %s\n", version)
		return
	}
	msg, err := readMessage(os.Stdin)
	if err != nil {
		writeMessage(os.Stdout, failure(outcomeInternalError, fmt.Sprintf("read: %v", err)))
		os.Exit(1)
	}

	var in inMessage
	if err := json.Unmarshal(msg, &in); err != nil {
		writeMessage(os.Stdout, failure(outcomeInternalError, fmt.Sprintf("parse: %v", err)))
		os.Exit(1)
	}

	if in.Action == "" {
		in.Action = "download"
	}
	result := handleRequest(in)
	if err := writeMessage(os.Stdout, result); err != nil {
		os.Exit(1)
	}
}

func failure(outcome, message string) outMessage {
	return outMessage{Outcome: outcome, Message: message, Error: message}
}

func handleRequest(in inMessage) outMessage {
	if in.Action != "download" && in.Action != "health" {
		return failure(outcomeInternalError, "unknown action")
	}
	if in.Action == "download" && !validHTTPURL(in.URL) {
		return failure(outcomeInvalidURL, "URL must use HTTP or HTTPS")
	}
	return sendToApp(in)
}

func validHTTPURL(raw string) bool {
	u, err := url.ParseRequestURI(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// readMessage reads one native messaging message from r.
func readMessage(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return nil, fmt.Errorf("read length: %w", err)
	}
	if length == 0 || length > maxMessageSize {
		return nil, fmt.Errorf("message length %d exceeds limit", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return buf, nil
}

// writeMessage writes one native messaging message to w.
func writeMessage(w io.Writer, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(len(body))); err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}

func downloadPayload(in inMessage) ([]byte, error) {
	return json.Marshal(struct {
		URL      string `json:"url"`
		Filename string `json:"filename,omitempty"`
		SaveDir  string `json:"save_dir,omitempty"`
	}{URL: in.URL, Filename: in.Filename, SaveDir: in.SaveDir})
}

// sendToApp forwards a health check or download request to the local API.
func sendToApp(in inMessage) outMessage {
	tokenData, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "lunefetch", "api-token"))
	if err != nil {
		return failure(outcomeAppUnavailable, "Lunefetch is not installed or has not been started")
	}
	method := http.MethodGet
	endpoint := "http://127.0.0.1:7474/ping"
	var body io.Reader
	if in.Action == "download" {
		method = http.MethodPost
		endpoint = apiURL
		payload, err := downloadPayload(in)
		if err != nil {
			return failure(outcomeInternalError, err.Error())
		}
		body = bytes.NewReader(payload)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return failure(outcomeInternalError, err.Error())
	}
	if in.Action == "download" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(tokenData)))
	resp, err := client.Do(req)
	if err != nil {
		return failure(outcomeAppUnavailable, "Start Lunefetch and try again")
	}
	defer resp.Body.Close()
	return resultForStatus(resp.StatusCode)
}

func resultForStatus(status int) outMessage {
	switch status {
	case http.StatusOK, http.StatusAccepted:
		return outMessage{Success: true, Outcome: outcomeAccepted}
	case http.StatusBadRequest:
		return failure(outcomeInvalidURL, "Lunefetch rejected the URL")
	case http.StatusUnauthorized:
		return failure(outcomeUnauthorized, "Restart Lunefetch or reinstall the native host")
	case http.StatusServiceUnavailable, http.StatusTooManyRequests:
		return failure(outcomeQueueFull, "Lunefetch download queue is busy")
	default:
		return failure(outcomeInternalError, fmt.Sprintf("Lunefetch returned status %d", status))
	}
}
