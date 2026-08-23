package theme

import (
	"image/color"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"
)

func TestNavyUsesSameDarkPaletteForEveryVariant(t *testing.T) {
	navy := NewNavy()
	names := []fyne.ThemeColorName{
		fynetheme.ColorNameBackground,
		fynetheme.ColorNameHeaderBackground,
		fynetheme.ColorNameForeground,
		fynetheme.ColorNamePrimary,
		fynetheme.ColorNameSuccess,
		fynetheme.ColorNameWarning,
		fynetheme.ColorNameError,
	}

	for _, name := range names {
		light := navy.Color(name, fynetheme.VariantLight)
		dark := navy.Color(name, fynetheme.VariantDark)
		if light != dark {
			t.Errorf("color %q changes with variant: light=%v dark=%v", name, light, dark)
		}
	}
}

func TestNavyPaletteHasReadableHierarchy(t *testing.T) {
	navy := NewNavy()
	background := asNRGBA(navy.Color(fynetheme.ColorNameBackground, fynetheme.VariantDark))
	panel := asNRGBA(navy.Color(fynetheme.ColorNameHeaderBackground, fynetheme.VariantDark))
	elevated := asNRGBA(navy.Color(fynetheme.ColorNameInputBackground, fynetheme.VariantDark))
	foreground := asNRGBA(navy.Color(fynetheme.ColorNameForeground, fynetheme.VariantDark))
	primary := asNRGBA(navy.Color(fynetheme.ColorNamePrimary, fynetheme.VariantDark))

	if background.A != 0xFF || panel.A != 0xFF || elevated.A != 0xFF {
		t.Fatal("structural surfaces must remain opaque for compositor readability")
	}
	if luminance(background) >= luminance(panel) || luminance(panel) >= luminance(elevated) {
		t.Fatalf("surface hierarchy is not increasing: background=%v panel=%v elevated=%v", background, panel, elevated)
	}
	if contrastRatio(foreground, background) < 7 {
		t.Fatalf("foreground contrast is below 7:1: foreground=%v background=%v", foreground, background)
	}
	if contrastRatio(primary, background) < 7 {
		t.Fatalf("primary contrast is below 7:1: primary=%v background=%v", primary, background)
	}
}

func TestNavyUsesMoonlightOverlayForInteractionStates(t *testing.T) {
	navy := NewNavy()
	hover := asNRGBA(navy.Color(fynetheme.ColorNameHover, fynetheme.VariantDark))
	pressed := asNRGBA(navy.Color(fynetheme.ColorNamePressed, fynetheme.VariantDark))
	focus := asNRGBA(navy.Color(fynetheme.ColorNameFocus, fynetheme.VariantDark))

	if hover.A == 0 || hover.A >= pressed.A || pressed.A >= focus.A || focus.A == 0xFF {
		t.Fatalf("interaction opacity should increase from hover to pressed to focus: %v %v %v", hover, pressed, focus)
	}
}

func TestNavySpacingAndRadiusScale(t *testing.T) {
	navy := NewNavy()
	if got := navy.Size(fynetheme.SizeNamePadding); got != 8 {
		t.Fatalf("padding = %v, want 8", got)
	}
	if got := navy.Size(fynetheme.SizeNameButtonRadius); got != 10 {
		t.Fatalf("button radius = %v, want 10", got)
	}
	if got := navy.Size(fynetheme.SizeNameCardRadius); got != 12 {
		t.Fatalf("card radius = %v, want 12", got)
	}
	if got := navy.Size(fynetheme.SizeNameDialogRadius); got != 14 {
		t.Fatalf("dialog radius = %v, want 14", got)
	}
}

func asNRGBA(c color.Color) color.NRGBA {
	return color.NRGBAModel.Convert(c).(color.NRGBA)
}

func luminance(c color.NRGBA) float64 {
	channel := func(value uint8) float64 {
		v := float64(value) / 255
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}

	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}

func contrastRatio(a, b color.NRGBA) float64 {
	lighter, darker := luminance(a), luminance(b)
	if lighter < darker {
		lighter, darker = darker, lighter
	}
	return (lighter + 0.05) / (darker + 0.05)
}
