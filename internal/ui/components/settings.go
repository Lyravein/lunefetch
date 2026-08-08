package components

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/lyravein/lunefetch/internal/config"
	"github.com/lyravein/lunefetch/internal/core"
	"github.com/lyravein/lunefetch/internal/queue"
)

// ShowSettingsDialog displays the settings dialog for network and download options.
func ShowSettingsDialog(w fyne.Window, cfg *config.Config, globalLimiter **core.Limiter, qm *queue.Manager) {
	globalSpeedEntry := widget.NewEntry()
	if cfg.GlobalSpeedLimit > 0 {
		globalSpeedEntry.SetText(strconv.FormatInt(cfg.GlobalSpeedLimit/1024, 10))
	}
	globalSpeedEntry.SetPlaceHolder("0 (unlimited)")

	proxyEntry := widget.NewEntry()
	proxyEntry.SetText(cfg.ProxyURL)
	proxyEntry.SetPlaceHolder("http://host:port  or  socks5://host:port")

	maxConcurrentEntry := widget.NewEntry()
	maxConcurrentEntry.SetText(strconv.Itoa(cfg.MaxConcurrent))
	maxConcurrentEntry.SetPlaceHolder("2")

	form := container.NewVBox(
		widget.NewLabel("Global speed limit (KB/s, 0 = unlimited):"),
		globalSpeedEntry,
		widget.NewSeparator(),
		widget.NewLabel("Proxy URL:"),
		proxyEntry,
		widget.NewSeparator(),
		widget.NewLabel("Max concurrent downloads:"),
		maxConcurrentEntry,
	)

	d := dialog.NewCustomConfirm("Settings", "Save", "Cancel", form,
		func(confirmed bool) {
			if !confirmed {
				return
			}

			var globalLimit int64
			if s := strings.TrimSpace(globalSpeedEntry.Text); s != "" {
				if kb, err := strconv.ParseInt(s, 10, 64); err == nil && kb >= 0 {
					globalLimit = kb * 1024
				}
			}
			cfg.GlobalSpeedLimit = globalLimit

			if globalLimit > 0 {
				if *globalLimiter == nil {
					*globalLimiter = core.NewLimiter(globalLimit)
				} else {
					(*globalLimiter).SetRate(globalLimit)
				}
			} else {
				if *globalLimiter != nil {
					(*globalLimiter).SetRate(0)
				}
				*globalLimiter = nil
			}

			cfg.ProxyURL = strings.TrimSpace(proxyEntry.Text)

			if n, err := strconv.Atoi(strings.TrimSpace(maxConcurrentEntry.Text)); err == nil && n > 0 {
				cfg.MaxConcurrent = n
				qm.SetMaxConcurrent(n)
			}

			if err := cfg.Save(); err != nil {
				ShowError(w, "Failed to save settings: "+err.Error())
			}
		}, w)

	d.Resize(fyne.NewSize(480, 300))
	d.Show()
}
