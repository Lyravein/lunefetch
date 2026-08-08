package components

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// StatusBar is a thin bar at the bottom of the window showing global stats.
type StatusBar struct {
	bar   fyne.CanvasObject
	label *widget.Label
}

// NewStatusBar creates a new StatusBar.
func NewStatusBar() *StatusBar {
	sb := &StatusBar{label: widget.NewLabel("0 B/s  ·  0 active  ·  0 downloads")}
	sb.label.Alignment = fyne.TextAlignTrailing
	sb.label.Importance = widget.LowImportance
	content := container.New(layout.NewCustomPaddedLayout(4, 4, 12, 12), sb.label)
	sb.bar = container.NewBorder(widget.NewSeparator(), nil, nil, nil, content)

	return sb
}

// Container returns the status bar canvas object.
func (sb *StatusBar) Container() fyne.CanvasObject { return sb.bar }

// Update refreshes all labels.
//   - totalSpeed: combined bytes/sec across all active downloads.
//   - activeCount: number of downloads currently downloading.
//   - totalCount: total non-deleted records.
func (sb *StatusBar) Update(totalSpeed float64, activeCount, totalCount int) {
	fyne.Do(func() {
		sb.label.SetText(fmt.Sprintf("%s/s  ·  %d active  ·  %d downloads",
			FormatSize(int64(totalSpeed)), activeCount, totalCount))
	})
}
