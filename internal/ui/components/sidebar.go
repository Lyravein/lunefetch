package components

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/lyravein/lunefetch/internal/filecat"
	"github.com/lyravein/lunefetch/internal/storage"
	"github.com/lyravein/lunefetch/internal/ui/store"
)

// SidebarItem represents one entry in the sidebar.
type SidebarItem struct {
	Label  string
	Filter store.DownloadStatus
}

type categoryItem struct{ Label string }

var categoryItems = func() []categoryItem {
	items := filecat.All()
	result := make([]categoryItem, len(items))
	for i, category := range items {
		result[i] = categoryItem{Label: string(category)}
	}
	return result
}()

var sidebarItems = []SidebarItem{
	{"All", store.StatusAll},
	{"Downloading", store.StatusDownloading},
	{"Paused", store.StatusPaused},
	{"Queue", store.StatusQueued},
	{"Scheduled", store.StatusScheduled},
	{"Failed", store.StatusFailed},
	{"Completed", store.StatusCompleted},
}

// Sidebar is a fixed-width navigation panel that filters the download list.
type Sidebar struct {
	container      fyne.CanvasObject
	list           *widget.List
	categoryList   *widget.List
	selected       int
	counts         map[store.DownloadStatus]int
	categoryCounts map[string]int
	onSelect       func(store.DownloadStatus)
	onCategory     func(string)
}

// NewSidebar creates a sidebar with fixed width.
func NewSidebar(onSelect func(store.DownloadStatus), onCategory func(string), onHistory func()) *Sidebar {
	sb := &Sidebar{
		selected:       0,
		counts:         make(map[store.DownloadStatus]int),
		categoryCounts: make(map[string]int),
		onSelect:       onSelect,
		onCategory:     onCategory,
	}

	sb.categoryList = widget.NewList(
		func() int { return len(categoryItems) },
		func() fyne.CanvasObject {
			icon := widget.NewIcon(theme.FolderIcon())
			label := widget.NewLabel("category")
			badge := widget.NewLabel("")
			badge.Alignment = fyne.TextAlignTrailing
			return container.NewHBox(icon, label, layout.NewSpacer(), badge)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			hbox := o.(*fyne.Container)
			label := hbox.Objects[1].(*widget.Label)
			badge := hbox.Objects[3].(*widget.Label)
			label.SetText(categoryItems[i].Label)
			if count := sb.categoryCounts[categoryItems[i].Label]; count > 0 {
				badge.SetText(fmt.Sprintf("%d", count))
			} else {
				badge.SetText("")
			}
		},
	)
	for i := range categoryItems {
		sb.categoryList.SetItemHeight(i, 36)
	}
	sb.categoryList.OnSelected = func(i widget.ListItemID) {
		sb.list.UnselectAll()
		if sb.onCategory != nil {
			sb.onCategory(categoryItems[i].Label)
		}
	}

	sb.list = widget.NewList(
		func() int { return len(sidebarItems) },
		func() fyne.CanvasObject {
			icon := widget.NewIcon(theme.DocumentIcon())
			label := widget.NewLabel("item")
			badge := widget.NewLabel("")
			badge.Alignment = fyne.TextAlignTrailing
			return container.NewHBox(icon, label, layout.NewSpacer(), badge)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			hbox := o.(*fyne.Container)
			// HBox objects: [0]=icon, [1]=label, [2]=spacer, [3]=badge
			icon := hbox.Objects[0].(*widget.Icon)
			label := hbox.Objects[1].(*widget.Label)
			badge := hbox.Objects[3].(*widget.Label)

			item := sidebarItems[i]

			if i == sb.selected {
				label.TextStyle = fyne.TextStyle{Bold: true}
			} else {
				label.TextStyle = fyne.TextStyle{}
			}
			label.SetText(item.Label)
			icon.SetResource(iconForFilter(item.Filter))

			// Show count badge.
			count := sb.counts[item.Filter]
			if item.Filter == store.StatusAll {
				// Sum all statuses for "All".
				var total int
				for _, c := range sb.counts {
					total += c
				}
				count = total
			}
			if count > 0 {
				badge.SetText(fmt.Sprintf("%d", count))
			} else {
				badge.SetText("")
			}
		},
	)
	for i := range sidebarItems {
		sb.list.SetItemHeight(i, 36)
	}

	sb.list.OnSelected = func(i widget.ListItemID) {
		sb.categoryList.UnselectAll()
		sb.selected = i
		sb.list.Refresh()
		if sb.onSelect != nil {
			sb.onSelect(sidebarItems[i].Filter)
		}
	}

	// The transparent background fixes only the sidebar's minimum width.
	// NewGridWrapLayout previously fixed both dimensions to 180x100, which
	// clipped the list and left most of the left side empty.
	background := canvas.NewRectangle(theme.Color(theme.ColorNameHeaderBackground))
	background.SetMinSize(fyne.NewSize(176, 1))
	heading := widget.NewLabelWithStyle("DOWNLOADS", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	heading.Importance = widget.LowImportance
	categoryHeading := widget.NewLabelWithStyle("CATEGORIES", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	categoryHeading.Importance = widget.LowImportance
	header := container.New(layout.NewCustomPaddedLayout(14, 8, 12, 12), heading)
	list := container.New(layout.NewCustomPaddedLayout(0, 8, 6, 6), sb.list)
	categoryListSize := canvas.NewRectangle(color.Transparent)
	categoryListSize.SetMinSize(fyne.NewSize(1, float32(len(categoryItems))*36))
	categoryList := container.NewStack(categoryListSize, sb.categoryList)
	categories := container.NewBorder(container.New(layout.NewCustomPaddedLayout(14, 8, 12, 12), categoryHeading), nil, nil, nil, categoryList)
	historyButton := widget.NewButtonWithIcon("History", theme.HistoryIcon(), func() {
		sb.list.UnselectAll()
		sb.categoryList.UnselectAll()
		if onHistory != nil {
			onHistory()
		}
	})
	history := container.New(layout.NewCustomPaddedLayout(8, 8, 12, 12), historyButton)
	sb.container = container.NewStack(background, container.NewBorder(header, container.NewBorder(nil, history, nil, nil, categories), nil, nil, list))

	return sb
}

// Container returns the sidebar canvas object.
func (sb *Sidebar) Container() fyne.CanvasObject { return sb.container }

// UpdateCounts refreshes badge counts from the given records.
// Called from refreshLoop — does not query DB directly.
func (sb *Sidebar) UpdateCounts(records []storage.DownloadRecord) {
	counts := make(map[store.DownloadStatus]int)
	categoryCounts := make(map[string]int)
	for i := range records {
		r := &records[i]
		counts[store.DownloadStatus(r.Status)]++
		categoryCounts[r.Category]++
	}
	fyne.Do(func() {
		sb.counts = counts
		sb.categoryCounts = categoryCounts
		sb.list.Refresh()
		sb.categoryList.Refresh()
	})
}

// SelectFilter programmatically selects the sidebar entry matching status.
func (sb *Sidebar) SelectFilter(status store.DownloadStatus) {
	for i, item := range sidebarItems {
		if item.Filter == status {
			sb.selected = i
			sb.list.Select(i)
			return
		}
	}
}

func iconForFilter(f store.DownloadStatus) fyne.Resource {
	switch f {
	case store.StatusAll:
		return theme.ListIcon()
	case store.StatusDownloading:
		return theme.DownloadIcon()
	case store.StatusPaused:
		return theme.MediaPauseIcon()
	case store.StatusQueued:
		return theme.StorageIcon()
	case store.StatusScheduled:
		return theme.HistoryIcon()
	case store.StatusCompleted:
		return theme.ConfirmIcon()
	case store.StatusFailed:
		return theme.WarningIcon()
	default:
		return theme.DocumentIcon()
	}
}
