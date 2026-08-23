package components

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/lyravein/lunefetch/internal/config"
	"github.com/lyravein/lunefetch/internal/storage"
	uitheme "github.com/lyravein/lunefetch/internal/ui/theme"
)

// visualOutDir returns the directory snapshot PNGs are written to. Set
// LUNEFETCH_VISUAL_DIR to inspect the renders; otherwise a temp dir is used so
// normal test runs leave no artefacts behind.
func visualOutDir(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv("LUNEFETCH_VISUAL_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create visual dir: %v", err)
		}
		return dir
	}
	return t.TempDir()
}

// captureCanvas renders content at the given size and writes a PNG snapshot.
func captureCanvas(t *testing.T, name string, content fyne.CanvasObject, size fyne.Size) {
	t.Helper()
	w := test.NewWindow(content)
	defer w.Close()
	// Resize the window only: the test canvas insets content by theme padding,
	// so resizing the content to the full size as well pushed it off the right
	// edge and produced misleading "overflow" in snapshots.
	w.Resize(size)
	content.Refresh()

	img := w.Canvas().Capture()
	if img == nil {
		t.Fatalf("%s: canvas capture returned no image", name)
	}
	path := filepath.Join(visualOutDir(t), name+".png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("%s: create snapshot: %v", name, err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("%s: encode snapshot: %v", name, err)
	}
	t.Logf("%s snapshot: %s", name, path)
}

func visualRecords() []*storage.DownloadRecord {
	return []*storage.DownloadRecord{
		{ID: 1, Filename: "NTR Classroom saborage.mkv", TotalSize: 1_300_000_000, DownloadedSize: 1_300_000_000, Status: "completed"},
		{ID: 2, Filename: "ubuntu-24.04-desktop-amd64.iso", TotalSize: 5_400_000_000, DownloadedSize: 2_100_000_000, Status: "downloading"},
		{ID: 3, Filename: "a-very-long-filename-that-should-be-truncated-instead-of-overflowing-the-row.tar.zst", TotalSize: 900_000_000, DownloadedSize: 12_000_000, Status: "paused"},
	}
}

func newVisualTable(t *testing.T) *DownloadTable {
	t.Helper()
	dt := NewDownloadTable(nil, nil, nil, nil)
	dt.records = visualRecords()
	dt.speeds = map[int64]float64{2: 8_400_000}
	return dt
}

func TestVisualDownloadRowSnapshots(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(uitheme.NewNavy())

	dt := newVisualTable(t)
	for i, rec := range dt.records {
		row := newDownloadRow()
		dt.updateRow(i, row)
		captureCanvas(t, "row-"+rec.Status, row, fyne.NewSize(1180, downloadRowHeight))
	}
}

func TestVisualDownloadListSnapshot(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(uitheme.NewNavy())

	dt := newVisualTable(t)
	dt.list.Length = func() int { return len(dt.records) }
	dt.list.Refresh()
	captureCanvas(t, "download-list", dt.Widget(), fyne.NewSize(1180, 360))
}

func TestVisualHeaderAndSidebarSnapshots(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(uitheme.NewNavy())

	w := test.NewWindow(nil)
	defer w.Close()

	header := NewContentHeader(w, config.Default(), make(chan AddURLRequest, 1), nil)
	captureCanvas(t, "content-header", header.Root, fyne.NewSize(1180, header.Root.MinSize().Height))

	cards := NewSummaryCards()
	cards.Update(map[string]int{"downloading": 1, "completed": 4, "paused": 2, "failed": 0})
	captureCanvas(t, "summary-cards", cards.Root, fyne.NewSize(1180, cards.Root.MinSize().Height))

	sidebar := NewSidebar(nil, nil, nil)
	captureCanvas(t, "sidebar", sidebar.Container(), fyne.NewSize(260, 720))

	status := NewStatusBar(config.Default(), nil)
	captureCanvas(t, "status-bar", status.Container(), fyne.NewSize(1180, status.Container().MinSize().Height))
}

// TestDownloadRowProgressStaysInsideInfoColumn pins the two regressions the
// screenshots exposed: a progress bar that grew tall enough to escape the row,
// and a wrapper that collapsed its width to nothing.
func TestDownloadRowProgressStaysInsideInfoColumn(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(uitheme.NewNavy())

	dt := newVisualTable(t)
	row := newDownloadRow()
	dt.updateRow(1, row)
	rowSize := fyne.NewSize(1180, downloadRowHeight)
	row.Resize(rowSize)
	renderer := test.TempWidgetRenderer(t, row)
	renderer.Layout(rowSize)

	progressRect := absoluteRect(renderer.Objects(), fyne.NewPos(0, 0), row.progress)
	if progressRect == nil {
		t.Fatal("progress bar is not part of the rendered row")
	}
	nameRect := absoluteRect(renderer.Objects(), fyne.NewPos(0, 0), row.name)
	if nameRect == nil {
		t.Fatal("filename label is not part of the rendered row")
	}
	metaRect := absoluteRect(renderer.Objects(), fyne.NewPos(0, 0), row.meta)
	if metaRect == nil {
		t.Fatal("metadata label is not part of the rendered row")
	}

	if got := progressRect.Height; got > 16 {
		t.Fatalf("progress height = %v, want a compact bar", got)
	}
	if progressRect.Width < nameRect.Width {
		t.Fatalf("progress width = %v, want at least the filename width %v", progressRect.Width, nameRect.Width)
	}
	if bottom := progressRect.Y + progressRect.Height; bottom > downloadRowHeight {
		t.Fatalf("progress bottom = %v, want it inside the %v row", bottom, downloadRowHeight)
	}
	if progressRect.Y < metaRect.Y+metaRect.Height {
		t.Fatalf("progress y = %v, want it below the metadata line ending at %v", progressRect.Y, metaRect.Y+metaRect.Height)
	}
}

// TestDesktopShellRendersWithoutOverflow renders the assembled shell so layout
// regressions in the header, sidebar, list, and status bar are caught together.
func TestDesktopShellRendersWithoutOverflow(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(uitheme.NewNavy())

	w := test.NewWindow(nil)
	defer w.Close()

	header := NewContentHeader(w, config.Default(), make(chan AddURLRequest, 1), nil)
	cards := NewSummaryCards()
	sidebar := NewSidebar(nil, nil, nil)
	status := NewStatusBar(config.Default(), nil)
	dt := newVisualTable(t)
	dt.list.Length = func() int { return len(dt.records) }

	mainContent := container.NewBorder(cards.Root, nil, nil, nil, dt.Widget())
	main := container.NewBorder(header.Root, nil, nil, nil, mainContent)
	body := container.NewBorder(nil, nil, sidebar.Container(), nil, main)
	root := container.NewBorder(nil, status.Container(), nil, nil, body)

	captureCanvas(t, "desktop-shell", root, fyne.NewSize(1000, 640))

	if got := root.MinSize(); got.Width > 1000 || got.Height > 640 {
		t.Fatalf("shell minimum size = %v, want it to fit the 1000x640 minimum window", got)
	}
}

// absoluteRect walks the rendered tree and returns target's position relative
// to the row origin, so nested container offsets are accounted for.
type rect struct {
	X, Y, Width, Height float32
}

func absoluteRect(objects []fyne.CanvasObject, origin fyne.Position, target fyne.CanvasObject) *rect {
	for _, object := range objects {
		if object == nil {
			continue
		}
		pos := origin.Add(object.Position())
		if object == target {
			size := object.Size()
			return &rect{X: pos.X, Y: pos.Y, Width: size.Width, Height: size.Height}
		}
		if nested, ok := object.(*fyne.Container); ok {
			if found := absoluteRect(nested.Objects, pos, target); found != nil {
				return found
			}
		}
		if nested, ok := object.(fyne.Widget); ok {
			if found := absoluteRect(test.WidgetRenderer(nested).Objects(), pos, target); found != nil {
				return found
			}
		}
	}
	return nil
}

// TestSidebarKeepsEveryRouteReachable pins the sidebar invariant the snapshots
// exposed: at the minimum window height the nav area must scroll instead of
// silently hiding filters/categories, and it must never overlap the pinned
// footer routes.
func TestSidebarKeepsEveryRouteReachable(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(uitheme.NewNavy())

	sb := NewSidebar(nil, nil, nil)
	root := sb.Container()
	w := test.NewWindow(root)
	defer w.Close()
	size := fyne.NewSize(260, minShellHeight)
	w.Resize(size)
	root.Resize(size)
	root.Refresh()

	scroll := findScroll([]fyne.CanvasObject{root})
	if scroll == nil {
		t.Fatal("sidebar has no scrollable nav area")
	}
	wantRows := len(sidebarItems) + len(categoryItems)
	if got := countButtons([]fyne.CanvasObject{scroll.Content}); got < wantRows {
		t.Fatalf("nav route buttons = %d, want at least %d so every route is reachable", got, wantRows)
	}
	if scroll.Content.MinSize().Height > scroll.Size().Height {
		// The rail is taller than the viewport, which is fine as long as it scrolls.
		if scroll.Direction == container.ScrollNone {
			t.Fatal("nav area overflows but does not scroll")
		}
	}

	footer := lowestButtonBottom([]fyne.CanvasObject{root}, fyne.NewPos(0, 0))
	if footer > size.Height {
		t.Fatalf("footer bottom = %v, want it inside the %v sidebar", footer, size.Height)
	}
	scrollRect := absoluteRect([]fyne.CanvasObject{root}, fyne.NewPos(0, 0), scroll)
	if scrollRect == nil {
		t.Fatal("could not locate the nav scroll area")
	}
	if bottom := scrollRect.Y + scrollRect.Height; bottom > size.Height {
		t.Fatalf("nav scroll bottom = %v, want it inside the %v sidebar", bottom, size.Height)
	}
}

const minShellHeight float32 = 640

func findScroll(objects []fyne.CanvasObject) *container.Scroll {
	for _, object := range objects {
		if object == nil {
			continue
		}
		if scroll, ok := object.(*container.Scroll); ok {
			return scroll
		}
		if nested, ok := object.(*fyne.Container); ok {
			if found := findScroll(nested.Objects); found != nil {
				return found
			}
		}
	}
	return nil
}

func countButtons(objects []fyne.CanvasObject) int {
	total := 0
	for _, object := range objects {
		if object == nil {
			continue
		}
		if _, ok := object.(*widget.Button); ok {
			total++
		}
		if nested, ok := object.(*fyne.Container); ok {
			total += countButtons(nested.Objects)
		}
	}
	return total
}

func lowestButtonBottom(objects []fyne.CanvasObject, origin fyne.Position) float32 {
	var lowest float32
	for _, object := range objects {
		if object == nil {
			continue
		}
		pos := origin.Add(object.Position())
		if _, ok := object.(*widget.Button); ok {
			if bottom := pos.Y + object.Size().Height; bottom > lowest {
				lowest = bottom
			}
		}
		if nested, ok := object.(*fyne.Container); ok {
			if bottom := lowestButtonBottom(nested.Objects, pos); bottom > lowest {
				lowest = bottom
			}
		}
	}
	return lowest
}
