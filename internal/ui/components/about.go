package components

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

func ShowAboutDialog(w fyne.Window) {
	dialog.ShowInformation("About Lunefetch", "Lunefetch is a desktop HTTP download manager.", w)
}
