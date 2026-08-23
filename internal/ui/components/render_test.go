package components

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"github.com/lyravein/lunefetch/internal/storage"
)

func TestDownloadRowRendererKeepsGeometryValidAtDesktopWidth(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	row := newDownloadRow()
	row.Resize(fyne.NewSize(960, downloadRowHeight))
	renderer := test.TempWidgetRenderer(t, row)
	renderer.Layout(row.Size())

	assertCanvasGeometry(t, renderer.Objects(), "row")
	if row.actions.MinSize().Width <= 0 || row.actions.MinSize().Height <= 0 {
		t.Fatalf("actions minimum size = %v, want usable dimensions", row.actions.MinSize())
	}
}

func TestDownloadRowRendererHandlesEmptyAndOversizedRecords(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	dt := NewDownloadTable(nil, nil, nil, nil)
	dt.records = []*storage.DownloadRecord{{ID: 1, Filename: "", TotalSize: -1, DownloadedSize: 99}}
	row := newDownloadRow()
	dt.updateRow(0, row)
	row.Resize(fyne.NewSize(320, downloadRowHeight))
	renderer := test.TempWidgetRenderer(t, row)
	renderer.Layout(row.Size())

	assertCanvasGeometry(t, renderer.Objects(), "empty record row")
	if row.progress.Value < 0 || row.progress.Value > 1 {
		t.Fatalf("progress = %v, want a bounded value", row.progress.Value)
	}
	if got := row.progress.MinSize().Height; got > 16 {
		t.Fatalf("progress minimum height = %v, want a compact bar", got)
	}
}

func TestDownloadTableMinimumSizeIsUsable(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	dt := NewDownloadTable(nil, nil, nil, nil)
	minimum := dt.Widget().MinSize()
	if minimum.Width <= 0 || minimum.Height < downloadRowHeight {
		t.Fatalf("table minimum size = %v, want positive width and row-height coverage", minimum)
	}
}

func assertCanvasGeometry(t *testing.T, objects []fyne.CanvasObject, path string) {
	t.Helper()
	for i, object := range objects {
		if object == nil {
			continue
		}
		size := object.Size()
		position := object.Position()
		if size.Width < 0 || size.Height < 0 {
			t.Fatalf("%s[%d] has negative size %v", path, i, size)
		}
		if position.X < 0 || position.Y < 0 {
			t.Fatalf("%s[%d] has negative position %v", path, i, position)
		}
		if nested, ok := object.(*fyne.Container); ok {
			assertCanvasGeometry(t, nested.Objects, path)
		}
		if nested, ok := object.(*container.Scroll); ok {
			if nested.Content != nil {
				assertCanvasGeometry(t, []fyne.CanvasObject{nested.Content}, path)
			}
		}
	}
}
