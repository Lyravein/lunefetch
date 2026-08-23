// Package fatal reports a startup failure to the user before the main window
// exists.
//
// Lunefetch is built as a GUI binary (`-H=windowsgui` on Windows, launched from
// a .desktop entry on Linux), so nothing is attached to stdout or stderr. A
// startup error written only to the log is therefore invisible: the user just
// sees the application fail to appear. This shows a real window instead, and
// still logs the message for anyone running from a terminal.
package fatal

import (
	"log"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/lyravein/lunefetch/internal/ui/assets"
	uitheme "github.com/lyravein/lunefetch/internal/ui/theme"
)

// exit is indirected so tests can observe it instead of terminating.
var exit = os.Exit

const (
	windowWidth  = 520
	windowHeight = 240
)

// Show displays a modal startup error and exits once the user dismisses it.
// It must be called from the main goroutine and never returns.
func Show(title, message string) {
	log.Printf("%s: %s", title, message)

	a := app.NewWithID("io.github.lyravein.lunefetch.startup-error")
	a.Settings().SetTheme(uitheme.NewNavy())
	a.SetIcon(assets.AppIcon)

	w := a.NewWindow("Lunefetch")
	w.SetIcon(assets.AppIcon)
	w.SetContent(newErrorContent(title, message, w.Close))
	w.Resize(fyne.NewSize(windowWidth, windowHeight))
	// A fixed size also tells tiling compositors (Hyprland, sway) to float this
	// window. Without it the message is stretched across a full tile.
	w.SetFixedSize(true)
	w.CenterOnScreen()
	w.SetOnClosed(func() { exit(1) })
	w.ShowAndRun()
	exit(1)
}

// newErrorContent builds the window body. A plain window is used rather than a
// dialog over placeholder content: a dialog needs something behind it, and that
// backing content showed through and collided with the dialog's own button.
func newErrorContent(title, message string, dismiss func()) fyne.CanvasObject {
	icon := widget.NewIcon(theme.WarningIcon())

	heading := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	body := widget.NewLabel(message)
	body.Wrapping = fyne.TextWrapWord

	dismissButton := widget.NewButton("OK", dismiss)
	dismissButton.Importance = widget.HighImportance

	header := container.NewHBox(icon, heading)
	// GridWrap gives the button a fixed size so it cannot stretch across the
	// full width; the spacer pushes it to the trailing edge.
	actions := container.NewHBox(
		layout.NewSpacer(),
		container.New(layout.NewGridWrapLayout(fyne.NewSize(96, 36)), dismissButton),
	)

	return container.New(layout.NewCustomPaddedLayout(18, 16, 20, 20),
		container.NewBorder(header, actions, nil, nil, body),
	)
}
