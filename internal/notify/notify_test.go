package notify_test

import (
	"testing"

	"fyne.io/fyne/v2"

	"github.com/lyravein/lunefetch/internal/notify"
)

func TestNoOpWhenDisabled(t *testing.T) {
	n := notify.New(false)
	// Tidak boleh panik, tidak ada side effect.
	n.Send("title", "body")
}

func TestNoOpWhenBinAbsent(t *testing.T) {
	// Buat notifier dengan enabled=true tapi bin sengaja dikosongkan
	// lewat New(false) lalu SetEnabled(true), simulasi kondisi
	// notify-send tidak ada di PATH.
	n := notify.New(false)
	n.SetEnabled(true)
	// bin masih kosong karena LookPath tidak dipanggil ulang setelah SetEnabled.
	// Send harus no-op (bin == "").
	n.Send("title", "body")
}

func TestSendDoesNotBlockCaller(t *testing.T) {
	// Send dipanggil secara sinkron di sini; caller yang bertugas
	// menjalankannya di goroutine. Test ini hanya verifikasi tidak ada
	// deadlock atau panic.
	n := notify.New(false)
	done := make(chan struct{})
	go func() {
		n.Send("done", "file.bin selesai diunduh")
		close(done)
	}()
	<-done
}

// stubApp records notifications so delivery through Fyne can be asserted without
// a real desktop session. This is the only path that works on Windows.
type stubApp struct {
	fyne.App
	sent []*fyne.Notification
}

func (s *stubApp) SendNotification(n *fyne.Notification) {
	s.sent = append(s.sent, n)
}

func TestSendUsesFyneAppWhenAvailable(t *testing.T) {
	app := &stubApp{}
	n := notify.New(true)
	n.SetApp(app)
	n.Send("Download complete", "file.bin")

	if len(app.sent) != 1 {
		t.Fatalf("delivered %d notifications, want 1", len(app.sent))
	}
	if app.sent[0].Title != "Download complete" || app.sent[0].Content != "file.bin" {
		t.Fatalf("notification = %+v", app.sent[0])
	}
}

func TestSendStaysSilentWhenDisabledEvenWithApp(t *testing.T) {
	app := &stubApp{}
	n := notify.New(false)
	n.SetApp(app)
	n.Send("Download complete", "file.bin")

	if len(app.sent) != 0 {
		t.Fatalf("delivered %d notifications while disabled, want 0", len(app.sent))
	}
}

func TestSetAppOnNilNotifierIsSafe(t *testing.T) {
	var n *notify.Notifier
	n.SetApp(&stubApp{})
	n.Send("title", "body")
}
