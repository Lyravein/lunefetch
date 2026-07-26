// Package notify mengirim notifikasi desktop via notify-send.
// Shell-out ke notify-send dilakukan sekali saat New() dipanggil untuk
// mengecek ketersediaannya. Kalau tidak ada, semua operasi jadi no-op
// sehingga tidak perlu cek di tiap pemanggil.
package notify

import (
	"os/exec"
)

// Notifier mengirim notifikasi desktop.
type Notifier struct {
	bin     string // path ke notify-send, kosong = tidak tersedia
	enabled bool   // toggle dari config
}

// New membuat Notifier baru. enabled=false membuat semua Send jadi no-op
// tanpa perlu cek ketersediaan notify-send.
func New(enabled bool) *Notifier {
	n := &Notifier{enabled: enabled}
	if enabled {
		if path, err := exec.LookPath("notify-send"); err == nil {
			n.bin = path
		}
	}
	return n
}

// SetEnabled mengubah toggle notifikasi saat runtime.
func (n *Notifier) SetEnabled(v bool) {
	n.enabled = v
}

// Send mengirim notifikasi dengan title dan body. No-op kalau notify-send
// tidak tersedia atau notifikasi dinonaktifkan. Dipanggil dari goroutine
// supaya tidak memblokir TUI.
func (n *Notifier) Send(title, body string) {
	if !n.enabled || n.bin == "" {
		return
	}
	// --app-name supaya notif teridentifikasi, urgency normal.
	cmd := exec.Command(n.bin,
		"--app-name=Lunefetch",
		"--urgency=normal",
		title,
		body,
	)
	cmd.Run() //nolint:errcheck — best-effort, gagal diam-diam
}
