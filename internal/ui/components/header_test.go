package components

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/lyravein/lunefetch/internal/config"
)

func TestContentHeaderProvidesSearchAndAction(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	w := app.NewWindow("test")
	defer w.Close()

	header := NewContentHeader(w, config.Default(), make(chan AddURLRequest), nil)
	if header.Search == nil || header.Root == nil {
		t.Fatal("content header did not create its search or root object")
	}
	if got := header.Root.MinSize().Width; got <= 0 {
		t.Fatalf("header minimum width = %v, want positive", got)
	}
	header.Root.Resize(header.Root.MinSize())
	if got := header.Root.MinSize().Height; got > 120 {
		t.Fatalf("header minimum height = %v, want compact controls", got)
	}
}

func TestSummaryCardsUpdateStatusCounts(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	cards := NewSummaryCards()
	cards.Update(map[string]int{"downloading": 2, "completed": 8, "paused": 1, "failed": 3})
	if cards.items[0].Text != "2" || cards.items[1].Text != "8" || cards.items[2].Text != "1" || cards.items[3].Text != "3" {
		t.Fatalf("summary values = %q, %q, %q, %q", cards.items[0].Text, cards.items[1].Text, cards.items[2].Text, cards.items[3].Text)
	}
}

func TestStatusBarUsesConfiguredConcurrency(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	cfg := config.Default()
	status := NewStatusBar(cfg, nil)
	if status.concurrent.Selected != "2" {
		t.Fatalf("concurrency selection = %q, want 2", status.concurrent.Selected)
	}
}
