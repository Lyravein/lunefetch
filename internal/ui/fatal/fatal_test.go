package fatal

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// The dialog must be self-contained: an earlier version drew a dialog over
// placeholder content, and that backing text showed through and collided with
// the dialog's own button.
func TestErrorContentFitsWithoutOverlap(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	content := newErrorContent(
		"Lunefetch is already running",
		"Another Lunefetch window is already open for this user.\n\n"+
			"Use the Lunefetch icon in your system tray to bring it back, "+
			"or quit it from the tray menu before starting a new one.",
		func() {},
	)

	w := test.NewWindow(content)
	defer w.Close()
	w.Resize(fyne.NewSize(windowWidth, windowHeight))
	content.Refresh()

	if min := content.MinSize(); min.Width > windowWidth || min.Height > windowHeight {
		t.Fatalf("content minimum size = %v, want it to fit the %vx%v window", min, windowWidth, windowHeight)
	}

	labels := collectLabels([]fyne.CanvasObject{content})
	if len(labels) < 2 {
		t.Fatalf("found %d labels, want a heading and a body", len(labels))
	}
	buttons := collectButtons([]fyne.CanvasObject{content})
	if len(buttons) != 1 {
		t.Fatalf("found %d buttons, want exactly one dismiss action", len(buttons))
	}
	if buttons[0].Text != "OK" {
		t.Fatalf("button text = %q, want OK", buttons[0].Text)
	}
}

func TestDismissButtonInvokesCallback(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	var dismissed bool
	content := newErrorContent("Startup error", "something went wrong", func() { dismissed = true })

	w := test.NewWindow(content)
	defer w.Close()
	w.Resize(fyne.NewSize(windowWidth, windowHeight))

	buttons := collectButtons([]fyne.CanvasObject{content})
	if len(buttons) != 1 {
		t.Fatalf("found %d buttons, want 1", len(buttons))
	}
	test.Tap(buttons[0])
	if !dismissed {
		t.Fatal("tapping OK did not invoke the dismiss callback")
	}
}

func collectLabels(objects []fyne.CanvasObject) []*widget.Label {
	var found []*widget.Label
	for _, object := range objects {
		if object == nil {
			continue
		}
		if label, ok := object.(*widget.Label); ok {
			found = append(found, label)
		}
		if nested, ok := object.(*fyne.Container); ok {
			found = append(found, collectLabels(nested.Objects)...)
		}
	}
	return found
}

func collectButtons(objects []fyne.CanvasObject) []*widget.Button {
	var found []*widget.Button
	for _, object := range objects {
		if object == nil {
			continue
		}
		if button, ok := object.(*widget.Button); ok {
			found = append(found, button)
		}
		if nested, ok := object.(*fyne.Container); ok {
			found = append(found, collectButtons(nested.Objects)...)
		}
	}
	return found
}
