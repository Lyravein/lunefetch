package components

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/lyravein/lunefetch/internal/storage"
	"github.com/lyravein/lunefetch/internal/ui/store"
)

// colHeaders defines the display name for each column.
var colHeaders = []string{"#", "Name", "Size", "Progress", "Speed", "Status", "Added", ""}

// colWidths defines the initial width for each column.
var colWidths = []float32{34, 350, 80, 160, 80, 80, 120, 10}

// colMinWidths defines the minimum width used by responsive resizing.
var colMinWidths = []float32{28, 60, 42, 60, 44, 66, 42, 10}

// headerHeight is the height of the custom (always visible) header row.
const headerHeight = 34

// Fyne's table renderer adds theme padding between cells. The custom header
// must use the same gap or its columns drift away from the table columns.
const tableScrollbarWidth = 16

// colToTableCol maps column index (0-based) to store.TableColumn.
// Speed is runtime-only and the actions column carries no store data.
var colToTableCol = []store.TableColumn{
	store.ColAdded, // row numbers follow the default Added ordering
	store.ColName,
	store.ColSize,
	store.ColProgress,
	store.ColSpeed, // placeholder — runtime speed is not sortable
	store.ColStatus,
	store.ColAdded,
	store.ColStatus, // placeholder — actions are not sortable
}

// isSortableCol reports whether clicking the header of column i toggles sort.
// Speed is runtime-only and cannot be sorted through the store.
func isSortableCol(i int) bool {
	switch i {
	case 0, 1, 2, 3, 5, 6:
		return true
	default:
		return false
	}
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

func statusColor(status string) color.Color {
	switch status {
	case "downloading":
		return theme.Color(theme.ColorNamePrimary)
	case "completed":
		return theme.Color(theme.ColorNameSuccess)
	case "failed", "cancelled":
		return theme.Color(theme.ColorNameError)
	case "paused", "scheduled":
		return theme.Color(theme.ColorNameWarning)
	default:
		return theme.Color(theme.ColorNamePlaceHolder)
	}
}

// humanDate formats a time.Time into a friendly string.
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

// tapLabel is a bold label that calls onTap when clicked.
type tapLabel struct {
	widget.Label
	onTap func()
}

func newTapLabel(onTap func()) *tapLabel {
	l := &tapLabel{onTap: onTap}
	l.TextStyle = fyne.TextStyle{Bold: true}
	l.Truncation = fyne.TextTruncateEllipsis
	l.ExtendBaseWidget(l)
	return l
}

// Tapped implements fyne.Tappable.
func (l *tapLabel) Tapped(*fyne.PointEvent) {
	if l.onTap != nil {
		l.onTap()
	}
}

// headerLayout positions header cells according to the current column widths.
type headerLayout struct {
	dt *DownloadTable
}

// Layout implements fyne.Layout.
func (l *headerLayout) Layout(objects []fyne.CanvasObject, _ fyne.Size) {
	n := len(colHeaders)
	x := float32(0)
	gap := theme.Size(theme.SizeNamePadding)
	for i := 0; i < n; i++ {
		w := l.dt.widths[i]
		objects[i].Move(fyne.NewPos(x, 0))
		objects[i].Resize(fyne.NewSize(w, headerHeight))
		x += w
		if i < n-1 {
			x += gap
		}
	}
}

// MinSize implements fyne.Layout.
//
// IMPORTANT: this must NOT report the summed column widths. The header sits
// in a Border's top slot, and a width-hungry MinSize propagates up to the
// window, forcing it to grow — which feeds back into SetAvailableWidth and
// grows the columns again in an endless loop. Report a minimal width and
// let the Border stretch us instead.
func (l *headerLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(1, headerHeight)
}

// DownloadTable wraps widget.Table with a custom sortable header row.
type DownloadTable struct {
	table  *widget.Table
	header *fyne.Container
	root   *fyne.Container

	records  []*storage.DownloadRecord
	speeds   map[int64]float64 // id → bytes/sec
	selected int               // selected row index, -1 = none
	window   fyne.Window       // for popup menus + clipboard

	widths    []float32
	lastWidth float32

	sortCol store.TableColumn
	sortAsc bool

	headerLabels []*tapLabel

	onSort   func(col store.TableColumn, asc bool)
	onSelect func(id int64)
	onAction func(id int64, action string)
}

// NewDownloadTable creates a new DownloadTable.
func NewDownloadTable(
	onSort func(col store.TableColumn, asc bool),
	onSelect func(id int64),
	onAction func(id int64, action string),
) *DownloadTable {
	dt := &DownloadTable{
		speeds:   make(map[int64]float64),
		selected: -1,
		sortCol:  store.ColAdded,
		sortAsc:  true,
		onSort:   onSort,
		onSelect: onSelect,
		onAction: onAction,
		widths:   append([]float32(nil), colWidths...),
	}

	dt.table = widget.NewTable(
		func() (int, int) { return len(dt.records), len(colHeaders) },

		// Template cell: progress bar, labels and action button stacked; visibility
		// is controlled per cell in the update func.
		func() fyne.CanvasObject {
			bar := widget.NewProgressBar()
			bar.Min = 0
			bar.Max = 1
			lbl := widget.NewLabel("")
			lbl.Truncation = fyne.TextTruncateEllipsis
			status := canvas.NewText("", theme.Color(theme.ColorNameForeground))
			status.Alignment = fyne.TextAlignLeading
			status.TextSize = theme.Size(theme.SizeNameText)
			btn := widget.NewButtonWithIcon("", theme.MoreVerticalIcon(), nil)
			btn.Importance = widget.LowImportance
			return container.NewStack(bar, lbl, status, btn)
		},

		func(id widget.TableCellID, o fyne.CanvasObject) {
			c := o.(*fyne.Container)
			bar := c.Objects[0].(*widget.ProgressBar)
			lbl := c.Objects[1].(*widget.Label)
			status := c.Objects[2].(*canvas.Text)
			btn := c.Objects[3].(*widget.Button)

			row := id.Row
			if row >= len(dt.records) {
				bar.Hide()
				status.Hide()
				btn.Hide()
				lbl.SetText("")
				return
			}
			rec := dt.records[row]
			lbl.TextStyle = fyne.TextStyle{}
			lbl.Alignment = fyne.TextAlignLeading
			status.Hide()

			// Action column — show only the ⋮ button.
			if id.Col == len(colHeaders)-1 {
				bar.Hide()
				lbl.Hide()
				btn.Show()
				btn.OnTapped = func() { dt.showRowMenu(rec, btn) }
				return
			}
			btn.Hide()

			switch id.Col {
			case 0: // Row number
				bar.Hide()
				lbl.Show()
				lbl.Alignment = fyne.TextAlignTrailing
				lbl.SetText(fmt.Sprintf("%d", row+1))

			case 1: // Name
				bar.Hide()
				lbl.Show()
				lbl.SetText(rec.Filename)

			case 2: // Size
				bar.Hide()
				lbl.Show()
				lbl.Alignment = fyne.TextAlignTrailing
				lbl.SetText(FormatSize(rec.TotalSize))

			case 3: // Progress
				if rec.TotalSize > 0 {
					pct := float64(rec.DownloadedSize) / float64(rec.TotalSize)
					bar.SetValue(pct)
					bar.Show()
					lbl.Hide()
				} else {
					bar.Hide()
					lbl.Show()
					lbl.SetText("—")
				}

			case 4: // Speed
				bar.Hide()
				lbl.Show()
				lbl.Alignment = fyne.TextAlignTrailing
				if spd, ok := dt.speeds[rec.ID]; ok && spd > 0 {
					lbl.SetText(FormatSize(int64(spd)) + "/s")
				} else {
					lbl.SetText("—")
				}

			case 5: // Status
				bar.Hide()
				lbl.Hide()
				status.Text = statusText(rec.Status)
				status.Color = statusColor(rec.Status)
				status.Show()
				status.Refresh()

			case 6: // Added
				bar.Hide()
				lbl.Show()
				lbl.Alignment = fyne.TextAlignTrailing
				lbl.SetText(humanDate(rec.CreatedAt))
			}
		},
	)

	dt.table.OnSelected = func(id widget.TableCellID) {
		if id.Row < len(dt.records) {
			dt.selected = id.Row
			if dt.onSelect != nil {
				dt.onSelect(dt.records[id.Row].ID)
			}
		}
	}
	dt.table.OnUnselected = func(widget.TableCellID) {
		dt.selected = -1
	}

	for i, w := range dt.widths {
		dt.table.SetColumnWidth(i, w)
	}

	dt.buildHeader()

	dt.root = container.NewBorder(dt.header, nil, nil, nil, dt.table)
	return dt
}

// buildHeader constructs the custom header row with clickable sort labels.
func (dt *DownloadTable) buildHeader() {
	n := len(colHeaders)
	objects := make([]fyne.CanvasObject, 0, n)
	dt.headerLabels = make([]*tapLabel, n)

	for i := 0; i < n; i++ {
		col := i
		var lbl *tapLabel
		if isSortableCol(col) {
			lbl = newTapLabel(func() { dt.toggleSort(col) })
		} else {
			lbl = newTapLabel(nil)
			lbl.TextStyle = fyne.TextStyle{}
		}
		dt.headerLabels[col] = lbl
		if col == 0 || col == 2 || col == 3 || col == 4 || col == 6 {
			lbl.Alignment = fyne.TextAlignTrailing
		}
		objects = append(objects, container.New(layout.NewCustomPaddedLayout(0, 3, 4, 4), lbl))
	}

	headerContent := container.New(&headerLayout{dt: dt}, objects...)
	headerBackground := canvas.NewRectangle(theme.Color(theme.ColorNameHeaderBackground))
	dt.header = container.NewStack(headerBackground, headerContent)
	dt.refreshHeaderLabels()
}

// toggleSort flips or switches the sort column and notifies the listener.
func (dt *DownloadTable) toggleSort(col int) {
	tc := colToTableCol[col]
	if tc == dt.sortCol {
		dt.sortAsc = !dt.sortAsc
	} else {
		dt.sortCol = tc
		dt.sortAsc = true
	}
	dt.refreshHeaderLabels()
	if dt.onSort != nil {
		dt.onSort(dt.sortCol, dt.sortAsc)
	}
}

// refreshHeaderLabels redraws header text with sort direction arrows.
func (dt *DownloadTable) refreshHeaderLabels() {
	for i, lbl := range dt.headerLabels {
		text := colHeaders[i]
		if isSortableCol(i) && colToTableCol[i] == dt.sortCol {
			if dt.sortAsc {
				text += " ▲"
			} else {
				text += " ▼"
			}
		}
		lbl.SetText(text)
	}
}

// applyWidths pushes the width slice to both the table and the header.
func (dt *DownloadTable) applyWidths() {
	for i, w := range dt.widths {
		dt.table.SetColumnWidth(i, w)
	}
	dt.header.Refresh()
}

// SetWindow injects the parent window (needed for popup menus + clipboard).
func (dt *DownloadTable) SetWindow(w fyne.Window) { dt.window = w }

// SetRecords updates the table data and redraws.
func (dt *DownloadTable) SetRecords(records []*storage.DownloadRecord) {
	fyne.Do(func() {
		dt.records = records
		for row := range records {
			dt.table.SetRowHeight(row, 38)
		}
		dt.table.Refresh()
	})
}

// SetSpeeds updates the speed map and redraws.
func (dt *DownloadTable) SetSpeeds(speeds map[int64]float64) {
	fyne.Do(func() {
		dt.speeds = speeds
		dt.table.Refresh()
	})
}

// SetAvailableWidth scales the configured column widths to fill the table.
// colWidths therefore remains the visible proportion source instead of only
// acting as an initial value that gets overwritten by the Name column.
func (dt *DownloadTable) SetAvailableWidth(width float32) {
	if width < 1 || width == dt.lastWidth {
		return
	}
	dt.lastWidth = width

	dt.widths = proportionalWidths(width, colWidths, colMinWidths)
	fyne.Do(dt.applyWidths)
}

// ContentWidth returns the width needed by the table columns without causing
// widget.Table to create a horizontal scrollbar. Fyne adds one padding gap
// between each pair of columns and reserves space for its vertical scrollbar.
func ContentWidth(width float32) float32 {
	columnGaps := float32(len(colHeaders)-1) * theme.Size(theme.SizeNamePadding)
	return width - tableScrollbarWidth - columnGaps
}

// Widget returns the root canvas object (header + table).
func (dt *DownloadTable) Widget() fyne.CanvasObject { return dt.root }

// showRowMenu pops up a context menu for the given record, anchored to btn.
func (dt *DownloadTable) showRowMenu(rec *storage.DownloadRecord, btn *widget.Button) {
	if dt.window == nil {
		return
	}
	fire := func(action string) func() {
		return func() {
			if dt.onAction != nil {
				dt.onAction(rec.ID, action)
			}
		}
	}

	items := make([]*fyne.MenuItem, 0, 8)
	switch rec.Status {
	case "downloading":
		items = append(items,
			fyne.NewMenuItem("Pause", fire("pause")),
			fyne.NewMenuItem("Cancel", fire("cancel")),
		)
	case "paused", "failed", "cancelled", "queued", "scheduled", "pending":
		items = append(items,
			fyne.NewMenuItem("Resume", fire("resume")),
			fyne.NewMenuItem("Cancel", fire("cancel")),
		)
	case "completed":
		items = append(items,
			fyne.NewMenuItem("Open File", fire("open_file")),
		)
	}
	items = append(items,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Copy URL", func() {
			dt.window.Clipboard().SetContent(rec.URL)
		}),
		fyne.NewMenuItem("Open Folder", fire("open_folder")),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Remove from List", fire("delete")),
	)

	menu := fyne.NewMenu("", items...)
	canvas := fyne.CurrentApp().Driver().CanvasForObject(btn)
	if canvas == nil {
		return
	}
	pop := widget.NewPopUpMenu(menu, canvas)
	// Position the menu at the button's bottom-left corner.
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(btn)
	pop.ShowAtPosition(fyne.NewPos(pos.X-btn.Size().Width*2, pos.Y+btn.Size().Height))
}

// sumWidths returns the total of a width slice.
func sumWidths(ws []float32) float32 {
	total := float32(0)
	for _, w := range ws {
		total += w
	}
	return total
}

// proportionalWidths treats configured widths as relative weights while
// preserving each column's minimum size.
func proportionalWidths(available float32, configured, minimums []float32) []float32 {
	widths := append([]float32(nil), minimums...)
	minimumTotal := sumWidths(minimums)
	if available <= minimumTotal {
		return widths
	}

	extra := available - minimumTotal
	weightTotal := float32(0)
	for i, configuredWidth := range configured {
		weight := configuredWidth - minimums[i]
		if weight > 0 {
			weightTotal += weight
		}
	}
	if weightTotal == 0 {
		return widths
	}

	for i, configuredWidth := range configured {
		weight := configuredWidth - minimums[i]
		if weight > 0 {
			widths[i] += extra * weight / weightTotal
		}
	}
	return widths
}
