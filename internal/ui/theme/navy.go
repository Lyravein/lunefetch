// Package theme provides the Lunefetch custom color theme.
package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Palette sampled from lunefetch.ico: midnight navy with a crisp cyan mark.
var (
	colBackground  = color.NRGBA{R: 0x07, G: 0x12, B: 0x26, A: 0xFF}
	colPanel       = color.NRGBA{R: 0x0B, G: 0x1A, B: 0x33, A: 0xFF}
	colButton      = color.NRGBA{R: 0x13, G: 0x2D, B: 0x52, A: 0xFF}
	colHover       = color.NRGBA{R: 0x1A, G: 0x3C, B: 0x67, A: 0xFF}
	colSelection   = color.NRGBA{R: 0x1D, G: 0x48, B: 0x78, A: 0xFF}
	colSeparator   = color.NRGBA{R: 0x14, G: 0x30, B: 0x50, A: 0xFF}
	colPrimary     = color.NRGBA{R: 0x67, G: 0xCB, B: 0xFC, A: 0xFF}
	colForeground  = color.NRGBA{R: 0xD6, G: 0xF4, B: 0xFE, A: 0xFF}
	colPlaceholder = color.NRGBA{R: 0x8C, G: 0xAE, B: 0xC4, A: 0xFF}
	colDisabled    = color.NRGBA{R: 0x4E, G: 0x74, B: 0x9A, A: 0xFF}
	colHyperlink   = color.NRGBA{R: 0x9E, G: 0xE0, B: 0xFC, A: 0xFF}
)

// Navy is a dark brand theme wrapping the default theme and overriding
// only the color palette; fonts, icons and sizes stay untouched.
type Navy struct {
	base fyne.Theme
}

// NewNavy returns the Lunefetch brand theme.
func NewNavy() fyne.Theme {
	return &Navy{base: theme.DefaultTheme()}
}

// Color returns the palette color for the given name.
func (t *Navy) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colBackground
	case theme.ColorNameHeaderBackground:
		return colPanel
	case theme.ColorNameButton:
		return colButton
	case theme.ColorNameDisabled:
		return colDisabled
	case theme.ColorNameForeground:
		return colForeground
	case theme.ColorNameHover, theme.ColorNamePressed, theme.ColorNameScrollBar,
		theme.ColorNameInputBorder:
		return colHover
	case theme.ColorNameSeparator:
		return colSeparator
	case theme.ColorNameInputBackground, theme.ColorNameMenuBackground,
		theme.ColorNameOverlayBackground:
		return colPanel
	case theme.ColorNamePlaceHolder:
		return colPlaceholder
	case theme.ColorNamePrimary, theme.ColorNameFocus:
		return colPrimary
	case theme.ColorNameSelection:
		return colSelection
	case theme.ColorNameHyperlink:
		return colHyperlink
	case theme.ColorNameSuccess:
		return color.NRGBA{R: 0x79, G: 0xC9, B: 0x9B, A: 0xFF}
	case theme.ColorNameWarning:
		return color.NRGBA{R: 0xE1, G: 0xB8, B: 0x68, A: 0xFF}
	case theme.ColorNameError:
		return color.NRGBA{R: 0xE3, G: 0x78, B: 0x78, A: 0xFF}
	case theme.ColorNameShadow:
		return color.NRGBA{A: 0x66}
	default:
		// Unmodified semantic colors follow the default dark palette.
		return t.base.Color(name, theme.VariantDark)
	}
}

// Font delegates to the base theme.
func (t *Navy) Font(style fyne.TextStyle) fyne.Resource { return t.base.Font(style) }

// Icon delegates to the base theme.
func (t *Navy) Icon(name fyne.ThemeIconName) fyne.Resource { return t.base.Icon(name) }

// Size keeps controls compact and uses restrained corner radii.
func (t *Navy) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 6
	case theme.SizeNameButtonRadius, theme.SizeNameInputRadius,
		theme.SizeNameSelectionRadius, theme.SizeNamePopupRadius,
		theme.SizeNameMenuRadius, theme.SizeNameDialogRadius:
		return 6
	case theme.SizeNameSeparatorThickness:
		return 1
	default:
		return t.base.Size(name)
	}
}
