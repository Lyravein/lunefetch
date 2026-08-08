package components

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/lyravein/lunefetch/internal/storage"
	"github.com/lyravein/lunefetch/internal/ui/store"
)

func TestProportionalWidthsRespectMinimums(t *testing.T) {
	minimumTotal := sumWidths(colMinWidths)
	widths := proportionalWidths(minimumTotal-1, colWidths, colMinWidths)
	if len(widths) != len(colMinWidths) {
		t.Fatalf("got %d columns, want %d", len(widths), len(colMinWidths))
	}
	for i, width := range widths {
		if width != colMinWidths[i] {
			t.Errorf("column %d = %v, want minimum %v", i, width, colMinWidths[i])
		}
	}
}

func TestProportionalWidthsFillAvailableSpace(t *testing.T) {
	available := float32(1200)
	widths := proportionalWidths(available, colWidths, colMinWidths)
	if got := sumWidths(widths); got != available {
		t.Errorf("width sum = %v, want %v", got, available)
	}
	for i, width := range widths {
		if width < colMinWidths[i] {
			t.Errorf("column %d = %v below minimum %v", i, width, colMinWidths[i])
		}
	}
	if widths[1] <= widths[2] {
		t.Errorf("name column width = %v, should remain wider than size = %v", widths[1], widths[2])
	}
}

func TestContentWidthAccountsForChrome(t *testing.T) {
	if got := ContentWidth(800); got >= 800 || got <= 0 {
		t.Errorf("ContentWidth(800) = %v, want positive width smaller than available", got)
	}
}

func TestSidebarSelectionAndMinimumWidth(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	var selected store.DownloadStatus
	sb := NewSidebar(func(status store.DownloadStatus) { selected = status }, nil, nil)
	if got := sb.Container().MinSize().Width; got < 176 {
		t.Fatalf("sidebar minimum width = %v, want at least 176", got)
	}
	sb.SelectFilter(store.StatusFailed)
	if selected != store.StatusFailed {
		t.Fatalf("selected = %q, want failed", selected)
	}
}

func TestToolbarRouteAndSelectionState(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	w := app.NewWindow("test")
	defer w.Close()

	tb := NewToolbarFull(w, nil, nil, nil, func() *storage.DownloadRecord { return nil }, nil, nil, nil)
	tb.UpdateSelection(&storage.DownloadRecord{Status: "downloading"})
	if tb.pause.Disabled() || !tb.resume.Disabled() {
		t.Fatalf("downloading state: pause disabled=%v resume disabled=%v", tb.pause.Disabled(), tb.resume.Disabled())
	}
	tb.SetDownloadsActive(false)
	if !tb.Search.Disabled() || !tb.pause.Disabled() || !tb.resume.Disabled() {
		t.Fatal("history route left download controls enabled")
	}
	tb.SetDownloadsActive(true)
	if tb.Search.Disabled() {
		t.Fatal("downloads route did not re-enable search")
	}
}
