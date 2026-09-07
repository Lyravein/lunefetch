package components

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/lyravein/lunefetch/internal/storage"
	"github.com/lyravein/lunefetch/internal/ui/store"
)

const (
	downloadRowHeight float32 = 84
	rowPadX           float32 = 12
	rowPadY           float32 = 8
	rowGap            float32 = 8
	progressHeight    float32 = 8
	// statusColWidth keeps the status/speed column a fixed width so every
	// progress bar in the list starts and ends at the same x positions.
	statusColWidth float32 = 150
)

var visibleSortColumns = []struct {
	label string
	col   store.TableColumn
}{
	{label: "Name", col: store.ColName},
	{label: "Progress", col: store.ColProgress},
	{label: "Status", col: store.ColStatus},
}

func statusText(status string) string {
	switch status {
	case "downloading":
		return "Downloading"
	case "paused":
		return "Paused"
	case "completed":
		return "Completed"
	case "failed":
		return "Failed"
	case "cancelled":
		return "Cancelled"
	case "queued":
		return "Queued"
	case "scheduled":
		return "Scheduled"
	case "pending":
		return "Pending"
	default:
		return status
	}
}

func humanDate(t time.Time) string {
	now := time.Now()
	y, m, d := t.Date()
	ny, nm, nd := now.Date()
	switch {
	case y == ny && m == nm && d == nd:
		return t.Format("Today 15:04")
	case y == ny && m == nm && d == nd-1:
		return t.Format("Yesterday 15:04")
	case y == ny:
		return t.Format("02 Jan")
	default:
		return t.Format("02 Jan 06")
	}
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// rowMetaText is the secondary line under a filename: transferred size, total
// size, and completion percentage.
func rowMetaText(rec *storage.DownloadRecord) string {
	if rec == nil {
		return ""
	}
	if rec.TotalSize <= 0 {
		return FormatSize(rec.DownloadedSize)
	}
	return fmt.Sprintf("%s  •  %s  •  %.0f%%", FormatSize(rec.DownloadedSize), FormatSize(rec.TotalSize), progressOf(rec)*100)
}

func progressOf(rec *storage.DownloadRecord) float64 {
	if rec == nil || rec.TotalSize <= 0 {
		return 0
	}
	pct := float64(rec.DownloadedSize) / float64(rec.TotalSize)
	if pct < 0 {
		return 0
	}
	if pct > 1 {
		return 1
	}
	return pct
}

type downloadRow struct {
	widget.BaseWidget
	check    *widget.Check
	icon     *widget.Icon
	name     *widget.Label
	meta     *widget.Label
	progress *compactProgress
	status   *widget.Label
	speed    *widget.Label
	eta      *widget.Label
	actions  *widget.Button
}

func newDownloadRow() *downloadRow {
	r := &downloadRow{
		check:    widget.NewCheck("", nil),
		icon:     widget.NewIcon(theme.FileIcon()),
		name:     widget.NewLabel(""),
		meta:     widget.NewLabel(""),
		progress: newCompactProgress(),
		status:   widget.NewLabel(""),
		speed:    widget.NewLabel(""),
		eta:      widget.NewLabel(""),
		actions:  widget.NewButtonWithIcon("Actions", theme.MoreVerticalIcon(), nil),
	}
	r.name.TextStyle = fyne.TextStyle{Bold: true}
	r.name.Truncation = fyne.TextTruncateEllipsis
	r.meta.Importance = widget.LowImportance
	r.status.TextStyle = fyne.TextStyle{Bold: true}
	r.status.Importance = widget.MediumImportance
	r.speed.Importance = widget.LowImportance
	r.eta.Importance = widget.LowImportance
	r.actions.Importance = widget.LowImportance
	r.ExtendBaseWidget(r)
	return r
}

type compactProgress struct {
	widget.BaseWidget
	Value float64
}

func newCompactProgress() *compactProgress {
	p := &compactProgress{}
	p.ExtendBaseWidget(p)
	return p
}

func (p *compactProgress) SetValue(value float64) {
	if value < 0 {
		value = 0
	} else if value > 1 {
		value = 1
	}
	if p.Value == value {
		return
	}
	p.Value = value
	p.Refresh()
}

// CreateRenderer draws a thin track/fill pair. The percentage is shown in the
// row's metadata line instead of inside the bar: text does not fit in a bar this
// thin and previously spilled outside the row.
func (p *compactProgress) CreateRenderer() fyne.WidgetRenderer {
	track := canvas.NewRectangle(theme.Color(theme.ColorNameSeparator))
	fill := canvas.NewRectangle(theme.Color(theme.ColorNamePrimary))
	track.CornerRadius = progressHeight / 2
	fill.CornerRadius = progressHeight / 2
	return &compactProgressRenderer{progress: p, track: track, fill: fill, objects: []fyne.CanvasObject{track, fill}}
}

type compactProgressRenderer struct {
	progress *compactProgress
	track    *canvas.Rectangle
	fill     *canvas.Rectangle
	objects  []fyne.CanvasObject
}

func (r *compactProgressRenderer) Layout(size fyne.Size) {
	r.track.Resize(size)
	r.track.Move(fyne.NewPos(0, 0))
	r.fill.Resize(fyne.NewSize(size.Width*float32(r.progress.Value), size.Height))
	r.fill.Move(fyne.NewPos(0, 0))
}

func (r *compactProgressRenderer) MinSize() fyne.Size {
	return fyne.NewSize(80, progressHeight)
}

func (r *compactProgressRenderer) Refresh() {
	r.track.FillColor = theme.Color(theme.ColorNameSeparator)
	r.fill.FillColor = theme.Color(theme.ColorNamePrimary)
	r.track.Refresh()
	r.fill.Refresh()
	r.Layout(r.progress.Size())
}

func (r *compactProgressRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *compactProgressRenderer) Destroy() {}

// CreateRenderer uses a documented custom layout instead of nested containers.
// Fyne's box/border layouts stack minimum sizes, which previously pushed the
// progress bar past the bottom edge of the row; explicit geometry keeps every
// child inside the fixed row height on both Linux and Windows.
func (r *downloadRow) CreateRenderer() fyne.WidgetRenderer {
	background := canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))
	return &downloadRowRenderer{
		row:        r,
		background: background,
		objects: []fyne.CanvasObject{
			background, r.check, r.icon, r.name, r.meta, r.progress,
			r.status, r.speed, r.eta, r.actions,
		},
	}
}

type downloadRowRenderer struct {
	row        *downloadRow
	background *canvas.Rectangle
	objects    []fyne.CanvasObject
}

func (r *downloadRowRenderer) Layout(size fyne.Size) {
	r.background.Resize(size)
	r.background.Move(fyne.NewPos(0, 0))

	left := rowPadX
	if !r.row.check.Hidden {
		checkSize := r.row.check.MinSize()
		r.row.check.Resize(checkSize)
		r.row.check.Move(fyne.NewPos(left, (size.Height-checkSize.Height)/2))
		left += checkSize.Width + rowGap
	} else {
		r.row.check.Resize(fyne.NewSize(0, 0))
	}

	iconSize := fyne.NewSize(theme.Size(theme.SizeNameInlineIcon), theme.Size(theme.SizeNameInlineIcon))
	r.row.icon.Resize(iconSize)
	r.row.icon.Move(fyne.NewPos(left, (size.Height-iconSize.Height)/2))
	left += iconSize.Width + rowGap

	actionsSize := r.row.actions.MinSize()
	actionsX := size.Width - rowPadX - actionsSize.Width
	r.row.actions.Resize(actionsSize)
	r.row.actions.Move(fyne.NewPos(actionsX, (size.Height-actionsSize.Height)/2))

	statusSize := fyne.NewSize(r.row.status.MinSize().Width, lineHeight(fyne.TextStyle{Bold: true}))
	speedSize := fyne.NewSize(r.row.speed.MinSize().Width, lineHeight(fyne.TextStyle{}))
	etaSize := fyne.NewSize(r.row.eta.MinSize().Width, lineHeight(fyne.TextStyle{}))
	metaRowWidth := speedSize.Width + etaSize.Width
	rightWidth := statusColWidth
	if statusSize.Width > rightWidth {
		rightWidth = statusSize.Width
	}
	if metaRowWidth > rightWidth {
		rightWidth = metaRowWidth
	}
	rightX := actionsX - rowGap - rightWidth
	if rightX < left {
		rightX = left
	}
	stackHeight := statusSize.Height + speedSize.Height
	top := (size.Height - stackHeight) / 2
	r.row.status.Resize(fyne.NewSize(rightWidth, statusSize.Height))
	r.row.status.Move(fyne.NewPos(rightX, top))
	r.row.speed.Resize(speedSize)
	r.row.speed.Move(fyne.NewPos(rightX, top+statusSize.Height))
	r.row.eta.Resize(etaSize)
	r.row.eta.Move(fyne.NewPos(rightX+speedSize.Width, top+statusSize.Height))

	infoWidth := rightX - rowGap - left
	if infoWidth < 0 {
		infoWidth = 0
	}
	nameHeight := lineHeight(fyne.TextStyle{Bold: true})
	metaHeight := lineHeight(fyne.TextStyle{})
	r.row.name.Resize(fyne.NewSize(infoWidth, nameHeight))
	r.row.name.Move(fyne.NewPos(left, rowPadY))
	r.row.meta.Resize(fyne.NewSize(infoWidth, metaHeight))
	r.row.meta.Move(fyne.NewPos(left, rowPadY+nameHeight))
	r.row.progress.Resize(fyne.NewSize(infoWidth, progressHeight))
	r.row.progress.Move(fyne.NewPos(left, size.Height-rowPadY-progressHeight))
}

func (r *downloadRowRenderer) MinSize() fyne.Size {
	width := rowPadX*2 + rowGap*2 + theme.Size(theme.SizeNameInlineIcon)
	width += r.row.status.MinSize().Width + r.row.actions.MinSize().Width
	return fyne.NewSize(width, downloadRowHeight)
}

// lineHeight is the height of a single line of text for the given style. Label
// minimum sizes add widget padding, which stacked past the fixed row height.
func lineHeight(style fyne.TextStyle) float32 {
	return fyne.MeasureText("Ag", theme.TextSize(), style).Height
}

func (r *downloadRowRenderer) Refresh() {
	r.background.FillColor = theme.Color(theme.ColorNameInputBackground)
	r.background.Refresh()
	r.Layout(r.row.Size())
}

func (r *downloadRowRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *downloadRowRenderer) Destroy() {}

type DownloadTable struct {
	list         *widget.List
	header       fyne.CanvasObject
	root         *fyne.Container
	records      []*storage.DownloadRecord
	speeds       map[int64]float64
	window       fyne.Window
	sortCol      store.TableColumn
	sortAsc      bool
	onSort       func(store.TableColumn, bool)
	onSelect     func(int64)
	onAction     func(int64, string)
	onBulkAction func([]int64, string)
	onSpeedLimit func(int64, int64)
	multiHandler *multiSelectHandler
	selectButton *widget.Button
}

func NewDownloadTable(onSort func(store.TableColumn, bool), onSelect func(int64), onAction func(int64, string), onBulkAction func([]int64, string)) *DownloadTable {
	dt := &DownloadTable{
		speeds: make(map[int64]float64), sortCol: store.ColAdded, sortAsc: true,
		onSort: onSort, onSelect: onSelect, onAction: onAction, onBulkAction: onBulkAction,
	}
	dt.multiHandler = newMultiSelectHandler(dt)
	dt.list = widget.NewList(
		func() int { return len(dt.records) },
		func() fyne.CanvasObject { return newDownloadRow() },
		func(id widget.ListItemID, obj fyne.CanvasObject) { dt.updateRow(id, obj.(*downloadRow)) },
	)
	dt.list.SetItemHeight(0, downloadRowHeight)
	dt.list.OnSelected = func(id widget.ListItemID) {
		if id < len(dt.records) && dt.onSelect != nil {
			dt.onSelect(dt.records[id].ID)
		}
	}
	dt.buildHeader()
	dt.root = container.NewBorder(dt.header, nil, nil, nil, dt.list)
	return dt
}

func setLabelText(label *widget.Label, text string) {
	if label.Text != text {
		label.SetText(text)
	}
}

func (dt *DownloadTable) updateRow(id widget.ListItemID, row *downloadRow) {
	if id >= len(dt.records) {
		return
	}
	rec := dt.records[id]
	if dt.multiHandler.mode {
		if row.check.Hidden {
			row.check.Show()
		}
	} else if !row.check.Hidden {
		row.check.Hide()
	}
	icon := fileIcon(rec.Filename)
	if row.icon.Resource == nil || row.icon.Resource.Name() != icon.Name() {
		row.icon.SetResource(icon)
	}
	setLabelText(row.name, rec.Filename)
	setLabelText(row.meta, rowMetaText(rec))
	row.progress.SetValue(progressOf(rec))
	setLabelText(row.status, statusText(rec.Status))

	speedText, etaText := "", ""
	if speed := dt.speeds[rec.ID]; speed > 0 {
		speedText = FormatSize(int64(speed)) + "/s"
		remaining := rec.TotalSize - rec.DownloadedSize
		if remaining > 0 {
			etaText = "ETA " + formatDuration(time.Duration(float64(remaining)/speed)*time.Second)
		}
	}
	setLabelText(row.speed, speedText)
	setLabelText(row.eta, etaText)
	row.check.OnChanged = nil
	selected := dt.multiHandler.isSelected(rec.ID)
	if row.check.Checked != selected {
		row.check.SetChecked(selected)
	}
	row.check.OnChanged = func(checked bool) {
		if checked != dt.multiHandler.isSelected(rec.ID) {
			dt.multiHandler.toggle(rec.ID)
		}
	}
	row.actions.OnTapped = func() { dt.showRowMenu(rec, row.actions) }
	row.Refresh()
}

func (dt *DownloadTable) buildHeader() {
	buttons := make([]fyne.CanvasObject, 0, len(visibleSortColumns)+1)
	for _, item := range visibleSortColumns {
		item := item
		btn := widget.NewButton(item.label, func() { dt.toggleSort(item.col) })
		btn.Importance = widget.LowImportance
		buttons = append(buttons, btn)
	}
	sortMenu := widget.NewButtonWithIcon("Sort", theme.MenuIcon(), nil)
	sortMenu.Importance = widget.LowImportance
	sortMenu.OnTapped = func() { dt.showSortMenu(sortMenu) }
	buttons = append(buttons, sortMenu)
	dt.selectButton = widget.NewButton("Select", func() {
		dt.multiHandler.toggleMode()
		if dt.multiHandler.mode {
			dt.selectButton.SetText("Done")
		} else {
			dt.selectButton.SetText("Select")
		}
		dt.list.Refresh()
	})
	dt.selectButton.Importance = widget.LowImportance
	dt.header = container.NewBorder(nil, widget.NewSeparator(), nil, container.NewHBox(dt.selectButton, sortMenu), container.New(layout.NewCustomPaddedLayout(4, 4, 10, 10), container.NewHBox(buttons[:len(buttons)-1]...)))
}

func fileIcon(filename string) fyne.Resource {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".mp3", ".flac", ".aac", ".ogg", ".wav", ".m4a", ".opus":
		return theme.FileAudioIcon()
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".ico":
		return theme.FileImageIcon()
	case ".mp4", ".mkv", ".avi", ".mov", ".webm", ".m4v":
		return theme.FileVideoIcon()
	case ".exe", ".msi", ".appimage", ".apk":
		return theme.FileApplicationIcon()
	case ".txt", ".md", ".pdf", ".doc", ".docx", ".odt":
		return theme.FileTextIcon()
	default:
		return theme.FileIcon()
	}
}

func (dt *DownloadTable) toggleSort(col store.TableColumn) {
	if col == dt.sortCol {
		dt.sortAsc = !dt.sortAsc
	} else {
		dt.sortCol, dt.sortAsc = col, true
	}
	if dt.onSort != nil {
		dt.onSort(dt.sortCol, dt.sortAsc)
	}
}

func (dt *DownloadTable) showSortMenu(anchor fyne.CanvasObject) {
	if dt.window == nil {
		return
	}
	items := []*fyne.MenuItem{}
	for _, item := range []struct {
		label string
		col   store.TableColumn
	}{{"Size", store.ColSize}, {"Date Added", store.ColAdded}} {
		item := item
		items = append(items, fyne.NewMenuItem(item.label+" ascending", func() { dt.applySort(item.col, true) }), fyne.NewMenuItem(item.label+" descending", func() { dt.applySort(item.col, false) }))
	}
	canvas := fyne.CurrentApp().Driver().CanvasForObject(anchor)
	if canvas == nil {
		return
	}
	pop := widget.NewPopUpMenu(fyne.NewMenu("Sort", items...), canvas)
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(anchor)
	pop.ShowAtPosition(fyne.NewPos(pos.X-anchor.Size().Width, pos.Y+anchor.Size().Height))
}

func (dt *DownloadTable) SetWindow(w fyne.Window) { dt.window = w }

func (dt *DownloadTable) SetSpeedLimitHandler(fn func(int64, int64)) { dt.onSpeedLimit = fn }

func (dt *DownloadTable) applySort(col store.TableColumn, asc bool) {
	dt.sortCol, dt.sortAsc = col, asc
	if dt.onSort != nil {
		dt.onSort(col, asc)
	}
}

func (dt *DownloadTable) SetRecords(records []*storage.DownloadRecord) {
	dt.SetData(records, nil)
}

// SetData replaces records and speeds in a single UI pass so callers doing
// both do not trigger two widget.List refreshes per tick.
func (dt *DownloadTable) SetData(records []*storage.DownloadRecord, speeds map[int64]float64) {
	fyne.Do(func() {
		dt.records = records
		if speeds != nil {
			dt.speeds = speeds
		}
		valid := make(map[int64]bool, len(records))
		for i, rec := range records {
			valid[rec.ID] = true
			dt.list.SetItemHeight(widget.ListItemID(i), downloadRowHeight)
		}
		for id := range dt.multiHandler.selected {
			if !valid[id] {
				delete(dt.multiHandler.selected, id)
			}
		}
		dt.list.Length = func() int { return len(dt.records) }
		dt.list.Refresh()
	})
}

func (dt *DownloadTable) SetSpeeds(speeds map[int64]float64) {
	fyne.Do(func() { dt.speeds = speeds; dt.list.Refresh() })
}

func (dt *DownloadTable) Widget() fyne.CanvasObject { return dt.root }

func (dt *DownloadTable) showRowMenu(rec *storage.DownloadRecord, btn *widget.Button) {
	if dt.window == nil {
		return
	}
	items := dt.rowMenuItems(rec)
	canvas := fyne.CurrentApp().Driver().CanvasForObject(btn)
	if canvas == nil {
		return
	}
	pop := widget.NewPopUpMenu(fyne.NewMenu("", items...), canvas)
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(btn)
	pop.ShowAtPosition(fyne.NewPos(pos.X-btn.Size().Width*2, pos.Y+btn.Size().Height))
}

// rowMenuItems builds the action menu for one row. Split out from showRowMenu so
// the menu contents can be asserted without a driver or canvas.
func (dt *DownloadTable) rowMenuItems(rec *storage.DownloadRecord) []*fyne.MenuItem {
	fire := func(action string) func() {
		return func() {
			if dt.onAction != nil {
				dt.onAction(rec.ID, action)
			}
		}
	}
	items := []*fyne.MenuItem{}
	selectedIDs := dt.multiHandler.getSelectedIDs()
	if len(selectedIDs) > 1 {
		fireSelected := func(action string) func() {
			return func() {
				for _, id := range selectedIDs {
					if dt.onAction != nil {
						dt.onAction(id, action)
					}
				}
				dt.multiHandler.clearSelection()
				dt.list.Refresh()
			}
		}
		items = append(items,
			fyne.NewMenuItemWithIcon("Pause selected", theme.MediaPauseIcon(), fireSelected("pause")),
			fyne.NewMenuItemWithIcon("Resume selected", theme.MediaPlayIcon(), fireSelected("resume")),
			fyne.NewMenuItemWithIcon("Cancel selected", theme.CancelIcon(), func() {
				if dt.onBulkAction != nil {
					dt.onBulkAction(selectedIDs, "cancel")
				}
				dt.multiHandler.clearSelection()
				dt.list.Refresh()
			}),
			fyne.NewMenuItemWithIcon("Remove selected", theme.DeleteIcon(), func() {
				if dt.onBulkAction != nil {
					dt.onBulkAction(selectedIDs, "delete")
				}
				dt.multiHandler.clearSelection()
				dt.list.Refresh()
			}),
			fyne.NewMenuItemSeparator(),
		)
	}
	switch rec.Status {
	case "downloading":
		items = append(items,
			fyne.NewMenuItemWithIcon("Pause", theme.MediaPauseIcon(), fire("pause")),
			fyne.NewMenuItemWithIcon("Cancel", theme.CancelIcon(), fire("cancel")),
		)
	case "completed":
		// Open File opens the file. Re-fetching is a separate, destructive
		// action with its own confirmation; it must never be what a user gets
		// when they ask to open something.
		items = append(items,
			fyne.NewMenuItemWithIcon("Open File", theme.FileIcon(), fire("open_file")),
			fyne.NewMenuItemWithIcon("Download Again…", theme.ViewRefreshIcon(), fire("download_again")),
		)
	case "cancelled":
		items = append(items,
			fyne.NewMenuItemWithIcon("Download Again…", theme.ViewRefreshIcon(), fire("download_again")),
			fyne.NewMenuItemWithIcon("Cancel", theme.CancelIcon(), fire("cancel")),
		)
	default:
		items = append(items,
			fyne.NewMenuItemWithIcon("Resume", theme.MediaPlayIcon(), fire("resume")),
			fyne.NewMenuItemWithIcon("Cancel", theme.CancelIcon(), fire("cancel")),
		)
	}
	// Reordering only means something while a download is still waiting its turn.
	if rec.Status == "queued" {
		items = append(items,
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItemWithIcon("Move Up in Queue", theme.MoveUpIcon(), fire("queue_up")),
			fyne.NewMenuItemWithIcon("Move Down in Queue", theme.MoveDownIcon(), fire("queue_down")),
		)
	}
	items = append(items,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItemWithIcon("Copy URL", theme.ContentCopyIcon(), func() { dt.window.Clipboard().SetContent(rec.URL) }),
		fyne.NewMenuItemWithIcon("Open Folder", theme.FolderOpenIcon(), fire("open_folder")),
		fyne.NewMenuItemWithIcon("Set Speed Limit…", theme.SettingsIcon(), func() { dt.showSpeedLimitDialog(rec) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItemWithIcon("Remove from List", theme.DeleteIcon(), fire("delete")),
	)
	return items
}

func (dt *DownloadTable) showSpeedLimitDialog(rec *storage.DownloadRecord) {
	if dt.window == nil || dt.onSpeedLimit == nil {
		return
	}
	entry := widget.NewEntry()
	entry.SetPlaceHolder("0 (unlimited)")
	if rec.SpeedLimit > 0 {
		entry.SetText(strconv.FormatInt(rec.SpeedLimit/1024, 10))
	}
	dialog.ShowForm("Set Speed Limit", "Apply", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Speed limit (KB/s)", entry),
		widget.NewFormItem("Applies to", widget.NewLabel(rec.Filename)),
	}, func(ok bool) {
		if !ok {
			return
		}
		kb, err := strconv.ParseInt(entry.Text, 10, 64)
		if err != nil || kb < 0 {
			ShowError(dt.window, "Enter a non-negative speed limit in KB/s.")
			return
		}
		dt.onSpeedLimit(rec.ID, kb*1024)
	}, dt.window)
}
