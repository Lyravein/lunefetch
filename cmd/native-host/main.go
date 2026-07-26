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
	"os"
	"time"
)

const apiURL = "http://127.0.0.1:7474/download"

type inMessage struct {
	URL string `json:"url"`
}

type outMessage struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func main() {
	msg, err := readMessage(os.Stdin)
	if err != nil {
		writeMessage(os.Stdout, outMessage{Success: false, Error: fmt.Sprintf("read: %v", err)})
		os.Exit(1)
	}

	var in inMessage
	if err := json.Unmarshal(msg, &in); err != nil {
		writeMessage(os.Stdout, outMessage{Success: false, Error: fmt.Sprintf("parse: %v", err)})
		os.Exit(1)
	}

	if in.URL == "" {
		writeMessage(os.Stdout, outMessage{Success: false, Error: "empty url"})
		os.Exit(1)
	}

	if err := sendToApp(in.URL); err != nil {
		writeMessage(os.Stdout, outMessage{Success: false, Error: err.Error()})
		os.Exit(1)
	}

	writeMessage(os.Stdout, outMessage{Success: true})
}

// readMessage reads one native messaging message from r.
func readMessage(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return nil, fmt.Errorf("read length: %w", err)
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

// sendToApp POSTs the URL to the download-manager HTTP API.
func sendToApp(url string) error {
	body, _ := json.Marshal(map[string]string{"url": url})
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(apiURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("app not running or unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("app returned status %d", resp.StatusCode)
	}
	return nil
}
