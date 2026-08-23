// Package notify sends desktop notifications.
//
// Delivery is Fyne's cross-platform App.SendNotification when an app is
// available (it uses the native mechanism on Windows and macOS), with
// notify-send as a Linux fallback for the case where no Fyne app is running.
// The previous implementation only shelled out to notify-send, so completion
// notifications never appeared on Windows.
package notify

import (
	"os/exec"
	"runtime"

	"fyne.io/fyne/v2"
)

// Notifier sends desktop notifications.
type Notifier struct {
	bin     string   // path to notify-send; empty = unavailable
	enabled bool     // toggle from config
	app     fyne.App // optional; preferred delivery path
}

// New creates a Notifier. enabled=false makes every Send a no-op without
// probing for a delivery mechanism.
func New(enabled bool) *Notifier {
	n := &Notifier{enabled: enabled}
	if enabled && runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		if path, err := exec.LookPath("notify-send"); err == nil {
			n.bin = path
		}
	}
	return n
}

// SetApp registers the running Fyne app as the preferred delivery mechanism.
// This is what makes notifications work on Windows, where notify-send does not
// exist.
func (n *Notifier) SetApp(app fyne.App) {
	if n == nil {
		return
	}
	n.app = app
}

// SetEnabled toggles notifications at runtime.
func (n *Notifier) SetEnabled(v bool) {
	n.enabled = v
}

// Send delivers a notification. It is a no-op when notifications are disabled or
// no delivery mechanism is available.
func (n *Notifier) Send(title, body string) {
	if n == nil || !n.enabled {
		return
	}
	if n.app != nil {
		n.app.SendNotification(fyne.NewNotification(title, body))
		return
	}
	if n.bin == "" {
		return
	}
	// --app-name identifies the notification; "--" stops option parsing so a
	// title or body beginning with "-" is not read as a flag.
	cmd := exec.Command(n.bin,
		"--app-name=Lunefetch",
		"--urgency=normal",
		"--",
		title,
		body,
	)
	cmd.Run() //nolint:errcheck — best effort, failures are silent
}
