package components

import (
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/lyravein/lunefetch/internal/config"
)

// ContentHeader contains the primary action and search field for downloads.
type ContentHeader struct {
	Root   fyne.CanvasObject
	Search *widget.Entry
}

func NewContentHeader(w fyne.Window, cfg *config.Config, addCh chan<- AddURLRequest, onSearch func(string)) *ContentHeader {
	search := widget.NewEntry()
	search.SetPlaceHolder("Search downloads")
	search.OnChanged = onSearch
	searchBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(240, 36)), search)

	add := widget.NewButtonWithIcon("New Download", theme.ContentAddIcon(), func() {
		ShowAddURLDialog(w, cfg, addCh)
	})
	add.Importance = widget.HighImportance
	addBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(150, 40)), add)

	hour := time.Now().Hour()
	greeting := "Good evening"
	if hour < 12 {
		greeting = "Good morning"
	} else if hour < 18 {
		greeting = "Good afternoon"
	}
	title := widget.NewLabelWithStyle(greeting, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Manage your downloads with ease.")
	subtitle.Importance = widget.LowImportance
	copy := container.NewVBox(title, subtitle)
	tools := container.NewHBox(searchBox, addBox)
	head := container.NewBorder(nil, nil, copy, tools, layout.NewSpacer())
	background := canvas.NewRectangle(theme.Color(theme.ColorNameBackground))
	content := container.New(layout.NewCustomPaddedLayout(16, 12, 20, 20), head)

	return &ContentHeader{
		Root:   container.NewStack(background, content),
		Search: search,
	}
}

// SummaryCards displays live status counts in a compact dashboard row.
type SummaryCards struct {
	Root  fyne.CanvasObject
	items [4]*widget.Label
}

func NewSummaryCards() *SummaryCards {
	names := [4]string{"Downloading", "Completed", "Paused", "Failed"}
	items := [4]*widget.Label{}
	objects := make([]fyne.CanvasObject, 0, len(names))
	for i, name := range names {
		value := widget.NewLabelWithStyle("0", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		items[i] = value
		label := widget.NewLabel(name)
		label.Importance = widget.LowImportance
		body := container.NewVBox(label, value)
		background := canvas.NewRectangle(theme.Color(theme.ColorNameHeaderBackground))
		objects = append(objects, container.NewStack(background, container.New(layout.NewCustomPaddedLayout(12, 12, 14, 14), body)))
	}
	return &SummaryCards{Root: container.NewGridWithColumns(4, objects...), items: items}
}

func (sc *SummaryCards) Update(counts map[string]int) {
	keys := [...]string{"downloading", "completed", "paused", "failed"}
	fyne.Do(func() {
		for i, key := range keys {
			sc.items[i].SetText(strconv.Itoa(counts[key]))
		}
	})
}
