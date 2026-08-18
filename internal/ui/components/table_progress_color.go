package components

import (
	"encoding/hex"
	
	"image/color"
	
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
)

// progressBarColorHex returns hex color string based on completion percentage
func progressBarColorHex(pct float64) string {
	switch {
	case pct >= 0.95:
		return "#2ecc71" // Green - Nearly complete (success)
	case pct >= 0.8:
		return "#f39c12" // Orange/Yellow - High progress (warning)
	case pct >= 0.5:
		return "#3498db" // Blue - Mid progress (primary)
	case pct >= 0.2:
		return "#e67e22" // Light orange - Low-mid progress
	default:
		return "#95a5a6" // Gray - Very low progress (disabled)
	}
}

// hexToColor converts hex string to fyne.Color using theme
func hexToColor(hexStr string) color.Color {
	b, err := hex.DecodeString(hexStr[1:])
	if err != nil || len(b) < 3 {
		return theme.Color(theme.ColorNameDisabled)
	}
	return color.RGBA{R: b[0], G: b[1], B: b[2], A: 255}
}

// CreateOverlayRect creates a colored rectangle to overlay on ProgressBar
func CreateOverlayRect(bar fyne.CanvasObject, pct float64) *canvas.Rectangle {
	hexColor := progressBarColorHex(pct)
	c := hexToColor(hexColor)
	
	rect := canvas.NewRectangle(c)
	rect.Resize(bar.Size())
	
	return rect
}
