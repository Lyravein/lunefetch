package layout

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/lyravein/lunefetch/internal/config"
	"github.com/lyravein/lunefetch/internal/ui/components"
)

func TestMinimumWindowSizeProtectsDesktopShell(t *testing.T) {
	minimum := fyne.NewSize(minWindowWidth, minWindowHeight)
	if minimum.Width < 1000 || minimum.Height < 640 {
		t.Fatalf("minimum window size = %v, want at least 1000x640", minimum)
	}
}

func TestDefaultWindowSizeMatchesMinimumShell(t *testing.T) {
	if minWindowWidth != 1000 || minWindowHeight != 640 {
		t.Fatalf("default shell size = %dx%d, want 1000x640", minWindowWidth, minWindowHeight)
	}
}

func TestMinimumWindowLayoutKeepsRequiredControlsInBounds(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	w := app.NewWindow("layout test")
	defer w.Close()

	cfg := config.Default()
	header := components.NewContentHeader(w, cfg, make(chan components.AddURLRequest), nil)
	summaries := components.NewSummaryCards()
	sidebar := components.NewSidebar(nil, nil, nil)
	status := components.NewStatusBar(cfg, nil)
	table := components.NewDownloadTable(nil, nil, nil, nil)

	sidebarPane := container.NewBorder(nil, nil, nil, widget.NewSeparator(), sidebar.Container())
	mainContent := container.NewBorder(summaries.Root, nil, nil, nil, table.Widget())
	main := container.NewBorder(header.Root, nil, nil, nil, mainContent)
	body := container.NewBorder(nil, nil, sidebarPane, nil, main)
	root := container.NewBorder(nil, status.Container(), nil, nil, body)
	root.Resize(fyne.NewSize(minWindowWidth, minWindowHeight))
	root.Refresh()

	assertContainerBounds(t, root, "desktop")
}

func assertContainerBounds(t *testing.T, parent *fyne.Container, path string) {
	t.Helper()
	parentSize := parent.Size()
	for i, object := range parent.Objects {
		if object == nil || !object.Visible() {
			continue
		}
		position := object.Position()
		size := object.Size()
		if position.X < 0 || position.Y < 0 || size.Width < 0 || size.Height < 0 {
			t.Fatalf("%s[%d] has invalid geometry: position=%v size=%v", path, i, position, size)
		}
		if position.X+size.Width > parentSize.Width+1 || position.Y+size.Height > parentSize.Height+1 {
			t.Fatalf("%s[%d] overflows parent %v: position=%v size=%v", path, i, parentSize, position, size)
		}
		if nested, ok := object.(*fyne.Container); ok {
			assertContainerBounds(t, nested, path)
		}
	}
}
