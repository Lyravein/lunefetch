package components

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/lyravein/lunefetch/internal/config"
	"github.com/lyravein/lunefetch/internal/queue"
)

type StatusBar struct {
	bar        fyne.CanvasObject
	speed      *widget.Label
	concurrent *widget.Select
	cfg        *config.Config
	qm         *queue.Manager
}

func NewStatusBar(cfg *config.Config, qm *queue.Manager) *StatusBar {
	sb := &StatusBar{cfg: cfg, qm: qm, speed: widget.NewLabel("Overall Speed: 0 B/s")}
	sb.speed.Importance = widget.LowImportance
	values := []string{"1", "2", "3", "4", "5", "8", "12", "16"}
	selectWidget := widget.NewSelect(values, func(value string) {
		if cfg == nil || qm == nil {
			return
		}
		var n int
		if _, err := fmt.Sscanf(value, "%d", &n); err == nil {
			cfg.MaxConcurrent = n
			qm.SetMaxConcurrent(n)
			_ = cfg.Save()
		}
	})
	sb.concurrent = selectWidget
	if cfg != nil {
		selectWidget.SetSelected(fmt.Sprintf("%d", cfg.MaxConcurrent))
	}
	label := widget.NewLabel("Concurrent Downloads")
	label.Importance = widget.LowImportance
	right := container.NewHBox(label, selectWidget)
	content := container.NewBorder(nil, nil, container.NewHBox(widget.NewIcon(theme.DownloadIcon()), sb.speed), right, layout.NewSpacer())
	sb.bar = container.NewBorder(widget.NewSeparator(), nil, nil, nil, container.New(layout.NewCustomPaddedLayout(6, 6, 18, 18), content))
	return sb
}

func (sb *StatusBar) Container() fyne.CanvasObject { return sb.bar }

func (sb *StatusBar) Update(totalSpeed float64, _ int, _ int) {
	text := fmt.Sprintf("Overall Speed: %s/s", FormatSize(int64(totalSpeed)))
	fyne.Do(func() {
		if sb.speed.Text == text {
			return
		}
		sb.speed.SetText(text)
	})
}
