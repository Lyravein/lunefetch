package components

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/lyravein/lunefetch/internal/config"
	"github.com/lyravein/lunefetch/internal/core"
	"github.com/lyravein/lunefetch/internal/queue"
)

// ShowSettingsDialog displays the complete application settings form.
// Values are validated as a candidate config before changing the live config,
// so a malformed field cannot partially apply a settings dialog.
func ShowSettingsDialog(w fyne.Window, cfg *config.Config, globalLimiter **core.Limiter, qm *queue.Manager) {
	downloadDir := widget.NewEntry()
	downloadDir.SetText(cfg.DownloadDir)
	browse := widget.NewButton("Browse…", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err == nil && uri != nil {
				downloadDir.SetText(uri.Path())
			}
		}, w)
	})
	downloadDirRow := container.NewBorder(nil, nil, nil, browse, downloadDir)

	globalSpeed := widget.NewEntry()
	globalSpeed.SetPlaceHolder("0 = unlimited")
	if cfg.GlobalSpeedLimit > 0 {
		globalSpeed.SetText(strconv.FormatInt(cfg.GlobalSpeedLimit/1024, 10))
	}

	maxConcurrent := widget.NewEntry()
	maxConcurrent.SetText(strconv.Itoa(cfg.MaxConcurrent))
	maxRetries := widget.NewEntry()
	maxRetries.SetText(strconv.Itoa(cfg.MaxRetries))
	retryBackoff := widget.NewEntry()
	retryBackoff.SetText(strconv.Itoa(cfg.RetryBackoffS))
	timeout := widget.NewEntry()
	timeout.SetText(strconv.Itoa(cfg.Timeout))
	minFree := widget.NewEntry()
	minFree.SetText(strconv.Itoa(cfg.MinFreeSpaceMB))
	retention := widget.NewEntry()
	retention.SetText(strconv.Itoa(cfg.HistoryRetentionDays))

	proxy := widget.NewEntry()
	proxy.SetText(cfg.ProxyURL)
	proxy.SetPlaceHolder("http://host:port or socks5://host:port")

	notifications := widget.NewCheck("Show desktop notifications", nil)
	notifications.SetChecked(cfg.Notifications)
	closeToTray := widget.NewCheck("Keep running in the system tray when closing", nil)
	closeToTray.SetChecked(cfg.CloseToTray)
	allowLocal := widget.NewCheck("Allow local/LAN download hosts", nil)
	allowLocal.SetChecked(cfg.AllowLocalHosts)
	localWarning := widget.NewLabel("Only enable this for trusted URLs from your own network.")
	localWarning.Importance = widget.LowImportance

	section := func(title string) fyne.CanvasObject {
		label := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		return container.New(layout.NewCustomPaddedLayout(0, 6, 0, 0), label)
	}
	field := func(label string, entry *widget.Entry, hint string) fyne.CanvasObject {
		name := widget.NewLabel(label)
		name.Importance = widget.LowImportance
		if hint != "" {
			entry.SetPlaceHolder(hint)
		}
		return container.NewVBox(name, entry)
	}

	content := container.NewVBox(
		section("Download location"),
		widget.NewLabel("New downloads are saved here by default."),
		downloadDirRow,
		widget.NewSeparator(),
		section("Performance and limits"),
		container.NewGridWithColumns(2,
			field("Global speed limit (KB/s)", globalSpeed, "0 = unlimited"),
			field("Concurrent downloads", maxConcurrent, "1–64"),
			field("Max retries", maxRetries, "0–20"),
			field("Retry backoff (seconds)", retryBackoff, "1–300"),
			field("Request timeout (seconds)", timeout, "1–3600"),
			field("Minimum free space (MB)", minFree, "0 disables the check"),
		),
		widget.NewSeparator(),
		section("Network"),
		field("Proxy URL", proxy, "http://host:port or socks5://host:port"),
		allowLocal,
		localWarning,
		widget.NewSeparator(),
		section("History and behavior"),
		field("History retention (days)", retention, "0 = keep forever"),
		notifications,
		closeToTray,
	)

	d := dialog.NewCustomConfirm("Settings", "Save", "Cancel", container.NewVScroll(content), func(confirmed bool) {
		if !confirmed {
			return
		}

		candidate := *cfg
		candidate.DownloadDir = strings.TrimSpace(downloadDir.Text)
		candidate.ProxyURL = strings.TrimSpace(proxy.Text)
		candidate.Notifications = notifications.Checked
		candidate.CloseToTray = closeToTray.Checked
		candidate.AllowLocalHosts = allowLocal.Checked

		var err error
		if candidate.GlobalSpeedLimit, err = parseKB(globalSpeed.Text); err != nil {
			ShowError(w, "Global speed limit: "+err.Error())
			return
		}
		if candidate.MaxConcurrent, err = parseSettingInt(maxConcurrent.Text, "concurrent downloads"); err != nil {
			ShowError(w, err.Error())
			return
		}
		if candidate.MaxRetries, err = parseSettingInt(maxRetries.Text, "max retries"); err != nil {
			ShowError(w, err.Error())
			return
		}
		if candidate.RetryBackoffS, err = parseSettingInt(retryBackoff.Text, "retry backoff"); err != nil {
			ShowError(w, err.Error())
			return
		}
		if candidate.Timeout, err = parseSettingInt(timeout.Text, "request timeout"); err != nil {
			ShowError(w, err.Error())
			return
		}
		if candidate.MinFreeSpaceMB, err = parseSettingInt(minFree.Text, "minimum free space"); err != nil {
			ShowError(w, err.Error())
			return
		}
		if candidate.HistoryRetentionDays, err = parseSettingInt(retention.Text, "history retention"); err != nil {
			ShowError(w, err.Error())
			return
		}
		if err := candidate.Validate(); err != nil {
			ShowError(w, "Invalid settings: "+err.Error())
			return
		}

		*cfg = candidate
		applyGlobalLimiter(globalLimiter, cfg.GlobalSpeedLimit)
		if qm != nil {
			qm.SetMaxConcurrent(cfg.MaxConcurrent)
		}
		if err := cfg.Save(); err != nil {
			ShowError(w, "Failed to save settings: "+err.Error())
		}
	}, w)
	d.Resize(fyne.NewSize(620, 680))
	d.Show()
}

func parseSettingInt(raw, name string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return value, nil
}

func parseKB(raw string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("must be a non-negative integer")
	}
	return value * 1024, nil
}

func applyGlobalLimiter(target **core.Limiter, rate int64) {
	if target == nil {
		return
	}
	if rate == 0 {
		if *target != nil {
			(*target).SetRate(0)
		}
		*target = nil
		return
	}
	if *target == nil {
		*target = core.NewLimiter(rate)
		return
	}
	(*target).SetRate(rate)
}
