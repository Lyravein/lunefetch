// Package assets embeds the images the desktop UI needs at runtime.
//
// The application used to load "lunefetch.ico" from the working directory. That
// failed twice over: the installer never shipped the .ico next to the binary,
// and Go's image decoders do not understand the ICO container anyway, so both
// the window and tray icons silently fell back to a blank placeholder. Embedding
// a PNG makes the icon independent of the working directory and of any install
// layout.
package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.png
var iconPNG []byte

// AppIcon is the Lunefetch window, taskbar, and system-tray icon.
var AppIcon = fyne.NewStaticResource("lunefetch.png", iconPNG)
