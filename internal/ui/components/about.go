package components

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

const appVersionLabel = "0.1.0-beta"

func ShowAboutDialog(w fyne.Window) {
	dialog.ShowInformation("About Lunefetch", "Lunefetch "+appVersionLabel+" is a desktop HTTP download manager.", w)
}
