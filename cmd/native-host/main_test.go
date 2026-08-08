package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"testing"
)

func TestHandleRequestRejectsInvalidURL(t *testing.T) {
	result := handleRequest(inMessage{Action: "download", URL: "file:///etc/passwd"})
	if result.Outcome != outcomeInvalidURL {
		t.Fatalf("outcome = %q, want %q", result.Outcome, outcomeInvalidURL)
	}
}

func TestHandleRequestRejectsUnknownAction(t *testing.T) {
	result := handleRequest(inMessage{Action: "unknown"})
	if result.Outcome != outcomeInternalError {
		t.Fatalf("outcome = %q, want %q", result.Outcome, outcomeInternalError)
	}
}

func TestValidHTTPURL(t *testing.T) {
	for _, raw := range []string{"https://example.com/file", "http://example.com/file?q=1"} {
		if !validHTTPURL(raw) {
			t.Fatalf("validHTTPURL(%q) = false", raw)
		}
	}
	for _, raw := range []string{"", "file:///tmp/file", "https:///missing-host", "not a URL"} {
		if validHTTPURL(raw) {
			t.Fatalf("validHTTPURL(%q) = true", raw)
		}
	}
}

func TestResultForStatus(t *testing.T) {
	tests := []struct {
		status  int
		outcome string
	}{
		{http.StatusAccepted, outcomeAccepted},
		{http.StatusBadRequest, outcomeInvalidURL},
		{http.StatusUnauthorized, outcomeUnauthorized},
		{http.StatusServiceUnavailable, outcomeQueueFull},
		{http.StatusInternalServerError, outcomeInternalError},
	}
	for _, tt := range tests {
		if got := resultForStatus(tt.status).Outcome; got != tt.outcome {
			t.Errorf("status %d: outcome = %q, want %q", tt.status, got, tt.outcome)
		}
	}
}

func TestDownloadPayloadIncludesOnlyExplicitHints(t *testing.T) {
	body, err := downloadPayload(inMessage{
		Action: "download", URL: "https://example.com/file", Filename: "archive.zip", SaveDir: "/tmp/downloads",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["url"] != "https://example.com/file" || got["filename"] != "archive.zip" || got["save_dir"] != "/tmp/downloads" {
		t.Fatalf("payload = %#v", got)
	}
	if len(got) != 3 {
		t.Fatalf("payload contains unexpected data: %#v", got)
	}
}

func TestReadMessageRejectsOversizedLength(t *testing.T) {
	var input bytes.Buffer
	if err := binary.Write(&input, binary.LittleEndian, uint32(maxMessageSize+1)); err != nil {
		t.Fatal(err)
	}
	if _, err := readMessage(&input); err == nil {
		t.Fatal("readMessage accepted oversized message")
	}
}

func TestReadMessageRejectsZeroLength(t *testing.T) {
	var input bytes.Buffer
	if err := binary.Write(&input, binary.LittleEndian, uint32(0)); err != nil {
		t.Fatal(err)
	}
	if _, err := readMessage(&input); err == nil {
		t.Fatal("readMessage accepted empty message")
	}
}
