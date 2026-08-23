package components

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/lyravein/lunefetch/internal/filecat"
	"github.com/lyravein/lunefetch/internal/ui/store"
)

type SidebarItem struct {
	Label  string
	Filter store.DownloadStatus
}

type categoryItem struct{ Label string }

// sidebarWidth is the fixed width of the navigation rail.
const sidebarWidth float32 = 260

var categoryItems = func() []categoryItem {
	items := filecat.All()
	result := make([]categoryItem, 0, len(items)+1)
	result = append(result, categoryItem{Label: "All"})
	for _, category := range items {
		result = append(result, categoryItem{Label: string(category)})
	}
	return result
}()

var sidebarItems = []SidebarItem{
	{"All Downloads", store.StatusAll},
	{"Downloading", store.StatusDownloading},
	{"Queued", store.StatusQueued},
	{"Scheduled", store.StatusScheduled},
	{"Completed", store.StatusCompleted},
	{"Paused", store.StatusPaused},
	{"Failed", store.StatusFailed},
}

type Sidebar struct {
	container  fyne.CanvasObject
	filters    []*widget.Button
	categories []*widget.Button
	selected   int
	onSelect   func(store.DownloadStatus)
	onCategory func(string)
	onSettings func()
	onHistory  func()
	onAbout    func()
}

// NewSidebar builds the navigation rail. Routes are plain buttons inside one
// scroll area rather than nested widget.List scrollers: the rail has a small,
// fixed set of rows, and nested scrollers previously clipped or leaked rows at
// the 1000x640 minimum window size.
func NewSidebar(onSelect func(store.DownloadStatus), onCategory func(string), onHistory func()) *Sidebar {
	sb := &Sidebar{selected: 0, onSelect: onSelect, onCategory: onCategory, onHistory: onHistory}

	filterRows := make([]fyne.CanvasObject, 0, len(sidebarItems))
	sb.filters = make([]*widget.Button, 0, len(sidebarItems))
	for i, item := range sidebarItems {
		i, item := i, item
		btn := widget.NewButtonWithIcon(item.Label, iconForFilter(item.Filter), func() {
			sb.selectFilterAt(i, true)
		})
		btn.Alignment = widget.ButtonAlignLeading
		btn.IconPlacement = widget.ButtonIconLeadingText
		btn.Importance = widget.LowImportance
		sb.filters = append(sb.filters, btn)
		filterRows = append(filterRows, btn)
	}

	categoryRows := make([]fyne.CanvasObject, 0, len(categoryItems))
	sb.categories = make([]*widget.Button, 0, len(categoryItems))
	for i, item := range categoryItems {
		i, item := i, item
		btn := widget.NewButtonWithIcon(item.Label, theme.FolderIcon(), func() {
			sb.selectCategoryAt(i)
			category := item.Label
			if category == "All" {
				category = ""
			}
			if sb.onCategory != nil {
				sb.onCategory(category)
			}
		})
		btn.Alignment = widget.ButtonAlignLeading
		btn.IconPlacement = widget.ButtonIconLeadingText
		btn.Importance = widget.LowImportance
		sb.categories = append(sb.categories, btn)
		categoryRows = append(categoryRows, btn)
	}
	sb.markSelected()

	background := canvas.NewRectangle(theme.Color(theme.ColorNameHeaderBackground))
	background.SetMinSize(fyne.NewSize(sidebarWidth, 1))

	brand := widget.NewLabelWithStyle("Lunefetch", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Download Manager")
	subtitle.Importance = widget.LowImportance
	header := container.New(layout.NewCustomPaddedLayout(16, 10, 16, 16), container.NewVBox(brand, subtitle))

	categoryHeading := widget.NewLabelWithStyle("CATEGORIES", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	categoryHeading.Importance = widget.LowImportance

	navRows := make([]fyne.CanvasObject, 0, len(filterRows)+len(categoryRows)+2)
	navRows = append(navRows, filterRows...)
	navRows = append(navRows, widget.NewSeparator())
	navRows = append(navRows, container.New(layout.NewCustomPaddedLayout(8, 2, 6, 6), categoryHeading))
	navRows = append(navRows, categoryRows...)
	nav := container.New(layout.NewCustomPaddedLayout(0, 8, 12, 12), container.NewVBox(navRows...))
	navScroll := container.NewVScroll(nav)

	settingsButton := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), func() {
		if sb.onSettings != nil {
			sb.onSettings()
		}
	})
	historyButton := widget.NewButtonWithIcon("History", theme.HistoryIcon(), func() {
		sb.clearSelection()
		if sb.onHistory != nil {
			sb.onHistory()
		}
	})
	aboutButton := widget.NewButtonWithIcon("About", theme.InfoIcon(), func() {
		if sb.onAbout != nil {
			sb.onAbout()
		}
	})

	// The footer is pinned below a scrolling nav area, so it needs its own opaque
	// surface and a divider; otherwise scrolled rows show through behind it.
	footerBackground := canvas.NewRectangle(theme.Color(theme.ColorNameHeaderBackground))
	footerContent := container.New(layout.NewCustomPaddedLayout(10, 12, 16, 16), container.NewVBox(settingsButton, historyButton, aboutButton))
	footer := container.NewStack(footerBackground, container.NewBorder(widget.NewSeparator(), nil, nil, nil, footerContent))

	sb.container = container.NewStack(background, container.NewBorder(header, footer, nil, nil, navScroll))
	return sb
}

func (sb *Sidebar) Container() fyne.CanvasObject { return sb.container }

func (sb *Sidebar) SetFooterActions(onSettings, onAbout func()) {
	sb.onSettings = onSettings
	sb.onAbout = onAbout
}

// SelectFilter activates a status route, mirroring a user click.
func (sb *Sidebar) SelectFilter(status store.DownloadStatus) {
	for i, item := range sidebarItems {
		if item.Filter == status {
			sb.selectFilterAt(i, true)
			return
		}
	}
}

func (sb *Sidebar) selectFilterAt(index int, notify bool) {
	if index < 0 || index >= len(sidebarItems) {
		return
	}
	sb.selected = index
	sb.markSelected()
	if notify && sb.onSelect != nil {
		sb.onSelect(sidebarItems[index].Filter)
	}
}

func (sb *Sidebar) selectCategoryAt(index int) {
	sb.selected = -1
	sb.markSelected()
	if index >= 0 && index < len(sb.categories) {
		sb.categories[index].Importance = widget.MediumImportance
		sb.categories[index].Refresh()
	}
}

func (sb *Sidebar) clearSelection() {
	sb.selected = -1
	sb.markSelected()
}

// markSelected paints the active route. Fyne buttons have no dedicated selected
// state, so medium importance is used as the active surface.
func (sb *Sidebar) markSelected() {
	for i, btn := range sb.filters {
		want := widget.LowImportance
		if i == sb.selected {
			want = widget.MediumImportance
		}
		if btn.Importance != want {
			btn.Importance = want
			btn.Refresh()
		}
	}
	for _, btn := range sb.categories {
		if btn.Importance != widget.LowImportance {
			btn.Importance = widget.LowImportance
			btn.Refresh()
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
