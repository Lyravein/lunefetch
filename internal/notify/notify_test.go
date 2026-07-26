package notify_test

import (
	"testing"

	"github.com/lyravein/lunefetch/internal/notify"
)

func TestNoOpWhenDisabled(t *testing.T) {
	n := notify.New(false)
	// Tidak boleh panik, tidak ada side effect.
	n.Send("title", "body")
}

func TestNoOpWhenBinAbsent(t *testing.T) {
	// Buat notifier dengan enabled=true tapi bin sengaja dikosongkan
	// lewat New(false) lalu SetEnabled(true) — simulasi kondisi
	// notify-send tidak ada di PATH.
	n := notify.New(false)
	n.SetEnabled(true)
	// bin masih kosong karena LookPath tidak dipanggil ulang setelah SetEnabled.
	// Send harus no-op (bin == "").
	n.Send("title", "body")
}

func TestSendDoesNotBlockCaller(t *testing.T) {
	// Send dipanggil secara sinkron di sini — caller yang bertugas
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
