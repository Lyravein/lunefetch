// Package theme provides the Lunefetch custom color theme.
package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// The palette uses quiet moonlit blues instead of saturated dashboard colors.
// Translucent interaction colors are overlays; structural surfaces stay opaque
// so text remains readable when the compositor also applies window opacity.
var (
	// A neutral charcoal foundation keeps the interface dark without making
	// every surface read as blue. Teal is reserved for primary interaction.
	colBackground           = color.NRGBA{R: 0x0E, G: 0x11, B: 0x17, A: 0xFF}
	colPanel                = color.NRGBA{R: 0x14, G: 0x18, B: 0x22, A: 0xFF}
	colElevated             = color.NRGBA{R: 0x1A, G: 0x20, B: 0x2C, A: 0xFF}
	colButton               = color.NRGBA{R: 0x22, G: 0x2A, B: 0x38, A: 0xFF}
	colDisabledButton       = color.NRGBA{R: 0x16, G: 0x1B, B: 0x24, A: 0xFF}
	colHover                = color.NRGBA{R: 0x5E, G: 0xEA, B: 0xD4, A: 0x18}
	colPressed              = color.NRGBA{R: 0x5E, G: 0xEA, B: 0xD4, A: 0x34}
	colSelection            = color.NRGBA{R: 0x1E, G: 0x3A, B: 0x4A, A: 0xFF}
	colSeparator            = color.NRGBA{R: 0x27, G: 0x31, B: 0x42, A: 0xFF}
	colInputBorder          = color.NRGBA{R: 0x2E, G: 0x3A, B: 0x4E, A: 0xFF}
	colPrimary              = color.NRGBA{R: 0x5E, G: 0xEA, B: 0xD4, A: 0xFF}
	colFocus                = color.NRGBA{R: 0x5E, G: 0xEA, B: 0xD4, A: 0x70}
	colForeground           = color.NRGBA{R: 0xF1, G: 0xF5, B: 0xF9, A: 0xFF}
	colForegroundOnAccent   = color.NRGBA{R: 0x0A, G: 0x11, B: 0x18, A: 0xFF}
	colPlaceholder          = color.NRGBA{R: 0x79, G: 0x87, B: 0x9D, A: 0xFF}
	colDisabled             = color.NRGBA{R: 0x52, G: 0x5E, B: 0x73, A: 0xFF}
	colHyperlink            = color.NRGBA{R: 0x7E, G: 0xD4, B: 0xFC, A: 0xFF}
	colScrollBar            = color.NRGBA{R: 0x8A, G: 0x9D, B: 0xB6, A: 0xA0}
	colScrollBarBackground  = color.NRGBA{R: 0x0B, G: 0x0E, B: 0x14, A: 0x90}
	colSuccess              = color.NRGBA{R: 0x4A, G: 0xDE, B: 0x80, A: 0xFF}
	colWarning              = color.NRGBA{R: 0xFB, G: 0xBF, B: 0x24, A: 0xFF}
	colError                = color.NRGBA{R: 0xF8, G: 0x71, B: 0x71, A: 0xFF}
	colShadow               = color.NRGBA{R: 0x00, G: 0x03, B: 0x0A, A: 0x78}
	colInnerWindowBorder    = color.NRGBA{R: 0x2E, G: 0x3A, B: 0x4E, A: 0xFF}
	colInactiveWindowBorder = color.NRGBA{R: 0x1B, G: 0x21, B: 0x2D, A: 0xFF}
)

// Navy is the dark Lunefetch theme. Fonts and icons stay delegated to Fyne;
// colors and a small set of spacing/radius tokens define the visual system.
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
	case theme.ColorNameDisabledButton:
		return colDisabledButton
	case theme.ColorNameDisabled:
		return colDisabled
	case theme.ColorNameForeground:
		return colForeground
	case theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnSuccess,
		theme.ColorNameForegroundOnWarning, theme.ColorNameForegroundOnError:
		return colForegroundOnAccent
	case theme.ColorNameHover:
		return colHover
	case theme.ColorNamePressed:
		return colPressed
	case theme.ColorNameScrollBar:
		return colScrollBar
	case theme.ColorNameScrollBarBackground:
		return colScrollBarBackground
	case theme.ColorNameInputBorder:
		return colInputBorder
	case theme.ColorNameSeparator:
		return colSeparator
	case theme.ColorNameInputBackground:
		return colElevated
	case theme.ColorNameMenuBackground:
		return colButton
	case theme.ColorNameOverlayBackground:
		return colPanel
	case theme.ColorNamePlaceHolder:
		return colPlaceholder
	case theme.ColorNamePrimary:
		return colPrimary
	case theme.ColorNameFocus:
		return colFocus
	case theme.ColorNameSelection:
		return colSelection
	case theme.ColorNameHyperlink:
		return colHyperlink
	case theme.ColorNameSuccess:
		return colSuccess
	case theme.ColorNameWarning:
		return colWarning
	case theme.ColorNameError:
		return colError
	case theme.ColorNameShadow:
		return colShadow
	case theme.ColorNameInnerWindowBorder:
		return colInnerWindowBorder
	case theme.ColorNameInnerWindowBorderInactive:
		return colInactiveWindowBorder
	default:
		// Unmodified semantic colors follow the default dark palette.
		return t.base.Color(name, theme.VariantDark)
	}
}

// Font delegates to the base theme.
func (t *Navy) Font(style fyne.TextStyle) fyne.Resource { return t.base.Font(style) }

// Icon delegates to the base theme.
func (t *Navy) Icon(name fyne.ThemeIconName) fyne.Resource { return t.base.Icon(name) }

// Size creates an airy utility layout with restrained, consistent rounding.
func (t *Navy) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding, theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameButtonRadius, theme.SizeNameInputRadius,
		theme.SizeNameSelectionRadius, theme.SizeNameScrollBarRadius,
		theme.SizeNameMenuRadius:
		return 10
	case theme.SizeNameCardRadius, theme.SizeNamePopupRadius,
		theme.SizeNameInnerWindowRadius:
		return 12
	case theme.SizeNameDialogRadius:
		return 14
	case theme.SizeNameSeparatorThickness:
		return 1
	default:
		return t.base.Size(name)
	}
}
